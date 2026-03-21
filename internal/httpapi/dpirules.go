// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"time"

	"github.com/bassosimone/2026-03-23-lab/internal/vis"
)

// dpiRuleEnvelope is the JSON envelope for a DPI rule.
type dpiRuleEnvelope struct {
	// Type is the rule type ("dns" or "tcp").
	Type string `json:"type"`

	// Rule is the rule-specific JSON payload.
	Rule json.RawMessage `json:"rule"`
}

// dpiDNSRuleJSON is the JSON representation of a [vis.DPIDNSRule].
type dpiDNSRuleJSON struct {
	// Domain is the domain to match.
	Domain string `json:"domain"`

	// Addresses contains optional IP addresses for the spoofed response.
	// If empty, the spoofed response contains NXDOMAIN.
	Addresses []string `json:"addresses,omitempty"`
}

// dpiTCPRuleJSON is the JSON representation of a [vis.DPITCPRule].
type dpiTCPRuleJSON struct {
	// Action is the action to take: "drop", "reset", or "throttle".
	Action string `json:"action"`

	// Contains is an optional string to match in the TCP payload.
	Contains string `json:"contains,omitempty"`

	// DelayMs is the extra delay in milliseconds for throttle.
	DelayMs int `json:"delay_ms,omitempty"`

	// PLR is the extra packet loss rate for throttle (0.0–1.0).
	PLR float64 `json:"plr,omitempty"`

	// ServerAddr is the optional server IP address to match.
	ServerAddr string `json:"server_addr,omitempty"`

	// ServerPort is the optional server port to match.
	ServerPort uint16 `json:"server_port,omitempty"`
}

// applyDPIRules clears the existing rules and applies the given rules.
// All rules are parsed and validated before any state is modified.
func applyDPIRules(dpi *vis.DPIEngine, envelopes []dpiRuleEnvelope) error {
	// Parse all rules first to validate before modifying state.
	var rules []vis.DPIRule
	for i, env := range envelopes {
		rule, err := parseDPIRule(&env)
		if err != nil {
			return fmt.Errorf("rule %d: %w", i, err)
		}
		rules = append(rules, rule)
	}

	// Clear existing rules and apply new ones.
	dpi.ClearRules()
	for _, rule := range rules {
		dpi.AddRule(rule)
	}
	return nil
}

// parseDPIRule parses a [dpiRuleEnvelope] into a [vis.DPIRule].
func parseDPIRule(env *dpiRuleEnvelope) (vis.DPIRule, error) {
	switch env.Type {
	case "dns":
		return parseDPIDNSRule(env.Rule)
	case "tcp":
		return parseDPITCPRule(env.Rule)
	default:
		return nil, fmt.Errorf("unknown rule type: %q", env.Type)
	}
}

// parseDPIDNSRule parses a JSON payload into a [*vis.DPIDNSRule].
func parseDPIDNSRule(raw json.RawMessage) (*vis.DPIDNSRule, error) {
	var j dpiDNSRuleJSON
	if err := json.Unmarshal(raw, &j); err != nil {
		return nil, fmt.Errorf("parsing DNS rule: %w", err)
	}
	if j.Domain == "" {
		return nil, fmt.Errorf("DNS rule: domain is required")
	}
	rule := &vis.DPIDNSRule{Domain: j.Domain}
	for _, s := range j.Addresses {
		addr, err := netip.ParseAddr(s)
		if err != nil {
			return nil, fmt.Errorf("DNS rule: invalid address %q: %w", s, err)
		}
		rule.Addresses = append(rule.Addresses, addr)
	}
	return rule, nil
}

// parseDPITCPRule parses a JSON payload into a [*vis.DPITCPRule].
func parseDPITCPRule(raw json.RawMessage) (*vis.DPITCPRule, error) {
	var j dpiTCPRuleJSON
	if err := json.Unmarshal(raw, &j); err != nil {
		return nil, fmt.Errorf("parsing TCP rule: %w", err)
	}
	rule := &vis.DPITCPRule{
		Contains:   j.Contains,
		Delay:      time.Duration(j.DelayMs) * time.Millisecond,
		PLR:        j.PLR,
		ServerPort: j.ServerPort,
	}

	// Parse action.
	switch j.Action {
	case "drop":
		rule.Action = vis.DPITCPActionDrop
	case "reset":
		rule.Action = vis.DPITCPActionReset
	case "throttle":
		rule.Action = vis.DPITCPActionThrottle
	default:
		return nil, fmt.Errorf("TCP rule: unknown action %q", j.Action)
	}

	// Parse server address if provided.
	if j.ServerAddr != "" {
		addr, err := netip.ParseAddr(j.ServerAddr)
		if err != nil {
			return nil, fmt.Errorf("TCP rule: invalid server_addr %q: %w", j.ServerAddr, err)
		}
		rule.ServerAddr = addr
	}

	return rule, nil
}

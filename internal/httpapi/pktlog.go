// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"

	"github.com/bassosimone/2026-03-23-lab/internal/dissector"
	"github.com/bassosimone/2026-03-23-lab/internal/pktlog"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

// handleGetPktlog handles GET /api/pktlog.
func (h *Handler) handleGetPktlog(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	switch format {
	case "json":
		h.servePktlogJSON(w, r)
	case "pcap":
		h.servePktlogPCAP(w, r)
	default:
		http.Error(w, fmt.Sprintf("unsupported format: %q", format), http.StatusBadRequest)
	}
}

// handleDeletePktlog handles DELETE /api/pktlog.
func (h *Handler) handleDeletePktlog(w http.ResponseWriter, r *http.Request) {
	h.pktlog.Clear()
	w.WriteHeader(http.StatusNoContent)
}

// pktlogResponse is the JSON response for GET /api/pktlog?format=json.
type pktlogResponse struct {
	// Count is the total number of entries in the unfiltered log.
	Count int `json:"count"`

	// Capacity is the maximum number of entries the log can hold.
	Capacity int `json:"capacity"`

	// Packets is the list of packet summaries after filtering.
	Packets []*dissector.PacketSummary `json:"packets"`
}

// servePktlogJSON writes the packet log as JSON, optionally filtered
// by the perspective of a given IP address.
func (h *Handler) servePktlogJSON(w http.ResponseWriter, r *http.Request) {
	// Get a snapshot of the entries and dissect.
	entries := h.pktlog.Entries()
	dissected := pktlog.DissectEntries(entries)

	// Optionally filter by perspective.
	filtered := dissected
	if addrStr := r.URL.Query().Get("addr"); addrStr != "" {
		addr, err := netip.ParseAddr(addrStr)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid addr: %q", addrStr), http.StatusBadRequest)
			return
		}
		filtered = pktlog.FilterByPerspective(dissected, addr)
	}

	// Build packet summaries.
	packets := make([]*dissector.PacketSummary, len(filtered))
	for i, entry := range filtered {
		packets[i] = dissector.Summarize(entry, i+1)
	}

	// Write the JSON response.
	resp := pktlogResponse{
		Count:    len(entries),
		Capacity: h.pktlog.Capacity(),
		Packets:  packets,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// servePktlogPCAP writes the packet log as a pcap file filtered
// by the perspective of a given IP address.
func (h *Handler) servePktlogPCAP(w http.ResponseWriter, r *http.Request) {
	// The addr parameter is required for pcap format.
	addrStr := r.URL.Query().Get("addr")
	if addrStr == "" {
		http.Error(w, "pcap format requires addr parameter", http.StatusBadRequest)
		return
	}
	addr, err := netip.ParseAddr(addrStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid addr: %q", addrStr), http.StatusBadRequest)
		return
	}

	// Get a snapshot of the entries, dissect, and filter by perspective.
	entries := h.pktlog.Entries()
	dissected := pktlog.DissectEntries(entries)
	filtered := pktlog.FilterByPerspective(dissected, addr)

	// Write the pcap to the response.
	w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", addr.String()+".pcap"))
	pw := pcapgo.NewWriter(w)
	if err := pw.WriteFileHeader(65535, layers.LinkTypeRaw); err != nil {
		return
	}
	for _, entry := range filtered {
		ci := gopacket.CaptureInfo{
			Timestamp:     entry.Time,
			CaptureLength: len(entry.RawPacket),
			Length:        len(entry.RawPacket),
		}
		if err := pw.WritePacket(ci, entry.RawPacket); err != nil {
			return
		}
	}
}

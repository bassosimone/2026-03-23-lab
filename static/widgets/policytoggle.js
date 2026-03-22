// SPDX-License-Identifier: GPL-3.0-or-later

"use strict";

// PolicyToggle implements a reusable widget for displaying and
// toggling a specific DPI censorship preset. It shows the preset
// description, renders each rule in human-readable form, and
// provides apply/remove controls.
class PolicyToggle {
  // Private instance state.
  #container;
  #preset;
  #statusEl;
  #rulesEl;
  #applyBtn;
  #removeBtn;
  #errorEl;
  #isActive = false;

  constructor(container, options = {}) {
    // Assign from constructor.
    this.#container = container;
    this.#preset = options.preset || "";

    // Build DOM inside container.
    container.classList.add("policytoggle-widget");

    // Header row: title + status badge.
    const header = document.createElement("div");
    header.className = "policytoggle-header";
    container.appendChild(header);

    const title = document.createElement("h3");
    title.className = "policytoggle-title";
    title.textContent = options.title || this.#preset;
    header.appendChild(title);

    this.#statusEl = document.createElement("span");
    this.#statusEl.className = "policytoggle-status";
    this.#statusEl.textContent = "inactive";
    header.appendChild(this.#statusEl);

    // Description paragraph (filled after fetch).
    const desc = document.createElement("p");
    desc.className = "policytoggle-description";
    container.appendChild(desc);

    // Rules display (filled after fetch).
    this.#rulesEl = document.createElement("div");
    this.#rulesEl.className = "policytoggle-rules";
    this.#rulesEl.textContent = "Loading rules...";
    container.appendChild(this.#rulesEl);

    // Controls bar.
    const controls = document.createElement("div");
    controls.className = "policytoggle-controls";
    container.appendChild(controls);

    this.#applyBtn = document.createElement("button");
    this.#applyBtn.className = "apply";
    this.#applyBtn.textContent = "Apply";
    this.#applyBtn.addEventListener("click", () => this.#apply());
    controls.appendChild(this.#applyBtn);

    this.#removeBtn = document.createElement("button");
    this.#removeBtn.className = "remove";
    this.#removeBtn.textContent = "Remove";
    this.#removeBtn.addEventListener("click", () => this.#remove());
    controls.appendChild(this.#removeBtn);

    // Error display.
    this.#errorEl = document.createElement("div");
    this.#errorEl.className = "policytoggle-error";
    container.appendChild(this.#errorEl);

    // Load preset data and active status.
    this.#load(desc);
  }

  // Fetches the preset definition and the current DPI status,
  // then renders the description, rules, and button states.
  async #load(descEl) {
    const [presetResp, dpiResp] = await Promise.all([
      fetch("/api/presets/dpi/" + encodeURIComponent(this.#preset)),
      fetch("/api/dpi"),
    ]);

    if (!presetResp.ok) {
      this.#errorEl.textContent = "Failed to load preset: " + this.#preset;
      return;
    }

    const preset = await presetResp.json();
    const dpi = await dpiResp.json();

    // Fill description.
    descEl.textContent = preset.description;

    // Render rules.
    this.#renderRules(preset.rules || []);

    // Update active state.
    this.#setActive(dpi.name === this.#preset);
  }

  // Renders each DPI rule as a human-readable line.
  #renderRules(rules) {
    this.#rulesEl.innerHTML = "";

    if (rules.length === 0) {
      this.#rulesEl.textContent = "No rules (clears all filtering).";
      return;
    }

    for (const envelope of rules) {
      const div = document.createElement("div");
      div.className = "policytoggle-rule";
      this.#rulesEl.appendChild(div);

      if (envelope.type === "dns") {
        this.#renderDNSRule(div, envelope.rule);
      } else if (envelope.type === "tcp") {
        this.#renderTCPRule(div, envelope.rule);
      }
    }
  }

  // Renders a DNS rule as a human-readable line.
  #renderDNSRule(container, rule) {
    const match = document.createElement("span");
    match.className = "policytoggle-rule-match";
    match.textContent = "\u25b8 DNS: query for \"" + rule.domain + "\"";
    container.appendChild(match);

    container.appendChild(document.createTextNode(" "));

    const arrow = document.createElement("span");
    arrow.className = "policytoggle-rule-arrow";
    arrow.textContent = "\u2192";
    container.appendChild(arrow);

    container.appendChild(document.createTextNode(" "));

    const action = document.createElement("span");
    action.className = "policytoggle-rule-action";
    if (rule.addresses && rule.addresses.length > 0) {
      action.textContent = "spoof response: " + rule.addresses.join(", ");
    } else {
      action.textContent = "NXDOMAIN";
    }
    container.appendChild(action);
  }

  // Renders a TCP rule as a human-readable line.
  #renderTCPRule(container, rule) {
    const match = document.createElement("span");
    match.className = "policytoggle-rule-match";

    // Build match description.
    let matchText = "\u25b8 TCP";
    if (rule.server_addr || rule.server_port) {
      matchText += " to";
      if (rule.server_addr) matchText += " " + rule.server_addr;
      if (rule.server_port) matchText += ":" + rule.server_port;
    }
    if (rule.contains) {
      matchText += ", payload contains \"" + rule.contains + "\"";
    }
    match.textContent = matchText;
    container.appendChild(match);

    container.appendChild(document.createTextNode(" "));

    const arrow = document.createElement("span");
    arrow.className = "policytoggle-rule-arrow";
    arrow.textContent = "\u2192";
    container.appendChild(arrow);

    container.appendChild(document.createTextNode(" "));

    const action = document.createElement("span");
    action.className = "policytoggle-rule-action";
    switch (rule.action) {
      case "drop":
        action.textContent = "drop (silent)";
        break;
      case "reset":
        action.textContent = "inject RST";
        break;
      case "throttle": {
        let text = "throttle";
        const parts = [];
        if (rule.delay_ms) parts.push("+" + rule.delay_ms + "ms delay");
        if (rule.plr) parts.push((rule.plr * 100) + "% packet loss");
        if (parts.length > 0) text += ": " + parts.join(", ");
        action.textContent = text;
        break;
      }
      default:
        action.textContent = rule.action;
    }
    container.appendChild(action);
  }

  // Updates the visual state to reflect whether this preset is active.
  #setActive(active) {
    this.#isActive = active;
    this.#container.classList.toggle("active", active);
    this.#statusEl.classList.toggle("active", active);
    this.#statusEl.textContent = active ? "active" : "inactive";
    this.#applyBtn.disabled = active;
    this.#removeBtn.disabled = !active;
  }

  // Applies this preset via the API.
  async #apply() {
    this.#errorEl.textContent = "";
    this.#applyBtn.disabled = true;

    const resp = await fetch(
      "/api/presets/dpi/" + encodeURIComponent(this.#preset) + "/apply",
      { method: "POST" },
    );

    if (!resp.ok) {
      const text = await resp.text();
      this.#errorEl.textContent = "Error: " + text;
      this.#applyBtn.disabled = false;
      return;
    }

    this.#setActive(true);
  }

  // Removes this preset by applying the "clear" preset.
  async #remove() {
    this.#errorEl.textContent = "";
    this.#removeBtn.disabled = true;

    const resp = await fetch(
      "/api/presets/dpi/clear/apply",
      { method: "POST" },
    );

    if (!resp.ok) {
      const text = await resp.text();
      this.#errorEl.textContent = "Error: " + text;
      this.#removeBtn.disabled = false;
      return;
    }

    this.#setActive(false);
  }
}

// SPDX-License-Identifier: GPL-3.0-or-later

"use strict";

// CensorshipPanel implements a reusable widget for selecting
// and applying DPI censorship presets. Each preset card shows
// its rules in human-readable form and the active card is
// color-coded: green for "clear", red for censorship.
class CensorshipPanel {
  // Private instance state.
  #container;
  #presetsDiv;
  #statusDiv;
  #activeName = "";

  constructor(container, options = {}) {
    // Assign from constructor.
    this.#container = container;

    // Build DOM inside container.
    container.classList.add("censorship-widget");

    const title = document.createElement("h2");
    title.className = "censorship-title";
    title.textContent = options.title || "Censorship Policy";
    container.appendChild(title);

    const subtitle = document.createElement("p");
    subtitle.className = "censorship-subtitle";
    subtitle.textContent = options.subtitle || "Select the active DPI filtering regime.";
    container.appendChild(subtitle);

    this.#presetsDiv = document.createElement("div");
    container.appendChild(this.#presetsDiv);

    this.#statusDiv = document.createElement("div");
    this.#statusDiv.className = "censorship-status";
    container.appendChild(this.#statusDiv);

    // Load presets on creation.
    this.#loadPresets();
  }

  // Fetches the list of available presets, each preset's full
  // content (for rules), and the currently active preset.
  async #loadPresets() {
    const [presetsResp, dpiResp] = await Promise.all([
      fetch("/api/presets/dpi"),
      fetch("/api/dpi"),
    ]);

    const presets = await presetsResp.json();
    const dpi = await dpiResp.json();
    this.#activeName = dpi.name || "";

    // Fetch full content for each preset in parallel.
    const fullPresets = await Promise.all(
      presets.map(async (p) => {
        const resp = await fetch("/api/presets/dpi/" + encodeURIComponent(p.name));
        const data = resp.ok ? await resp.json() : null;
        return { name: p.name, description: p.description, data };
      })
    );

    this.#presetsDiv.innerHTML = "";

    for (const preset of fullPresets) {
      const btn = document.createElement("button");
      btn.className = "censorship-preset";
      this.#applyActiveClass(btn, preset.name);

      const nameDiv = document.createElement("div");
      nameDiv.className = "censorship-preset-name";
      nameDiv.textContent = preset.name;
      btn.appendChild(nameDiv);

      const descDiv = document.createElement("div");
      descDiv.className = "censorship-preset-description";
      descDiv.textContent = preset.description;
      btn.appendChild(descDiv);

      // Render rules if available.
      if (preset.data) {
        const rulesDiv = document.createElement("div");
        rulesDiv.className = "censorship-preset-rules";
        this.#renderRules(rulesDiv, preset.data.rules || []);
        btn.appendChild(rulesDiv);
      }

      btn.addEventListener("click", () => this.#applyPreset(preset.name));

      this.#presetsDiv.appendChild(btn);
    }
  }

  // Renders DPI rules as human-readable lines inside the given container.
  #renderRules(container, rules) {
    if (rules.length === 0) {
      container.textContent = "No rules (clears all filtering).";
      return;
    }

    for (const envelope of rules) {
      const div = document.createElement("div");
      div.className = "censorship-preset-rule";
      container.appendChild(div);

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
    match.className = "censorship-preset-rule-match";
    match.textContent = "\u25b8 DNS: query for \"" + rule.domain + "\"";
    container.appendChild(match);

    container.appendChild(document.createTextNode(" "));

    const arrow = document.createElement("span");
    arrow.className = "censorship-preset-rule-arrow";
    arrow.textContent = "\u2192";
    container.appendChild(arrow);

    container.appendChild(document.createTextNode(" "));

    const action = document.createElement("span");
    action.className = "censorship-preset-rule-action";
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
    match.className = "censorship-preset-rule-match";

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
    arrow.className = "censorship-preset-rule-arrow";
    arrow.textContent = "\u2192";
    container.appendChild(arrow);

    container.appendChild(document.createTextNode(" "));

    const action = document.createElement("span");
    action.className = "censorship-preset-rule-action";
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

  // Applies the appropriate active class to a preset button
  // based on whether it is the active preset and whether it
  // is the "clear" preset or a censorship preset.
  #applyActiveClass(btn, name) {
    btn.classList.remove("active-clear", "active-censored");
    if (name === this.#activeName) {
      btn.classList.add(name === "clear" ? "active-clear" : "active-censored");
    }
  }

  // Applies a preset by name via POST and updates the button highlights.
  async #applyPreset(name) {
    this.#statusDiv.textContent = "Applying " + name + "...";

    const resp = await fetch(
      "/api/presets/dpi/" + encodeURIComponent(name) + "/apply",
      { method: "POST" },
    );

    if (!resp.ok) {
      const text = await resp.text();
      this.#statusDiv.textContent = "Error: " + text;
      return;
    }

    this.#activeName = name;
    this.#statusDiv.textContent = "Active: " + name;

    // Update button highlights.
    for (const btn of this.#presetsDiv.querySelectorAll(".censorship-preset")) {
      const btnName = btn.querySelector(".censorship-preset-name").textContent;
      this.#applyActiveClass(btn, btnName);
    }
  }
}

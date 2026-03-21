// SPDX-License-Identifier: GPL-3.0-or-later

"use strict";

// CensorshipPanel implements a reusable widget for selecting
// and applying DPI censorship presets.
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

  // Fetches the list of available presets and the currently
  // active preset, then builds the preset buttons.
  async #loadPresets() {
    const [presetsResp, dpiResp] = await Promise.all([
      fetch("/api/presets/dpi"),
      fetch("/api/dpi"),
    ]);

    const presets = await presetsResp.json();
    const dpi = await dpiResp.json();
    this.#activeName = dpi.name || "";

    this.#presetsDiv.innerHTML = "";

    for (const preset of presets) {
      const btn = document.createElement("button");
      btn.className = "censorship-preset" +
        (preset.name === this.#activeName ? " active" : "");

      const nameDiv = document.createElement("div");
      nameDiv.className = "censorship-preset-name";
      nameDiv.textContent = preset.name;
      btn.appendChild(nameDiv);

      const descDiv = document.createElement("div");
      descDiv.className = "censorship-preset-description";
      descDiv.textContent = preset.description;
      btn.appendChild(descDiv);

      btn.addEventListener("click", () => this.#applyPreset(preset.name));

      this.#presetsDiv.appendChild(btn);
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
      btn.classList.toggle("active", btnName === name);
    }
  }
}

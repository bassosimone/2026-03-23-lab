// SPDX-License-Identifier: GPL-3.0-or-later

"use strict";

/**
 * TopologyMap renders an SVG network topology diagram and provides
 * step-through packet replay. Each node can display multiple domain
 * names and IP addresses. Stepping through packet events highlights
 * the edge connecting the relevant node to the internet cloud.
 */
class TopologyMap {
  #container;
  #edgeByIP = new Map();
  #allEdges = [];
  #packets = [];
  #cursor = -1;
  #statusEl;
  #infoEl;

  static #SVG_NS = "http://www.w3.org/2000/svg";
  static #NODE_W = 260;
  static #NODE_R = 8;
  static #LINE_H = 18;
  static #PAD_TOP = 14;
  static #PAD_BOT = 12;
  static #GAP = 24;
  static #CLOUD_CX = 420;
  static #CLOUD_EDGE = 75;

  // Cloud path centered at origin; positioned via transform.
  static #CLOUD_PATH =
    "M -60,50 " +
    "C -85,50 -92,20 -72,3 " +
    "C -82,-25 -50,-37 -25,-20 " +
    "C -12,-49 18,-49 32,-20 " +
    "C 48,-37 80,-25 68,3 " +
    "C 90,20 85,50 60,50 " +
    "Z";

  static #CLIENT = {
    lines: [
      { text: "shelob.polito.it", bold: true },
      { text: "130.192.91.211", bold: false },
    ],
    ips: ["130.192.91.211"],
    x: 20, fill: "#e8f5e9", stroke: "#4caf50",
  };

  static #SERVERS = [
    {
      lines: [
        { text: "giove.polito.it", bold: true },
        { text: "130.192.3.21", bold: false },
      ],
      ips: ["130.192.3.21"],
      fill: "#e8f5e9", stroke: "#4caf50",
    },
    {
      lines: [
        { text: "dns.google, dns.google.com", bold: true },
        { text: "8.8.8.8, 8.8.4.4", bold: false },
      ],
      ips: ["8.8.8.8", "8.8.4.4"],
      fill: "#fff8e1", stroke: "#ffc107",
    },
    {
      lines: [
        { text: "www.example.com, example.com", bold: true },
        { text: "www.example.org, example.org", bold: true },
        { text: "104.18.26.120, 104.18.27.120", bold: false },
      ],
      ips: ["104.18.26.120", "104.18.27.120"],
      fill: "#e3f2fd", stroke: "#2196f3",
    },
  ];

  constructor(container) {
    this.#container = container;
    container.classList.add("topology-widget");
    this.#build();
  }

  #nodeHeight(def) {
    return TopologyMap.#PAD_TOP + def.lines.length * TopologyMap.#LINE_H + TopologyMap.#PAD_BOT;
  }

  // ── Build ──────────────────────────────────────────────

  #build() {
    const nw = TopologyMap.#NODE_W;
    const gap = TopologyMap.#GAP;
    const servers = TopologyMap.#SERVERS;
    const client = TopologyMap.#CLIENT;
    const cloudCx = TopologyMap.#CLOUD_CX;
    const cloudEdge = TopologyMap.#CLOUD_EDGE;
    const serverX = 680;

    // Compute server node heights and vertical positions.
    const heights = servers.map(s => this.#nodeHeight(s));
    const totalH = heights.reduce((sum, h) => sum + h, 0) + gap * (heights.length - 1);
    const margin = 40;
    const viewH = totalH + 2 * margin;

    const serverYs = [];
    let y = margin;
    for (const h of heights) {
      serverYs.push(y);
      y += h + gap;
    }

    // Cloud and client are vertically centered on the server column.
    const cloudCy = margin + totalH / 2;
    const clientH = this.#nodeHeight(client);
    const clientY = cloudCy - clientH / 2;

    // SVG dimensions.
    const viewW = serverX + nw + 20;
    const svg = this.#svgEl("svg", {
      viewBox: `0 0 ${viewW} ${viewH}`,
      class: "topology-svg",
    });

    // Edges (behind everything). Store references for highlighting.
    const clientEdge = this.#edge(svg,
      client.x + nw, clientY + clientH / 2,
      cloudCx - cloudEdge, cloudCy);
    for (const ip of client.ips) {
      this.#edgeByIP.set(ip, clientEdge);
    }

    for (let i = 0; i < servers.length; i++) {
      const sCy = serverYs[i] + heights[i] / 2;
      const line = this.#edge(svg, cloudCx + cloudEdge, cloudCy, serverX, sCy);
      for (const ip of servers[i].ips) {
        this.#edgeByIP.set(ip, line);
      }
    }

    // Cloud shape.
    const cloudG = this.#svgEl("g", {
      transform: `translate(${cloudCx},${cloudCy})`,
    });
    cloudG.appendChild(this.#svgEl("path", {
      d: TopologyMap.#CLOUD_PATH,
      fill: "#f5f5f5", stroke: "#999",
      "stroke-width": 2, "stroke-dasharray": "6,4",
    }));
    const label = this.#svgEl("text", {
      x: 0, y: 5,
      "text-anchor": "middle", "font-size": 16, fill: "#555",
    });
    label.textContent = "Internet";
    cloudG.appendChild(label);
    svg.appendChild(cloudG);

    // Client node.
    this.#node(svg, { ...client, y: clientY });

    // Server nodes.
    for (let i = 0; i < servers.length; i++) {
      this.#node(svg, { ...servers[i], x: serverX, y: serverYs[i] });
    }

    this.#container.appendChild(svg);

    // Control bar below the SVG.
    this.#buildControls();

    // Load packets (will show "no packets" if log is empty).
    this.#loadPackets();
  }

  #buildControls() {
    const bar = document.createElement("div");
    bar.className = "topology-controls";

    bar.appendChild(this.#btn("\u21bb", "Refresh packets", () => this.#loadPackets()));
    bar.appendChild(this.#sep());
    bar.appendChild(this.#btn("|\u25c0", "First", () => this.#goTo(0)));
    bar.appendChild(this.#btn("\u25c0", "Previous", () => this.#step(-1)));
    bar.appendChild(this.#btn("\u25b6", "Next", () => this.#step(1)));
    bar.appendChild(this.#btn("\u25b6|", "Last", () => this.#goTo(this.#packets.length - 1)));

    this.#statusEl = document.createElement("span");
    this.#statusEl.className = "topology-status";
    bar.appendChild(this.#statusEl);

    this.#infoEl = document.createElement("span");
    this.#infoEl.className = "topology-info";
    bar.appendChild(this.#infoEl);

    this.#container.appendChild(bar);
    this.#updateDisplay();
  }

  // ── Packet loading & stepping ──────────────────────────

  async #loadPackets() {
    try {
      const resp = await fetch("/api/pktlog?format=json");
      const data = await resp.json();
      this.#packets = data.packets || [];
    } catch (_) {
      this.#packets = [];
    }
    this.#cursor = -1;
    this.#resetEdges();
    this.#updateDisplay();
  }

  #step(delta) {
    const next = this.#cursor + delta;
    if (next < -1 || next >= this.#packets.length) return;
    if (next === -1) {
      this.#cursor = -1;
      this.#resetEdges();
      this.#updateDisplay();
      return;
    }
    this.#goTo(next);
  }

  #goTo(index) {
    if (this.#packets.length === 0) return;
    if (index < 0 || index >= this.#packets.length) return;
    this.#cursor = index;
    this.#resetEdges();

    const pkt = this.#packets[index];

    // Determine which node's edge to highlight.
    // "entered" = packet leaving the source node.
    // "delivered" = packet arriving at the destination node.
    const ip = pkt.event === "entered" ? pkt.src : pkt.dst;
    const line = this.#edgeByIP.get(ip);
    if (line) {
      let color = pkt.event === "entered" ? "#4caf50" : "#2196f3";
      if (pkt.flags && pkt.flags.includes("RST")) {
        color = "#e53935";
      }
      line.setAttribute("stroke", color);
      line.setAttribute("stroke-width", "4");
    }

    this.#updateDisplay();
  }

  #resetEdges() {
    for (const line of this.#allEdges) {
      line.setAttribute("stroke", "#bbb");
      line.setAttribute("stroke-width", "2");
    }
  }

  #updateDisplay() {
    const total = this.#packets.length;
    if (total === 0) {
      this.#statusEl.textContent = "no packets";
      this.#infoEl.textContent = "";
      return;
    }

    if (this.#cursor < 0) {
      this.#statusEl.textContent = `\u2013 / ${total}`;
      this.#infoEl.textContent = "press \u25b6 to step through";
      return;
    }

    this.#statusEl.textContent = `${this.#cursor + 1} / ${total}`;
    const pkt = this.#packets[this.#cursor];

    // Show absolute time (truncated to ms) and delta from previous event.
    const timeMs = pkt.time.slice(0, 12); // "HH:MM:SS.mmm"
    let delta = "";
    if (this.#cursor > 0) {
      const prev = this.#packets[this.#cursor - 1];
      const dt = this.#parseTimeMicros(pkt.time) - this.#parseTimeMicros(prev.time);
      delta = ` (\u0394${this.#formatDelta(dt)})`;
    }

    this.#infoEl.textContent =
      `${timeMs}${delta}  #${pkt.number} ${pkt.event}  ${pkt.protocol}  ${pkt.src} \u2192 ${pkt.dst}  ${pkt.info}`;
  }

  // Parse "HH:MM:SS.uuuuuu" into total microseconds since midnight.
  #parseTimeMicros(timeStr) {
    const [hms, us] = timeStr.split(".");
    const [h, m, s] = hms.split(":").map(Number);
    return ((h * 3600 + m * 60 + s) * 1000000) + Number(us);
  }

  // Format a duration in microseconds as a human-readable string.
  #formatDelta(us) {
    if (us < 0) return "0";
    if (us < 1000) return `${us}\u00b5s`;
    if (us < 1000000) return `${(us / 1000).toFixed(1)}ms`;
    return `${(us / 1000000).toFixed(3)}s`;
  }

  // ── SVG helpers ────────────────────────────────────────

  #node(svg, def) {
    const nw = TopologyMap.#NODE_W;
    const nr = TopologyMap.#NODE_R;
    const lineH = TopologyMap.#LINE_H;
    const padTop = TopologyMap.#PAD_TOP;
    const h = this.#nodeHeight(def);

    const g = this.#svgEl("g", { "data-ips": def.ips.join(",") });

    g.appendChild(this.#svgEl("rect", {
      x: def.x, y: def.y, width: nw, height: h,
      rx: nr, ry: nr,
      fill: def.fill, stroke: def.stroke, "stroke-width": 2,
    }));

    for (let i = 0; i < def.lines.length; i++) {
      const line = def.lines[i];
      const ty = def.y + padTop + i * lineH + 13;
      const text = this.#svgEl("text", {
        x: def.x + nw / 2, y: ty,
        "text-anchor": "middle",
        "font-size": line.bold ? 13 : 12,
        "font-weight": line.bold ? "bold" : "normal",
        fill: line.bold ? "#333" : "#666",
      });
      text.textContent = line.text;
      g.appendChild(text);
    }

    svg.appendChild(g);
  }

  #edge(svg, x1, y1, x2, y2) {
    const line = this.#svgEl("line", {
      x1, y1, x2, y2,
      stroke: "#bbb", "stroke-width": 2,
    });
    svg.appendChild(line);
    this.#allEdges.push(line);
    return line;
  }

  #svgEl(tag, attrs) {
    const el = document.createElementNS(TopologyMap.#SVG_NS, tag);
    for (const [k, v] of Object.entries(attrs || {})) {
      el.setAttribute(k, String(v));
    }
    return el;
  }

  // ── HTML helpers ───────────────────────────────────────

  #btn(label, title, handler) {
    const btn = document.createElement("button");
    btn.textContent = label;
    btn.title = title;
    btn.addEventListener("click", handler);
    return btn;
  }

  #sep() {
    const el = document.createElement("span");
    el.className = "topology-separator";
    return el;
  }
}

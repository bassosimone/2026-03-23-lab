// SPDX-License-Identifier: GPL-3.0-or-later

"use strict";

/**
 * TopologyMap renders an SVG network topology diagram showing
 * the simulated network: client, internet cloud, and servers.
 *
 * Each node can display multiple domain names and IP addresses.
 * Node heights are computed dynamically from the number of text lines.
 */
class TopologyMap {
  #container;

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
    const svg = this.#el("svg", {
      viewBox: `0 0 ${viewW} ${viewH}`,
      class: "topology-svg",
    });

    // Edges (behind everything).
    this.#line(svg, client.x + nw, clientY + clientH / 2, cloudCx - cloudEdge, cloudCy);
    for (let i = 0; i < servers.length; i++) {
      const sCy = serverYs[i] + heights[i] / 2;
      this.#line(svg, cloudCx + cloudEdge, cloudCy, serverX, sCy);
    }

    // Cloud shape.
    const cloudG = this.#el("g", {
      transform: `translate(${cloudCx},${cloudCy})`,
    });
    cloudG.appendChild(this.#el("path", {
      d: TopologyMap.#CLOUD_PATH,
      fill: "#f5f5f5", stroke: "#999",
      "stroke-width": 2, "stroke-dasharray": "6,4",
    }));
    const label = this.#el("text", {
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
  }

  #node(svg, def) {
    const nw = TopologyMap.#NODE_W;
    const nr = TopologyMap.#NODE_R;
    const lineH = TopologyMap.#LINE_H;
    const padTop = TopologyMap.#PAD_TOP;
    const h = this.#nodeHeight(def);

    const g = this.#el("g", { "data-ips": def.ips.join(",") });

    g.appendChild(this.#el("rect", {
      x: def.x, y: def.y, width: nw, height: h,
      rx: nr, ry: nr,
      fill: def.fill, stroke: def.stroke, "stroke-width": 2,
    }));

    for (let i = 0; i < def.lines.length; i++) {
      const line = def.lines[i];
      const ty = def.y + padTop + i * lineH + 13;
      const text = this.#el("text", {
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

  #line(svg, x1, y1, x2, y2) {
    svg.appendChild(this.#el("line", {
      x1, y1, x2, y2,
      stroke: "#bbb", "stroke-width": 2,
    }));
  }

  #el(tag, attrs) {
    const el = document.createElementNS(TopologyMap.#SVG_NS, tag);
    for (const [k, v] of Object.entries(attrs || {})) {
      el.setAttribute(k, String(v));
    }
    return el;
  }
}

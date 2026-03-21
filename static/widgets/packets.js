// SPDX-License-Identifier: GPL-3.0-or-later

"use strict";

// PacketViewer implements a reusable widget for viewing captured
// packets with perspective filtering, a packet table, and a
// collapsible detail pane showing dissected headers.
class PacketViewer {
  // Private instance state.
  #container;
  #tbody;
  #statusEl;
  #detailPane;
  #perspectiveButtons;
  #currentAddr = "";
  #selectedRow = null;

  // Default perspectives matching the lab scenario.
  static #DEFAULT_PERSPECTIVES = [
    { label: "All", addr: "" },
    { label: "Client", addr: "130.192.91.211" },
    { label: "Server", addr: "104.18.26.120" },
    { label: "DNS polito", addr: "130.192.3.21" },
    { label: "DNS google", addr: "8.8.8.8" },
  ];

  constructor(container, options = {}) {
    // Assign from constructor.
    this.#container = container;
    const perspectives = options.perspectives || PacketViewer.#DEFAULT_PERSPECTIVES;

    // Build DOM inside container.
    container.classList.add("packets-widget");

    // Controls bar.
    const controls = document.createElement("div");
    controls.className = "packets-controls";
    container.appendChild(controls);

    // Perspective buttons.
    this.#perspectiveButtons = [];
    for (const p of perspectives) {
      const btn = document.createElement("button");
      btn.textContent = p.label;
      btn.dataset.addr = p.addr;
      if (p.addr === "") btn.classList.add("active");
      btn.addEventListener("click", () => this.#onPerspective(btn));
      controls.appendChild(btn);
      this.#perspectiveButtons.push(btn);
    }

    // Separator.
    const sep = document.createElement("div");
    sep.className = "packets-separator";
    controls.appendChild(sep);

    // Refresh button.
    const refreshBtn = document.createElement("button");
    refreshBtn.textContent = "Refresh";
    refreshBtn.addEventListener("click", () => this.#refresh());
    controls.appendChild(refreshBtn);

    // Clear button.
    const clearBtn = document.createElement("button");
    clearBtn.textContent = "Clear";
    clearBtn.addEventListener("click", () => this.#clear());
    controls.appendChild(clearBtn);

    // Status label.
    this.#statusEl = document.createElement("span");
    this.#statusEl.className = "packets-status";
    controls.appendChild(this.#statusEl);

    // List pane with table.
    const listPane = document.createElement("div");
    listPane.className = "packets-list-pane";
    container.appendChild(listPane);

    const table = document.createElement("table");
    listPane.appendChild(table);

    const thead = document.createElement("thead");
    const headerRow = document.createElement("tr");
    for (const col of ["#", "Time", "Event", "Source", "Destination", "Protocol", "Length", "Info"]) {
      const th = document.createElement("th");
      th.textContent = col;
      headerRow.appendChild(th);
    }
    thead.appendChild(headerRow);
    table.appendChild(thead);

    const tbody = document.createElement("tbody");
    table.appendChild(tbody);
    this.#tbody = tbody;

    // Detail pane.
    this.#detailPane = document.createElement("div");
    this.#detailPane.className = "packets-detail-pane empty";
    this.#detailPane.textContent = "Click a packet to see details.";
    container.appendChild(this.#detailPane);

    // Initial load.
    this.#refresh();
  }

  // Handles a perspective button click: updates the active
  // button highlight and refreshes the packet list.
  #onPerspective(btn) {
    this.#currentAddr = btn.dataset.addr;

    for (const b of this.#perspectiveButtons) {
      b.classList.toggle("active", b === btn);
    }

    this.#refresh();
  }

  // Clears the packet log on the server, then refreshes.
  async #clear() {
    await fetch("/api/pktlog", { method: "DELETE" });
    this.#refresh();
  }

  // Fetches the packet log from the server and rebuilds
  // the table and status display.
  async #refresh() {
    let url = "/api/pktlog?format=json";
    if (this.#currentAddr) {
      url += "&addr=" + encodeURIComponent(this.#currentAddr);
    }

    const resp = await fetch(url);
    if (!resp.ok) {
      this.#statusEl.textContent = "Error: " + resp.statusText;
      return;
    }

    const data = await resp.json();
    const packets = data.packets || [];

    // Update status.
    this.#statusEl.textContent = data.count + " / " + data.capacity + " packets";
    this.#statusEl.classList.toggle("full", data.count >= data.capacity);

    // Clear detail pane.
    this.#detailPane.className = "packets-detail-pane empty";
    this.#detailPane.textContent = "Click a packet to see details.";
    this.#selectedRow = null;

    // Update table.
    this.#tbody.innerHTML = "";

    if (packets.length === 0) {
      const tr = document.createElement("tr");
      const td = document.createElement("td");
      td.colSpan = 8;
      td.style.textAlign = "center";
      td.style.color = "#999";
      td.style.padding = "20px";
      td.textContent = "No packets captured.";
      tr.appendChild(td);
      this.#tbody.appendChild(tr);
      return;
    }

    for (const pkt of packets) {
      const tr = document.createElement("tr");

      // Color by event, RST overrides.
      if (pkt.flags && pkt.flags.includes("RST")) {
        tr.className = "rst";
      } else {
        tr.className = pkt.event;
      }

      const cells = [
        pkt.number, pkt.time, pkt.event, pkt.src,
        pkt.dst, pkt.protocol, pkt.length, pkt.info,
      ];
      for (const value of cells) {
        const td = document.createElement("td");
        td.textContent = value;
        tr.appendChild(td);
      }

      tr.addEventListener("click", () => {
        if (this.#selectedRow) this.#selectedRow.classList.remove("selected");
        tr.classList.add("selected");
        this.#selectedRow = tr;
        this.#showDetail(pkt);
      });

      this.#tbody.appendChild(tr);
    }
  }

  // Populates the detail pane with collapsible header sections
  // for the selected packet.
  #showDetail(pkt) {
    this.#detailPane.className = "packets-detail-pane";
    this.#detailPane.innerHTML = "";

    // IP section.
    if (pkt.detail && pkt.detail.ip) {
      const ip = pkt.detail.ip;
      this.#detailPane.appendChild(this.#makeSection(
        "Internet Protocol Version " + ip.version +
        ", Src: " + ip.src + ", Dst: " + ip.dst,
        [
          ["Version", ip.version],
          ["Header Length", ip.ihl * 4 + " bytes (" + ip.ihl + ")"],
          ["Type of Service", "0x" + ip.tos.toString(16).padStart(2, "0")],
          ["Total Length", ip.total_length],
          ["Identification", "0x" + ip.id.toString(16).padStart(4, "0") + " (" + ip.id + ")"],
          ["Don't Fragment", ip.flag_df ? "Set" : "Not set"],
          ["More Fragments", ip.flag_mf ? "Set" : "Not set"],
          ["Fragment Offset", ip.frag_offset],
          ["Time to Live", ip.ttl],
          ["Protocol", PacketViewer.#protocolName(ip.protocol) + " (" + ip.protocol + ")"],
          ["Header Checksum", "0x" + ip.checksum.toString(16).padStart(4, "0")],
          ["Source Address", ip.src],
          ["Destination Address", ip.dst],
        ]
      ));
    }

    // TCP section.
    if (pkt.detail && pkt.detail.tcp) {
      const tcp = pkt.detail.tcp;

      const flags = [];
      if (tcp.flag_syn) flags.push("SYN");
      if (tcp.flag_ack) flags.push("ACK");
      if (tcp.flag_fin) flags.push("FIN");
      if (tcp.flag_rst) flags.push("RST");
      if (tcp.flag_psh) flags.push("PSH");
      if (tcp.flag_urg) flags.push("URG");

      this.#detailPane.appendChild(this.#makeSection(
        "Transmission Control Protocol, Src Port: " + tcp.src_port +
        ", Dst Port: " + tcp.dst_port,
        [
          ["Source Port", tcp.src_port],
          ["Destination Port", tcp.dst_port],
          ["Sequence Number", tcp.seq],
          ["Acknowledgment Number", tcp.ack],
          ["Data Offset", tcp.data_offset * 4 + " bytes (" + tcp.data_offset + ")"],
          ["Flags", flags.join(", ") || "(none)"],
          ["Window", tcp.window],
          ["Checksum", "0x" + tcp.checksum.toString(16).padStart(4, "0")],
          ["Urgent Pointer", tcp.urgent],
          ["Payload Length", tcp.payload_length],
        ]
      ));
    }

    // UDP section.
    if (pkt.detail && pkt.detail.udp) {
      const udp = pkt.detail.udp;
      this.#detailPane.appendChild(this.#makeSection(
        "User Datagram Protocol, Src Port: " + udp.src_port +
        ", Dst Port: " + udp.dst_port,
        [
          ["Source Port", udp.src_port],
          ["Destination Port", udp.dst_port],
          ["Length", udp.length],
          ["Checksum", "0x" + udp.checksum.toString(16).padStart(4, "0")],
          ["Payload Length", udp.payload_length],
        ]
      ));
    }
  }

  // Builds a collapsible detail section with a title header
  // and a list of label/value rows.
  #makeSection(title, rows) {
    const section = document.createElement("div");
    section.className = "packets-detail-section";

    const header = document.createElement("div");
    header.className = "packets-detail-header";
    header.textContent = title;

    const body = document.createElement("div");
    body.className = "packets-detail-body";

    for (const [label, value] of rows) {
      const row = document.createElement("div");
      row.className = "packets-detail-row";

      const labelSpan = document.createElement("span");
      labelSpan.className = "packets-detail-label";
      labelSpan.textContent = label + ":";
      row.appendChild(labelSpan);
      row.appendChild(document.createTextNode(" " + String(value)));

      body.appendChild(row);
    }

    header.addEventListener("click", () => {
      header.classList.toggle("open");
      body.classList.toggle("open");
    });

    section.appendChild(header);
    section.appendChild(body);
    return section;
  }

  // Maps an IP protocol number to its name.
  static #protocolName(num) {
    switch (num) {
      case 6: return "TCP";
      case 17: return "UDP";
      default: return "Unknown";
    }
  }
}

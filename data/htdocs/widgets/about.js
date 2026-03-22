// SPDX-License-Identifier: GPL-3.0-or-later

"use strict";

/**
 * AboutPanel renders a static overview page explaining
 * what the lab is and how it works.
 */
class AboutPanel {
  constructor(container) {
    container.classList.add("about-widget");
    container.innerHTML = `
      <h1 class="about-title">Censorship Lab</h1>
      <p class="about-subtitle">
        A self-contained simulation of a tiny subset of the internet,
        built to explore how internet censorship works and how to detect it.
      </p>

      <div class="about-section">
        <h2>How it works</h2>
        <p>
          Everything runs locally in your browser and a Go server.
          No real network traffic leaves your machine. When you type a
          command in the Console tab (like <code>curl</code>,
          <code>dig</code>, or <code>host</code>), it is sent to the
          Go server via HTTP. The server runs the command inside a
          simulated internet powered by gVisor, a userspace network stack.
        </p>
        <p>
          The simulation contains a client, a router with a configurable
          DPI (Deep Packet Inspection) engine, DNS servers, and HTTP
          servers. Packets flow through the router, which can be
          configured to spoof DNS, drop packets, inject RST segments,
          or throttle bandwidth &mdash; just like real-world censorship.
        </p>
      </div>

      <div class="about-section">
        <h2>Architecture</h2>
        <div class="about-diagram">
          <div class="about-box level-0">
            <span class="about-box-label">Your Browser</span>
            <div class="about-box-inner">
              <div class="about-flow">
                <span class="about-flow-item">Console tab</span>
                <span class="about-flow-arrow">&harr;</span>
                <span class="about-flow-item">HTTP API</span>
                <span class="about-flow-arrow">&rarr;</span>
                <span class="about-flow-item">Command Runner</span>
              </div>

              <div class="about-box level-1">
                <span class="about-box-label">Go Server (localhost)</span>
                <div class="about-box-inner">

                  <div class="about-box level-2">
                    <span class="about-box-label">gVisor Network Stack</span>
                    <div class="about-box-inner">
                      <div class="about-flow">
                        <span class="about-flow-item">Client<br>130.192.91.211</span>
                        <span class="about-flow-arrow">&harr;</span>
                        <span class="about-flow-item">Router + DPI</span>
                        <span class="about-flow-arrow">&harr;</span>
                        <span class="about-flow-item">DNS Servers<br>8.8.8.8, 130.192.3.21</span>
                      </div>
                      <div class="about-flow">
                        <span class="about-flow-item">&nbsp;</span>
                        <span class="about-flow-arrow">&nbsp;</span>
                        <span class="about-flow-item">Router + DPI</span>
                        <span class="about-flow-arrow">&harr;</span>
                        <span class="about-flow-item">HTTP Servers<br>104.18.26.120, 104.18.27.120</span>
                      </div>
                    </div>
                  </div>

                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="about-section">
        <h2>Tabs</h2>
        <p>
          <strong>Network</strong> &mdash; topology map with packet replay
          and sound. Step through events or fast-forward to see packets
          flow (or not flow) between nodes.
        </p>
        <p>
          <strong>Console</strong> &mdash; type commands that run inside
          the simulation: <code>curl</code> to fetch pages,
          <code>dig</code> for raw DNS queries,
          <code>host</code> for quick DNS lookups.
        </p>
        <p>
          <strong>Packets</strong> &mdash; inspect every packet that
          crossed the simulated network. Filter by perspective, view
          headers, or download as PCAP.
        </p>
        <p>
          <strong>Censorship</strong> &mdash; switch DPI presets to
          simulate different censorship techniques: DNS spoofing,
          IP blocking, RST injection, or throttling.
        </p>
        <p>
          <strong>Focus</strong> &mdash; presentation guide. A focus tree
          to track progress through the lecture.
        </p>
      </div>
    `;
  }
}

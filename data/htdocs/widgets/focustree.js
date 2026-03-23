// SPDX-License-Identifier: GPL-3.0-or-later

"use strict";

/**
 * FocusTree renders a Paradox-style focus tree with four node states:
 *
 *   unavailable → available → selected → done
 *
 * The tree layout is derived automatically from the prerequisite
 * graph defined in DEFS. To change the tree structure, just edit
 * the DEFS array — the DOM is built recursively from the data.
 */
class FocusTree {
  #container;
  #nodes = new Map();   // id → { el, state }
  #selected = null;
  #lines = [];

  static #STORAGE_KEY = "focustree-state";

  // ── Tree data ──────────────────────────────────────────
  //
  // prereqs: ids that must be done before this node unlocks.
  //   - Single prereq: this node is a child of that node.
  //   - Multiple prereqs + prereqsAll: merge node (appears
  //     after all prereqs are done, laid out below their branch).

  static #DEFS = [
    { id: "whoami", label: "Who am I?", desc: "SWE|RS, Neubot, MK, NDTv7", prereqs: [],
      title: "Simone, this one is simple: make sure they understand you are the research engineer working with M-Lab and LEAP, not the software developer based in Turin, not the university professor, and crucially not the former football player." },

    { id: "ifreedom", label: "Internet Freedom", desc: "The bigger picture", prereqs: ["whoami"],
      title: "Set the stage: why does internet freedom matter, and who works on it? One or two sentences, then dive into OONI." },

    { id: "ooni", label: "OONI", desc: "What OONI does", prereqs: ["ifreedom"],
      title: "Make sure you mention OONI Probe, Web Connectivity, OONI Explorer, and the non profit. Also, remember to mention you do not work for OONI anymore. Your knowledge is historical!" },

    { id: "governance", label: "Governance", desc: "Policy & regulation", prereqs: ["ifreedom"], optional: true,
      title: "Internet governance, the bureaucratic apparatus. France has an appeals process for blocked sites. Most countries don't." },
    { id: "platforms", label: "Other Platforms", desc: "IODA, Censored Planet", prereqs: ["ifreedom"], optional: true,
      title: "You are not alone. IODA detects shutdowns, Censored Planet does remote measurements, and there is a whole ecosystem out there." },
    { id: "resources", label: "Resources", desc: "Further reading", prereqs: ["ifreedom"], optional: true,
      title: "censorbib.nymity.ch for the bibliography, net4people/bbs for community discussion. Leave these as breadcrumbs for the curious." },
    { id: "shutdowns", label: "Shutdowns", desc: "Killing the internet", prereqs: ["ifreedom"], optional: true,
      title: "When censorship is not subtle enough: just turn the whole thing off. IODA and other tools detect these events." },

    { id: "methods", label: "Methods", desc: "OONI methodologies", prereqs: ["ooni"], optional: true,
      title: "Deep dive into how OONI actually measures censorship. Only go here if the audience wants more." },
    { id: "wc", label: "Web Connectivity", desc: "The flagship test", prereqs: ["methods"], optional: true,
      title: "How do you tell if a website is blocked? Compare measurements from a probe and a test helper." },
    { id: "dnscheck", label: "DNSCheck", desc: "DNS resolver testing", prereqs: ["methods"], optional: true,
      title: "The experiment and the paper. Testing DNS resolvers at scale." },
    { id: "testlists", label: "Test Lists", desc: "What to measure", prereqs: ["methods"], optional: true,
      title: "Citizen Lab test lists. The boring but crucial question: which URLs do you test?" },
    { id: "blockpages", label: "Block Pages", desc: "Fingerprinting blocks", prereqs: ["methods"], optional: true,
      title: "Block pages and block addresses. How to recognize when an ISP is showing you a censorship page." },

    { id: "research", label: "Research", desc: "Papers & reports", prereqs: ["ooni"], optional: true,
      title: "Academic output and field investigations from the OONI period. Pick whichever resonates with the audience." },

    { id: "papers", label: "Papers", desc: "Peer-reviewed", prereqs: ["research"], optional: true,
      title: "The academic side. Co-authored papers with actual peer review and everything." },
    { id: "http3paper", label: "HTTP/3", desc: "QUIC blocking", prereqs: ["papers"], optional: true,
      title: "Measuring HTTP/3 blocking. What happens when censors encounter a protocol they don't fully understand yet." },
    { id: "dohpaper", label: "DoT/DoH", desc: "Encrypted DNS", prereqs: ["papers"], optional: true,
      title: "Measuring blocking of encrypted DNS. The cat-and-mouse game between resolvers and censors." },
    { id: "spainpaper", label: "Spain", desc: "Ververis et al.", prereqs: ["papers"], optional: true,
      title: "Blocking in Spain. A study of how censorship works in a European democracy." },
    { id: "russiapaper", label: "Russia", desc: "Xue et al.", prereqs: ["papers"], optional: true,
      title: "Measuring the emerging practice of throttling Twitter in Russia." },

    { id: "reports", label: "Reports", desc: "Field investigations", prereqs: ["research"], optional: true,
      title: "Real-world censorship events you investigated or contributed to. The war stories." },
    { id: "snireport", label: "SNI Iran", desc: "SNI blocking", prereqs: ["reports"], optional: true,
      title: "Measuring SNI-based blocking in Iran. Ties directly to the RST injection demo." },
    { id: "jordan", label: "Jordan", desc: "Facebook live", prereqs: ["reports"], optional: true,
      title: "Measuring Facebook live streaming interference during protests. The first time you used curl to investigate something." },
    { id: "kazakhstan", label: "Kazakhstan", desc: "Election throttling", prereqs: ["reports"], optional: true,
      title: "Throttling during the elections. When the government decides democracy needs a bandwidth limit." },
    { id: "russia2022", label: "Russia 2022", desc: "Conflict censorship", prereqs: ["reports"], optional: true,
      title: "How internet censorship changed in Russia during the first year of the conflict. Public talk given for OONI in 2024." },
    { id: "cuba", label: "Cuba", desc: "Parknets", prereqs: ["reports"], optional: true,
      title: "Exploring censorship in Cuba's park networks. A different kind of internet." },

    { id: "tool", label: "This Lab", desc: "Self-contained internet sim", prereqs: ["whoami"],
      title: "A self-contained simulation of a tiny subset of the internet, built on OONI code and gVisor. Walk them through the topology and give a quick tour of the tabs before touching the console." },

    { id: "testability", label: "Testability", desc: "Making OONI testable", prereqs: ["tool"],
      title: "Why you built this instead of using tc/netem like a normal person. Now defend your choices in front of your MSc thesis advisor and her students. Good luck, soldier." },
    { id: "gvisor", label: "gVisor", desc: "vs tc/netem/iptables", prereqs: ["tool"],
      title: "A userspace network stack. Yes, the whole thing. In Go. It does not make any sense, if you are OONI, unless you happen to also have a large engine written in Go." },
    { id: "handson", label: "Hands On", desc: "Live demo begins", prereqs: ["tool"],
      title: "Two kinds of bad days: things that don't work at all, or things that work but poorly. Censorship is the same. And the tools are the same: curl, dig, host, traceroute. This lab simulates what OONI and its community actually did to understand censorship — with the same tools a sysadmin would use." },

    { id: "network", label: "Network", desc: "Show the topology", prereqs: ["handson"],
      title: "Switch to the Network tab. Walk them through the nodes: client, DNS servers, web servers, and the internet cloud in the middle where the DPI engine lives." },

    { id: "host", label: "host", desc: "Quick DNS lookup", prereqs: ["network"],
      title: "Like dig but for people who value their time." },
    { id: "dig", label: "dig", desc: "Raw DNS queries", prereqs: ["network"],
      title: "For when curl fails and you need to figure out who lied about the IP address." },
    { id: "curl", label: "curl", desc: "Visit a website", prereqs: ["network"],
      title: "If it works, great. If it doesn't, that's the whole point of this lecture." },

    { id: "packets", label: "Packets & Animation", desc: "Inspect and replay",
      prereqs: ["host", "dig", "curl"], prereqsAll: true,
      title: "Switch to Packets to show the raw data, then back to Network to replay the dance with sound. They need to see the packets flowing before censorship breaks them." },

    { id: "censorship", label: "Censorship", desc: "Enable DPI policies",
      prereqs: ["packets"],
      title: "You are about to ruin the internet for everyone in this simulation." },

    { id: "dns", label: "DNS", desc: "DNS spoofing", prereqs: ["censorship"],
      title: "The cheapest trick in the book. Also the easiest to bypass." },
    { id: "ip", label: "IP", desc: "IP blocking", prereqs: ["censorship"],
      title: "Packets go in, packets don't come out. Can't explain that." },
    { id: "rst", label: "RST", desc: "RST injection", prereqs: ["censorship"],
      title: "The firewall politely asks you to stop. By forging TCP packets." },
    { id: "throttle", label: "Throttle", desc: "Bandwidth limit", prereqs: ["censorship"],
      title: "Make the audience feel what dial-up was like. The younger ones won't believe you." },

    { id: "currentwork", label: "Current Work", desc: "M-Lab & LEAP", prereqs: ["whoami"], optional: true,
      title: "What you are doing right now. Keep it brief — one sentence each." },
    { id: "mlab", label: "M-Lab", desc: "Giga / school perf.", prereqs: ["currentwork"], optional: true,
      title: "Characterizing school network performance in collaboration with the Giga project." },
    { id: "leap", label: "LEAP", desc: "VPN measurement", prereqs: ["currentwork"], optional: true,
      title: "Defining a measurement framework for a mid-scale VPN." },
  ];

  constructor(container) {
    this.#container = container;
    container.classList.add("focus-widget");
    this.#build();
    this.#loadState();
    this.#updateStyles();

    const observer = new ResizeObserver(() => {
      if (container.offsetWidth > 0) {
        this.#adjustBars();
        observer.disconnect();
      }
    });
    observer.observe(container);
  }

  // ── Data helpers ───────────────────────────────────────

  #def(id) {
    return FocusTree.#DEFS.find(d => d.id === id);
  }

  // Direct children: nodes with a single prereq equal to parentId.
  #childrenOf(parentId) {
    return FocusTree.#DEFS.filter(d =>
      d.prereqs.length === 1 && d.prereqs[0] === parentId
    );
  }

  // Merge node: a node with prereqsAll whose prereqs are exactly childIds.
  #mergeAfter(childIds) {
    return FocusTree.#DEFS.find(d =>
      d.prereqsAll &&
      d.prereqs.length > 1 &&
      d.prereqs.length === childIds.length &&
      d.prereqs.every(p => childIds.includes(p))
    );
  }

  // ── State logic ────────────────────────────────────────

  #isAvailable(id) {
    const def = this.#def(id);
    if (!def || def.prereqs.length === 0) return true;
    if (def.prereqsAll) {
      return def.prereqs.every(p => this.#nodes.get(p)?.state === "done");
    }
    return def.prereqs.some(p => this.#nodes.get(p)?.state === "done");
  }

  #onClick(id) {
    const node = this.#nodes.get(id);
    if (!node || !this.#isAvailable(id)) return;

    if (node.state === "done") {
      node.state = "available";
      if (this.#selected === id) this.#selected = null;
      this.#resetDependents(id);
    } else if (node.state === "selected") {
      node.state = "done";
      this.#selected = null;
    } else {
      if (this.#selected) {
        const prev = this.#nodes.get(this.#selected);
        if (prev && prev.state === "selected") prev.state = "available";
      }
      node.state = "selected";
      this.#selected = id;
    }

    this.#updateStyles();
    this.#saveState();
  }

  #resetDependents(id) {
    for (const def of FocusTree.#DEFS) {
      if (!def.prereqs.includes(id)) continue;
      const node = this.#nodes.get(def.id);
      if (!node) continue;
      if (node.state === "done" || node.state === "selected") {
        node.state = "available";
        if (this.#selected === def.id) this.#selected = null;
        this.#resetDependents(def.id);
      }
    }
  }

  // ── Recursive build ────────────────────────────────────

  #build() {
    // Inner wrapper so the tree can be scrolled in both directions
    // without the centering bug clipping the left side.
    const inner = document.createElement("div");
    inner.className = "focus-widget-inner";

    const roots = FocusTree.#DEFS.filter(d => d.prereqs.length === 0);
    for (const root of roots) {
      inner.appendChild(this.#makeNode(root.id));
      this.#buildSubtree(root.id, inner);
    }

    this.#container.appendChild(inner);
  }

  #buildSubtree(parentId, container) {
    const children = this.#childrenOf(parentId);
    if (children.length === 0) return;

    // Single child: inline (no branch).
    if (children.length === 1) {
      const child = children[0];
      container.appendChild(this.#makeLine(parentId));
      container.appendChild(this.#makeNode(child.id));
      this.#buildSubtree(child.id, container);
      return;
    }

    // Multiple children: create a branch.
    container.appendChild(this.#makeLine(parentId));
    const branch = this.#makeBranch(parentId);

    for (const child of children) {
      const col = this.#makeBranchCol(child.id);
      this.#buildSubtree(child.id, col);
      branch.appendChild(col);
    }

    // Check for a merge node after this branch.
    const childIds = children.map(c => c.id);
    const merge = this.#mergeAfter(childIds);
    if (merge) {
      // Wrap branch + merge in a group so they share the same width.
      const group = document.createElement("div");
      group.className = "focus-branch-merge-group";
      group.appendChild(branch);

      // Merge bar: mirrors the fork. Each column gets a rise line,
      // connected by a horizontal bar, then a single line to the node.
      const mergeWrapper = document.createElement("div");
      mergeWrapper.className = "focus-merge";

      const mergeRow = document.createElement("div");
      mergeRow.className = "focus-merge-row";
      for (let i = 0; i < children.length; i++) {
        const rise = document.createElement("div");
        rise.className = "focus-rise";
        mergeRow.appendChild(rise);
      }
      mergeWrapper.appendChild(mergeRow);

      const mergeBar = document.createElement("div");
      mergeBar.className = "focus-mbar";
      mergeWrapper.appendChild(mergeBar);

      this.#lines.push({ el: mergeWrapper, type: "merge", prereqs: childIds });
      group.appendChild(mergeWrapper);
      container.appendChild(group);

      const mergeLine = document.createElement("div");
      mergeLine.className = "focus-line";
      this.#lines.push({ el: mergeLine, type: "merge", prereqs: childIds });
      container.appendChild(mergeLine);

      container.appendChild(this.#makeNode(merge.id));
      this.#buildSubtree(merge.id, container);
    } else {
      container.appendChild(branch);
    }
  }

  // ── DOM builders ───────────────────────────────────────

  #makeBranch(parentId) {
    const wrapper = document.createElement("div");
    wrapper.className = "focus-branch";

    const bar = document.createElement("div");
    bar.className = "focus-hbar";
    this.#lines.push({ el: bar, afterId: parentId, type: "hbar" });
    wrapper.appendChild(bar);

    const row = document.createElement("div");
    row.className = "focus-branch-row";
    wrapper.appendChild(row);

    wrapper.appendChild = (child) => row.appendChild(child);
    return wrapper;
  }

  #makeBranchCol(id) {
    const col = document.createElement("div");
    col.className = "focus-branch-col";

    const drop = document.createElement("div");
    drop.className = "focus-drop";
    col.appendChild(drop);

    col.appendChild(this.#makeNode(id));
    return col;
  }

  #makeNode(id) {
    const def = this.#def(id);
    const node = document.createElement("div");
    node.className = "focus-node";
    node.dataset.id = id;

    const label = document.createElement("div");
    label.className = "focus-node-label";
    label.textContent = def.label;
    node.appendChild(label);

    const desc = document.createElement("div");
    desc.className = "focus-node-desc";
    desc.textContent = def.desc;
    node.appendChild(desc);

    if (def.title) node.title = def.title;
    node.addEventListener("click", () => this.#onClick(id));
    this.#nodes.set(id, { el: node, state: "available" });
    return node;
  }

  #makeLine(afterId) {
    const line = document.createElement("div");
    line.className = "focus-line";
    this.#lines.push({ el: line, afterId });
    return line;
  }

  #adjustBars() {
    for (const line of this.#lines) {
      if (line.type === "hbar") {
        // Fork bar: spans center of first node to center of last node.
        const bar = line.el;
        const row = bar.nextElementSibling;
        if (!row || row.children.length < 2) continue;

        const firstNode = row.children[0].querySelector(".focus-node");
        const lastNode = row.children[row.children.length - 1].querySelector(".focus-node");
        if (!firstNode || !lastNode) continue;

        const parentRect = bar.parentElement.getBoundingClientRect();
        const firstRect = firstNode.getBoundingClientRect();
        const lastRect = lastNode.getBoundingClientRect();

        bar.style.marginLeft = `${(firstRect.left + firstRect.width / 2) - parentRect.left}px`;
        bar.style.marginRight = `${parentRect.right - (lastRect.left + lastRect.width / 2)}px`;
      }

      if (line.type === "merge") {
        // Merge bar: spans center of first rise to center of last rise.
        const mbar = line.el.querySelector(".focus-mbar");
        const rises = line.el.querySelectorAll(".focus-rise");
        if (!mbar || rises.length < 2) continue;

        const parentRect = line.el.getBoundingClientRect();
        const firstRect = rises[0].getBoundingClientRect();
        const lastRect = rises[rises.length - 1].getBoundingClientRect();

        mbar.style.marginLeft = `${(firstRect.left + firstRect.width / 2) - parentRect.left}px`;
        mbar.style.marginRight = `${parentRect.right - (lastRect.left + lastRect.width / 2)}px`;
      }
    }
  }

  // ── Styles ─────────────────────────────────────────────

  #updateStyles() {
    for (const [id, node] of this.#nodes) {
      const available = this.#isAvailable(id);
      if (!available && node.state !== "done") {
        node.state = "unavailable";
        if (this.#selected === id) this.#selected = null;
      }
      if (available && node.state === "unavailable") {
        node.state = "available";
      }

      const def = this.#def(id);
      node.el.className = "focus-node";
      node.el.classList.add(`focus-${node.state}`);
      if (def.optional) node.el.classList.add("focus-optional");
      node.el.title = (node.state !== "unavailable" && def.title) ? def.title : "";
    }

    for (const line of this.#lines) {
      if (line.type === "merge") {
        const allDone = line.prereqs.every(
          id => this.#nodes.get(id)?.state === "done"
        );
        line.el.classList.toggle("complete", allDone);
      } else if (line.type === "hbar") {
        const done = this.#nodes.get(line.afterId)?.state === "done";
        line.el.classList.toggle("complete", done);
        const row = line.el.nextElementSibling;
        if (row) {
          for (const drop of row.querySelectorAll(".focus-drop")) {
            drop.classList.toggle("complete", done);
          }
        }
      } else {
        const done = this.#nodes.get(line.afterId)?.state === "done";
        line.el.classList.toggle("complete", done);
      }
    }
  }

  // ── Persistence ────────────────────────────────────────

  #saveState() {
    const state = { selected: this.#selected };
    for (const [id, node] of this.#nodes) {
      state[id] = node.state;
    }
    try {
      localStorage.setItem(FocusTree.#STORAGE_KEY, JSON.stringify(state));
    } catch (_) {}
  }

  #loadState() {
    try {
      const raw = localStorage.getItem(FocusTree.#STORAGE_KEY);
      if (!raw) return;
      const state = JSON.parse(raw);
      this.#selected = state.selected || null;
      for (const [id, node] of this.#nodes) {
        if (state[id]) node.state = state[id];
      }
    } catch (_) {}
  }
}

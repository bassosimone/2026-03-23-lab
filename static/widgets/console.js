// SPDX-License-Identifier: GPL-3.0-or-later

"use strict";

// Console implements a reusable console widget.
class Console {
  // Private instance state.
  #term;
  #container;
  #termDiv;
  #prompt;
  #line = "";
  #cursor = 0;
  #running = false;
  #abortController = null;
  #history = [];
  #historyIndex = -1;

  constructor(container, options = {}) {
    // Assign from constructor.
    this.#prompt = options.prompt || "$ ";
    this.#container = container;

    // Build DOM inside container.
    container.classList.add("console-widget");
    this.#termDiv = document.createElement("div");
    this.#termDiv.className = "console-terminal";
    container.appendChild(this.#termDiv);

    // Create xterm instance.
    this.#term = new Terminal({
      cursorBlink: true,
      fontFamily: options.fontFamily || "'Ubuntu Mono', 'Courier New', monospace",
      fontSize: options.fontSize || 24,
      theme: options.theme || {
        background: "#ffffff",
        foreground: "#000000",
        cursor: "#000000",
        cursorAccent: "#ffffff",
        selectionBackground: "#b0c4de",
      },
    });
    this.#term.open(this.#termDiv);

    // Fit terminal to container.
    this.#fitTerminal();
    window.addEventListener("resize", () => this.#fitTerminal());

    // Wire up input handling.
    this.#term.onData((data) => this.#onData(data));

    // Show initial prompt.
    this.#term.write("Censorship Lab Console\r\n");
    this.#term.write("Type a command (e.g., host www.example.com)\r\n");
    this.#showPrompt();
  }

  // Public: recalculates the terminal size to fit the container.
  // Call this after the container becomes visible (e.g., tab switch).
  resize() {
    this.#fitTerminal();
  }

  // Resizes the terminal grid (cols x rows) to fill the container,
  // using xterm.js internal cell dimensions.
  #fitTerminal() {
    const core = this.#term._core;
    const cellWidth = core._renderService.dimensions.css.cell.width;
    const cellHeight = core._renderService.dimensions.css.cell.height;
    if (cellWidth > 0 && cellHeight > 0) {
      const cols = Math.floor(this.#termDiv.clientWidth / cellWidth);
      const rows = Math.floor(this.#termDiv.clientHeight / cellHeight);
      this.#term.resize(cols, rows);
    }
  }

  // Writes a newline and the prompt, then resets the line editing state.
  #showPrompt() {
    // \r\n = carriage return + line feed (move to start of next line)
    this.#term.write("\r\n" + this.#prompt);
    this.#line = "";
    this.#cursor = 0;
    this.#historyIndex = -1;
  }

  // Erases the current input and replaces it with newLine.
  // Used by history navigation (up/down arrows).
  #replaceLine(newLine) {
    if (this.#cursor > 0) {
      // \x1b[ND = CSI cursor left N columns
      this.#term.write("\x1b[" + this.#cursor + "D");
    }
    // \x1b[0K = CSI erase from cursor to end of line
    this.#term.write("\x1b[0K");
    this.#line = newLine;
    this.#cursor = this.#line.length;
    this.#term.write(this.#line);
  }

  // POSTs a command to /api/run and streams the SSE response
  // (base64-encoded stdout/stderr chunks) into the terminal.
  async #runCommand(command) {
    this.#running = true;
    this.#abortController = new AbortController();
    try {

      const resp = await fetch("/api/run", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ command }),
        signal: this.#abortController.signal,
      });
      if (!resp.ok) {
        const text = await resp.text();
        this.#term.write("\r\nerror: " + text);
        this.#showPrompt();
        return;
      }

      const reader = resp.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";

      while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });

        // Process complete SSE events (separated by double newline).
        const parts = buffer.split("\n\n");
        buffer = parts.pop();

        for (const part of parts) {
          if (!part.trim()) continue;
          let event = "";
          let data = "";
          for (const line of part.split("\n")) {
            if (line.startsWith("event: ")) event = line.slice(7);
            else if (line.startsWith("data: ")) data = line.slice(6);
          }

          if (event === "stdout" || event === "stderr") {
            const bytes = atob(data);
            this.#term.write(bytes.replaceAll("\n", "\r\n"));
          }
        }
      }

    } catch (err) {
      if (err.name !== "AbortError") {
        this.#term.write("\r\nerror: " + err.message);
      }
    } finally {
      this.#abortController = null;
      this.#running = false;
      this.#showPrompt();
    }
  }

  // Handles raw keyboard input from xterm.js. The data parameter
  // is a string containing either a single character or an escape
  // sequence for special keys.
  #onData(data) {
    // \x03 = Ctrl+C: abort the running command.
    if (this.#running) {
      if (data === "\x03" && this.#abortController) {
        this.#term.write("^C");
        this.#abortController.abort();
      }
      return;
    }

    // \x1b[A = CSI cursor up (up arrow: previous history entry)
    if (data === "\x1b[A") {
      if (this.#history.length > 0) {
        if (this.#historyIndex === -1) {
          this.#historyIndex = this.#history.length - 1;
        } else if (this.#historyIndex > 0) {
          this.#historyIndex--;
        }
        this.#replaceLine(this.#history[this.#historyIndex]);
      }
      return;
    }

    // \x1b[B = CSI cursor down (down arrow: next history entry)
    if (data === "\x1b[B") {
      if (this.#historyIndex !== -1) {
        if (this.#historyIndex < this.#history.length - 1) {
          this.#historyIndex++;
          this.#replaceLine(this.#history[this.#historyIndex]);
        } else {
          this.#historyIndex = -1;
          this.#replaceLine("");
        }
      }
      return;
    }

    // \x1b[C = CSI cursor right (right arrow: move cursor forward)
    if (data === "\x1b[C") {
      if (this.#cursor < this.#line.length) {
        this.#cursor++;
        this.#term.write(data);
      }
      return;
    }

    // \x1b[D = CSI cursor left (left arrow: move cursor backward)
    if (data === "\x1b[D") {
      if (this.#cursor > 0) {
        this.#cursor--;
        this.#term.write(data);
      }
      return;
    }

    for (const ch of data) {
      if (ch === "\r") { // Enter (carriage return)
        this.#term.write("\r\n");
        const trimmed = this.#line.trim();
        if (trimmed.length > 0) {
          this.#history.push(trimmed);
          if (trimmed === "clear") {
            this.#term.clear();
            this.#term.write(this.#prompt);
            this.#line = "";
            this.#cursor = 0;
            this.#historyIndex = -1;
          } else {
            this.#runCommand(trimmed);
          }
        } else {
          this.#term.write(this.#prompt);
        }
        this.#line = "";

      } else if (ch === "\x0c") { // Ctrl+L: clear screen
        this.#term.clear();
        // \x1b[H = CSI cursor home (move to row 1, col 1)
        this.#term.write("\x1b[H" + this.#prompt + this.#line);
        // Move visual cursor back to match #cursor position.
        if (this.#cursor < this.#line.length) {
          this.#term.write("\x1b[" + (this.#line.length - this.#cursor) + "D");
        }

      } else if (ch === "\x7f" || ch === "\b") { // Backspace (DEL or BS)
        if (this.#cursor > 0) {
          const tail = this.#line.slice(this.#cursor);
          this.#line = this.#line.slice(0, this.#cursor - 1) + tail;
          this.#cursor--;
          // \b = move cursor left one column, then rewrite the tail
          // with a trailing space to erase the old last char, then
          // \x1b[ND to move cursor back to the right position.
          this.#term.write("\b" + tail + " " + "\x1b[" + (tail.length + 1) + "D");
        }

      } else if (ch === "\x03") { // Ctrl+C: cancel current input
        this.#line = "";
        this.#cursor = 0;
        this.#term.write("^C");
        this.#showPrompt();

      } else if (ch >= " ") { // Printable character: insert at cursor
        const tail = this.#line.slice(this.#cursor);
        this.#line = this.#line.slice(0, this.#cursor) + ch + tail;
        this.#cursor++;
        // Write the new char + everything after it, then
        // \x1b[ND to move cursor back over the re-drawn tail.
        this.#term.write(ch + tail);
        if (tail.length > 0) {
          this.#term.write("\x1b[" + tail.length + "D");
        }
      }
    }
  }
}

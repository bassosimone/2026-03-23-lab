// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/bassosimone/2026-03-23-lab/internal/command"
	"github.com/bassosimone/2026-03-23-lab/internal/pktlog"
	"github.com/bassosimone/2026-03-23-lab/internal/vis"
	shellquote "github.com/kballard/go-shellquote"
)

// Handler is the HTTP API handler for the simulation.
//
// Use [NewHandler] to construct.
type Handler struct {
	// activeMu protects activeName and activePreset.
	activeMu sync.Mutex

	// activeName is the name of the currently active DPI preset.
	// Empty if rules were set via POST /api/dpi (ad-hoc).
	activeName string

	// activePreset is the currently active DPI preset content.
	activePreset *dpiPreset

	// dpi is the DPI engine for adding/clearing rules.
	dpi *vis.DPIEngine

	// dpiDir is the directory containing DPI preset files.
	dpiDir string

	// pktlog is the packet event logger.
	pktlog *pktlog.Logger

	// runner executes commands against the simulation.
	runner *command.Runner
}

// NewHandler creates a new [*Handler].
func NewHandler(runner *command.Runner, dpi *vis.DPIEngine, pktlog *pktlog.Logger, dpiDir string) *Handler {
	return &Handler{dpi: dpi, dpiDir: dpiDir, pktlog: pktlog, runner: runner}
}

// Register registers the API routes on the given [*http.ServeMux].
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/pktlog", h.handleGetPktlog)
	mux.HandleFunc("DELETE /api/pktlog", h.handleDeletePktlog)
	mux.HandleFunc("GET /api/dpi", h.handleGetDPI)
	mux.HandleFunc("POST /api/dpi", h.handleDPI)
	mux.HandleFunc("GET /api/presets/dpi", h.handleListDPIPresets)
	mux.HandleFunc("GET /api/presets/dpi/{name}", h.handleGetDPIPreset)
	mux.HandleFunc("POST /api/presets/dpi/{name}/apply", h.handleApplyDPIPreset)
	mux.HandleFunc("POST /api/run", h.handleRun)
}

// dpiPreset is the JSON envelope for a DPI preset file.
type dpiPreset struct {
	// Description is the human-readable description of the preset.
	Description string `json:"description"`

	// Rules is the list of DPI rule envelopes.
	Rules []dpiRuleEnvelope `json:"rules"`
}

// dpiStatus is the JSON response for GET /api/dpi.
type dpiStatus struct {
	// Name is the active preset name, empty for ad-hoc rules.
	Name string `json:"name"`

	// Preset is the active preset content, nil if no rules are set.
	Preset *dpiPreset `json:"preset"`
}

// handleGetDPI handles GET /api/dpi by returning the currently
// active DPI preset name and content.
func (h *Handler) handleGetDPI(w http.ResponseWriter, r *http.Request) {
	h.activeMu.Lock()
	status := dpiStatus{Name: h.activeName, Preset: h.activePreset}
	h.activeMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleDPI handles POST /api/dpi by replacing the DPI ruleset
// with ad-hoc rules. The active preset name is cleared.
func (h *Handler) handleDPI(w http.ResponseWriter, r *http.Request) {
	var preset dpiPreset
	if err := json.NewDecoder(r.Body).Decode(&preset); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := applyDPIRules(h.dpi, preset.Rules); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.activeMu.Lock()
	h.activeName = ""
	h.activePreset = &preset
	h.activeMu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

// runRequest is the JSON request body for the /api/run endpoint.
type runRequest struct {
	// Command is the shell command line to execute.
	Command string `json:"command"`
}

// handleRun handles POST /api/run by running a command inside the
// simulation and streaming the output as SSE events.
func (h *Handler) handleRun(w http.ResponseWriter, r *http.Request) {
	// Parse the request body.
	var req runRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	argv, err := shellquote.Split(req.Command)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Prepare SSE response.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// Use an io.Pipe to decouple the command's writers from the
	// response writer. Background goroutines (e.g., stale transport
	// dial goroutines) that outlive this handler will get
	// io.ErrClosedPipe instead of panicking on a dead ResponseWriter.
	pr, pw := io.Pipe()

	// Reader goroutine: copy from pipe to response writer, flushing after
	// each read to ensure SSE events are delivered immediately.
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 32*1024)
		for {
			n, err := pr.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Create SSE writers for stdout and stderr. They share
	// a mutex to serialize concurrent writes to the pipe.
	mu := &sync.Mutex{}
	stdout := &sseWriter{w: pw, mu: mu, event: "stdout"}
	stderr := &sseWriter{w: pw, mu: mu, event: "stderr"}

	// Run the command.
	exitcode := 0
	params := command.Params{
		Argv:   argv,
		Stdout: stdout,
		Stderr: stderr,
	}
	if err := h.runner.Run(r.Context(), &params); err != nil {
		exitcode = 1
	}

	// Send the exitcode through the pipe
	fmt.Fprintf(pw, "event: exitcode\ndata: %d\n\n", exitcode)

	// Close the pipe writer. This causes the reader goroutine to see
	// EOF and stop. Any stale background goroutine that still holds
	// a reference to stdout/stderr will get io.ErrClosedPipe.
	pw.Close()
	<-done
}

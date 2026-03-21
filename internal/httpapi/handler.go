// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/bassosimone/2026-03-23-lab/internal/server"
)

// Handler is the HTTP API handler for a [*server.Simulation].
//
// Use [NewHandler] to construct.
type Handler struct {
	// sim is the simulation to execute commands against.
	sim *server.Simulation
}

// NewHandler creates a new [*Handler] for the given simulation.
func NewHandler(sim *server.Simulation) *Handler {
	return &Handler{sim: sim}
}

// Register registers the API routes on the given [*http.ServeMux].
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/run", h.handleRun)
}

// runRequest is the JSON request body for the /api/run endpoint.
type runRequest struct {
	// Argv is the command line to execute.
	Argv []string `json:"argv"`
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

	// Prepare SSE response.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// Create SSE writers for stdout and stderr. They share
	// a mutex to serialize concurrent writes to the response.
	mu := &sync.Mutex{}
	stdout := &sseWriter{w: w, mu: mu, event: "stdout"}
	stderr := &sseWriter{w: w, mu: mu, event: "stderr"}

	// Run the command.
	err := h.sim.RunCommand(r.Context(), &server.RunCommandParams{
		Argv:   req.Argv,
		Stdout: stdout,
		Stderr: stderr,
	})

	// Send the exit code.
	exitcode := 0
	if err != nil {
		exitcode = 1
	}
	fmt.Fprintf(w, "event: exitcode\ndata: %d\n\n", exitcode)

	// Flush if the writer supports it.
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

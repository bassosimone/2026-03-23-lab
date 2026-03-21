// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
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
	err := h.sim.RunCommand(r.Context(), &server.RunCommandParams{
		Argv:   req.Argv,
		Stdout: stdout,
		Stderr: stderr,
	})

	// Send the exit code through the pipe.
	exitcode := 0
	if err != nil {
		exitcode = 1
	}
	fmt.Fprintf(pw, "event: exitcode\ndata: %d\n\n", exitcode)

	// Close the pipe writer. This causes the reader goroutine to see
	// EOF and stop. Any stale background goroutine that still holds
	// a reference to stdout/stderr will get io.ErrClosedPipe.
	pw.Close()
	<-done
}

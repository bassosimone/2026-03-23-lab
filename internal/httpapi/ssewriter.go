// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
)

// sseWriter adapts an [http.ResponseWriter] into an [io.Writer] that emits SSE
// events. Each Write produces one SSE event with the configured event type.
//
// Multiple [sseWriter] instances sharing the same [*sync.Mutex] are safe
// for concurrent use; the mutex serializes writes to the response.
type sseWriter struct {
	// event is the SSE event type (e.g., "stdout", "stderr").
	event string

	// mu serializes writes to the response writer.
	mu *sync.Mutex

	// w is the underlying HTTP response writer.
	w http.ResponseWriter
}

// Write implements [io.Writer] by emitting an SSE event and
// flushing to ensure the event is delivered immediately.
// The payload is base64-encoded to preserve newlines and binary data.
func (sw *sseWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	encoded := base64.StdEncoding.EncodeToString(p)
	_, err := fmt.Fprintf(sw.w, "event: %s\ndata: %s\n\n", sw.event, encoded)
	if err != nil {
		return 0, err
	}
	if f, ok := sw.w.(http.Flusher); ok {
		f.Flush()
	}
	return len(p), nil
}

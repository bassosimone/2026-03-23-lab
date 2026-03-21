// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"encoding/base64"
	"fmt"
	"io"
	"sync"
)

// sseWriter adapts an [io.Writer] into an [io.Writer] that emits SSE
// events. Each Write produces one SSE event with the configured event type.
//
// Multiple [sseWriter] instances sharing the same [*sync.Mutex] are safe
// for concurrent use; the mutex serializes writes to the underlying writer.
type sseWriter struct {
	// event is the SSE event type (e.g., "stdout", "stderr").
	event string

	// mu serializes writes to the underlying writer.
	mu *sync.Mutex

	// w is the underlying writer.
	w io.Writer
}

// Write implements [io.Writer] by emitting an SSE event.
// The payload is base64-encoded to preserve newlines and binary data.
func (sw *sseWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	encoded := base64.StdEncoding.EncodeToString(p)
	if _, err := fmt.Fprintf(sw.w, "event: %s\ndata: %s\n\n", sw.event, encoded); err != nil {
		return 0, err
	}
	return len(p), nil
}

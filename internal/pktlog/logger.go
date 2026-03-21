// SPDX-License-Identifier: GPL-3.0-or-later

// Package pktlog provides a bounded, in-memory packet event log
// for recording packets as they enter and leave the [vis.Router].
package pktlog

import (
	"sync"
	"time"

	"github.com/bassosimone/2026-03-23-lab/internal/vis"
)

// Entry is a single recorded packet event.
type Entry struct {
	// Time is when the event was recorded.
	Time time.Time

	// Event is whether the packet entered or was delivered.
	Event vis.PacketEvent

	// Packet is the raw IP packet bytes. This slice MUST NOT
	// be modified. For enter/deliver pairs of the same packet,
	// this points to the same underlying array.
	Packet []byte
}

// Logger records packet events from a [vis.Router] hook into
// a bounded in-memory buffer. Use [NewLogger] to construct.
type Logger struct {
	// mu protects entries.
	mu sync.Mutex

	// entries is the packet event log.
	entries []Entry

	// maxEntries is the maximum number of entries to store.
	// Once reached, new events are silently dropped.
	maxEntries int
}

// NewLogger creates a [*Logger] that stores up to maxEntries
// packet events. A typical value is 10000 (~15MB worst case
// at 1500 bytes per packet).
func NewLogger(maxEntries int) *Logger {
	return &Logger{
		maxEntries: maxEntries,
	}
}

// Hook records a packet event. Its signature matches
// [vis.RouterHook], so pass logger.Hook to [vis.RouterOptionHook].
func (l *Logger) Hook(event vis.PacketEvent, packet []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.entries) >= l.maxEntries {
		return
	}
	l.entries = append(l.entries, Entry{
		Time:   time.Now(),
		Event:  event,
		Packet: packet,
	})
}

// Entries returns a copy of the current log entries.
func (l *Logger) Entries() []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}

// Clear removes all recorded entries.
func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = nil
}

// SPDX-License-Identifier: GPL-3.0-or-later

package pktlog

import (
	"time"

	"github.com/bassosimone/2026-03-23-lab/internal/vis"
)

// DissectedEntry is an [Entry] with its packet already dissected.
type DissectedEntry struct {
	// Time is when the event was recorded.
	Time time.Time

	// Event is whether the packet entered or was delivered.
	Event vis.PacketEvent

	// Packet is the dissected packet.
	Packet *vis.DissectedPacket

	// RawPacket is the original raw IP packet bytes.
	RawPacket []byte
}

// DissectEntries dissects each entry's packet and returns the
// successfully dissected entries. Entries that fail to dissect
// are silently skipped.
func DissectEntries(entries []Entry) []DissectedEntry {
	var out []DissectedEntry
	for _, entry := range entries {
		dp, err := vis.DissectPacket(entry.Packet)
		if err != nil {
			continue
		}
		out = append(out, DissectedEntry{
			Time:      entry.Time,
			Event:     entry.Event,
			Packet:    dp,
			RawPacket: entry.Packet,
		})
	}
	return out
}

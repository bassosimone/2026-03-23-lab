// SPDX-License-Identifier: GPL-3.0-or-later

package pktlog

import (
	"net/netip"

	"github.com/bassosimone/2026-03-23-lab/internal/vis"
)

// FilterByPerspective returns entries visible from the given IP
// address's perspective. A packet is visible if:
//   - It entered the router and was sent by addr (outgoing)
//   - It was delivered and is destined for addr (incoming)
//
// This mimics what a tcpdump on addr's network interface would see.
func FilterByPerspective(entries []DissectedEntry, addr netip.Addr) []DissectedEntry {
	var filtered []DissectedEntry
	for _, entry := range entries {
		switch entry.Event {
		case vis.PacketEntered:
			if entry.Packet.SourceAddr() == addr {
				filtered = append(filtered, entry)
			}
		case vis.PacketDelivered:
			if entry.Packet.DestinationAddr() == addr {
				filtered = append(filtered, entry)
			}
		}
	}
	return filtered
}

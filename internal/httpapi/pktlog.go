// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"fmt"
	"net/http"
	"net/netip"

	"github.com/bassosimone/2026-03-23-lab/internal/pktlog"
	"github.com/bassosimone/2026-03-23-lab/internal/vis"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

// handleGetPktlog handles GET /api/pktlog.
func (h *Handler) handleGetPktlog(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	switch format {
	case "pcap":
		h.servePktlogPCAP(w, r)
	default:
		http.Error(w, fmt.Sprintf("unsupported format: %q", format), http.StatusBadRequest)
	}
}

// handleDeletePktlog handles DELETE /api/pktlog.
func (h *Handler) handleDeletePktlog(w http.ResponseWriter, r *http.Request) {
	h.pktlog.Clear()
	w.WriteHeader(http.StatusNoContent)
}

// servePktlogPCAP writes the packet log as a pcap file filtered
// by the perspective of a given IP address.
func (h *Handler) servePktlogPCAP(w http.ResponseWriter, r *http.Request) {
	// The addr parameter is required for pcap format.
	addrStr := r.URL.Query().Get("addr")
	if addrStr == "" {
		http.Error(w, "pcap format requires addr parameter", http.StatusBadRequest)
		return
	}
	addr, err := netip.ParseAddr(addrStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid addr: %q", addrStr), http.StatusBadRequest)
		return
	}

	// Get a snapshot of the entries and filter by perspective.
	entries := h.pktlog.Entries()
	filtered := filterByPerspective(entries, addr)

	// Write the pcap to the response.
	w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", addr.String()+".pcap"))
	pw := pcapgo.NewWriter(w)
	if err := pw.WriteFileHeader(65535, layers.LinkTypeRaw); err != nil {
		return
	}
	for _, entry := range filtered {
		ci := gopacket.CaptureInfo{
			Timestamp:     entry.Time,
			CaptureLength: len(entry.Packet),
			Length:        len(entry.Packet),
		}
		if err := pw.WritePacket(ci, entry.Packet); err != nil {
			return
		}
	}
}

// filterByPerspective returns entries visible from the given IP
// address's perspective. A packet is visible if:
//   - It entered the router and was sent by addr (outgoing)
//   - It was delivered and is destined for addr (incoming)
//
// This mimics what a tcpdump on addr's network interface would see.
func filterByPerspective(entries []pktlog.Entry, addr netip.Addr) []pktlog.Entry {
	var filtered []pktlog.Entry
	for _, entry := range entries {
		dp, err := vis.DissectPacket(entry.Packet)
		if err != nil {
			continue
		}
		switch entry.Event {
		case vis.PacketEntered:
			// Outgoing: we see it leave our interface.
			if dp.SourceAddr() == addr {
				filtered = append(filtered, entry)
			}
		case vis.PacketDelivered:
			// Incoming: we see it arrive at our interface.
			if dp.DestinationAddr() == addr {
				filtered = append(filtered, entry)
			}
		}
	}
	return filtered
}

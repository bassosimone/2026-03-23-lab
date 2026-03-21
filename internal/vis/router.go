//
// SPDX-License-Identifier: BSD-3-Clause
//
// Adapted from: https://github.com/ooni/netem/blob/3882eda4fb66244b28766ef8b02003515f476b37/router.go
// Adapted from: https://github.com/ooni/netem/blob/3882eda4fb66244b28766ef8b02003515f476b37/linkfwdfull.go
//

package vis

import (
	"container/heap"
	"context"
	"math/rand/v2"
	"time"

	"github.com/bassosimone/uis"
)

// PacketEvent describes when a [RouterHook] is called.
type PacketEvent int

const (
	// PacketEntered means the packet just entered the router.
	PacketEntered PacketEvent = iota

	// PacketDelivered means the packet is being delivered.
	PacketDelivered
)

// RouterHook is called by the [*Router] for every packet at two
// points in its lifecycle. The packet bytes MUST NOT be modified.
type RouterHook func(PacketEvent, []byte)

// Router is a packet router for [*uis.Internet] with optional
// DPI and propagation delay. Every packet gets a uniform jitter
// distributed in the 1–2000µs interval.
//
// It structurally satisfies the [iss.Router] interface without
// importing [iss]. Use [NewRouter] to construct.
type Router struct {
	// delay is the base propagation delay applied to every packet.
	delay time.Duration

	// engine is the optional DPI engine for inspecting packets.
	engine *DPIEngine

	// hook is the optional packet observer called on enter and deliver.
	hook RouterHook
}

// RouterOption configures a [*Router].
type RouterOption func(r *Router)

// RouterOptionDelay sets the base propagation delay.
func RouterOptionDelay(d time.Duration) RouterOption {
	return func(r *Router) {
		r.delay = d
	}
}

// RouterOptionDPI sets the [*DPIEngine] for packet inspection.
func RouterOptionDPI(engine *DPIEngine) RouterOption {
	return func(r *Router) {
		r.engine = engine
	}
}

// RouterOptionHook sets the packet observer hook.
func RouterOptionHook(hook RouterHook) RouterOption {
	return func(r *Router) {
		r.hook = hook
	}
}

// NewRouter creates a new [*Router] with the given options.
func NewRouter(options ...RouterOption) *Router {
	r := &Router{}
	for _, opt := range options {
		opt(r)
	}
	return r
}

// Route reads packets from ix, applies DPI and propagation delay,
// and delivers them. It runs until the context is canceled.
//
// This method satisfies the [iss.Router] interface.
func (r *Router) Route(ctx context.Context, ix *uis.Internet) {
	// Initialize the heap keeping packets sorted by min delivery time.
	var pending deliveryHeap
	heap.Init(&pending)

	// The timer fires when the earliest pending frame is due.
	// We create it stopped and activate it when needed.
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()

	for {
		select {
		// 1. We need to stop immediately.
		case <-ctx.Done():
			return

		// 2. We received a new packet to deliver.
		case frame := <-ix.InFlight():
			// Notify the hook that a packet entered the router.
			if r.hook != nil {
				r.hook(PacketEntered, frame.Packet)
			}

			// Every packet gets base delay plus random jitter (1–2000µs) so that
			// any DPI rule always takes deterministic precedence.
			totalDelay := r.delay + time.Duration(1+rand.IntN(2000))*time.Microsecond

			// Run DPI inspection if an engine is configured.
			if r.engine != nil {
				policy, matched := r.engine.Inspect(frame.Packet, ix)
				if matched {
					// Apply probabilistic packet loss (PLR >= 1.0 means drop).
					if policy.PLR > 0 && rand.Float64() < policy.PLR {
						continue
					}

					// Possibly inflate the total delay.
					totalDelay += policy.Delay
				}
			}

			// Enqueue for delayed delivery.
			deadline := time.Now().Add(totalDelay)
			heap.Push(&pending, deliveryEntry{
				deadline: deadline,
				frame:    frame,
			})

			// If this frame became the earliest, reset the timer.
			if pending[0].deadline.Equal(deadline) {
				timer.Reset(time.Until(deadline))
			}

		// 3. Deadline expired for packet.
		case <-timer.C:
			// Deliver all frames whose deadline has passed.
			for now := time.Now(); pending.Len() > 0; {
				if pending[0].deadline.After(now) {
					break
				}
				entry := heap.Pop(&pending).(deliveryEntry)
				if r.hook != nil {
					r.hook(PacketDelivered, entry.frame.Packet)
				}
				ix.Deliver(entry.frame)
			}

			// Schedule the next delivery if there are pending frames
			// otherwise make the timer fire in the future.
			delta := time.Hour
			if pending.Len() > 0 {
				delta = max(time.Until(pending[0].deadline), 0)
			}
			timer.Reset(delta)
		}
	}
}

// deliveryEntry pairs a frame with its delivery deadline.
type deliveryEntry struct {
	// deadline is when this frame should be delivered.
	deadline time.Time

	// frame is the packet to deliver.
	frame uis.VNICFrame
}

// deliveryHeap is a min-heap of [deliveryEntry] ordered by deadline.
type deliveryHeap []deliveryEntry

var _ heap.Interface = &deliveryHeap{}

// Len implements [heap.Interface].
func (h deliveryHeap) Len() int {
	return len(h)
}

// Less implements [heap.Interface].
func (h deliveryHeap) Less(i, j int) bool {
	return h[i].deadline.Before(h[j].deadline)
}

// Swap implements [heap.Interface].
func (h deliveryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push implements [heap.Interface].
func (h *deliveryHeap) Push(x any) {
	*h = append(*h, x.(deliveryEntry))
}

// Pop implements [heap.Interface].
func (h *deliveryHeap) Pop() any {
	old := *h
	n := len(old)
	entry := old[n-1]
	*h = old[:n-1]
	return entry
}

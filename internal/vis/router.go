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
	// Initialize the delivery queue.
	dq := newRouterDeliveryQueue(r, ix)
	defer dq.Stop()

	for {
		select {
		// 1. We need to stop immediately.
		case <-ctx.Done():
			return

		// 2. We received a new packet to deliver.
		case frame := <-ix.InFlight():
			dq.OnFrameEnter(frame)

		// 3. Deadline expired for packet.
		case <-dq.PacketTimerCh():
			dq.OnPacketTimer()
		}
	}
}

// routerDeliveryQueue is the delivery queue used by the [*Router].
//
// Use [newRouterDeliveryQueue] to construct.
//
// Remember to defer a call to [*routerDeliveryQueue.Stop].
type routerDeliveryQueue struct {
	heap   routerDeliveryHeap
	ix     *uis.Internet
	router *Router
	timer  *time.Timer
}

var _ DPIPacketInjector = &routerDeliveryQueue{}

// newRouterDeliveryQueue constructs a new [*routerDeliveryQueue] instance.
func newRouterDeliveryQueue(r *Router, ix *uis.Internet) *routerDeliveryQueue {
	dq := &routerDeliveryQueue{
		ix:     ix,
		router: r,
		timer:  time.NewTimer(time.Hour),
	}
	heap.Init(&dq.heap)
	return dq
}

// Stop stops the [*routerDeliveryQueue] timer.
func (dq *routerDeliveryQueue) Stop() {
	dq.timer.Stop()
}

// OnFrameEnter is invoked when a packet enters the [*routerDeliveryQueue].
func (dq *routerDeliveryQueue) OnFrameEnter(frame uis.VNICFrame) {
	// Notify the hook that a packet entered the router.
	if dq.router.hook != nil {
		dq.router.hook(PacketEntered, frame.Packet)
	}

	// Every packet gets base delay plus random jitter (1–2000µs) so that
	// any DPI rule always takes deterministic precedence.
	totalDelay := dq.router.delay + time.Duration(1+rand.IntN(2000))*time.Microsecond

	// Run DPI inspection if an engine is configured.
	if dq.router.engine != nil {
		policy, matched := dq.router.engine.Inspect(frame.Packet, dq)
		if matched {
			// Apply probabilistic packet loss (PLR >= 1.0 means drop).
			if policy.PLR > 0 && rand.Float64() < policy.PLR {
				return
			}

			// Possibly inflate the total delay.
			totalDelay += policy.Delay
		}
	}

	// Enqueue for delayed delivery.
	deadline := time.Now().Add(totalDelay)
	heap.Push(&dq.heap, routerDeliveryEntry{
		deadline: deadline,
		frame:    frame,
	})

	// If this frame became the earliest, reset the timer.
	if dq.heap[0].deadline.Equal(deadline) {
		dq.timer.Reset(time.Until(deadline))
	}
}

// PacketTimerCh returns the channel posted when it's time to send a packet.
func (dq *routerDeliveryQueue) PacketTimerCh() <-chan time.Time {
	return dq.timer.C
}

// OnPacketTimer is invoked when it's time to send 1+ packets.
func (dq *routerDeliveryQueue) OnPacketTimer() {
	// Deliver all frames whose deadline has passed.
	for now := time.Now(); dq.heap.Len() > 0; {
		if dq.heap[0].deadline.After(now) {
			break
		}
		entry := heap.Pop(&dq.heap).(routerDeliveryEntry)
		dq.DeliverFrame(entry.frame)
	}

	// Schedule the next delivery if there are pending frames
	// otherwise make the timer fire in the future.
	delta := time.Hour
	if dq.heap.Len() > 0 {
		delta = max(time.Until(dq.heap[0].deadline), 0)
	}
	dq.timer.Reset(delta)
}

// DeliverFrame delivers the given frame.
func (dq *routerDeliveryQueue) DeliverFrame(frame uis.VNICFrame) {
	if dq.router.hook != nil {
		dq.router.hook(PacketDelivered, frame.Packet)
	}
	dq.ix.Deliver(frame)
}

// routerDeliveryEntry pairs a frame with its delivery deadline.
type routerDeliveryEntry struct {
	// deadline is when this frame should be delivered.
	deadline time.Time

	// frame is the packet to deliver.
	frame uis.VNICFrame
}

// routerDeliveryHeap is a min-heap of [routerDeliveryEntry] ordered by deadline.
type routerDeliveryHeap []routerDeliveryEntry

var _ heap.Interface = &routerDeliveryHeap{}

// Len implements [heap.Interface].
func (h routerDeliveryHeap) Len() int {
	return len(h)
}

// Less implements [heap.Interface].
func (h routerDeliveryHeap) Less(i, j int) bool {
	return h[i].deadline.Before(h[j].deadline)
}

// Swap implements [heap.Interface].
func (h routerDeliveryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push implements [heap.Interface].
func (h *routerDeliveryHeap) Push(x any) {
	*h = append(*h, x.(routerDeliveryEntry))
}

// Pop implements [heap.Interface].
func (h *routerDeliveryHeap) Pop() any {
	old := *h
	n := len(old)
	entry := old[n-1]
	*h = old[:n-1]
	return entry
}

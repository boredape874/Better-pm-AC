package ack

import (
	"sync"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// System implements meta.AckSystem. It hands out NetworkStackLatency markers
// on Dispatch and resolves their callbacks when the client echoes the
// matching timestamp back through OnResponse.
type System struct {
	mu      sync.Mutex
	pending map[int64]entry
}

type entry struct {
	cb meta.AckCallback
}

// NewSystem builds an empty System.
func NewSystem() *System {
	return &System{pending: make(map[int64]entry)}
}

// Dispatch allocates a marker timestamp, stores cb against it, and returns the
// NetworkStackLatency packet the proxy must forward to the client. cb will be
// invoked exactly once when the client echoes the timestamp back (via
// OnResponse) with the tick seen on that response.
func (s *System) Dispatch(cb meta.AckCallback) packet.Packet {
	ts := nextTimestamp()
	s.mu.Lock()
	s.pending[ts] = entry{cb: cb}
	s.mu.Unlock()
	return newMarker(ts)
}

// OnResponse is called by the proxy whenever the client sends a
// NetworkStackLatency back. The client divides the dispatched timestamp by
// 1000 before sending it; the proxy multiplies it back up before calling us,
// so the timestamp supplied here matches the value Dispatch originally
// stored. Unknown timestamps are ignored — they may be Mojang-initiated
// latency checks or pong packets routed from elsewhere.
func (s *System) OnResponse(timestamp int64, tick uint64) {
	s.mu.Lock()
	e, ok := s.pending[timestamp]
	if ok {
		delete(s.pending, timestamp)
	}
	s.mu.Unlock()
	if ok && e.cb != nil {
		e.cb(tick)
	}
}

// PendingCount is a debug accessor reporting how many callbacks are waiting.
func (s *System) PendingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
}

// compile-time contract check
var _ meta.AckSystem = (*System)(nil)

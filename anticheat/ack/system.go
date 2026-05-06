package ack

import (
	"sync"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Key identifies a dual-clock acknowledgement by the server tick and client
// tick at which an authoritative correction was sent.
type Key struct {
	ServerTick uint64
	ClientTick uint64
}

// ActionKind classifies the correction that requires acknowledgement.
type ActionKind int

const (
	ActionUnknown    ActionKind = iota
	ActionKnockback             // velocity impulse from server
	ActionTeleport              // position teleport
	ActionCorrection            // generic position correction
)

// Action describes a pending correction whose acknowledgement we expect.
type Action struct {
	Kind          ActionKind
	ExpectedDelta mgl32.Vec3 // expected positional delta on acknowledgement
}

// matchTol is the Euclidean tolerance for considering a resolved delta a match.
const matchTol = 0.15

// System implements meta.AckSystem. It hands out NetworkStackLatency markers
// on Dispatch and resolves their callbacks when the client echoes the
// matching timestamp back through OnResponse.
type System struct {
	mu             sync.Mutex
	pending        map[int64]entry
	pendingActions map[Key]Action
}

type entry struct {
	cb meta.AckCallback
}

// NewSystem builds an empty System.
func NewSystem() *System {
	return &System{
		pending:        make(map[int64]entry),
		pendingActions: make(map[Key]Action),
	}
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

// PurgeActions removes all pending dual-clock actions.
// Call this when the associated player disconnects.
func (s *System) PurgeActions() {
	s.mu.Lock()
	s.pendingActions = make(map[Key]Action)
	s.mu.Unlock()
}

// PendingActionsCount returns the number of unresolved dual-clock actions.
func (s *System) PendingActionsCount() int {
	s.mu.Lock()
	n := len(s.pendingActions)
	s.mu.Unlock()
	return n
}

// compile-time contract check
var _ meta.AckSystem = (*System)(nil)

// Push registers a correction Action under the given dual-clock Key.
// The action will be matched when the client acknowledges the tick pair.
func (s *System) Push(k Key, a Action) {
	s.mu.Lock()
	s.pendingActions[k] = a
	s.mu.Unlock()
}

// Resolve looks up the pending Action for k. If found, it removes the entry
// and returns (true, action) when actualDelta is within matchTol of
// action.ExpectedDelta, or (false, action) when the delta is too large.
// If no action is pending for k, it returns (false, Action{}).
func (s *System) Resolve(k Key, actualDelta mgl32.Vec3) (bool, Action) {
	s.mu.Lock()
	a, ok := s.pendingActions[k]
	if ok {
		delete(s.pendingActions, k)
	}
	s.mu.Unlock()
	if !ok {
		return false, Action{}
	}
	diff := actualDelta.Sub(a.ExpectedDelta)
	return diff.Len() <= matchTol, a
}

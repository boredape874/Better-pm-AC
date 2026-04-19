package ack

import (
	"sync/atomic"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestDispatchReturnsNetworkStackLatencyWithNeedsResponse(t *testing.T) {
	s := NewSystem()
	pk := s.Dispatch(func(uint64) {})
	nsl, ok := pk.(*packet.NetworkStackLatency)
	if !ok {
		t.Fatalf("expected *packet.NetworkStackLatency, got %T", pk)
	}
	if !nsl.NeedsResponse {
		t.Fatal("marker must set NeedsResponse=true")
	}
	if nsl.Timestamp%timestampStep != 0 {
		t.Fatalf("marker timestamp %d must be multiple of %d", nsl.Timestamp, timestampStep)
	}
}

func TestDispatchTimestampsAreMonotonic(t *testing.T) {
	s := NewSystem()
	a := s.Dispatch(func(uint64) {}).(*packet.NetworkStackLatency).Timestamp
	b := s.Dispatch(func(uint64) {}).(*packet.NetworkStackLatency).Timestamp
	if b <= a {
		t.Fatalf("expected %d > %d", b, a)
	}
}

func TestOnResponseFiresCallbackOnce(t *testing.T) {
	s := NewSystem()
	var hits atomic.Int32
	var seenTick atomic.Uint64
	pk := s.Dispatch(func(tick uint64) {
		hits.Add(1)
		seenTick.Store(tick)
	})
	ts := pk.(*packet.NetworkStackLatency).Timestamp
	if s.PendingCount() != 1 {
		t.Fatalf("expected 1 pending, got %d", s.PendingCount())
	}
	s.OnResponse(ts, 42)
	if hits.Load() != 1 {
		t.Fatalf("callback should fire once, got %d hits", hits.Load())
	}
	if seenTick.Load() != 42 {
		t.Fatalf("callback should see tick=42, got %d", seenTick.Load())
	}
	// Subsequent responses for the same timestamp are ignored.
	s.OnResponse(ts, 100)
	if hits.Load() != 1 {
		t.Fatalf("duplicate response should not re-fire, got %d hits", hits.Load())
	}
	if s.PendingCount() != 0 {
		t.Fatalf("expected 0 pending after response, got %d", s.PendingCount())
	}
}

func TestOnResponseUnknownTimestampIgnored(t *testing.T) {
	s := NewSystem()
	// Server didn't dispatch this — ignore rather than panic.
	s.OnResponse(99999999, 1)
	if s.PendingCount() != 0 {
		t.Fatalf("unknown response should not create pending, got %d", s.PendingCount())
	}
}

package ack

import (
	"sync/atomic"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// timestampStep is the granularity monotonically assigned to each new marker.
// Bedrock divides the echoed NetworkStackLatency timestamp by 1000 before
// sending it back, so stored values must already be multiples of 1000 for
// the round-trip comparison to hit — otherwise the response never matches
// a pending entry. Oomph uses the same convention.
const timestampStep int64 = 1000

// timestampSource produces monotonically increasing, round-trip-safe
// timestamps. Values never collide within a session even across multiple
// Ack Systems because the global atomic counter seeds every instance.
var timestampSource atomic.Int64

// nextTimestamp returns a fresh marker timestamp. Guaranteed divisible by
// timestampStep and strictly greater than any previously returned value.
func nextTimestamp() int64 {
	// Add returns the new value — divide-by-1000-safe so long as step is 1000.
	return timestampSource.Add(timestampStep)
}

// newMarker builds the packet.NetworkStackLatency the proxy sends to the
// client for a given timestamp. NeedsResponse is always true — a marker that
// cannot be echoed back is useless for acknowledgement.
func newMarker(timestamp int64) packet.Packet {
	return &packet.NetworkStackLatency{Timestamp: timestamp, NeedsResponse: true}
}

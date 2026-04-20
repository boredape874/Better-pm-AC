package world

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// HandleChunkPublisher implements chunk LRU pruning driven by the server's
// NetworkChunkPublisherUpdate packets. The packet tells us the radius (in
// blocks) around a center point within which chunks must remain loaded;
// everything outside is definitionally unloadable.
//
// Without this, a long-running session accumulates every chunk the player
// has ever seen — after an hour of travel the map easily pushes 100 MB of
// decoded chunks. Pruning outside the publisher radius keeps memory bounded
// to the render-distance footprint.
//
// This is optional — not every server sends the packet reliably — so the
// proxy wires it where available, but missing packets simply mean the
// cache grows unbounded (still safe, just wasteful). We don't prune on
// other signals (e.g. distance from last-seen player pos) because
// NetworkChunkPublisherUpdate is server-authoritative and matches the
// client's view exactly.
func (t *Tracker) HandleChunkPublisher(pk *packet.NetworkChunkPublisherUpdate) {
	if pk == nil {
		return
	}
	// Radius is in blocks; convert to chunk-space with ceil. Add 1 to be
	// safe — the client loads chunks slightly past the exact radius.
	chunkRadius := int32(pk.Radius>>4) + 1
	centerX := int32(pk.Position.X() >> 4)
	centerZ := int32(pk.Position.Z() >> 4)

	t.mu.Lock()
	defer t.mu.Unlock()
	for key := range t.chunks {
		dx := key.X - centerX
		dz := key.Z - centerZ
		if dx < -chunkRadius || dx > chunkRadius ||
			dz < -chunkRadius || dz > chunkRadius {
			delete(t.chunks, key)
		}
	}
}

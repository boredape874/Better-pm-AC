package world

import (
	"fmt"

	dfchunk "github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// HandleLevelChunk ingests a LevelChunk packet. The full-chunk path uses
// Dragonfly's chunk.NetworkDecode on the raw payload. SubChunkRequest mode
// chunks carry no block data in the initial packet; we insert an empty
// Chunk so ChunkLoaded reports true and SubChunk packets can merge into it.
//
// CacheEnabled (blob cache) is rejected at β — supporting it would require
// buffering blob hashes and joining with ClientCacheBlobStatus responses.
// Oomph skips this; public servers typically disable the cache. Revisit
// post-γ if a target server enables it.
func (t *Tracker) HandleLevelChunk(pk *packet.LevelChunk) error {
	if pk == nil {
		return fmt.Errorf("level chunk: nil packet")
	}
	if pk.CacheEnabled {
		// Fail-closed: leave chunk unloaded. ChunkLoaded=false keeps
		// world-dependent checks from false-flagging.
		return nil
	}

	key := chunkKey{pk.Position.X(), pk.Position.Z()}

	// SubChunkRequest mode: the real block data arrives later via SubChunk
	// packets. Store an empty chunk so subsequent updates can merge in.
	if pk.SubChunkCount == protocol.SubChunkRequestModeLimitless ||
		pk.SubChunkCount == protocol.SubChunkRequestModeLimited {
		c := dfchunk.New(t.air, t.rng)
		t.mu.Lock()
		t.chunks[key] = c
		t.mu.Unlock()
		return nil
	}

	c, err := dfchunk.NetworkDecode(t.air, pk.RawPayload, int(pk.SubChunkCount), t.rng)
	if err != nil {
		return fmt.Errorf("level chunk %v: decode: %w", pk.Position, err)
	}
	t.mu.Lock()
	t.chunks[key] = c
	t.mu.Unlock()
	return nil
}

// HandleSubChunk is a β-scope no-op.
//
// Background: in SubChunkRequest mode the server replies to client
// SubChunkRequest packets with SubChunk packets carrying individual
// sub-chunk payloads. Dragonfly's chunk package only exposes a combined
// NetworkDecode that reads [subchunks][biomes] together — decoding a
// single sub-chunk in isolation requires the unexported decodeSubChunk
// symbol or a correctly-synthesised biome tail.
//
// β trade-off: servers using SubChunkRequest mode will have chunks that
// report ChunkLoaded=true but return air for Block() queries. This
// fails-open (Fly/Speed won't false-flag on empty air) but degrades the
// accuracy of world-interactive checks (Phase/Scaffold) on those servers.
// Full support is a γ+1 Task once we vendor or re-implement the
// sub-chunk decoder.
func (t *Tracker) HandleSubChunk(pk *packet.SubChunk) error {
	if pk == nil {
		return fmt.Errorf("sub chunk: nil packet")
	}
	// Entries are dropped silently. Chunks from the preceding LevelChunk
	// remain as empty-air placeholders.
	return nil
}

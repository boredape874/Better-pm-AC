# testdata/

Captured Bedrock protocol payloads used by anticheat integration tests.

Files in this directory MUST be byte-exact captures from a real Bedrock
session. They feed tests that exercise the world-tracker / sim / check
code paths against realistic payloads rather than synthesised ones.

## Why not just synthesise?

`anticheat/world` consumes `LevelChunk`/`SubChunk` payloads whose wire
format depends on the `dragonfly/server/world/chunk` palette encoding
and sub-chunk version byte. Hand-rolling these would duplicate the
decoder and defeat the purpose of the test. Capturing once from a real
server guarantees the decoder handles payloads produced by the server
our proxy actually sits in front of.

## How to capture (planned for Task 2.W.0 runtime phase)

1. Build a throwaway proxy binary with packet-dump hooks:
   ```go
   // proxy/proxy.go — TEMPORARY, remove before commit
   case *packet.LevelChunk:
       os.WriteFile("testdata/level_chunk.bin", pk.RawPayload, 0644)
   case *packet.SubChunk:
       // encode protocol.SubChunkEntries + dimension into a wrapper
       dumpSubChunkPacket("testdata/subchunk.bin", pk)
   case *packet.UpdateBlock:
       dumpUpdateBlock("testdata/update_block.bin", pk)
   ```
2. Run the proxy against a test Bedrock server (vanilla + one modded +
   one PocketMine to cover payload dialects).
3. Walk in-game: spawn, move 20 blocks, place one block, break one
   block. That reliably produces all three packet types.
4. Verify captures decode via `dragonfly/server/world/chunk.NetworkDecode`
   in an ad-hoc test.
5. Copy captures into `testdata/`, remove the proxy hooks, commit.

## Files expected (β)

| File                           | Source packet                 | Min size |
|--------------------------------|-------------------------------|----------|
| `level_chunk_overworld.bin`    | `packet.LevelChunk` RawPayload | ~4 KiB   |
| `level_chunk_nether.bin`       | `packet.LevelChunk` (Nether)  | ~2 KiB   |
| `subchunk.bin`                 | Full `packet.SubChunk`        | ~8 KiB   |
| `update_block.bin`             | `packet.UpdateBlock` marshaled | ~16 B   |

Packets bigger than LevelChunk should be encoded with the packet's
`Marshal` method into a protocol.NewWriter buffer so tests can replay
them through `Unmarshal` symmetrically.

## Current state

**β release has zero captured testdata.** The unit tests in
`anticheat/world/tracker_test.go` use directly-injected `dfchunk.Chunk`
values via an internal helper, which exercises the query path but not
the decode path.

**Gap**: LevelChunk / SubChunk decode is unverified outside the types
compiling. Any palette-encoding regression in dragonfly's chunk package
or payload-shape change in gophertunnel would slip through CI.

**Mitigation**: Task 2.W.0 runtime phase (scheduled with β field test).
During the first proxy boot against a real server, capture the four
files above and commit them. Subsequent PRs add decode round-trip
tests. This is tracked as [plan task 2.W.0](../docs/plans/2026-04-19-work-board.md).

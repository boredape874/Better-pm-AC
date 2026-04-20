package proxy

import (
	"github.com/boredape874/Better-pm-AC/anticheat/ack"
	"github.com/boredape874/Better-pm-AC/anticheat/entity"
	"github.com/boredape874/Better-pm-AC/anticheat/world"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
)

// Session holds the paired client and server connections for one player.
type Session struct {
	ID     uuid.UUID
	Client *minecraft.Conn
	Server *minecraft.Conn

	// EntityRID is the server-assigned entity runtime ID for this player.
	// It is read from ServerConn.GameData().EntityRuntimeID at session start
	// and used to filter MobEffect packets so that only effects targeting the
	// player's own entity are applied to the anticheat data model.
	EntityRID uint64

	// World tracks server-sent chunks/blocks so checks can query the real
	// world state rather than trusting client-reported collisions. See
	// anticheat/world for the implementation. One per session because
	// chunks diverge across shards / dimensions.
	World *world.Tracker

	// Rewind is the per-session entity pose ring buffer used by Reach / Aim
	// lag compensation. Checks call rewind.At(rid, tick-latencyTicks) to
	// get the entity pose the attacking client could actually see.
	Rewind *entity.Rewind

	// Ack dispatches NetworkStackLatency markers and correlates responses.
	// Checks that need "client confirmed it processed this tick" use it for
	// acknowledgement-gated detection (e.g. Timer desync).
	Ack *ack.System

	// inWater is a persistent flag set on InputFlagStartSwimming and cleared
	// on InputFlagStopSwimming. Unlike InputFlagAutoJumpingInWater (which fires
	// only on the first auto-jump tick) and InputFlagStartSwimming (which fires
	// only once on entry), this flag stays true for every tick the player is
	// swimming. It is passed to player.SetInputFlags so that water exemptions
	// in Fly/A, NoFall/A, Speed/A, and Speed/B apply correctly throughout the
	// swim session rather than for only a single tick.
	inWater bool

	// isCrawling is a persistent flag set on InputFlagStartCrawling and cleared
	// on InputFlagStopCrawling. Crawling significantly reduces movement speed,
	// so Speed/A uses a dedicated crawl multiplier instead of the normal limits.
	// Maintaining sticky state here (rather than recomputing per-tick from the
	// start/stop flags) mirrors the same pattern used for inWater.
	isCrawling bool

	// isUsingItem is a persistent flag set on InputFlagStartUsingItem and
	// cleared when InputFlagPerformItemInteraction fires (item use completed /
	// cancelled). While active, Speed/A enforces the slower item-use speed
	// limit and NoSlow/A checks for speed that exceeds that limit.
	isUsingItem bool
}

// newSession creates a Session for the given player. World / Rewind / Ack
// are allocated eagerly because every session uses all three — lazy
// allocation would add a nil-check to every check's hot path.
func newSession(id uuid.UUID, client, server *minecraft.Conn) *Session {
	return &Session{
		ID:     id,
		Client: client,
		Server: server,
		World:  world.NewTracker(),
		Rewind: entity.NewRewind(),
		Ack:    ack.NewSystem(),
	}
}

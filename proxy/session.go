package proxy

import (
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

	// inWater is a persistent flag set on InputFlagStartSwimming and cleared
	// on InputFlagStopSwimming. Unlike InputFlagAutoJumpingInWater (which fires
	// only on the first auto-jump tick) and InputFlagStartSwimming (which fires
	// only once on entry), this flag stays true for every tick the player is
	// swimming. It is passed to player.SetInputFlags so that water exemptions
	// in Fly/A, NoFall/A, Speed/A, and Speed/B apply correctly throughout the
	// swim session rather than for only a single tick.
	inWater bool
}

// newSession creates a Session for the given player.
func newSession(id uuid.UUID, client, server *minecraft.Conn) *Session {
	return &Session{
		ID:     id,
		Client: client,
		Server: server,
	}
}

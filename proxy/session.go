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
}

// newSession creates a Session for the given player.
func newSession(id uuid.UUID, client, server *minecraft.Conn) *Session {
	return &Session{
		ID:     id,
		Client: client,
		Server: server,
	}
}

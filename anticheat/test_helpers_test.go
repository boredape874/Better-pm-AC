package anticheat

import (
	"sync/atomic"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/google/uuid"
)

// SetServerTickForTest forces the ServerTick counter (test helper only).
func (m *Manager) SetServerTickForTest(v uint64) {
	atomic.StoreUint64(&m.serverTick, v)
}

// LastTickContextForTest returns the most recent TickContext for a player.
func (m *Manager) LastTickContextForTest(id uuid.UUID) meta.TickContext {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastTickCtx[id]
}

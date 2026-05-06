package mitigate

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

// CorrectFunc is called when a Snap reconcile outcome requires sending a
// position correction to the client. The proxy layer provides the actual
// implementation (packet send); the anticheat layer remains agnostic of the
// wire format. Nil CorrectFunc is valid — corrections become no-ops.
type CorrectFunc func(id uuid.UUID, pos mgl32.Vec3)

// Corrector dispatches position corrections to clients on behalf of the
// reconciler. It wraps a CorrectFunc so callers never need to nil-check.
type Corrector struct {
	fn CorrectFunc
}

// NewCorrector creates a Corrector backed by fn. If fn is nil, all Send calls
// are silent no-ops — useful during initial rollout before packet wiring lands.
func NewCorrector(fn CorrectFunc) *Corrector {
	return &Corrector{fn: fn}
}

// Send dispatches a correction for the given player to pos.
// It is safe to call from any goroutine; the underlying fn is responsible for
// its own thread-safety.
func (c *Corrector) Send(id uuid.UUID, pos mgl32.Vec3) {
	if c.fn != nil {
		c.fn(id, pos)
	}
}

// Package meta defines the Detection interface and DetectionMetadata types used
// by all anti-cheat checks. It is a leaf package imported by both the checks
// and the anticheat manager, avoiding import cycles.
package meta

import "math"

// Detection is implemented by every anti-cheat check.
// The interface mirrors Oomph-AC's Detection interface.
type Detection interface {
// Type returns the primary check category, e.g. "KillAura".
Type() string
// SubType returns the variant letter, e.g. "A".
SubType() string
// Description briefly explains what the detection looks for.
Description() string
// Punishable returns true when exceeding MaxViolations should kick the player.
Punishable() bool
// DefaultMetadata returns the initial DetectionMetadata for a new player.
DefaultMetadata() *DetectionMetadata
}

// DetectionMetadata tracks the violation state for one Detection applied to
// one specific player. It implements Oomph's buffer-accumulation model.
type DetectionMetadata struct {
// Violations is the accumulated violation score.
Violations float64
// MaxViolations is the score at which the player is punished.
MaxViolations float64

// Buffer accumulates on every Fail() call. A violation is only recorded
// once Buffer reaches FailBuffer, giving the check a built-in "confirm"
// window. Buffer is capped at MaxBuffer.
Buffer     float64
FailBuffer float64
MaxBuffer  float64

// TrustDuration, when > 0, makes violations decay based on how long ago
// the player last flagged (measured in simulation ticks).
TrustDuration int64
// LastFlagged holds the SimulationFrame of the most recent recorded violation.
LastFlagged int64
}

// Fail records a failure for the current simulation tick. Returns true only
// when a violation is actually recorded (Buffer ≥ FailBuffer). currentTick is
// the player's SimulationFrame (PlayerAuthInput.Tick).
func (m *DetectionMetadata) Fail(currentTick int64) bool {
m.Buffer = math.Min(m.Buffer+1.0, m.MaxBuffer)
if m.Buffer < m.FailBuffer {
return false
}
if m.TrustDuration > 0 {
decay := float64(m.TrustDuration) - float64(currentTick-m.LastFlagged)
m.Violations += math.Max(0, decay) / float64(m.TrustDuration)
} else {
m.Violations++
}
m.LastFlagged = currentTick
return true
}

// Pass reduces the buffer by sub (floored at 0).
func (m *DetectionMetadata) Pass(sub float64) {
m.Buffer = math.Max(0, m.Buffer-sub)
}

// Exceeded returns true when the violation score has reached MaxViolations.
func (m *DetectionMetadata) Exceeded() bool {
return m.Violations >= m.MaxViolations
}

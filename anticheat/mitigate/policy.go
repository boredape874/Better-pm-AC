package mitigate

import (
	"log/slog"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// KickFunc disconnects a player by UUID. Supplied by the proxy layer; the
// dispatcher stays agnostic of how the connection is actually torn down.
type KickFunc func(uuid string, reason string)

// Dispatcher implements meta.MitigateDispatcher. It routes each Detection to
// the enforcement path encoded in its Policy:
//
//   - PolicyKick            → log; if MaxViolations is exceeded, invoke KickFunc
//   - PolicyClientRubberband → (2.M.3) to be wired with ack markers
//   - PolicyServerFilter    → (2.M.4) to be wired with packet rewriting
//   - PolicyNone            → log only; forward unchanged
//
// This 2.M.1/2.M.2 milestone delivers the Kick path plus the structural hook
// points for the other policies. Tasks 2.M.3 and 2.M.4 fill in the remaining
// behaviors behind those hooks.
type Dispatcher struct {
	log  *slog.Logger
	kick KickFunc
}

// NewDispatcher builds a Dispatcher. log must not be nil — the dispatcher
// always emits a structured record for every violation. kick may be nil in
// tests or dry-run contexts, in which case Kick policy decisions degrade to
// "log only" without terminating the session.
func NewDispatcher(log *slog.Logger, kick KickFunc) *Dispatcher {
	return &Dispatcher{log: log, kick: kick}
}

// Apply routes one Detection to its policy path and returns the packet the
// proxy should forward (nil means "drop"), plus whether the session must be
// kicked after this call returns.
func (d *Dispatcher) Apply(playerUUID string, det meta.Detection, m *meta.DetectionMetadata,
	original packet.Packet) (packet.Packet, bool) {
	policy := det.Policy()
	d.log.Warn("violation",
		"player", playerUUID,
		"check", det.Type()+"/"+det.SubType(),
		"violations", m.Violations,
		"max", m.MaxViolations,
		"policy", policyName(policy),
	)
	switch policy {
	case meta.PolicyKick:
		return d.applyKick(playerUUID, det, m, original)
	case meta.PolicyClientRubberband:
		// Structural hook; wiring lands in 2.M.3.
		return original, false
	case meta.PolicyServerFilter:
		// Structural hook; wiring lands in 2.M.4.
		return original, false
	case meta.PolicyNone:
		return original, false
	default:
		// Unknown policies fail safe: treat like PolicyNone rather than panic.
		d.log.Error("unknown policy, defaulting to none",
			"player", playerUUID, "check", det.Type()+"/"+det.SubType(), "policy", policy)
		return original, false
	}
}

// applyKick logs the violation and, when MaxViolations is exceeded, triggers
// the KickFunc. The packet is always forwarded so the server observes the
// event that caused the kick — helpful for log correlation.
func (d *Dispatcher) applyKick(playerUUID string, det meta.Detection, m *meta.DetectionMetadata,
	original packet.Packet) (packet.Packet, bool) {
	if !det.Punishable() || !m.Exceeded() {
		return original, false
	}
	if d.kick == nil {
		// Dry-run: report the decision but do not tear down the connection.
		d.log.Warn("kick suppressed — no KickFunc configured",
			"player", playerUUID, "check", det.Type()+"/"+det.SubType())
		return original, false
	}
	reason := "Kicked by Better-pm-AC: " + det.Type() + "/" + det.SubType()
	d.kick(playerUUID, reason)
	return original, true
}

// policyName renders the policy enum for log output. Keeping the mapping in
// one place prevents drift between log fields and config strings.
func policyName(p meta.MitigatePolicy) string {
	switch p {
	case meta.PolicyNone:
		return "none"
	case meta.PolicyClientRubberband:
		return "client_rubberband"
	case meta.PolicyServerFilter:
		return "server_filter"
	case meta.PolicyKick:
		return "kick"
	default:
		return "unknown"
	}
}

// compile-time contract check
var _ meta.MitigateDispatcher = (*Dispatcher)(nil)

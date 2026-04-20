package mitigate

import (
	"log/slog"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// KickFunc disconnects a player by UUID. Supplied by the proxy layer; the
// dispatcher stays agnostic of how the connection is actually torn down.
type KickFunc func(uuid string, reason string)

// RubberbandFunc snaps a client back to its last-known server-valid
// position by sending a MovePlayer teleport packet. The proxy implements
// this because only it owns the client connection.
type RubberbandFunc func(uuid string)

// ServerFilterFunc rewrites an offending packet in place — typically
// overwriting the Position with the last valid coordinate — so the
// server never observes the cheated value. Returns the packet to
// forward; returning nil drops the packet entirely.
type ServerFilterFunc func(uuid string, original packet.Packet) packet.Packet

// Dispatcher implements meta.MitigateDispatcher. It routes each Detection to
// the enforcement path encoded in its Policy:
//
//   - PolicyKick            → log; if MaxViolations is exceeded, invoke KickFunc
//   - PolicyClientRubberband → invoke RubberbandFunc; packet forwards unchanged
//   - PolicyServerFilter    → ServerFilterFunc rewrites the packet
//   - PolicyNone            → log only; forward unchanged
//
// Callbacks can be nil (dry-run / tests). Nil RubberbandFunc and
// ServerFilterFunc degrade to log-only for that policy — they never
// panic.
type Dispatcher struct {
	log    *slog.Logger
	kick   KickFunc
	rubber RubberbandFunc
	filter ServerFilterFunc
}

// NewDispatcher builds a Dispatcher with only the kick hook wired.
// Rubberband / server-filter default to nil — use NewDispatcherWithHooks
// to provide them. log must not be nil; it always emits a structured
// record for every violation.
func NewDispatcher(log *slog.Logger, kick KickFunc) *Dispatcher {
	return &Dispatcher{log: log, kick: kick}
}

// NewDispatcherWithHooks builds a Dispatcher with all three mitigation
// callbacks wired. Any callback may be nil; nil callbacks degrade their
// policy path to "log only".
func NewDispatcherWithHooks(log *slog.Logger, kick KickFunc, rubber RubberbandFunc, filter ServerFilterFunc) *Dispatcher {
	return &Dispatcher{log: log, kick: kick, rubber: rubber, filter: filter}
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
		return d.applyRubberband(playerUUID, original)
	case meta.PolicyServerFilter:
		return d.applyServerFilter(playerUUID, original)
	case meta.PolicyNone:
		return original, false
	default:
		// Unknown policies fail safe: treat like PolicyNone rather than panic.
		d.log.Error("unknown policy, defaulting to none",
			"player", playerUUID, "check", det.Type()+"/"+det.SubType(), "policy", policy)
		return original, false
	}
}

// applyRubberband invokes the session's rubberband hook. The returned
// packet is the original (unchanged) because the rubberband is an
// out-of-band corrective packet sent by the proxy — the triggering
// packet itself continues forward. This matches Oomph's "teleport the
// client, keep the server canonical" model.
//
// Rubberband is rate-limited downstream by the hook itself (typical:
// once per 20 ticks) to avoid packet-flooding the client.
func (d *Dispatcher) applyRubberband(playerUUID string, original packet.Packet) (packet.Packet, bool) {
	if d.rubber == nil {
		d.log.Warn("rubberband suppressed — no hook configured", "player", playerUUID)
		return original, false
	}
	d.rubber(playerUUID)
	return original, false
}

// applyServerFilter delegates to the filter hook which may rewrite or
// drop the packet. Returning (nil, false) from the hook means the
// packet is dropped entirely — the server never sees the cheated input.
// Returning the packet (mutated or not) means "forward this instead".
func (d *Dispatcher) applyServerFilter(playerUUID string, original packet.Packet) (packet.Packet, bool) {
	if d.filter == nil {
		d.log.Warn("server filter suppressed — no hook configured", "player", playerUUID)
		return original, false
	}
	forwarded := d.filter(playerUUID, original)
	return forwarded, false
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

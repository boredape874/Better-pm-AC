// Package anticheat is the core of Better-pm-AC. It coordinates all detection
// checks and maintains per-player violation state using Oomph-AC's buffer-based
// system (DetectionMetadata with Buffer/FailBuffer/MaxBuffer/Violations).
package anticheat

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/boredape874/Better-pm-AC/anticheat/checks/combat"
	"github.com/boredape874/Better-pm-AC/anticheat/checks/movement"
	pkt "github.com/boredape874/Better-pm-AC/anticheat/checks/packet"
	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/anticheat/mitigate"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

// InputDataLoader abstracts protocol.Bitset.Load so the anticheat package does
// not need to import the protocol package (which would cause an import cycle
// through the proxy package). The bit argument matches protocol.Bitset's
// actual Load(int) bool signature.
type InputDataLoader interface {
	Load(bit int) bool
}

// Re-export so callers only need to import anticheat, not anticheat/meta.
type Detection = meta.Detection
type DetectionMetadata = meta.DetectionMetadata

// Check key constants: canonical "Type/SubType" strings used to index the
// per-player DetectionMetadata map. They must exactly match the values
// returned by each Detection implementation's Type() and SubType() methods.
const (
	keySpeed        = "Speed/A"
	keySpeedB       = "Speed/B"
	keyFly          = "Fly/A"
	keyFlyB         = "Fly/B"
	keyNoFall       = "NoFall/A"
	keyNoFallB      = "NoFall/B"
	keyNoSlow       = "NoSlow/A"
	keyPhase        = "Phase/A"
	keyReach        = "Reach/A"
	keyKillAura     = "KillAura/A"
	keyKillAuraB    = "KillAura/B"
	keyKillAuraC    = "KillAura/C"
	keyAutoClicker  = "AutoClicker/A"
	keyAutoClickerB = "AutoClicker/B"
	keyAim          = "Aim/A"
	keyAimB         = "Aim/B"
	keyBadPacket    = "BadPacket/A"
	keyBadPacketB   = "BadPacket/B"
	keyBadPacketC   = "BadPacket/C"
	keyBadPacketD   = "BadPacket/D"
	keyBadPacketE   = "BadPacket/E"
	keyTimer        = "Timer/A"
	keyVelocity     = "Velocity/A"
	keyScaffold     = "Scaffold/A"
)

// playerDetections maps each check's canonical "Type/SubType" key to its
// per-player violation metadata. Using a map instead of a struct means that
// registering a new check requires no changes here: NewManager adds it to
// m.checks, and newPlayerDetections() iterates that slice automatically.
type playerDetections = map[string]*DetectionMetadata

// Manager coordinates all anti-cheat checks and the player registry.
type Manager struct {
	cfg config.AnticheatConfig
	log *slog.Logger

	mu         sync.RWMutex
	players    map[uuid.UUID]*data.Player
	detections map[uuid.UUID]playerDetections

	// checks is the ordered registry of all registered Detection
	// implementations. It is used by newPlayerDetections() to initialise
	// per-player metadata maps so that adding a new check requires no
	// changes to playerDetections itself — only a new entry here.
	checks []Detection

	// Stateless check instances (typed for direct invocation with their
	// specific call signatures).
	speed        *movement.SpeedCheck
	speedB       *movement.SpeedBCheck
	fly          *movement.FlyCheck
	noFall       *movement.NoFallCheck
	noFallB      *movement.NoFallBCheck
	noSlow       *movement.NoSlowCheck
	phase        *movement.PhaseACheck
	reach        *combat.ReachCheck
	killAura     *combat.KillAuraCheck
	killAuraB    *combat.KillAuraBCheck
	killAuraC    *combat.KillAuraCCheck
	autoClicker  *combat.AutoClickerCheck
	autoClickerB *combat.AutoClickerBCheck
	aim          *combat.AimCheck
	aimB         *combat.AimBCheck
	badPacket    *pkt.BadPacketCheck
	badPacketB   *pkt.BadPacketBCheck
	badPacketC   *pkt.BadPacketCCheck
	badPacketD   *pkt.BadPacketDCheck
	badPacketE   *pkt.BadPacketECheck
	flyB         *movement.FlyBCheck
	timer        *movement.TimerCheck
	velocity     *movement.VelocityCheck
	scaffold     *movement.ScaffoldCheck

	// KickFunc is called when a player should be disconnected. It is also
	// the kick hook for the internal mitigate.Dispatcher built on demand
	// inside handleViolation. Nil → dry-run (log only). Used to adapt the
	// uuid-typed KickFunc into the dispatcher's string-typed contract.
	KickFunc func(id uuid.UUID, reason string)

	// Mitigation hooks. When non-nil, Rubberband/ServerFilter dispatcher paths
	// invoke them; nil hooks degrade to log-only inside the Dispatcher. Set by
	// the proxy layer during Phase 5a integration.
	RubberbandFunc   mitigate.RubberbandFunc
	ServerFilterFunc mitigate.ServerFilterFunc
}

// NewManager creates a Manager ready to process packets.
func NewManager(cfg config.AnticheatConfig, log *slog.Logger) *Manager {
	m := &Manager{
		cfg:        cfg,
		log:        log,
		players:    make(map[uuid.UUID]*data.Player),
		detections: make(map[uuid.UUID]playerDetections),
		speed:        movement.NewSpeedCheck(cfg.Speed),
		speedB:       movement.NewSpeedBCheck(cfg.SpeedB),
		fly:          movement.NewFlyCheck(cfg.Fly),
		noFall:       movement.NewNoFallCheck(cfg.NoFall),
		noFallB:      movement.NewNoFallBCheck(cfg.NoFallB),
		noSlow:       movement.NewNoSlowCheck(cfg.NoSlow),
		phase:        movement.NewPhaseACheck(cfg.Phase),
		reach:        combat.NewReachCheck(cfg.Reach),
		killAura:     combat.NewKillAuraCheck(cfg.KillAura),
		killAuraB:    combat.NewKillAuraBCheck(cfg.KillAuraB),
		killAuraC:    combat.NewKillAuraCCheck(cfg.KillAuraC),
		autoClicker:  combat.NewAutoClickerCheck(cfg.AutoClicker),
		autoClickerB: combat.NewAutoClickerBCheck(cfg.AutoClickerB),
		aim:          combat.NewAimCheck(cfg.Aim),
		aimB:         combat.NewAimBCheck(cfg.AimB),
		badPacket:    pkt.NewBadPacketCheck(cfg.BadPacket),
		badPacketB:   pkt.NewBadPacketBCheck(cfg.BadPacketB),
		badPacketC:   pkt.NewBadPacketCCheck(cfg.BadPacketC),
		badPacketD:   pkt.NewBadPacketDCheck(cfg.BadPacketD),
		badPacketE:   pkt.NewBadPacketECheck(cfg.BadPacketE),
		flyB:         movement.NewFlyBCheck(cfg.FlyB),
		timer:        movement.NewTimerCheck(cfg.Timer),
		velocity:     movement.NewVelocityCheck(cfg.Velocity),
		scaffold:     movement.NewScaffoldCheck(cfg.Scaffold),
	}
	// Register every check so newPlayerDetections() can iterate the slice
	// instead of enumerating fields. To add a new check: create its typed
	// field above, then append it here — no other registry changes needed.
	m.checks = []Detection{
		m.speed, m.speedB,
		m.fly, m.flyB,
		m.noFall, m.noFallB,
		m.noSlow,
		m.phase,
		m.reach,
		m.killAura, m.killAuraB, m.killAuraC,
		m.autoClicker, m.autoClickerB,
		m.aim, m.aimB,
		m.badPacket, m.badPacketB, m.badPacketC, m.badPacketD, m.badPacketE,
		m.timer,
		m.velocity,
		m.scaffold,
	}
	return m
}

// newPlayerDetections creates a fresh playerDetections map for a new player
// by iterating the check registry. Adding a new check to m.checks is the
// only change required — this function needs no modification.
func (m *Manager) newPlayerDetections() playerDetections {
	det := make(playerDetections, len(m.checks))
	for _, chk := range m.checks {
		det[chk.Type()+"/"+chk.SubType()] = chk.DefaultMetadata()
	}
	return det
}

// AddPlayer registers a new player session.
func (m *Manager) AddPlayer(id uuid.UUID, username string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.players[id] = data.NewPlayer(id, username)
	m.detections[id] = m.newPlayerDetections()
	m.log.Info("player joined", "uuid", id, "username", username)
}

// RemovePlayer removes a player session and frees its detection state.
func (m *Manager) RemovePlayer(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.players, id)
	delete(m.detections, id)
}

func (m *Manager) getPlayer(id uuid.UUID) *data.Player {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.players[id]
}

// GetPlayer returns the Player data for the given UUID (thread-safe).
// Used by the proxy layer to update per-tick input state before running checks.
func (m *Manager) GetPlayer(id uuid.UUID) *data.Player {
	return m.getPlayer(id)
}

func (m *Manager) getDet(id uuid.UUID) playerDetections {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.detections[id]
}

// UpdateEntityPos records the server-authoritative position of an entity.
// Called from the serverToClient goroutine for AddPlayer, AddActor,
// MovePlayer, and MoveActorAbsolute packets.
func (m *Manager) UpdateEntityPos(playerID uuid.UUID, entityRID uint64, pos mgl32.Vec3) {
	if p := m.getPlayer(playerID); p != nil {
		p.UpdateEntityPos(entityRID, pos)
	}
}

// MapEntityUID stores a uniqueID→runtimeID mapping for a non-player actor
// so it can be cleaned up when RemoveActor is received.
func (m *Manager) MapEntityUID(playerID uuid.UUID, uid int64, rid uint64) {
	if p := m.getPlayer(playerID); p != nil {
		p.MapEntityUID(uid, rid)
	}
}

// RemoveEntity removes an entity from a player's position table.
// Called when the server sends a RemoveActor packet.
func (m *Manager) RemoveEntity(playerID uuid.UUID, uid int64) {
	if p := m.getPlayer(playerID); p != nil {
		p.RemoveEntity(uid)
	}
}

// OnInput is called for every PlayerAuthInput packet.
// inputMode is PlayerAuthInput.InputMode (1=Mouse, 2=Touch, 3=GamePad).
// inputData is the InputData bitset from the packet, used by BadPacket/E to
// detect contradictory start+stop flag pairs.
func (m *Manager) OnInput(id uuid.UUID, tick uint64, pos mgl32.Vec3, onGround bool, yaw, pitch float32, inputMode uint32, inputData InputDataLoader) {
	p := m.getPlayer(id)
	det := m.getDet(id)
	if p == nil || det == nil {
		return
	}

	// Record arrival time for Timer/A before any state updates.
	p.RecordInputTime()

	// BadPacket/D: NaN/Infinity position — run before UpdatePosition so a
	// poison packet cannot corrupt the player's position state.
	if flagged, info := m.badPacketD.Check(pos); flagged {
		if det[keyBadPacketD].Fail(int64(tick)) {
			m.handleViolation(p, m.badPacketD, det[keyBadPacketD], info)
		}
		// Do not process further: updating state with NaN/Inf would corrupt checks.
		return
	}

	// BadPacket/A (tick validation) runs before UpdateTick so it can compare
	// the incoming tick against the previously stored simulation frame.
	if flagged, info := m.badPacket.Check(p, tick); flagged {
		if det[keyBadPacket].Fail(int64(tick)) {
			m.handleViolation(p, m.badPacket, det[keyBadPacket], info)
		}
	}

	p.UpdateTick(tick)
	p.UpdateRotation(yaw, pitch)
	p.UpdatePosition(pos, onGround)
	p.SetInputMode(inputMode)

	// BadPacket/B: pitch range validation.
	if flagged, info := m.badPacketB.Check(p, pitch); flagged {
		if det[keyBadPacketB].Fail(int64(tick)) {
			m.handleViolation(p, m.badPacketB, det[keyBadPacketB], info)
		}
	}

	// BadPacket/C: simultaneous Sprint+Sneak — impossible in vanilla.
	// Input flags are set by SetInputFlags() (called before OnInput from proxy)
	// so InputSnapshot() already reflects the current tick's flags.
	if flagged, info := m.badPacketC.Check(p); flagged {
		if det[keyBadPacketC].Fail(int64(tick)) {
			m.handleViolation(p, m.badPacketC, det[keyBadPacketC], info)
		}
	}

	// BadPacket/E: contradictory start+stop input flags in same tick.
	if flagged, info := m.badPacketE.Check(inputData); flagged {
		if det[keyBadPacketE].Fail(int64(tick)) {
			m.handleViolation(p, m.badPacketE, det[keyBadPacketE], info)
		}
	}

	// Timer/A: high-rate packet detection.
	if flagged, info := m.timer.Check(p); flagged {
		if det[keyTimer].Fail(int64(tick)) {
			m.handleViolation(p, m.timer, det[keyTimer], info)
		}
	} else {
		det[keyTimer].Pass(0.05)
	}

	// Consume teleport grace: if the client just handled a server teleport,
	// skip position-based movement checks for this tick to avoid a false-positive
	// velocity spike from the position discontinuity.
	teleportGrace := p.ConsumeTeleportGrace()

	// Velocity/A (Anti-KB): consume any pending knockback snapshot recorded when
	// the server sent SetActorMotion / MotionPredictionHints and check whether the
	// player's current positional delta reflects the applied impulse. This runs
	// after UpdatePosition so that p.PositionDelta() reflects the current tick.
	// We only run the check when there is no active knockback grace window; the
	// first tick after the grace expires is when Anti-KB cheats become visible
	// because the player should still be carrying residual horizontal velocity.
	if !p.HasKnockbackGrace() {
		if kb := p.KnockbackSnapshot(); kb.Len() >= movement.VelocityAMinKB {
			if flagged, info := m.velocity.Check(p, kb); flagged {
				if det[keyVelocity].Fail(int64(tick)) {
					m.handleViolation(p, m.velocity, det[keyVelocity], info)
				}
			} else {
				det[keyVelocity].Pass(1.0)
			}
		}
	}

	// Speed/A — skip if teleporting or in creative mode.
	if !teleportGrace {
		if flagged, info := m.speed.Check(p); flagged {
			if det[keySpeed].Fail(int64(tick)) {
				m.handleViolation(p, m.speed, det[keySpeed], info)
			}
		} else {
			det[keySpeed].Pass(0.05)
		}

		// Speed/B — aerial horizontal speed.
		if flagged, info := m.speedB.Check(p); flagged {
			if det[keySpeedB].Fail(int64(tick)) {
				m.handleViolation(p, m.speedB, det[keySpeedB], info)
			}
		} else {
			det[keySpeedB].Pass(0.05)
		}

		// Phase/A — impossible position delta without teleport.
		if flagged, info := m.phase.Check(p, false); flagged {
			if det[keyPhase].Fail(int64(tick)) {
				m.handleViolation(p, m.phase, det[keyPhase], info)
			}
		}
	}

	// Fly/A — creative and water exemptions are handled inside Check.
	if flagged, info := m.fly.Check(p); flagged {
		if det[keyFly].Fail(int64(tick)) {
			m.handleViolation(p, m.fly, det[keyFly], info)
		}
	} else {
		det[keyFly].Pass(0.5)
	}

	// Fly/B — gravity bypass detection (float / anti-gravity cheats).
	// Runs after Fly/A so that Fly/A handles the obvious hover/upward-fly cases
	// and Fly/B focuses on the subtler slow-fall-without-effect signature.
	if flagged, info := m.flyB.Check(p); flagged {
		if det[keyFlyB].Fail(int64(tick)) {
			m.handleViolation(p, m.flyB, det[keyFlyB], info)
		}
	} else {
		det[keyFlyB].Pass(0.3)
	}

	// NoFall/A
	if flagged, info := m.noFall.Check(p); flagged {
		if det[keyNoFall].Fail(int64(tick)) {
			m.handleViolation(p, m.noFall, det[keyNoFall], info)
		}
	} else {
		det[keyNoFall].Pass(0.5)
	}

	// NoFall/B: OnGround spoof detection.
	if !teleportGrace {
		if flagged, info := m.noFallB.Check(p); flagged {
			if det[keyNoFallB].Fail(int64(tick)) {
				m.handleViolation(p, m.noFallB, det[keyNoFallB], info)
			}
		} else {
			det[keyNoFallB].Pass(0.2)
		}
	}

	// NoSlow/A — item-use speed bypass (eating, bow, shield).
	if flagged, info := m.noSlow.Check(p); flagged {
		if det[keyNoSlow].Fail(int64(tick)) {
			m.handleViolation(p, m.noSlow, det[keyNoSlow], info)
		}
	} else {
		det[keyNoSlow].Pass(0.2)
	}

	// Aim/A
	if flagged, info, passAmount := m.aim.Check(p); flagged {
		if det[keyAim].Fail(int64(tick)) {
			m.handleViolation(p, m.aim, det[keyAim], info)
		}
	} else if passAmount > 0 {
		det[keyAim].Pass(passAmount)
	}

	// Aim/B — constant pitch during yaw rotation (mouse clients only).
	if flagged, info := m.aimB.Check(p); flagged {
		if det[keyAimB].Fail(int64(tick)) {
			m.handleViolation(p, m.aimB, det[keyAimB], info)
		}
	} else {
		det[keyAimB].Pass(0.2)
	}
}

// OnMove is called for legacy MovePlayer packets (non-authoritative clients).
func (m *Manager) OnMove(id uuid.UUID, pos mgl32.Vec3, onGround bool) {
	p := m.getPlayer(id)
	det := m.getDet(id)
	if p == nil || det == nil {
		return
	}

	p.UpdatePosition(pos, onGround)
	tick := int64(p.SimFrame())
	teleportGrace := p.ConsumeTeleportGrace()

	if !teleportGrace {
		if flagged, info := m.speed.Check(p); flagged {
			if det[keySpeed].Fail(tick) {
				m.handleViolation(p, m.speed, det[keySpeed], info)
			}
		} else {
			det[keySpeed].Pass(0.05)
		}

		if flagged, info := m.speedB.Check(p); flagged {
			if det[keySpeedB].Fail(tick) {
				m.handleViolation(p, m.speedB, det[keySpeedB], info)
			}
		} else {
			det[keySpeedB].Pass(0.05)
		}

		if flagged, info := m.phase.Check(p, false); flagged {
			if det[keyPhase].Fail(tick) {
				m.handleViolation(p, m.phase, det[keyPhase], info)
			}
		}
	}

	if flagged, info := m.fly.Check(p); flagged {
		if det[keyFly].Fail(tick) {
			m.handleViolation(p, m.fly, det[keyFly], info)
		}
	} else {
		det[keyFly].Pass(0.5)
	}

	if flagged, info := m.noFall.Check(p); flagged {
		if det[keyNoFall].Fail(tick) {
			m.handleViolation(p, m.noFall, det[keyNoFall], info)
		}
	} else {
		det[keyNoFall].Pass(0.5)
	}
}

// OnAttack is called when a player sends a UseItemOnEntity attack transaction.
// targetRID is the entity runtime ID of the attacked entity, used to look up
// the server-authoritative position from the entity position table.
// This replaces the broken ClickedPosition-based approach.
func (m *Manager) OnAttack(attackerID, targetID uuid.UUID, targetRID uint64) {
	p := m.getPlayer(attackerID)
	det := m.getDet(attackerID)
	if p == nil || det == nil {
		return
	}

	tick := int64(p.SimFrame())

	p.RecordClick()
	p.RecordAttack(targetID)

	// Reach/A: only run when we have a server-authoritative entity position.
	// If the entity is not in the table (e.g. the first attack before any
	// server position has been received), skip to avoid false positives.
	if targetPos, ok := p.EntityPos(targetRID); ok {
		if flagged, info := m.reach.Check(p, targetPos); flagged {
			if det[keyReach].Fail(tick) {
				m.handleViolation(p, m.reach, det[keyReach], info)
			}
		} else {
			det[keyReach].Pass(0.0015)
		}

		// KillAura/B: flag if the target is outside the player's FOV.
		if flagged, info := m.killAuraB.Check(p, targetPos); flagged {
			if det[keyKillAuraB].Fail(tick) {
				m.handleViolation(p, m.killAuraB, det[keyKillAuraB], info)
			}
		} else {
			det[keyKillAuraB].Pass(1.0)
		}
	}

	// KillAura/A
	if flagged, info := m.killAura.Check(p); flagged {
		if det[keyKillAura].Fail(tick) {
			m.handleViolation(p, m.killAura, det[keyKillAura], info)
		}
	} else {
		det[keyKillAura].Pass(1.0)
	}

	// KillAura/C: multi-target per-tick.
	// RecordAttack (called above) already updated the per-tick attack count.
	if flagged, info := m.killAuraC.Check(p); flagged {
		if det[keyKillAuraC].Fail(tick) {
			m.handleViolation(p, m.killAuraC, det[keyKillAuraC], info)
		}
	}

	// AutoClicker/A: CPS limit.
	// Pass() is called when CPS is within the allowed limit so that the buffer
	// decays during legitimate play, preventing false positives from brief
	// bursts that occurred before the player settled back to a normal rate.
	if flagged, info := m.autoClicker.Check(p); flagged {
		if det[keyAutoClicker].Fail(tick) {
			m.handleViolation(p, m.autoClicker, det[keyAutoClicker], info)
		}
	} else {
		det[keyAutoClicker].Pass(0.5)
	}

	// AutoClicker/B: click interval regularity.
	// Autoclickers produce suspiciously uniform inter-click intervals (std dev
	// close to 0 ms). Human clicks have naturally high variance (> ~15 ms).
	if flagged, info := m.autoClickerB.Check(p); flagged {
		if det[keyAutoClickerB].Fail(tick) {
			m.handleViolation(p, m.autoClickerB, det[keyAutoClickerB], info)
		}
	} else {
		det[keyAutoClickerB].Pass(0.3)
	}
}

// OnSwing records an arm-swing event.
func (m *Manager) OnSwing(id uuid.UUID) {
	if p := m.getPlayer(id); p != nil {
		p.RecordSwing()
	}
}

// OnBlockPlace is called when a player sends a UseItem (ClickBlock) inventory
// transaction. blockPos is the position of the base block being clicked (the
// block whose face was clicked to place a new block on), and face is the face
// index (0–5, matching Bedrock's BlockFace constants). The proxy extracts both
// values from UseItemTransactionData before calling here.
func (m *Manager) OnBlockPlace(id uuid.UUID, blockPos mgl32.Vec3, face int32) {
	p := m.getPlayer(id)
	det := m.getDet(id)
	if p == nil || det == nil {
		return
	}
	tick := int64(p.SimFrame())
	if flagged, info := m.scaffold.Check(p, blockPos, face); flagged {
		if det[keyScaffold].Fail(tick) {
			m.handleViolation(p, m.scaffold, det[keyScaffold], info)
		}
	} else {
		det[keyScaffold].Pass(0.3)
	}
}

// handleViolation logs the violation with its detection context and routes the
// enforcement decision through a mitigate.Dispatcher built from the Manager's
// current hook configuration. The dispatcher itself emits a structured
// "violation" record with the policy name — we additionally log the extraInfo
// string here so check-specific diagnostics survive alongside the routing log.
//
// The dispatcher is constructed per call because it is a tiny struct (four
// fields + a logger reference) and the caller may mutate m.KickFunc /
// m.RubberbandFunc / m.ServerFilterFunc at runtime. Rebuilding avoids any
// lock or staleness concern; the allocation cost is negligible next to the
// logging the dispatcher performs.
//
// The packet argument to Apply is nil because Manager-scope checks do not
// have the offending packet in hand — ServerFilter / Rubberband needing the
// packet are wired at the proxy layer in Phase 5a.2 where the packet IS in
// scope. Nil hooks degrade to log-only inside the dispatcher, so nothing
// panics when a check with PolicyServerFilter fires through here.
func (m *Manager) handleViolation(p *data.Player, d Detection, md *DetectionMetadata, extraInfo string) {
	// Per-check diagnostic line (dispatcher logs a structured "violation" too).
	m.log.Warn("violation detected",
		"player", p.Username,
		"uuid", p.UUID,
		"check", d.Type()+"/"+d.SubType(),
		"violations", fmt.Sprintf("%.2f", md.Violations),
		"max", md.MaxViolations,
		"info", extraInfo,
	)

	// Adapt uuid-typed KickFunc into the dispatcher's string-typed contract.
	// Nil stays nil so the dispatcher knows to dry-run the kick path.
	var kick mitigate.KickFunc
	if m.KickFunc != nil {
		uid := p.UUID
		kickCb := m.KickFunc
		kick = func(_ string, reason string) { kickCb(uid, reason) }
	}

	disp := mitigate.NewDispatcherWithHooks(m.log, kick, m.RubberbandFunc, m.ServerFilterFunc)
	disp.Apply(p.UUID.String(), d, md, nil)
}

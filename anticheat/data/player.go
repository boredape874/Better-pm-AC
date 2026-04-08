package data

import (
	"math"
	"sync"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

// cpsWindow is the rolling time-window used to count clicks per second.
const cpsWindow = time.Second

// hoverDeltaThreshold is the maximum absolute Y displacement (blocks/tick)
// that is still considered "hovering". Values smaller than this while airborne
// for an extended period indicate a Fly cheat.
const hoverDeltaThreshold = float32(0.005)

// waterExitGraceTicks is the number of ticks after a water→ground transition
// during which NoFall/A is suppressed. Covers the case where a player exits
// a body of water and lands on ground within half a second (~10 ticks at 20 TPS).
const waterExitGraceTicks = 10

// knockbackGraceTicks is the number of ticks after the server sends a
// SetActorMotion or MotionPredictionHints packet targeting the player (i.e.
// the server applies knockback, an explosion, a wind charge, etc.) during which
// Speed/A and Speed/B do not flag. Knockback can legitimately produce a
// horizontal velocity spike for several ticks that would otherwise be flagged.
const knockbackGraceTicks = 6

// noFallBSpeedThreshold is the minimum downward Y displacement (blocks/tick)
// that triggers GroundFallTicks accumulation. If a player claims OnGround=true
// while their Y position delta exceeds this threshold downward, it is counted
// as a potential OnGround-spoof tick. The value is conservative (0.3 b/tick)
// to avoid counting legitimate landing frames where some downward velocity
// remains before the collision resolves.
const noFallBSpeedThreshold = float32(0.3)

// Player stores per-session state used by all anti-cheat checks.
type Player struct {
	mu sync.RWMutex

	// Identity
	UUID     uuid.UUID
	Username string

	// Simulation tick (PlayerAuthInput.Tick)
	SimulationFrame     uint64
	LastSimulationFrame uint64

	// Position / velocity
	// Velocity is the raw per-packet position delta (blocks/tick for
	// PlayerAuthInput). Unlike the previous wall-clock-based calculation,
	// this is not affected by network jitter and matches how Oomph tracks
	// movement deltas.
	Position     mgl32.Vec3
	LastPosition mgl32.Vec3
	OnGround     bool
	LastOnGround bool
	Velocity     mgl32.Vec3
	LastVelocity mgl32.Vec3

	// Airborne state counters (used by Fly/A)
	// AirTicks counts consecutive packets where the player is airborne.
	// Mirroring Oomph's grace-period approach: we skip the fly check for
	// the first N airborne ticks to cover the natural jump arc.
	AirTicks int
	// LastAirTicks holds the AirTicks value captured just before the last
	// on-ground reset. Used by NoFall/B to know how long the player was
	// airborne before they claimed to land.
	LastAirTicks int
	// HoverTicks counts consecutive airborne packets where |dy| < hoverDeltaThreshold.
	HoverTicks int
	// GroundFallTicks counts consecutive ticks where the player claims to be
	// on the ground (OnGround=true) while their Y position delta is negative
	// beyond the NoFall/B detection threshold. Used to detect clients that
	// spoof OnGround=true on every packet while continuously falling.
	GroundFallTicks int

	// Fall tracking
	FallDistance     float32
	LastFallDistance float32 // captured before landing reset so NoFall/A can read it
	FallStartY       float32
	fallTracking     bool // true once the fall start Y has been established

	// Rotation
	Rotation      mgl32.Vec2
	LastRotation  mgl32.Vec2
	RotationDelta mgl32.Vec2

	// InputMode mirrors PlayerAuthInput.InputMode.
	// 1 = Mouse, 2 = Touch, 3 = GamePad.
	// Aim/A is only applicable to mouse clients (Oomph: if InputMode != Mouse { return }).
	InputMode uint32

	// GameMode is the player's current game type, updated from SetPlayerGameType
	// server packets.  1 = Creative.  Fly/A and Speed/A exempt creative players.
	GameMode int32

	// TeleportGrace is set to true when the client reports InputFlagHandledTeleport,
	// indicating it has just processed a server-sent teleport.  The flag is
	// consumed (reset to false) by ConsumeTeleportGrace at the start of each
	// OnInput processing cycle so that Speed/A skips exactly one tick.
	TeleportGrace bool

	// Input state flags derived from PlayerAuthInput.InputData each tick.
	// Sprinting and Sneaking affect the expected horizontal speed limits.
	// InWater indicates the player is swimming / auto-jumping in water, which
	// exempts them from NoFall checks (water absorbs fall damage).
	// Gliding is true while the player is wearing and using an elytra (set from
	// InputFlagStartGliding / InputFlagStopGliding events). Fly/A exempts
	// gliding players because elytras legitimately sustain horizontal flight.
	Sprinting bool
	Sneaking  bool
	InWater   bool
	Gliding   bool

	// waterExitGrace counts down (ticks) after the player transitions from
	// in-water to out-of-water. While positive, NoFall/A is suppressed so that
	// a player who exits a body of water and immediately lands on the ground is
	// not falsely flagged (the fall distance counter accumulated while in the
	// water is not meaningful for damage purposes).
	waterExitGrace int

	// knockbackGrace counts down (ticks) after the server sends SetActorMotion
	// or MotionPredictionHints for the player (knockback, explosions, etc.).
	// Speed/A and Speed/B skip their checks while this is positive because the
	// server-applied velocity can legitimately exceed normal sprint speed.
	knockbackGrace int

	// Combat
	LastSwingTick    uint64
	ClickTimestamps  []time.Time
	LastAttackTime   time.Time
	LastAttackTarget uuid.UUID
	// LastAttackTick is the SimulationFrame of the most recent attack event.
	// LastAttackCount is the number of distinct entities attacked in that tick.
	// KillAura/C uses these to detect bots that hit multiple targets in one tick.
	LastAttackTick  uint64
	LastAttackCount int
	// ConstPitchTicks counts consecutive ticks where the pitch delta is exactly
	// zero while the yaw delta is non-trivial (the player is turning but not
	// adjusting pitch). Aim/B uses this to flag aimbot software that rotates
	// the yaw to track targets but keeps the pitch perfectly locked.
	ConstPitchTicks int

	// inputTimestamps records the wall-clock arrival time of each
	// PlayerAuthInput packet within a rolling one-second window.
	// Timer/A uses this to detect faster-than-normal packet rates.
	inputTimestamps []time.Time

	// Entity position table (Reach/A)
	// entityPos maps server-assigned entity runtime IDs to their last known
	// world positions, populated from AddPlayer / AddActor / MovePlayer /
	// MoveActorAbsolute packets forwarded by serverToClient.
	// This replaces the broken ClickedPosition-based approach: ClickedPosition
	// is a hitbox-relative click offset sent by the client and can be spoofed.
	entityPosMu sync.RWMutex
	entityPos   map[uint64]mgl32.Vec3
	// uniqueToRID maps EntityUniqueID (from AddActor / RemoveActor) to the
	// corresponding EntityRuntimeID for entity removal bookkeeping.
	uniqueToRID map[int64]uint64

	// activeEffects maps effect type ID (e.g. packet.EffectSpeed = 1) to
	// the amplifier value (0-based: Speed I = 0, Speed II = 1).
	// Populated from server-sent MobEffect packets. Only effects targeting
	// the player's own entity are recorded.
	activeEffects map[int32]int32

	// posInitialised is false until the first UpdatePosition call has been
	// processed. The initial Position is the zero vector, so the first
	// velocity computation would produce a teleport-sized spike equal to the
	// player's spawn coordinates; we skip it to avoid Speed/A false positives
	// on join (mirrors Oomph's exempt-on-spawn behaviour).
	posInitialised bool

	// latency is the round-trip time between the client and this proxy,
	// measured by gophertunnel and updated each PlayerAuthInput packet.
	// Reach/A and KillAura/A use it to apply lag compensation so that
	// high-latency players are not falsely flagged.
	latency time.Duration

	Violations map[string]int
}

// NewPlayer creates a fresh Player for the given UUID and username.
func NewPlayer(id uuid.UUID, username string) *Player {
	return &Player{
		UUID:          id,
		Username:      username,
		Violations:    make(map[string]int),
		entityPos:     make(map[uint64]mgl32.Vec3),
		uniqueToRID:   make(map[int64]uint64),
		activeEffects: make(map[int32]int32),
	}
}

// UpdateTick records the latest simulation frame from PlayerAuthInput.Tick.
func (p *Player) UpdateTick(tick uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.LastSimulationFrame = p.SimulationFrame
	p.SimulationFrame = tick
}

// SimFrame returns the current simulation frame (thread-safe).
func (p *Player) SimFrame() uint64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.SimulationFrame
}

// UpdateRotation records the latest [yaw, pitch] from PlayerAuthInput and
// computes RotationDelta as the absolute per-tick change.
func (p *Player) UpdateRotation(yaw, pitch float32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.LastRotation = p.Rotation
	p.Rotation = mgl32.Vec2{yaw, pitch}

	yawDelta := yaw - p.LastRotation[0]
	if yawDelta > 180 {
		yawDelta -= 360
	} else if yawDelta < -180 {
		yawDelta += 360
	}
	pitchDelta := pitch - p.LastRotation[1]
	p.RotationDelta = mgl32.Vec2{
		float32(math.Abs(float64(yawDelta))),
		float32(math.Abs(float64(pitchDelta))),
	}

	// Track ConstPitchTicks for Aim/B: count how many consecutive ticks the
	// player turns their yaw by more than 0.5° without adjusting pitch at all.
	// A perfectly locked pitch combined with active yaw rotation is a signature
	// of aimbot software that tracks targets horizontally.
	const minYawForConstPitchCheck = float32(0.5) // degrees
	if float32(math.Abs(float64(yawDelta))) > minYawForConstPitchCheck && pitchDelta == 0 {
		p.ConstPitchTicks++
	} else {
		p.ConstPitchTicks = 0
	}
}

// ConstPitchSnapshot returns the number of consecutive ticks in which the
// player moved their yaw without adjusting pitch. Used by Aim/B.
func (p *Player) ConstPitchSnapshot() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ConstPitchTicks
}

// RotationSnapshot returns the current rotation delta (yawDelta, pitchDelta)
// in absolute values, safe for use outside the lock.
func (p *Player) RotationSnapshot() (yawDelta, pitchDelta float32) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.RotationDelta[0], p.RotationDelta[1]
}

// RotationAbsolute returns the player's current absolute yaw and pitch values
// (i.e. the latest rotation from PlayerAuthInput, not deltas).
// Used by KillAura/B to compute the look-direction vector.
func (p *Player) RotationAbsolute() (yaw, pitch float32) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Rotation[0], p.Rotation[1]
}

// UpdatePosition records a new position and computes the per-packet velocity
// as a raw position delta (blocks/packet, equivalent to blocks/tick for
// PlayerAuthInput which arrives at 20 TPS).
// This replaces the previous wall-clock-based calculation which was sensitive
// to network jitter and could produce false Speed/Fly flags.
func (p *Player) UpdatePosition(pos mgl32.Vec3, onGround bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.LastPosition = p.Position
	p.LastOnGround = p.OnGround
	p.LastVelocity = p.Velocity

	if p.posInitialised {
		delta := pos.Sub(p.Position)
		p.Velocity = delta // blocks/tick (raw positional delta)
	} else {
		// First packet: no previous position to diff against; skip velocity
		// so checks do not see a teleport-sized spike from {0,0,0} to spawn.
		p.Velocity = mgl32.Vec3{}
		p.posInitialised = true
	}

	p.Position = pos
	p.OnGround = onGround

	// Decrement the water-exit grace counter each tick so that it expires
	// automatically after waterExitGraceTicks ticks regardless of landing.
	if p.waterExitGrace > 0 {
		p.waterExitGrace--
	}
	// Decrement the knockback grace counter each tick.
	if p.knockbackGrace > 0 {
		p.knockbackGrace--
	}

	if onGround {
		// Capture the fall distance BEFORE zeroing it so that NoFall/A can
		// still read it via NoFallSnapshot on this same tick.
		p.LastFallDistance = p.FallDistance
		// Capture AirTicks before resetting so NoFall/B can check how long
		// the player was airborne before they claimed to land.
		p.LastAirTicks = p.AirTicks
		// Reset all airborne counters on landing.
		p.AirTicks = 0
		p.HoverTicks = 0
		p.FallDistance = 0
		p.FallStartY = 0
		p.fallTracking = false

		// GroundFallTicks: accumulate when player claims ground while still
		// falling; reset when the Y delta is not significantly negative.
		dy := p.Velocity[1]
		if dy < -noFallBSpeedThreshold {
			p.GroundFallTicks++
		} else {
			p.GroundFallTicks = 0
		}
	} else {
		p.LastFallDistance = 0
		p.GroundFallTicks = 0 // player is genuinely airborne; reset the spoof counter
		p.AirTicks++
		dy := p.Velocity[1]
		if dy > -hoverDeltaThreshold && dy < hoverDeltaThreshold {
			p.HoverTicks++
		} else {
			p.HoverTicks = 0
		}
		// Fall distance tracking: use a bool flag instead of FallStartY == 0
		// so that players falling from Y≈0 are handled correctly.
		if pos[1] < p.LastPosition[1] {
			if !p.fallTracking {
				p.FallStartY = p.LastPosition[1]
				p.fallTracking = true
			}
			p.FallDistance = p.FallStartY - pos[1]
		}
	}
}

// PositionDelta returns the raw per-tick position delta (Velocity), which is
// the displacement from the previous position to the current one in blocks/tick.
// Used by Phase/A to detect impossible position jumps.
func (p *Player) PositionDelta() mgl32.Vec3 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Velocity
}

// HorizontalSpeed returns the horizontal speed as blocks/tick (XZ plane).
// Because Velocity is now a raw positional delta, this equals the XZ distance
// moved in one packet/tick — no wall-clock conversion needed.
func (p *Player) HorizontalSpeed() float32 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return mgl32.Vec2{p.Velocity[0], p.Velocity[2]}.Len()
}

// NoFallSnapshot returns whether the player just landed and the fall distance
// recorded before the landing reset was applied.
func (p *Player) NoFallSnapshot() (justLanded bool, fallDistance float32) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.OnGround && !p.LastOnGround, p.LastFallDistance
}

// FlySnapshot returns the data needed by Fly/A:
//   - airborne: true if the player is not on the ground
//   - yDeltaPerTick: Y component of the positional delta (blocks/tick)
//   - airTicks: consecutive airborne ticks since last landing
//   - hoverTicks: consecutive ticks with near-zero Y delta while airborne
func (p *Player) FlySnapshot() (airborne bool, yDeltaPerTick float32, airTicks, hoverTicks int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return !p.OnGround, p.Velocity[1], p.AirTicks, p.HoverTicks
}

// GroundFallSnapshot returns data needed by NoFall/B to detect OnGround spoofing:
//   - groundFallTicks: consecutive ticks the player claimed OnGround while
//     their Y delta was below -noFallBSpeedThreshold (potential spoof counter)
//   - yDelta: current Y positional delta (blocks/tick)
//   - onGround: whether the client currently claims to be on the ground
func (p *Player) GroundFallSnapshot() (groundFallTicks int, yDelta float32, onGround bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.GroundFallTicks, p.Velocity[1], p.OnGround
}

// CurrentPosition returns the player's current position (thread-safe).
func (p *Player) CurrentPosition() mgl32.Vec3 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Position
}

// SetInputMode stores the latest InputMode from PlayerAuthInput.
func (p *Player) SetInputMode(mode uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.InputMode = mode
}

// SetGameMode records the player's current game type, received from the server
// via SetPlayerGameType.  A value of 1 (GameTypeCreative) causes Fly/A and
// Speed/A to exempt the player from detection.
func (p *Player) SetGameMode(mode int32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.GameMode = mode
}

// IsCreative returns true when the player is in Creative mode (GameType == 1).
func (p *Player) IsCreative() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.GameMode == 1
}

// SetTeleportGrace marks that the client has just handled a server teleport.
// Speed/A will skip the next OnInput tick to avoid a false-positive spike.
func (p *Player) SetTeleportGrace() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.TeleportGrace = true
}

// ConsumeTeleportGrace returns whether a teleport grace is pending and resets
// the flag.  Called once at the start of each OnInput processing cycle.
func (p *Player) ConsumeTeleportGrace() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	v := p.TeleportGrace
	p.TeleportGrace = false
	return v
}

// RecordInputTime appends the current wall-clock time to the rolling
// inputTimestamps list and prunes entries older than one second.
// Called once per PlayerAuthInput packet arrival for Timer/A.
func (p *Player) RecordInputTime() {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-cpsWindow)
	start := 0
	for start < len(p.inputTimestamps) && p.inputTimestamps[start].Before(cutoff) {
		start++
	}
	p.inputTimestamps = append(p.inputTimestamps[start:], now)
}

// InputRate returns the number of PlayerAuthInput packets recorded in the last
// second, used by Timer/A to detect a higher-than-normal packet rate.
func (p *Player) InputRate() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	cutoff := time.Now().Add(-cpsWindow)
	count := 0
	for _, t := range p.inputTimestamps {
		if !t.Before(cutoff) {
			count++
		}
	}
	return count
}

// SetInputFlags stores per-tick boolean state flags derived from
// PlayerAuthInput.InputData. These are read by movement checks to apply
// appropriate speed limits and exemptions.
// It also detects the water→not-water transition and sets a grace window so
// that NoFall/A does not flag players who exit a body of water and land
// within a few ticks (the accumulated fall-distance counter is not meaningful
// for damage purposes in that scenario).
func (p *Player) SetInputFlags(sprinting, sneaking, inWater bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Detect water→not-water transition.
	if p.InWater && !inWater {
		p.waterExitGrace = waterExitGraceTicks
	}
	p.Sprinting = sprinting
	p.Sneaking = sneaking
	p.InWater = inWater
}

// InputSnapshot returns the current input state flags in a single lock
// acquisition so checks can read them consistently.
func (p *Player) InputSnapshot() (sprinting, sneaking, inWater bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Sprinting, p.Sneaking, p.InWater
}

// IsOnGround returns whether the player is currently on the ground (thread-safe).
func (p *Player) IsOnGround() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.OnGround
}

// IsGliding returns whether the player is currently gliding with an elytra.
// Gliding state is managed by StartGliding() / StopGliding() which are called
// from the proxy layer when InputFlagStartGliding / InputFlagStopGliding are
// observed in PlayerAuthInput.InputData.
func (p *Player) IsGliding() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Gliding
}

// StartGliding marks the player as gliding (elytra activated).
// Called from the proxy clientToServer handler when InputFlagStartGliding is set.
func (p *Player) StartGliding() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Gliding = true
}

// StopGliding marks the player as no longer gliding.
// Called from the proxy clientToServer handler when InputFlagStopGliding is set.
func (p *Player) StopGliding() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Gliding = false
}

// RecordKnockback sets the knockback grace window. Called when the server
// sends a SetActorMotion or MotionPredictionHints packet targeting the player's
// own entity runtime ID, indicating the server has applied an external velocity
// (knockback from damage, explosion, wind charge, etc.).
// Speed/A and Speed/B will not flag for knockbackGraceTicks ticks after this.
func (p *Player) RecordKnockback() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.knockbackGrace = knockbackGraceTicks
}

// HasKnockbackGrace returns true while the knockback grace window is active.
// Speed/A and Speed/B skip their checks during this window to avoid false
// positives from server-applied velocity spikes.
func (p *Player) HasKnockbackGrace() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.knockbackGrace > 0
}

// SetLatency stores the latest measured round-trip time between the client and
// the proxy. Called once per PlayerAuthInput packet from proxy.clientToServer
// using conn.Latency().
func (p *Player) SetLatency(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.latency = d
}

// Latency returns the last measured client-to-proxy round-trip time.
// Reach/A and KillAura/A use this for lag compensation.
func (p *Player) Latency() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.latency
}

// HasRecentWaterExit returns true when the player has left water within the
// last waterExitGraceTicks ticks. NoFall/A uses this to suppress false positives
// from fall-distance that was accumulated while swimming.
func (p *Player) HasRecentWaterExit() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.waterExitGrace > 0
}

// GetInputMode returns the latest InputMode (thread-safe).
func (p *Player) GetInputMode() uint32 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.InputMode
}

// RecordSwing updates LastSwingTick to the current SimulationFrame.
func (p *Player) RecordSwing() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.LastSwingTick = p.SimulationFrame
}

// SwingTick returns the simulation frame of the last recorded arm swing.
func (p *Player) SwingTick() uint64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.LastSwingTick
}

// RecordClick appends the current time to the rolling click-timestamp list and
// prunes entries older than cpsWindow.
func (p *Player) RecordClick() {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-cpsWindow)
	start := 0
	for start < len(p.ClickTimestamps) && p.ClickTimestamps[start].Before(cutoff) {
		start++
	}
	p.ClickTimestamps = append(p.ClickTimestamps[start:], now)
}

// CPS returns the number of clicks recorded in the last second.
func (p *Player) CPS() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	cutoff := time.Now().Add(-cpsWindow)
	count := 0
	for _, t := range p.ClickTimestamps {
		if !t.Before(cutoff) {
			count++
		}
	}
	return count
}

// ClickIntervalStdDev computes the standard deviation of the inter-click
// interval times (in milliseconds) over the rolling one-second click window.
// It returns (stdDev, n) where n is the number of intervals measured.
// Returns (0, 0) when fewer than 3 timestamps are available.
// Used by AutoClicker/B to detect unnaturally regular clicking patterns.
func (p *Player) ClickIntervalStdDev() (stdDevMs float64, n int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.ClickTimestamps) < 3 {
		return 0, 0
	}
	intervals := make([]float64, len(p.ClickTimestamps)-1)
	for i := 1; i < len(p.ClickTimestamps); i++ {
		intervals[i-1] = float64(p.ClickTimestamps[i].Sub(p.ClickTimestamps[i-1]).Milliseconds())
	}
	mean := 0.0
	for _, v := range intervals {
		mean += v
	}
	mean /= float64(len(intervals))
	variance := 0.0
	for _, v := range intervals {
		d := v - mean
		variance += d * d
	}
	variance /= float64(len(intervals))
	return math.Sqrt(variance), len(intervals)
}

// LastAttackInfo returns the time and target UUID of the most recent attack.
func (p *Player) LastAttackInfo() (time.Time, uuid.UUID) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.LastAttackTime, p.LastAttackTarget
}

// RecordAttack records the time and target of the most recent attack.
// It also tracks the per-tick attack count for KillAura/C.
func (p *Player) RecordAttack(target uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.LastAttackTime = time.Now()
	p.LastAttackTarget = target
	currentTick := p.SimulationFrame
	if currentTick == p.LastAttackTick {
		p.LastAttackCount++
	} else {
		p.LastAttackTick = currentTick
		p.LastAttackCount = 1
	}
}

// AttackTickCount returns the current SimulationFrame and the number of distinct
// attack events recorded in that tick. Used by KillAura/C.
func (p *Player) AttackTickCount() (tick uint64, count int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.LastAttackTick, p.LastAttackCount
}

// UpdateEntityPos records the latest world position for an entity runtime ID.
// Called from the serverToClient goroutine when MovePlayer, MoveActorAbsolute,
// AddPlayer, or AddActor packets are received.
func (p *Player) UpdateEntityPos(rid uint64, pos mgl32.Vec3) {
	p.entityPosMu.Lock()
	defer p.entityPosMu.Unlock()
	p.entityPos[rid] = pos
}

// MapEntityUID stores the uniqueID→runtimeID mapping so RemoveEntity can clean
// up the position table when RemoveActor is received.
func (p *Player) MapEntityUID(uid int64, rid uint64) {
	p.entityPosMu.Lock()
	defer p.entityPosMu.Unlock()
	p.uniqueToRID[uid] = rid
}

// RemoveEntity removes an entity from the position table using its unique ID.
func (p *Player) RemoveEntity(uid int64) {
	p.entityPosMu.Lock()
	defer p.entityPosMu.Unlock()
	if rid, ok := p.uniqueToRID[uid]; ok {
		delete(p.entityPos, rid)
		delete(p.uniqueToRID, uid)
	}
}

// EntityPos returns the last known world position for the given entity runtime
// ID, and false if the entity is not in the table.
func (p *Player) EntityPos(rid uint64) (mgl32.Vec3, bool) {
	p.entityPosMu.RLock()
	defer p.entityPosMu.RUnlock()
	pos, ok := p.entityPos[rid]
	return pos, ok
}

// AddViolation increments the legacy violation counter and returns the new total.
func (p *Player) AddViolation(checkName string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Violations[checkName]++
	return p.Violations[checkName]
}

// ResetViolations resets the legacy counter for a specific check.
func (p *Player) ResetViolations(checkName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Violations[checkName] = 0
}

// ViolationCount returns the current legacy violation count for a check.
func (p *Player) ViolationCount(checkName string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Violations[checkName]
}

// AddEffect records an active potion effect (or modifies an existing one).
// effectType is the effect ID (e.g. packet.EffectSpeed = 1).
// amplifier is 0-based (0 = level I, 1 = level II, etc.).
// Called from the serverToClient goroutine when a MobEffect Add/Modify packet
// arrives for the player's own entity.
func (p *Player) AddEffect(effectType, amplifier int32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activeEffects[effectType] = amplifier
}

// RemoveEffect removes a potion effect from the active set.
// Called from the serverToClient goroutine when a MobEffect Remove packet
// arrives for the player's own entity.
func (p *Player) RemoveEffect(effectType int32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.activeEffects, effectType)
}

// EffectAmplifier returns the amplifier of the given effect type and whether
// the effect is currently active. effectType is the effect ID (e.g. 1 = Speed).
func (p *Player) EffectAmplifier(effectType int32) (amplifier int32, active bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	amp, ok := p.activeEffects[effectType]
	return amp, ok
}

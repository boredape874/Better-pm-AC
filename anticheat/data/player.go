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
	// HoverTicks counts consecutive airborne packets where |dy| < hoverDeltaThreshold.
	HoverTicks int

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
	Sprinting bool
	Sneaking  bool
	InWater   bool

	// Combat
	LastSwingTick    uint64
	ClickTimestamps  []time.Time
	LastAttackTime   time.Time
	LastAttackTarget uuid.UUID

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

	// posInitialised is false until the first UpdatePosition call has been
	// processed. The initial Position is the zero vector, so the first
	// velocity computation would produce a teleport-sized spike equal to the
	// player's spawn coordinates; we skip it to avoid Speed/A false positives
	// on join (mirrors Oomph's exempt-on-spawn behaviour).
	posInitialised bool
	Violations map[string]int
}

// NewPlayer creates a fresh Player for the given UUID and username.
func NewPlayer(id uuid.UUID, username string) *Player {
	return &Player{
		UUID:        id,
		Username:    username,
		Violations:  make(map[string]int),
		entityPos:   make(map[uint64]mgl32.Vec3),
		uniqueToRID: make(map[int64]uint64),
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
}

// RotationSnapshot returns the current rotation delta (yawDelta, pitchDelta)
// in absolute values, safe for use outside the lock.
func (p *Player) RotationSnapshot() (yawDelta, pitchDelta float32) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.RotationDelta[0], p.RotationDelta[1]
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

	if onGround {
		// Capture the fall distance BEFORE zeroing it so that NoFall/A can
		// still read it via NoFallSnapshot on this same tick.
		p.LastFallDistance = p.FallDistance
		// Reset all airborne counters on landing.
		p.AirTicks = 0
		p.HoverTicks = 0
		p.FallDistance = 0
		p.FallStartY = 0
		p.fallTracking = false
	} else {
		p.LastFallDistance = 0
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
func (p *Player) SetInputFlags(sprinting, sneaking, inWater bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
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

// LastAttackInfo returns the time and target UUID of the most recent attack.
func (p *Player) LastAttackInfo() (time.Time, uuid.UUID) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.LastAttackTime, p.LastAttackTarget
}

// RecordAttack records the time and target of the most recent attack.
func (p *Player) RecordAttack(target uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.LastAttackTime = time.Now()
	p.LastAttackTarget = target
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

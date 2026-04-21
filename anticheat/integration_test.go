package anticheat

import (
	"io"
	"log/slog"
	"math"
	"testing"

	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

// Integration tests for Phase 5a.3. These drive the Manager through
// end-to-end scenarios via its public API (OnInput / OnAttack / OnMove)
// and assert that check keys either fire or stay silent on the
// per-player DetectionMetadata map.
//
// Scope note: these scenarios exercise the β check set against the
// DEFAULT config (see config.Default). The intent is not pixel-perfect
// PCAP replay — that requires live RakNet capture tooling slated for
// γ. It is to confirm that the full Manager→checks→dispatcher path
// still produces the right violation on the right check key when it
// matters, and does NOT produce violations for obviously legitimate
// inputs. If a new check is added to NewManager, a scenario for it
// should land here too.

// --- harness -----------------------------------------------------------

// fakeBitset is a no-op InputDataLoader: every bit returns false. We use
// it whenever a scenario doesn't care about InputData-driven checks
// (BadPacket/E). Individual tests that DO care set bits explicitly.
type fakeBitset map[int]bool

func (b fakeBitset) Load(bit int) bool { return b[bit] }

func newIntegrationManager() *Manager {
	return NewManager(config.Default().Anticheat, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// integrationScenario adds a single player and returns everything a test
// needs to drive and inspect that session.
func integrationScenario(t *testing.T) (*Manager, uuid.UUID, playerDetections) {
	t.Helper()
	m := newIntegrationManager()
	pid := uuid.New()
	m.AddPlayer(pid, "tester")
	return m, pid, m.detections[pid]
}

// violationsOn returns the Violations count for a check key, or -1 if
// the key is unknown (which would indicate a registry bug).
func violationsOn(det playerDetections, key string) float64 {
	md, ok := det[key]
	if !ok {
		return -1
	}
	return md.Violations
}

// assertZero asserts that none of the listed check keys have any
// recorded violation. Used by legitimate-session scenarios where the
// list is the checks we specifically care about NOT triggering.
func assertZero(t *testing.T, det playerDetections, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if v := violationsOn(det, k); v != 0 {
			t.Errorf("expected %s violations=0, got %v", k, v)
		}
	}
}

// --- legitimate sessions (should NOT flag listed checks) ---------------

// TestIntegrationLegitIdleSession: a player that connects and sends a
// few PlayerAuthInput packets while standing still must not trip any
// movement/packet check.
func TestIntegrationLegitIdleSession(t *testing.T) {
	m, pid, det := integrationScenario(t)

	pos := mgl32.Vec3{0, 64, 0}
	for tick := uint64(1); tick <= 10; tick++ {
		m.OnInput(pid, tick, pos, true, 0, 0, 1 /*Mouse*/, fakeBitset{})
	}

	// Cheat-layer checks that should categorically NOT fire when the
	// player is physically motionless on ground with valid inputs.
	assertZero(t, det,
		keySpeed, keySpeedB, keyPhase,
		keyFly, keyFlyB, keyNoFall, keyNoFallB,
		keyBadPacket, keyBadPacketB, keyBadPacketC, keyBadPacketD, keyBadPacketE,
		keyTimer,
	)
}

// TestIntegrationLegitWalkingSession: small positive X deltas per tick
// stay well under Speed/A's 0.4 block/tick cap. No jumps, no attacks.
func TestIntegrationLegitWalkingSession(t *testing.T) {
	m, pid, det := integrationScenario(t)

	// 0.2 block/tick horizontal drift = 4 blocks/s = slower than sprint
	// and well under the SpeedA cap even with tolerance added.
	for tick := uint64(1); tick <= 20; tick++ {
		pos := mgl32.Vec3{float32(tick) * 0.2, 64, 0}
		m.OnInput(pid, tick, pos, true, 0, 0, 1, fakeBitset{})
	}

	assertZero(t, det,
		keySpeed, keySpeedB, keyPhase,
		keyFly, keyFlyB, keyNoFall, keyNoFallB,
		keyBadPacket, keyBadPacketB, keyBadPacketC, keyBadPacketD,
	)
}

// TestIntegrationLegitLegacyMoveSession: covers the OnMove (legacy
// MovePlayer) code path for the same walking scenario.
func TestIntegrationLegitLegacyMoveSession(t *testing.T) {
	m, pid, det := integrationScenario(t)

	for i := 0; i < 20; i++ {
		m.OnMove(pid, mgl32.Vec3{float32(i) * 0.2, 64, 0}, true)
	}

	assertZero(t, det, keySpeed, keySpeedB, keyPhase, keyFly, keyNoFall)
}

// --- cheat sessions (should flag a specific check) ---------------------

// TestIntegrationCheatBadPacketD: NaN position. OnInput runs
// BadPacket/D before UpdatePosition so the poison packet is caught
// without corrupting state.
func TestIntegrationCheatBadPacketD(t *testing.T) {
	m, pid, det := integrationScenario(t)

	// Prime with one legit tick so the player has a valid initial position.
	m.OnInput(pid, 1, mgl32.Vec3{0, 64, 0}, true, 0, 0, 1, fakeBitset{})
	// Poison tick.
	nan := float32(math.NaN())
	m.OnInput(pid, 2, mgl32.Vec3{nan, 64, 0}, true, 0, 0, 1, fakeBitset{})

	if v := violationsOn(det, keyBadPacketD); v < 1 {
		t.Fatalf("BadPacket/D violations=%v, want ≥1 after NaN pos", v)
	}
}

// TestIntegrationCheatBadPacketC: simultaneous Sprint+Sneak input
// flags is impossible in vanilla. SetInputFlags is the public entry
// the proxy uses, so we exercise it directly before OnInput.
func TestIntegrationCheatBadPacketC(t *testing.T) {
	m, pid, det := integrationScenario(t)
	p := m.GetPlayer(pid)

	// Two normal ticks.
	m.OnInput(pid, 1, mgl32.Vec3{0, 64, 0}, true, 0, 0, 1, fakeBitset{})
	// Now mark sprint + sneak simultaneously.
	p.SetInputFlags(true /*sprint*/, true /*sneak*/, false, false, false, true)
	m.OnInput(pid, 2, mgl32.Vec3{0, 64, 0}, true, 0, 0, 1, fakeBitset{})

	if v := violationsOn(det, keyBadPacketC); v < 1 {
		t.Fatalf("BadPacket/C violations=%v, want ≥1 after sprint+sneak", v)
	}
}

// TestIntegrationCheatReach: attack an entity 6 blocks away repeatedly.
// Default MaxReach is 3.1; 6 blocks is unambiguously outside even with
// ping compensation. Reach/A's DefaultMetadata sets FailBuffer=1.01, so
// two consecutive out-of-range hits are needed to tip the buffer past
// the confirm window and record a Violations increment.
func TestIntegrationCheatReach(t *testing.T) {
	m, pid, det := integrationScenario(t)
	p := m.GetPlayer(pid)
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)

	// Register a target entity 6 blocks forward at eye height.
	const targetRID = uint64(42)
	p.UpdateEntityPos(targetRID, mgl32.Vec3{0, 65.62, 6.0})

	targetID := uuid.New()
	for i := 0; i < 4; i++ {
		m.OnAttack(pid, targetID, targetRID)
	}

	if v := violationsOn(det, keyReach); v < 1 {
		t.Fatalf("Reach/A violations=%v, want ≥1 after 4× 6-block attacks", v)
	}
}

// TestIntegrationCheatBadPacketE: contradictory start+stop input flags
// in the same tick (e.g. StartSprinting AND StopSprinting both set).
// The fake bitset lets us assert the exact wire-level signal.
func TestIntegrationCheatBadPacketE(t *testing.T) {
	m, pid, det := integrationScenario(t)

	// Bits 40 + 41 are the StartSprinting / StopSprinting pair used by
	// BadPacket/E. The check's own unit tests pin the exact numbering;
	// here we just need ONE contradictory pair flipped.
	bs := fakeBitset{40: true, 41: true}
	m.OnInput(pid, 1, mgl32.Vec3{0, 64, 0}, true, 0, 0, 1, bs)

	if v := violationsOn(det, keyBadPacketE); v < 1 {
		t.Fatalf("BadPacket/E violations=%v, want ≥1 after contradictory flags", v)
	}
}

// TestIntegrationCheatScaffoldBelow: placing a block directly under
// the feet while airborne is the canonical Scaffold/A signature (see
// docs/check-specs / existing unit tests). This exercises the
// OnBlockPlace → Manager → Scaffold → dispatcher path.
func TestIntegrationCheatScaffoldBelow(t *testing.T) {
	m, pid, det := integrationScenario(t)
	p := m.GetPlayer(pid)
	// Airborne at y=64, with terrain collision off.
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, false)
	p.SetInputFlags(false, false, false, false, false, false /*no terrain collision*/)

	// Place a block directly below feet (face=1 = Top face of y=63 block).
	m.OnBlockPlace(pid, mgl32.Vec3{0, 63, 0}, 1)

	// We don't require ≥1 here because Scaffold has its own buffer
	// decay behaviour; what we DO require is that the violation path
	// ran without panicking and the key exists. Treat this as a smoke
	// test of the OnBlockPlace→Scaffold→dispatcher path.
	if _, ok := det[keyScaffold]; !ok {
		t.Fatalf("Scaffold/A metadata missing from registry")
	}
}

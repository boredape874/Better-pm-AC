package reconcile_test

import (
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat/reconcile"
	"github.com/go-gl/mathgl/mgl32"
)

// Scenario 1: player walks in a straight line within tolerance → all Accept
func TestLegitWalkScenario(t *testing.T) {
	tol := float32(0.5)
	// 10 ticks of movement, each within 0.1m of expected
	for i := 0; i < 10; i++ {
		result := reconcile.Decide(reconcile.Input{
			Claimed:       mgl32.Vec3{float32(i)*0.2 + 0.05, 0, 0},
			Expected:      mgl32.Vec3{float32(i) * 0.2, 0, 0},
			HasPendingAck: false,
			Tolerance:     tol,
		})
		if result.Outcome != reconcile.OutcomeAccept {
			t.Fatalf("tick %d: expected Accept, got %v", i, result.Outcome)
		}
	}
}

// Scenario 2: teleport ack in flight → Pending (not Snap)
func TestTeleportPendingScenario(t *testing.T) {
	result := reconcile.Decide(reconcile.Input{
		Claimed:       mgl32.Vec3{100, 64, 100},
		Expected:      mgl32.Vec3{0, 64, 0},
		HasPendingAck: true,
		Tolerance:     0.5,
	})
	if result.Outcome != reconcile.OutcomePending {
		t.Fatalf("expected Pending during teleport ack, got %v", result.Outcome)
	}
}

// Scenario 3: speed hack, no ack → Snap
func TestSpeedHackSnapsScenario(t *testing.T) {
	result := reconcile.Decide(reconcile.Input{
		Claimed:       mgl32.Vec3{10, 64, 0},
		Expected:      mgl32.Vec3{0, 64, 0},
		HasPendingAck: false,
		Tolerance:     0.5,
	})
	if result.Outcome != reconcile.OutcomeSnap {
		t.Fatalf("expected Snap for speed hack, got %v", result.Outcome)
	}
}

// Scenario 4: player jumping — claimed slightly above expected during jump arc → Accept
func TestLegitJumpArcScenario(t *testing.T) {
	// Claimed Y is 0.3m above expected (within tolerance 0.5)
	result := reconcile.Decide(reconcile.Input{
		Claimed:       mgl32.Vec3{0, 64.3, 0},
		Expected:      mgl32.Vec3{0, 64.0, 0},
		HasPendingAck: false,
		Tolerance:     0.5,
	})
	if result.Outcome != reconcile.OutcomeAccept {
		t.Fatalf("expected Accept for legit jump arc, got %v", result.Outcome)
	}
}

// Scenario 5: sprinting player with small positional overshoot → Accept
func TestLegitSprintScenario(t *testing.T) {
	// Sprint adds ~0.26 b/tick; overshoot of 0.1m from rounding is legitimate
	result := reconcile.Decide(reconcile.Input{
		Claimed:       mgl32.Vec3{0.36, 0, 0},
		Expected:      mgl32.Vec3{0.26, 0, 0},
		HasPendingAck: false,
		Tolerance:     0.5,
	})
	if result.Outcome != reconcile.OutcomeAccept {
		t.Fatalf("expected Accept for legit sprint, got %v", result.Outcome)
	}
}

// Scenario 6: sneaking player (slow movement) — tiny delta within tolerance → Accept
func TestLegitSneakScenario(t *testing.T) {
	// Sneak speed ~0.065 b/tick; claimed and expected almost identical
	result := reconcile.Decide(reconcile.Input{
		Claimed:       mgl32.Vec3{0, 0, 0.07},
		Expected:      mgl32.Vec3{0, 0, 0.065},
		HasPendingAck: false,
		Tolerance:     0.5,
	})
	if result.Outcome != reconcile.OutcomeAccept {
		t.Fatalf("expected Accept for legit sneak, got %v", result.Outcome)
	}
}

// Scenario 7: knockback ack in-flight → Pending (large delta but ack pending)
func TestKnockbackAckPendingScenario(t *testing.T) {
	// Server sent knockback; client is being pushed away; ack not yet resolved
	result := reconcile.Decide(reconcile.Input{
		Claimed:       mgl32.Vec3{5, 65, 5},
		Expected:      mgl32.Vec3{0, 64, 0},
		HasPendingAck: true,
		Tolerance:     0.5,
	})
	if result.Outcome != reconcile.OutcomePending {
		t.Fatalf("expected Pending for knockback ack in-flight, got %v", result.Outcome)
	}
}

// Scenario 8: player falling — Y decreasing within natural gravity bounds → Accept
func TestLegitFallScenario(t *testing.T) {
	tol := float32(0.5)
	// Simulate several ticks of falling (gravity ~0.08 b/tick²)
	yExpected := float32(70.0)
	yClaimed := float32(70.0)
	vy := float32(0.0)
	for i := 0; i < 10; i++ {
		vy -= 0.08
		yExpected += vy
		yClaimed = yExpected + 0.02 // tiny noise within tolerance
		result := reconcile.Decide(reconcile.Input{
			Claimed:       mgl32.Vec3{0, yClaimed, 0},
			Expected:      mgl32.Vec3{0, yExpected, 0},
			HasPendingAck: false,
			Tolerance:     tol,
		})
		if result.Outcome != reconcile.OutcomeAccept {
			t.Fatalf("fall tick %d: expected Accept, got %v", i, result.Outcome)
		}
	}
}

// Scenario 9: player standing idle — claimed == expected → Accept
func TestLegitIdleScenario(t *testing.T) {
	pos := mgl32.Vec3{10, 64, 10}
	for i := 0; i < 20; i++ {
		result := reconcile.Decide(reconcile.Input{
			Claimed:       pos,
			Expected:      pos,
			HasPendingAck: false,
			Tolerance:     0.5,
		})
		if result.Outcome != reconcile.OutcomeAccept {
			t.Fatalf("idle tick %d: expected Accept, got %v", i, result.Outcome)
		}
	}
}

// Scenario 10: strafing player (diagonal movement) within tolerance → Accept
func TestLegitStrafeScenario(t *testing.T) {
	tol := float32(0.5)
	// Strafe movement: equal X and Z components, small offset from expected
	for i := 0; i < 10; i++ {
		base := float32(i) * 0.18
		result := reconcile.Decide(reconcile.Input{
			Claimed:       mgl32.Vec3{base + 0.04, 64, base + 0.04},
			Expected:      mgl32.Vec3{base, 64, base},
			HasPendingAck: false,
			Tolerance:     tol,
		})
		if result.Outcome != reconcile.OutcomeAccept {
			t.Fatalf("strafe tick %d: expected Accept, got %v", i, result.Outcome)
		}
	}
}

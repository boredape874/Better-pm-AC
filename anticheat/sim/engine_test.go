package sim

import (
	"math"
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// mockTracker is a minimal meta.WorldTracker for sim tests. Only the block
// query methods are exercised; the packet handlers are stubs. A block is
// "solid" when the bboxes map has an entry for its position; optional
// name overrides drive the surface classifiers.
type mockTracker struct {
	bboxes map[cube.Pos][]cube.BBox
	named  map[cube.Pos]world.Block
}

func newMockTracker() *mockTracker {
	return &mockTracker{bboxes: map[cube.Pos][]cube.BBox{}, named: map[cube.Pos]world.Block{}}
}

func (m *mockTracker) putSolid(pos cube.Pos) {
	m.bboxes[pos] = []cube.BBox{cube.Box(0, 0, 0, 1, 1, 1)}
}

func (m *mockTracker) HandleLevelChunk(pk *packet.LevelChunk) error         { return nil }
func (m *mockTracker) HandleSubChunk(pk *packet.SubChunk) error             { return nil }
func (m *mockTracker) HandleBlockUpdate(pos cube.Pos, rid uint32) error     { return nil }
func (m *mockTracker) ChunkLoaded(x, z int32) bool                          { return true }
func (m *mockTracker) Close() error                                         { return nil }

func (m *mockTracker) Block(pos cube.Pos) world.Block {
	if b, ok := m.named[pos]; ok {
		return b
	}
	return stubBlock{name: "minecraft:air"}
}

func (m *mockTracker) BlockBBoxes(pos cube.Pos) []cube.BBox {
	return m.bboxes[pos]
}

// compile-time sanity: mockTracker is a valid WorldTracker.
var _ meta.WorldTracker = (*mockTracker)(nil)

// stubBlock is a minimal world.Block returning a canned name. Model() is
// unused by the sim, so returning nil is fine.
type stubBlock struct{ name string }

func (s stubBlock) EncodeBlock() (string, map[string]any) { return s.name, nil }
func (s stubBlock) Hash() (uint64, uint64)                { return 0, 0 }
func (s stubBlock) Model() world.BlockModel               { return nil }

// --- Tests ---

// Free fall over 10 ticks with Bedrock's gravity + drag.
// Analytic: v[n] = -3.92*(1 - 0.98^n); sum over n=1..10 ≈ 4.067 blocks.
func TestGravityFreeFall(t *testing.T) {
	e := NewEngine()
	w := newMockTracker()
	s := NewState(mgl32.Vec3{0, 100, 0})
	s.OnGround = false

	totalDy := float32(0)
	for i := 0; i < 10; i++ {
		prevY := s.Position[1]
		s = e.Step(s, meta.SimInput{}, w)
		totalDy += prevY - s.Position[1]
	}
	if totalDy < 3.8 || totalDy > 4.3 {
		t.Fatalf("expected 3.8–4.3 blocks drop in 10 ticks, got %.3f", totalDy)
	}
}

func TestJumpFromGround(t *testing.T) {
	e := NewEngine()
	w := newMockTracker()
	w.putSolid(cube.Pos{0, 99, 0})

	s := NewState(mgl32.Vec3{0, 100, 0})
	s = e.Step(s, meta.SimInput{Jumping: true}, w)

	expected := (JumpVel - Gravity) * AirDragY
	if math.Abs(float64(s.Velocity[1]-expected)) > 0.01 {
		t.Fatalf("jump y-vel want ≈%.3f, got %.3f", expected, s.Velocity[1])
	}
}

func TestGroundFrictionReducesVelocity(t *testing.T) {
	e := NewEngine()
	w := newMockTracker()
	w.putSolid(cube.Pos{0, 99, 0})

	s := NewState(mgl32.Vec3{0, 100, 0})
	s.Velocity = mgl32.Vec3{0.5, 0, 0}
	s = e.Step(s, meta.SimInput{}, w)

	expected := float32(0.5) * BaseFriction * DefaultFriction
	if math.Abs(float64(s.Velocity[0]-expected)) > 0.01 {
		t.Fatalf("friction: want vx≈%.3f, got %.3f", expected, s.Velocity[0])
	}
}

func TestSprintInputFasterThanWalk(t *testing.T) {
	e := NewEngine()
	w := newMockTracker()
	w.putSolid(cube.Pos{0, 99, 0})

	walk := NewState(mgl32.Vec3{0, 100, 0})
	sprint := NewState(mgl32.Vec3{0, 100, 0})
	for i := 0; i < 5; i++ {
		walk = e.Step(walk, meta.SimInput{Forward: 1}, w)
		sprint = e.Step(sprint, meta.SimInput{Forward: 1, Sprinting: true}, w)
	}
	if sprint.Velocity[2] <= walk.Velocity[2] {
		t.Fatalf("sprint (%.3f) should exceed walk (%.3f)", sprint.Velocity[2], walk.Velocity[2])
	}
}

func TestSpeedEffectBoostsHorizontal(t *testing.T) {
	e := NewEngine()
	w := newMockTracker()
	w.putSolid(cube.Pos{0, 99, 0})

	base := NewState(mgl32.Vec3{0, 100, 0})
	boosted := NewState(mgl32.Vec3{0, 100, 0})
	fx := map[int32]int32{EffectSpeed: 2}
	for i := 0; i < 4; i++ {
		base = e.Step(base, meta.SimInput{Forward: 1}, w)
		boosted = e.Step(boosted, meta.SimInput{Forward: 1, Effects: fx}, w)
	}
	if boosted.Velocity[2] <= base.Velocity[2] {
		t.Fatalf("Speed II boosted=%.3f should exceed base=%.3f", boosted.Velocity[2], base.Velocity[2])
	}
}

// A wall at x=1 should stop horizontal motion — the BBox of the player
// (width 0.6) plus its center-at-0 means max x after collision should be
// approximately 1 - 0.3 = 0.7.
func TestWallStopsHorizontalMovement(t *testing.T) {
	e := NewEngine()
	w := newMockTracker()
	w.putSolid(cube.Pos{0, 99, 0})
	w.putSolid(cube.Pos{1, 100, 0})
	w.putSolid(cube.Pos{1, 101, 0})

	s := NewState(mgl32.Vec3{0, 100, 0})
	s.Velocity = mgl32.Vec3{0.8, 0, 0}
	s = e.Step(s, meta.SimInput{}, w)

	if s.Position[0] > 0.71 {
		t.Fatalf("player clipped through wall, x=%.3f", s.Position[0])
	}
}

// SlowFalling clamps descent at SlowFallCap regardless of how long the
// player has been airborne.
func TestSlowFallingCapsDescent(t *testing.T) {
	e := NewEngine()
	w := newMockTracker()
	s := NewState(mgl32.Vec3{0, 100, 0})
	s.OnGround = false
	fx := map[int32]int32{EffectSlowFalling: 1}

	for i := 0; i < 20; i++ {
		s = e.Step(s, meta.SimInput{Effects: fx}, w)
	}
	// With SlowFall + drag, Y velocity oscillates near -SlowFallCap×drag.
	// Loosely: must never drop below -0.05.
	if s.Velocity[1] < -0.05 {
		t.Fatalf("slow fall violated cap, vy=%.3f", s.Velocity[1])
	}
}

func TestLevitationReversesGravity(t *testing.T) {
	e := NewEngine()
	w := newMockTracker()
	s := NewState(mgl32.Vec3{0, 100, 0})
	s.OnGround = false
	fx := map[int32]int32{EffectLevitation: 2}

	startY := s.Position[1]
	for i := 0; i < 20; i++ {
		s = e.Step(s, meta.SimInput{Effects: fx}, w)
	}
	if s.Position[1] <= startY {
		t.Fatalf("levitation should rise, startY=%.3f endY=%.3f", startY, s.Position[1])
	}
}

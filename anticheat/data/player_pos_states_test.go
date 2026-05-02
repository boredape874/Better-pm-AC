package data

import (
	"testing"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

func TestPlayerCommitPosFlow(t *testing.T) {
	p := NewPlayer(uuid.New(), "u")
	claimed := mgl32.Vec3{1, 64, 0}
	expected := mgl32.Vec3{0.99, 64, 0}

	p.SetClaimedPos(claimed)
	p.SetExpectedPos(expected)
	if p.ClaimedPos() != claimed {
		t.Fatalf("ClaimedPos=%v want %v", p.ClaimedPos(), claimed)
	}
	if p.ExpectedPos() != expected {
		t.Fatalf("ExpectedPos=%v want %v", p.ExpectedPos(), expected)
	}

	p.Commit(claimed)
	if p.CommittedPos() != claimed {
		t.Fatalf("CommittedPos after Commit=%v want %v", p.CommittedPos(), claimed)
	}
	if p.PrevCommittedPos() != (mgl32.Vec3{}) {
		t.Fatalf("first Commit prev=%v want zero", p.PrevCommittedPos())
	}

	p.Commit(mgl32.Vec3{2, 64, 0})
	if p.PrevCommittedPos() != claimed {
		t.Fatalf("second Commit prev=%v want %v", p.PrevCommittedPos(), claimed)
	}
}

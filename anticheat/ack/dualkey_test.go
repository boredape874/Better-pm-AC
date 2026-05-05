package ack_test

import (
	"testing"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/boredape874/Better-pm-AC/anticheat/ack"
)

func TestPushAndResolveByDualKey(t *testing.T) {
	sys := ack.NewSystem()
	k := ack.Key{ServerTick: 100, ClientTick: 95}
	expected := mgl32.Vec3{0, 0, 0.1}
	sys.Push(k, ack.Action{Kind: ack.ActionKnockback, ExpectedDelta: expected})

	// actual delta within tolerance
	actual := mgl32.Vec3{0, 0, 0.12}
	ok, got := sys.Resolve(k, actual)
	if !ok {
		t.Fatalf("expected resolve match, got mismatch; action=%+v", got)
	}
	if got.Kind != ack.ActionKnockback {
		t.Fatalf("expected ActionKnockback, got %v", got.Kind)
	}

	// second resolve on same key returns nothing
	ok2, _ := sys.Resolve(k, actual)
	if ok2 {
		t.Fatal("expected no match on second resolve of same key")
	}
}

func TestResolveMismatchOnLargeDelta(t *testing.T) {
	sys := ack.NewSystem()
	k := ack.Key{ServerTick: 200, ClientTick: 190}
	expected := mgl32.Vec3{1, 0, 0}
	sys.Push(k, ack.Action{Kind: ack.ActionTeleport, ExpectedDelta: expected})

	// actual delta far from expected
	actual := mgl32.Vec3{5, 0, 0}
	ok, got := sys.Resolve(k, actual)
	if ok {
		t.Fatal("expected mismatch on large delta, got match")
	}
	if got.Kind != ack.ActionTeleport {
		t.Fatalf("expected ActionTeleport in returned action, got %v", got.Kind)
	}
}

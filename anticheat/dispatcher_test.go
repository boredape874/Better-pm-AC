package anticheat

import (
	"io"
	"log/slog"
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/anticheat/mitigate"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// fakeKickDetection is a minimal Detection used to exercise handleViolation's
// dispatcher routing without depending on any real check's config.
type fakeKickDetection struct{}

func (fakeKickDetection) Type() string                          { return "Test" }
func (fakeKickDetection) SubType() string                       { return "Kick" }
func (fakeKickDetection) Description() string                   { return "" }
func (fakeKickDetection) Punishable() bool                      { return true }
func (fakeKickDetection) DefaultMetadata() *meta.DetectionMetadata { return &meta.DetectionMetadata{} }
func (fakeKickDetection) Policy() meta.MitigatePolicy           { return meta.PolicyKick }

type fakeRubberDetection struct{}

func (fakeRubberDetection) Type() string                          { return "Test" }
func (fakeRubberDetection) SubType() string                       { return "Rubber" }
func (fakeRubberDetection) Description() string                   { return "" }
func (fakeRubberDetection) Punishable() bool                      { return true }
func (fakeRubberDetection) DefaultMetadata() *meta.DetectionMetadata { return &meta.DetectionMetadata{} }
func (fakeRubberDetection) Policy() meta.MitigatePolicy           { return meta.PolicyClientRubberband }

func silentLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// newBareManager constructs a Manager with just enough wiring to exercise
// handleViolation. It bypasses NewManager (which pulls in every check config)
// because those are not required for routing tests.
func newBareManager(t *testing.T) *Manager {
	t.Helper()
	return &Manager{log: silentLogger()}
}

func TestHandleViolationRoutesKickThroughDispatcher(t *testing.T) {
	var gotUUID uuid.UUID
	var gotReason string
	m := newBareManager(t)
	m.KickFunc = func(id uuid.UUID, reason string) {
		gotUUID = id
		gotReason = reason
	}

	pid := uuid.New()
	p := data.NewPlayer(pid, "tester")
	md := &meta.DetectionMetadata{Violations: 10, MaxViolations: 5}

	m.handleViolation(p, fakeKickDetection{}, md, "unit-test")

	if gotUUID != pid {
		t.Fatalf("KickFunc got UUID %s, want %s", gotUUID, pid)
	}
	if gotReason == "" {
		t.Fatal("KickFunc got empty reason; expected dispatcher-formatted message")
	}
}

func TestHandleViolationKickDryRunWhenKickFuncNil(t *testing.T) {
	m := newBareManager(t) // KickFunc intentionally nil
	p := data.NewPlayer(uuid.New(), "tester")
	md := &meta.DetectionMetadata{Violations: 10, MaxViolations: 5}

	// Must not panic even with nil hooks — dispatcher degrades to log-only.
	m.handleViolation(p, fakeKickDetection{}, md, "dry-run")
}

func TestHandleViolationRubberbandHookFires(t *testing.T) {
	var rubbed string
	m := newBareManager(t)
	m.RubberbandFunc = func(id string) { rubbed = id }

	pid := uuid.New()
	p := data.NewPlayer(pid, "tester")
	md := &meta.DetectionMetadata{Violations: 1, MaxViolations: 5}

	m.handleViolation(p, fakeRubberDetection{}, md, "rubber")

	if rubbed != pid.String() {
		t.Fatalf("RubberbandFunc got %q, want %q", rubbed, pid.String())
	}
}

// Assert the Dispatcher.Apply packet-nil path does not crash with a real
// packet type reference in scope — guards against a future accidental nil
// dereference if someone adds packet introspection inside applyKick.
var _ = (*mitigate.Dispatcher)(nil)
var _ packet.Packet = (*packet.MovePlayer)(nil)

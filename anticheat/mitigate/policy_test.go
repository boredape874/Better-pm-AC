package mitigate

import (
	"io"
	"log/slog"
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type fakeDetection struct {
	typ, sub    string
	punishable  bool
	policy      meta.MitigatePolicy
}

func (f fakeDetection) Type() string                          { return f.typ }
func (f fakeDetection) SubType() string                       { return f.sub }
func (f fakeDetection) Description() string                   { return "" }
func (f fakeDetection) Punishable() bool                      { return f.punishable }
func (f fakeDetection) DefaultMetadata() *meta.DetectionMetadata { return &meta.DetectionMetadata{} }
func (f fakeDetection) Policy() meta.MitigatePolicy           { return f.policy }

func silentLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestApplyKickTriggersKickFuncWhenExceeded(t *testing.T) {
	var kicked string
	kick := func(uuid, reason string) { kicked = uuid + "|" + reason }
	d := NewDispatcher(silentLogger(), kick)
	det := fakeDetection{typ: "Fly", sub: "A", punishable: true, policy: meta.PolicyKick}
	md := &meta.DetectionMetadata{Violations: 6, MaxViolations: 5}

	pk := &packet.MovePlayer{}
	fwd, shouldKick := d.Apply("player-1", det, md, pk)

	if !shouldKick {
		t.Fatal("expected kick=true when Exceeded")
	}
	if fwd != pk {
		t.Fatal("expected original packet forwarded")
	}
	if kicked == "" {
		t.Fatal("KickFunc was not invoked")
	}
}

func TestApplyKickNoOpBelowMaxViolations(t *testing.T) {
	var called bool
	d := NewDispatcher(silentLogger(), func(string, string) { called = true })
	det := fakeDetection{typ: "Fly", sub: "A", punishable: true, policy: meta.PolicyKick}
	md := &meta.DetectionMetadata{Violations: 2, MaxViolations: 5}

	_, shouldKick := d.Apply("p", det, md, nil)
	if shouldKick || called {
		t.Fatal("under MaxViolations should not kick")
	}
}

func TestApplyKickSuppressedWhenKickFuncNil(t *testing.T) {
	d := NewDispatcher(silentLogger(), nil)
	det := fakeDetection{typ: "Fly", sub: "A", punishable: true, policy: meta.PolicyKick}
	md := &meta.DetectionMetadata{Violations: 6, MaxViolations: 5}
	_, shouldKick := d.Apply("p", det, md, nil)
	if shouldKick {
		t.Fatal("no KickFunc → should return kick=false (dry run)")
	}
}

func TestApplyNonKickPoliciesForwardUnchanged(t *testing.T) {
	d := NewDispatcher(silentLogger(), func(string, string) { t.Fatal("kick should not fire") })
	md := &meta.DetectionMetadata{Violations: 10, MaxViolations: 5}
	pk := &packet.MovePlayer{}
	for _, pol := range []meta.MitigatePolicy{meta.PolicyNone, meta.PolicyClientRubberband, meta.PolicyServerFilter} {
		det := fakeDetection{typ: "X", sub: "Y", punishable: true, policy: pol}
		fwd, kick := d.Apply("p", det, md, pk)
		if kick {
			t.Fatalf("policy %v should not kick", pol)
		}
		if fwd != pk {
			t.Fatalf("policy %v should forward original", pol)
		}
	}
}

func TestPolicyNameMapping(t *testing.T) {
	cases := map[meta.MitigatePolicy]string{
		meta.PolicyNone:             "none",
		meta.PolicyClientRubberband: "client_rubberband",
		meta.PolicyServerFilter:     "server_filter",
		meta.PolicyKick:             "kick",
	}
	for pol, want := range cases {
		if got := policyName(pol); got != want {
			t.Fatalf("policy %v: want %q, got %q", pol, want, got)
		}
	}
}

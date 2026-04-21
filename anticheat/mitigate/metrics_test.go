package mitigate

import (
	"io"
	"log/slog"
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// fakeKickDet is a minimal Detection that always resolves to PolicyKick.
type fakeKickDet struct{}

func (fakeKickDet) Type() string                             { return "Test" }
func (fakeKickDet) SubType() string                          { return "Kick" }
func (fakeKickDet) Description() string                      { return "" }
func (fakeKickDet) Punishable() bool                         { return true }
func (fakeKickDet) DefaultMetadata() *meta.DetectionMetadata { return &meta.DetectionMetadata{} }
func (fakeKickDet) Policy() meta.MitigatePolicy              { return meta.PolicyKick }

type fakeRubberDet struct{}

func (fakeRubberDet) Type() string                             { return "Test" }
func (fakeRubberDet) SubType() string                          { return "Rubber" }
func (fakeRubberDet) Description() string                      { return "" }
func (fakeRubberDet) Punishable() bool                         { return true }
func (fakeRubberDet) DefaultMetadata() *meta.DetectionMetadata { return &meta.DetectionMetadata{} }
func (fakeRubberDet) Policy() meta.MitigatePolicy              { return meta.PolicyClientRubberband }

type fakeFilterDet struct{}

func (fakeFilterDet) Type() string                             { return "Test" }
func (fakeFilterDet) SubType() string                          { return "Filter" }
func (fakeFilterDet) Description() string                      { return "" }
func (fakeFilterDet) Punishable() bool                         { return true }
func (fakeFilterDet) DefaultMetadata() *meta.DetectionMetadata { return &meta.DetectionMetadata{} }
func (fakeFilterDet) Policy() meta.MitigatePolicy              { return meta.PolicyServerFilter }

type fakeNoneDet struct{}

func (fakeNoneDet) Type() string                             { return "Test" }
func (fakeNoneDet) SubType() string                          { return "None" }
func (fakeNoneDet) Description() string                      { return "" }
func (fakeNoneDet) Punishable() bool                         { return true }
func (fakeNoneDet) DefaultMetadata() *meta.DetectionMetadata { return &meta.DetectionMetadata{} }
func (fakeNoneDet) Policy() meta.MitigatePolicy              { return meta.PolicyNone }

func silentLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestMetricsViolationsCounterIncrementsForActivePolicies(t *testing.T) {
	ResetForTesting()
	d := NewDispatcherWithHooks(silentLog(),
		func(string, string) {},                                             // kick
		func(string) {},                                                     // rubber
		func(_ string, p packet.Packet) packet.Packet { return p }, // filter (pass-through)
	)
	exceeded := &meta.DetectionMetadata{Violations: 10, MaxViolations: 5}
	open := &meta.DetectionMetadata{Violations: 1, MaxViolations: 5}

	d.Apply("u", fakeKickDet{}, exceeded, nil)
	d.Apply("u", fakeRubberDet{}, open, nil)
	d.Apply("u", fakeFilterDet{}, open, nil)
	d.Apply("u", fakeNoneDet{}, open, nil) // PolicyNone — not counted

	v, _, _, _, _ := Snapshot()
	if v != 3 {
		t.Fatalf("violations=%d, want 3 (kick + rubberband + filter; none excluded)", v)
	}
}

func TestMetricsKickCounterFiresOnlyWhenExceeded(t *testing.T) {
	ResetForTesting()
	d := NewDispatcher(silentLog(), func(string, string) {})
	d.Apply("u", fakeKickDet{}, &meta.DetectionMetadata{Violations: 1, MaxViolations: 5}, nil)
	d.Apply("u", fakeKickDet{}, &meta.DetectionMetadata{Violations: 10, MaxViolations: 5}, nil)
	_, k, _, _, _ := Snapshot()
	if k != 1 {
		t.Fatalf("kicks=%d, want 1 (only the exceeded call kicks)", k)
	}
}

func TestMetricsDryRunSuppressedCountsForNilHooks(t *testing.T) {
	ResetForTesting()
	d := NewDispatcher(silentLog(), nil) // KickFunc nil
	d.Apply("u", fakeKickDet{}, &meta.DetectionMetadata{Violations: 10, MaxViolations: 5}, nil)
	d.Apply("u", fakeRubberDet{}, &meta.DetectionMetadata{Violations: 1, MaxViolations: 5}, nil)
	_, _, _, _, dry := Snapshot()
	if dry != 2 {
		t.Fatalf("dry_run_suppressed=%d, want 2", dry)
	}
}

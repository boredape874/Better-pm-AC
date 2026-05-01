package meta

import "testing"

func TestTickContextSkew(t *testing.T) {
	ctx := TickContext{ServerTick: 100, ClientTick: 95}
	if got := ctx.Skew(); got != 5 {
		t.Fatalf("Skew()=%d want 5", got)
	}
}

func TestTickContextSkewNegativeIsZero(t *testing.T) {
	// ClientTick > ServerTick should not happen but must not panic.
	ctx := TickContext{ServerTick: 5, ClientTick: 10}
	if got := ctx.Skew(); got != 0 {
		t.Fatalf("Skew()=%d want 0 (clamped)", got)
	}
}

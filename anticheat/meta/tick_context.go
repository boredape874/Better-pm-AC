package meta

// TickContext carries the dual-tick frame in which a check executes.
// It is constructed once per packet (PlayerAuthInput / UseItemOnEntity / etc.)
// in Manager and threaded through to every Detection.Check call.
type TickContext struct {
	// ServerTick is the proxy's monotonic tick (20 TPS, 50 ms each).
	ServerTick uint64
	// ClientTick is the tick the client claims via PlayerAuthInput.Tick
	// (or the last-seen value for non-input events).
	ClientTick uint64
}

// Skew returns ServerTick - ClientTick clamped to >= 0. Negative skew
// (client tick ahead of server) is a Timer/A signal but here we just
// report 0 so downstream rewind math never indexes negative.
func (c TickContext) Skew() uint64 {
	if c.ClientTick > c.ServerTick {
		return 0
	}
	return c.ServerTick - c.ClientTick
}

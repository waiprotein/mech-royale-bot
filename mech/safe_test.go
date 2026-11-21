package mech

import "testing"

// TestSafeFallback checks the crash breaker: after three panics our FFA additions are off.
func TestSafeFallback(t *testing.T) {
	b := &Brain{P: DefaultParams()}
	b.base = b.P
	b.base.DuelFlee, b.base.GunOnlyOut, b.base.TPJoin = 0.3, true, 10
	b.base.RegenHide = 0.6
	b.P = b.base
	b.regenOn = true
	b.Panics = 3
	b.safeFallback()
	if !b.safeMode || b.P.DuelFlee != 0 || b.P.GunOnlyOut || b.P.TPJoin != 0 || b.P.LateR != 70 || b.P.LateAlive != 3 ||
		b.P.RegenHide != 0 || b.regenOn {
		t.Fatalf("crash breaker did not reset params: %+v", b.P)
	}
}

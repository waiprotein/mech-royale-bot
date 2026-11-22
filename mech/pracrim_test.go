package mech

import (
	"reflect"
	"testing"
)

// Tests for pracrim (idle squad play holds the rim of the next circle). Reuse lrConfig / lrTestWorld / lrObs.

func rimBrain(frac float64, latched bool) *Brain {
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: lrConfig()}, lrTestWorld())
	b.P.PracRim = frac
	b.notPractice = latched
	return b
}

// rimObs: no mechs; we stand at (px, 0); the next circle is centred on the origin with radius nextR.
func rimObs(t, px, nextR float64) *Obs {
	o := lrObs(t)
	o.Self.Pos, o.Self.Vel = Vec3{px, 0, 0}, Vec3{0, 11.7, 0}
	o.Zone.NextRadius = nextR
	return o
}

func rimAct(t *testing.T, b *Brain, o *Obs) Action {
	t.Helper()
	act := b.Act(o)
	if b.Panics != 0 {
		t.Fatalf("step panicked (%d)", b.Panics)
	}
	return act
}

func TestPracRimWalksToRim(t *testing.T) {
	plain, rim := rimBrain(0, false), rimBrain(0.88, false)
	a := rimAct(t, plain, rimObs(30, 10, 380))
	c := rimAct(t, rim, rimObs(30, 10, 380))
	if plain.Mode == "rim" || hyp(a.Move[0], a.Move[1]) > 0.1 {
		t.Fatalf("test setup: plain squad play is not idle (mode %s, move %v)", plain.Mode, a.Move)
	}
	if rim.Mode != "rim" || c.Move[0] < 0.5 {
		t.Fatalf("pracrim did not walk out to the rim (mode %s, move %v)", rim.Mode, c.Move)
	}
	if rx, ry := rim.rimSpot(&rimObs(30, 10, 380).Zone, 10, 0); hyp(rx-334.4, ry) > 0.5 {
		t.Fatalf("rim spot (%.1f, %.1f), want (334.4, 0)", rx, ry)
	}
	// at the rim: hold (no rim move)
	b := rimBrain(0.88, false)
	rimAct(t, b, rimObs(30, 332, 380))
	if b.Mode == "rim" {
		t.Fatalf("kept walking at the rim spot")
	}
	// next circle below 180 m: off
	b = rimBrain(0.88, false)
	rimAct(t, b, rimObs(30, 10, 140))
	if b.Mode == "rim" {
		t.Fatalf("rim play with the next circle at 140 m")
	}
}

// Before PracAt (base/ghost params) and after the FFA proof, pracrim changes nothing.
func TestPracRimInertOutsideSquadPlay(t *testing.T) {
	for _, c := range []struct {
		latched bool
		tt      float64
	}{{false, 10}, {true, 30}, {true, 100}} {
		a, b := rimBrain(0, c.latched), rimBrain(0.88, c.latched)
		if x, y := rimAct(t, a, rimObs(c.tt, 10, 380)), rimAct(t, b, rimObs(c.tt, 10, 380)); !reflect.DeepEqual(x, y) {
			t.Fatalf("pracrim changed play (latched %v, t %v)", c.latched, c.tt)
		}
	}
}

func TestPracRimResetByBreaker(t *testing.T) {
	b := rimBrain(0.88, false)
	rimAct(t, b, rimObs(30, 10, 380))
	b.safeFallback()
	if b.base.PracRim != 0 || b.P.PracRim != 0 {
		t.Fatalf("safeFallback leaves pracrim on")
	}
}

package mech

import (
	"reflect"
	"testing"
)

// Tests for lightpad (practice-side distance from lightning holders). They reuse lrConfig / lrTestWorld / lrObs.

func padBrain(pad float64, latched bool) *Brain {
	c := lrConfig()
	c.Weapons["lightning"] = WeaponStats{Damage: 60, Cooldown: 0.25, Range: 45, ConeDeg: 30, Chain: 2, ChainRange: 14, Energy: 3}
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: c}, lrTestWorld())
	b.P.LightPad = pad
	b.notPractice = latched
	return b
}

func zapper(id int, x, y float64) MechSeen {
	return MechSeen{ID: id, Kind: "mech", Pos: Vec3{x, y, 0}, Aim: [2]float64{3.14159, 0}, HP: 1000, MaxHP: 1400, Weapons: []string{"gun", "lightning"}}
}

// padObs: we hold gun + lightning (giddy fights a lightning holder at LightRange 28 m with that kit).
func padObs(t float64, mechs ...MechSeen) *Obs {
	o := lrObs(t, mechs...)
	o.Self.Vel = Vec3{0, 11.7, 0}
	o.Self.Weapons[1] = &WeaponState{Kind: "lightning", Stats: WeaponStats{Damage: 60, Cooldown: 0.25, Range: 45, ConeDeg: 30, Energy: 3}}
	return o
}

func padAct(t *testing.T, b *Brain, o *Obs) Action {
	t.Helper()
	act := b.Act(o)
	if b.Panics != 0 {
		t.Fatalf("step panicked (%d)", b.Panics)
	}
	return act
}

func TestLightPadInertAfterFFAProof(t *testing.T) {
	plain, pad := padBrain(0, true), padBrain(12, true)
	for _, tt := range []float64{30, 30.1, 30.2, 31} {
		a, c := padAct(t, plain, padObs(tt, zapper(3, 40, 0))), padAct(t, pad, padObs(tt, zapper(3, 40, 0)))
		if !reflect.DeepEqual(a, c) {
			t.Fatalf("lightpad changed post-latch play at t=%v", tt)
		}
	}
}

func TestLightPadPreferredRange(t *testing.T) {
	plain, pad := padBrain(0, false), padBrain(12, false)
	o := padObs(30, zapper(3, 40, 0))
	padAct(t, plain, o)
	padAct(t, pad, padObs(30, zapper(3, 40, 0)))
	me := &padObs(30).Self
	rp, rq := plain.prefRange(me, plain.tracks[3]), pad.prefRange(me, pad.tracks[3])
	if rp >= 45 {
		t.Fatalf("test setup: plain range vs a lightning holder with our lightning is %.0f m, expected giddy's close range", rp)
	}
	if rq < 57 {
		t.Fatalf("lightpad=12 preferred range %.0f m, want >= 57", rq)
	}
	// with a laser we fight it at laser range (>= 57 and >= LaserRange)
	me.Weapons[1] = &WeaponState{Kind: "laser", Stats: lrLaserStats}
	if r := pad.prefRange(me, pad.tracks[3]); r < 57 || r < pad.P.LaserRange {
		t.Fatalf("laser kite range %.0f m", r)
	}
	// finishing mode shrinks giddy's ranges to Finish (squad 20 m); lightpad still keeps 57 m
	keep := pad.P
	pad.P.GunRange, pad.P.LaserRange, pad.P.LightRange = 20, 20, 20
	if r := pad.prefRange(me, pad.tracks[3]); r < 57 {
		t.Fatalf("finishing range %.0f m inside lightning reach", r)
	}
	pad.P = keep
	// a target without lightning is unaffected
	plain2, pad2 := padBrain(0, false), padBrain(12, false)
	gunner := MechSeen{ID: 4, Kind: "mech", Pos: Vec3{40, 0, 0}, Aim: [2]float64{3.14159, 0}, HP: 1000, MaxHP: 1400, Weapons: []string{"gun"}}
	padAct(t, plain2, padObs(30, gunner))
	padAct(t, pad2, padObs(30, gunner))
	if a, c := plain2.prefRange(me, plain2.tracks[4]), pad2.prefRange(me, pad2.tracks[4]); a != c {
		t.Fatalf("lightpad changed the range to a gun-only mech: %.0f vs %.0f", a, c)
	}
}

// A lightning holder that closes to 35 m: with lightpad we move away from it (kite), plain giddy does not.
func TestLightPadKitesBack(t *testing.T) {
	plain, pad := padBrain(0, false), padBrain(12, false)
	var a, c Action
	for i := 0; i < 3; i++ {
		tt := 30 + 0.1*float64(i)
		a = padAct(t, plain, padObs(tt, zapper(3, 35, 0)))
		c = padAct(t, pad, padObs(tt, zapper(3, 35, 0)))
	}
	if c.Move[0] >= 0 {
		t.Fatalf("lightpad did not back off from a lightning holder at 35 m: move %v (plain %v)", c.Move, a.Move)
	}
}

func TestLightPadResetByBreaker(t *testing.T) {
	b := padBrain(12, false)
	padAct(t, b, padObs(30, zapper(3, 40, 0)))
	b.safeFallback()
	if b.P.LightPad != 0 || b.base.LightPad != 0 {
		t.Fatalf("safeFallback leaves lightpad on")
	}
}

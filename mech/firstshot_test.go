package mech

import (
	"math"
	"testing"
)

// Tests for the strict-gated first-shot flags (regenhide, watchlast, homelos, chargeengage).

func testStats() MechStats {
	return MechStats{Radius: 1.6, Height: 9, EyeHeight: 8, MuzzleHeight: 6.5, HP: 1400, WalkSpeed: 9, BoostSpeed: 18,
		TurnRate: 4, PitchRate: 3, MinPitch: -1.2, MaxPitch: 1.2}
}

// testBrain builds a brain on the flat map plus one 30 m tall block spanning x in [bx0, bx0+10], y in [-10, 10]
// (geometry only: it blocks sight and shots, the nav grid still treats it as open ground). FFA play is on from
// t = 0 (FFAStrict and practice off); set adjusts the params before the first observation.
func testBrain(bx0 float64, set func(p *Params)) *Brain {
	w := FlatWorld()
	x0, x1 := bx0, bx0+10
	w.initGeo([]Prism{{Kind: "building", Base: -1, Top: 30,
		Edges: []float64{x0, -10, x1, -10, x1, -10, x1, 10, x1, 10, x0, 10, x0, 10, x0, -10},
		MinX:  x0, MinY: -10, MaxX: x1, MaxY: 10}}, nil, Rules{MaxStep: 1.5})
	h := &Hello{Seed: 7}
	h.Config.Mech = testStats()
	h.Config.Vision.Range, h.Config.Vision.FovDeg, h.Config.Vision.NearRange = 200, 120, 45
	b := NewBrain(h, w)
	b.P.FFAStrict, b.P.Practice = false, false
	set(&b.P)
	return b
}

// testObs is us at the origin (gun and an uncharged laser, full energy, in the zone) at time tm with hp.
func testObs(tick int, tm, hp float64, mechs ...MechSeen) *Obs {
	o := &Obs{Type: "obs", Tick: tick, T: tm, Alive: 10, Mechs: mechs}
	o.Self = Self{ID: 1, HP: hp, MaxHP: 1400, Energy: 100, MaxEnergy: 100, Grounded: true, InZone: true, Stats: testStats(),
		Weapons: []*WeaponState{
			{Kind: "gun", Ammo: 12, Stats: WeaponStats{Damage: 45, Cooldown: 0.24, Speed: 252, Range: 85, Mag: 12, Reload: 2.4}},
			{Kind: "laser", Stats: WeaponStats{Damage: 315, Cooldown: 1.65, Range: 280, Charge: 0.75, MinCharge: 0.15, MinFrac: 0.35, HoldS: 2}},
		}}
	o.Zone = Zone{Radius: 500, NextRadius: 300, ShrinkStart: 1000, ShrinkEnd: 1100}
	return o
}

// TestRegenHide: at 40% HP with nobody in view, regenhide hides from a mech last seen 10 s ago (west of us, the
// block is east), keeps hiding while HP is below 90%, and lets go at 90%. With the flag off nothing changes.
func TestRegenHide(t *testing.T) {
	west := MechSeen{ID: 5, Kind: "mech", Pos: Vec3{-150, 0, 0}, Aim: [2]float64{math.Pi / 2, 0}, HP: 1400, MaxHP: 1400,
		Weapons: []string{"gun"}}
	for _, on := range []bool{false, true} {
		b := testBrain(10, func(p *Params) {
			if on {
				p.RegenHide = 0.6
			}
		})
		var act Action
		b.step(testObs(1, 100, 1400, west), &act) // full HP: it passes 150 m away, not engaged
		act = Action{}
		b.step(testObs(2, 110, 560), &act) // 10 s later at 40% HP, nobody in view
		if !on {
			if b.regenOn || b.Mode == "regen" {
				t.Fatalf("regenhide off but regen mode: %q", b.Mode)
			}
			continue
		}
		if !b.regenOn || b.Mode != "regen" || act.Move[0] <= 0 {
			t.Fatalf("regenhide: want regen mode moving east into cover, got mode %q regenOn %v move %v", b.Mode, b.regenOn, act.Move)
		}
		act = Action{}
		b.step(testObs(3, 111, 0.75*1400), &act) // 75%: still regenerating
		if !b.regenOn || b.Mode != "regen" {
			t.Fatalf("regenhide released below 90%%: mode %q", b.Mode)
		}
		act = Action{}
		b.step(testObs(4, 112, 0.92*1400), &act) // 92%: done
		if b.regenOn || b.Mode == "regen" {
			t.Fatalf("regenhide not released at 92%%: mode %q", b.Mode)
		}
	}
}

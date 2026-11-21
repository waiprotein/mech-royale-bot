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

// TestWatchLast: idle, with a mech last seen 3 s ago 150 m north, the torso stays within ±0.9 rad of its bearing.
func TestWatchLast(t *testing.T) {
	north := MechSeen{ID: 5, Kind: "mech", Pos: Vec3{0, 150, 0}, Aim: [2]float64{0, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
	b := testBrain(300, func(p *Params) { p.WatchLast = 8 })
	var act Action
	b.step(testObs(1, 100, 1400, north), &act)
	for k, tm := range []float64{101, 102, 103} {
		act = Action{}
		b.step(testObs(2+k, tm, 1400), &act)
		if act.Aim == nil || math.Abs(angDiff(act.Aim[0], math.Pi/2)) > 0.9+1e-9 {
			t.Fatalf("watchlast at t=%v: aim %v, want within 0.9 rad of north", tm, act.Aim)
		}
	}
}

// TestHomeLOS: the sightline penalty is 5 x homelos within 85 m of a recently seen mech, homelos at 85-200 m,
// and nothing behind the block, beyond 200 m, or with the flag off.
func TestHomeLOS(t *testing.T) {
	west := MechSeen{ID: 5, Kind: "mech", Pos: Vec3{-50, 0, 0}, Aim: [2]float64{math.Pi / 2, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
	for _, w := range []float64{0, 60} {
		b := testBrain(10, func(p *Params) { p.HomeLOS = w })
		o := testObs(1, 100, 1400, west)
		var act Action
		b.step(o, &act)
		cases := []struct{ x, y, want float64 }{
			{0, 0, 5 * w}, // 50 m, in sight
			{0, 150, w},   // 158 m, in sight
			{40, 0, 0},    // behind the block
			{-50, 250, 0}, // 250 m
		}
		for _, c := range cases {
			got := 0.0
			if b.ffaNow && b.P.HomeLOS > 0 {
				got = b.homeLOSCost(o, c.x, c.y)
			}
			if got != c.want {
				t.Fatalf("homelos=%v at (%v,%v): cost %v, want %v", w, c.x, c.y, got, c.want)
			}
		}
	}
}

// TestChargeEngage: a visible mech 150 m east charging its laser straight at us (block 10 m west of us).
// Off: giddy ignores it (beyond 120 m). On with a cold laser: engaged, break LOS westward, no new laser charge.
// On with a full laser aimed at it: fight (the laser case releases on target).
func TestChargeEngage(t *testing.T) {
	charger := MechSeen{ID: 6, Kind: "mech", Pos: Vec3{150, 0, 0}, Aim: [2]float64{math.Pi, 0}, HP: 1400, MaxHP: 1400,
		Weapons: []string{"gun", "laser"}, Charging: true}

	b := testBrain(-20, func(p *Params) {})
	var act Action
	b.step(testObs(1, 100, 1400, charger), &act)
	if b.tracks[6].Engage || b.Mode == "fight" || b.Mode == "chargecover" {
		t.Fatalf("chargeengage off: charger at 150 m engaged (mode %q)", b.Mode)
	}

	b = testBrain(-20, func(p *Params) { p.ChargeEngage = 200 })
	act = Action{}
	b.step(testObs(1, 100, 1400, charger), &act)
	if !b.tracks[6].Engage || b.Mode != "chargecover" || act.Move[0] >= 0 || act.Fire[1] {
		t.Fatalf("chargeengage cold: engage %v mode %q move %v fire %v", b.tracks[6].Engage, b.Mode, act.Move, act.Fire)
	}

	b = testBrain(-20, func(p *Params) { p.ChargeEngage = 200 })
	o := testObs(1, 100, 1400, charger)
	o.Self.Weapons[1].Charge = 0.75
	act = Action{}
	b.step(o, &act)
	if b.Mode != "fight" || act.Fire[1] {
		t.Fatalf("chargeengage hot: mode %q fire %v, want fight with the laser released", b.Mode, act.Fire)
	}
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

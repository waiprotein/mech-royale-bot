package mech

import (
	"reflect"
	"testing"
)

// Tests for ffadrops (strict supply-drop capture). They reuse lrConfig / lrTestWorld / lrObs from lrlaser_test.go.

const dropID = 20000

func dropBrain(ffadrops, latched bool) *Brain {
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: lrConfig()}, lrTestWorld())
	b.P.FFADrops = ffadrops
	b.notPractice = latched
	return b
}

// dropObs: lrObs plus one supply drop 60 m north of us that lands in landsIn s.
func dropObs(t, landsIn float64, mechs ...MechSeen) *Obs {
	o := lrObs(t, mechs...)
	o.Drops = []Drop{{ID: dropID, Pos: Vec3{0, 60, 0}, Radius: 8, LandsIn: landsIn, CaptureS: 4}}
	return o
}

// bystander: a gun-only mech at distance d on -x, aim away from us (not engaged by giddy's ghost rule beyond 60 m).
func bystander(d float64) MechSeen {
	return MechSeen{ID: 5, Kind: "mech", Pos: Vec3{-d, 0, 0}, Aim: [2]float64{3.14159, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
}

func dropAct(t *testing.T, b *Brain, o *Obs) Action {
	t.Helper()
	act := b.Act(o)
	if b.Panics != 0 {
		t.Fatalf("step panicked (%d)", b.Panics)
	}
	return act
}

// Flag on without the FFA latch: every action equals the flag-off build's, before PracAt (base params) and after
// it (squad params). Flag off vs main is checked on the kit stream with tools/compare_actions.py.
func TestFFADropsInertWithoutLatch(t *testing.T) {
	plain, strict := dropBrain(false, false), dropBrain(true, false)
	for _, tt := range []float64{5, 10, 19.9, 20.5, 40, 75, 80, 85, 90} {
		a, c := dropAct(t, plain, dropObs(tt, 20)), dropAct(t, strict, dropObs(tt, 20))
		if !reflect.DeepEqual(a, c) {
			t.Fatalf("ffadrops=1 without the latch changed play at t=%v: %+v vs %+v", tt, a, c)
		}
	}
	if plain.lootID == dropID || strict.lootID == dropID {
		t.Fatalf("drop chosen without the latch")
	}
}

// usedrops=1 (giddy's own path) plays the same with or without ffadrops.
func TestFFADropsKeepsUseDrops(t *testing.T) {
	a, c := dropBrain(false, true), dropBrain(true, true)
	a.P.UseDrops, c.P.UseDrops = true, true
	for _, tt := range []float64{75, 80, 85} {
		if x, y := dropAct(t, a, dropObs(tt, 20)), dropAct(t, c, dropObs(tt, 20)); !reflect.DeepEqual(x, y) {
			t.Fatalf("ffadrops changed usedrops play at t=%v", tt)
		}
	}
}

func TestFFADropsGoesForDropAfterLatch(t *testing.T) {
	b := dropBrain(true, true)
	act := dropAct(t, b, dropObs(80, 20))
	if b.lootID != dropID {
		t.Fatalf("latched ffadrops did not choose the drop (lootID %d)", b.lootID)
	}
	if act.Move[1] <= 0.5 {
		t.Fatalf("not walking north to the drop: move %v", act.Move)
	}
	// unreachable before landing + 25 s (the existing test): 60 m at 11.7 m/s = 5.1 s > -21 + 25
	b2 := dropBrain(true, true)
	dropAct(t, b2, dropObs(80, -21))
	if b2.lootID == dropID {
		t.Fatalf("chose a drop it cannot reach in time")
	}
}

// walk: decision i of a run toward the drop. We advance 1.17 m north per decision at walk speed, so giddy's stuck
// detector (velocity below 1.2 m/s) stays quiet; the drop lands 20 s after the first decision.
func walk(t *testing.T, b *Brain, i int, mechs ...MechSeen) (*Obs, Action) {
	t.Helper()
	o := dropObs(80+0.1*float64(i), 20-0.1*float64(i), mechs...)
	o.Self.Pos, o.Self.Vel = Vec3{0, 1.17 * float64(i), 0}, Vec3{0, 11.7, 0}
	return o, dropAct(t, b, o)
}

func TestFFADropsGuards(t *testing.T) {
	// commit: a mech seen within 150 m blocks it
	b := dropBrain(true, true)
	walk(t, b, 0, bystander(120))
	if b.lootID == dropID {
		t.Fatalf("committed with a mech seen at 120 m")
	}
	// ... a mech at 170 m does not
	b = dropBrain(true, true)
	walk(t, b, 0, bystander(170))
	walk(t, b, 1, bystander(170)) // second consecutive approval: committed
	if b.lootID != dropID || b.Mode != "loot:drop" {
		t.Fatalf("not committed with the nearest mech at 170 m (lootID %d, mode %s)", b.lootID, b.Mode)
	}
	// committed: a mech at 120 m is tolerated, one at 90 m aborts and bars the drop for 8 s
	walk(t, b, 2, bystander(120))
	if b.lootID != dropID || b.Mode != "loot:drop" {
		t.Fatalf("aborted at 120 m (abort radius is 100 m): lootID %d, mode %s", b.lootID, b.Mode)
	}
	walk(t, b, 3, bystander(90))
	if until := b.bad[dropID]; b.lootID == dropID || until < 80.3+7.9 {
		t.Fatalf("committed drop not barred after a mech came within 100 m (lootID %d, bad until %v)", b.lootID, until)
	}
	// a lapsed commitment (a decision spent elsewhere) must pass the 150 m test again
	b = dropBrain(true, true)
	walk(t, b, 0)
	walk(t, b, 1)
	walk(t, b, 6, bystander(120)) // 0.5 s later: the commitment lapsed, a mech at 120 m blocks it
	if b.Mode == "loot:drop" {
		t.Fatalf("resumed a lapsed drop run with a mech seen at 120 m")
	}
	// committed, then hit by a mech we cannot see: abort
	b = dropBrain(true, true)
	walk(t, b, 0)
	walk(t, b, 1)
	b.tracks[9] = &Track{MechSeen: MechSeen{ID: 9, Kind: "mech"}, Seen: 70, HitMe: -100}
	o := dropObs(80.2, 19.8)
	o.Self.Pos, o.Self.Vel = Vec3{0, 2.34, 0}, Vec3{0, 11.7, 0}
	by, br := 9, 1.0
	o.Events = []Event{{Type: "damaged", Amount: 45, Weapon: "gun", By: &by, Bearing: &br}}
	dropAct(t, b, o)
	if b.Mode == "loot:drop" || b.bad[dropID] < 80.2+7.9 {
		t.Fatalf("committed drop kept after a mech hit us (mode %s, bad %v)", b.Mode, b.bad[dropID])
	}
	// another mech holds the capture clock: skip
	b = dropBrain(true, true)
	o = dropObs(80, 0)
	h := 7
	o.Drops[0].Holder = &h
	dropAct(t, b, o)
	if b.lootID == dropID {
		t.Fatalf("went for a drop another mech is capturing")
	}
}

func TestFFADropsResetBySquadAndBreaker(t *testing.T) {
	p := DefaultParams()
	p.FFADrops = true
	if squadParams(p).FFADrops {
		t.Fatalf("squadParams keeps ffadrops on")
	}
	b := dropBrain(true, true)
	dropAct(t, b, dropObs(80, 20))
	b.safeFallback()
	if b.P.FFADrops || b.base.FFADrops {
		t.Fatalf("safeFallback leaves ffadrops on")
	}
}

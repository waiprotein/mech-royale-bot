package mech

import (
	"reflect"
	"testing"
)

// Tests for ffadrops (conditional supply-drop capture). They reuse lrConfig / lrTestWorld / lrObs from
// lrlaser_test.go.

const dropID = 20000

// dropBrain: ffadrops = max distance (0 = off); latched = giddy's FFA detector has fired.
func dropBrain(ffadrops float64, latched bool) *Brain {
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: lrConfig()}, lrTestWorld())
	b.P.FFADrops = ffadrops
	b.notPractice = latched
	return b
}

// dropObs: lrObs plus one supply drop 60 m north of us that lands in landsIn s; we walk north at walk speed.
func dropObs(t, landsIn float64, mechs ...MechSeen) *Obs {
	o := lrObs(t, mechs...)
	o.Self.Vel = Vec3{0, 11.7, 0}
	o.Drops = []Drop{{ID: dropID, Pos: Vec3{0, 60, 0}, Radius: 8, LandsIn: landsIn, CaptureS: 4}}
	return o
}

// bystander: a gun-only mech at distance d on -x, aim away from us (not engaged by giddy's ghost rule beyond 60 m).
func bystander(d float64) MechSeen {
	return MechSeen{ID: 5, Kind: "mech", Pos: Vec3{-d, 0, 0}, Aim: [2]float64{3.14159, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
}

// oldTrack: a mech last seen ago s before now at distance d (not visible now).
func oldTrack(b *Brain, now, ago, d float64) {
	b.tracks[6] = &Track{MechSeen: MechSeen{ID: 6, Kind: "mech", Pos: Vec3{-d, 0, 0}, HP: 1400, MaxHP: 1400}, Seen: now - ago, Dist: d, HitMe: -100}
}

func dropAct(t *testing.T, b *Brain, o *Obs) Action {
	t.Helper()
	act := b.Act(o)
	if b.Panics != 0 {
		t.Fatalf("step panicked (%d)", b.Panics)
	}
	return act
}

// walk: decision i of a run toward the drop. We advance 1.17 m north per decision at walk speed, so giddy's stuck
// detector (velocity below 1.2 m/s) stays quiet; the drop lands 20 s after the first decision.
func walk(t *testing.T, b *Brain, i int, mechs ...MechSeen) (*Obs, Action) {
	t.Helper()
	o := dropObs(80+0.1*float64(i), 20-0.1*float64(i), mechs...)
	o.Self.Pos = Vec3{0, 1.17 * float64(i), 0}
	return o, dropAct(t, b, o)
}

// Flag on without the FFA latch: every action equals the flag-off build's, before PracAt (base params) and after
// it (squad params). Flag off vs main is checked on the kit stream with tools/compare_actions.py.
func TestFFADropsInertWithoutLatch(t *testing.T) {
	plain, strict := dropBrain(0, false), dropBrain(120, false)
	for _, tt := range []float64{5, 10, 19.9, 20.5, 40, 75, 80, 85, 90} {
		a, c := dropAct(t, plain, dropObs(tt, 20)), dropAct(t, strict, dropObs(tt, 20))
		if !reflect.DeepEqual(a, c) {
			t.Fatalf("ffadrops without the latch changed play at t=%v: %+v vs %+v", tt, a, c)
		}
	}
	if plain.lootID == dropID || strict.lootID == dropID {
		t.Fatalf("drop chosen without the latch")
	}
}

// usedrops=1 (giddy's own path) plays the same with or without ffadrops.
func TestFFADropsKeepsUseDrops(t *testing.T) {
	a, c := dropBrain(0, true), dropBrain(120, true)
	a.P.UseDrops, c.P.UseDrops = true, true
	for _, tt := range []float64{75, 80, 85} {
		if x, y := dropAct(t, a, dropObs(tt, 20)), dropAct(t, c, dropObs(tt, 20)); !reflect.DeepEqual(x, y) {
			t.Fatalf("ffadrops changed usedrops play at t=%v", tt)
		}
	}
}

func TestFFADropsGoesForDropAfterLatch(t *testing.T) {
	b := dropBrain(120, true)
	act := dropAct(t, b, dropObs(80, 20))
	if b.lootID != dropID {
		t.Fatalf("latched ffadrops did not choose the drop (lootID %d)", b.lootID)
	}
	if act.Move[1] <= 0.5 {
		t.Fatalf("not walking north to the drop: move %v", act.Move)
	}
	// unreachable before landing + 25 s (the existing test): 60 m at 11.7 m/s = 5.1 s > -21 + 25
	b = dropBrain(120, true)
	dropAct(t, b, dropObs(80, -21))
	if b.lootID == dropID {
		t.Fatalf("chose a drop it cannot reach in time")
	}
	// distance cap: ffadrops=50 skips the drop 60 m away
	b = dropBrain(50, true)
	dropAct(t, b, dropObs(80, 20))
	if b.lootID == dropID {
		t.Fatalf("ffadrops=50 went for a drop 60 m away")
	}
}

// Only the first two drops of the match (call order = order first listed).
func TestFFADropsFirstTwoOnly(t *testing.T) {
	far := func(id int) Drop { return Drop{ID: id, Pos: Vec3{0, -250, 0}, Radius: 8, LandsIn: 20, CaptureS: 4} }
	for _, c := range []struct {
		order []int
		want  bool
	}{{[]int{20000, 20001, 20002}, false}, {[]int{20001, 20002, 20000}, true}} {
		b := dropBrain(120, true)
		o := dropObs(80, 20)
		near := o.Drops[0]
		o.Drops = nil
		for _, id := range c.order {
			if id == dropID {
				o.Drops = append(o.Drops, near) // our drop, 60 m away
			} else {
				o.Drops = append(o.Drops, far(id)) // beyond the 120 m cap
			}
		}
		// listed third (our drop is 20000; order above makes it third / first)
		o.Drops[0], o.Drops[2] = o.Drops[2], o.Drops[0]
		dropAct(t, b, o)
		rank := -1
		for i, id := range b.dropOrder {
			if id == dropID {
				rank = i
			}
		}
		if got := b.lootID == dropID; got != (rank < 2) {
			t.Fatalf("drop listed at position %d: chosen = %v", rank, got)
		}
	}
}

func TestFFADropsGuards(t *testing.T) {
	// commit needs calm: any visible mech blocks it, even at 170 m
	b := dropBrain(120, true)
	walk(t, b, 0, bystander(170))
	if b.lootID == dropID {
		t.Fatalf("committed with a mech visible at 170 m")
	}
	// ... so does a mech seen within 150 m in the last 5 s, but not one seen at 160 m or 6 s ago
	b = dropBrain(120, true)
	oldTrack(b, 80, 3, 140)
	walk(t, b, 0)
	if b.lootID == dropID {
		t.Fatalf("committed with a mech seen at 140 m 3 s ago")
	}
	for _, c := range [][2]float64{{3, 160}, {6, 140}} {
		b = dropBrain(120, true)
		oldTrack(b, 80, c[0], c[1])
		walk(t, b, 0)
		if b.lootID != dropID {
			t.Fatalf("did not commit with the only mech seen %.0f s ago at %.0f m", c[0], c[1])
		}
	}
	// ... and being hit in the last 5 s (by anything)
	b = dropBrain(120, true)
	b.lastHurt = 76
	walk(t, b, 0)
	if b.lootID == dropID {
		t.Fatalf("committed 4 s after being hit")
	}
	// committed (two calm approvals): a mech appearing at 120 m is tolerated, one at 90 m aborts and bars it 8 s
	b = dropBrain(120, true)
	walk(t, b, 0)
	walk(t, b, 1)
	if b.lootID != dropID || b.Mode != "loot:drop" {
		t.Fatalf("not committed when calm (lootID %d, mode %s)", b.lootID, b.Mode)
	}
	walk(t, b, 2, bystander(120))
	if b.lootID != dropID || b.Mode != "loot:drop" {
		t.Fatalf("aborted at 120 m (abort radius is 100 m): lootID %d, mode %s", b.lootID, b.Mode)
	}
	walk(t, b, 3, bystander(90))
	if until := b.bad[dropID]; b.lootID == dropID || until < 80.3+7.9 {
		t.Fatalf("committed drop not barred after a mech came within 100 m (lootID %d, bad until %v)", b.lootID, until)
	}
	// a lapsed commitment (a decision spent elsewhere) must pass the calm test again
	b = dropBrain(120, true)
	walk(t, b, 0)
	walk(t, b, 1)
	walk(t, b, 6, bystander(120)) // 0.5 s later: the commitment lapsed, a visible mech blocks it
	if b.Mode == "loot:drop" {
		t.Fatalf("resumed a lapsed drop run with a mech visible at 120 m")
	}
	// committed, then hit by a mech we cannot see: abort
	b = dropBrain(120, true)
	walk(t, b, 0)
	walk(t, b, 1)
	b.tracks[9] = &Track{MechSeen: MechSeen{ID: 9, Kind: "mech"}, Seen: 70, Dist: 300, HitMe: -100}
	o := dropObs(80.2, 19.8)
	o.Self.Pos = Vec3{0, 2.34, 0}
	by, br := 9, 1.0
	o.Events = []Event{{Type: "damaged", Amount: 45, Weapon: "gun", By: &by, Bearing: &br}}
	dropAct(t, b, o)
	if b.Mode == "loot:drop" || b.bad[dropID] < 80.2+7.9 {
		t.Fatalf("committed drop kept after a mech hit us (mode %s, bad %v)", b.Mode, b.bad[dropID])
	}
	// committed below 50% HP: a mech seen within 200 m in the last 5 s aborts
	b = dropBrain(120, true)
	walk(t, b, 0)
	walk(t, b, 1)
	oldTrack(b, 80.2, 1, 180)
	o = dropObs(80.2, 19.8)
	o.Self.Pos, o.Self.HP = Vec3{0, 2.34, 0}, 600
	dropAct(t, b, o)
	if b.Mode == "loot:drop" {
		t.Fatalf("kept a drop run at 43%% HP with a mech seen at 180 m 1 s ago")
	}
	// dropalive=8: skip with 7 alive, go with 10 (lrObs has Alive 10)
	b = dropBrain(120, true)
	b.P.DropAlive = 8
	o = dropObs(80, 20)
	o.Alive = 7
	dropAct(t, b, o)
	if b.lootID == dropID {
		t.Fatalf("dropalive=8 went for a drop with 7 alive")
	}
	b = dropBrain(120, true)
	b.P.DropAlive = 8
	dropAct(t, b, dropObs(80, 20))
	if b.lootID != dropID {
		t.Fatalf("dropalive=8 skipped a drop with 10 alive")
	}
	// another mech holds the capture clock: skip
	b = dropBrain(120, true)
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
	p.FFADrops = 120
	if squadParams(p).FFADrops != 0 {
		t.Fatalf("squadParams keeps ffadrops on")
	}
	b := dropBrain(120, true)
	dropAct(t, b, dropObs(80, 20))
	b.safeFallback()
	if b.P.FFADrops != 0 || b.base.FFADrops != 0 {
		t.Fatalf("safeFallback leaves ffadrops on")
	}
}

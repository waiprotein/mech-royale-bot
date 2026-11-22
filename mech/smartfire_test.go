package mech

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// Tests for smartfire (practice-side fire control from the config damage rules). Reuse lrConfig / lrTestWorld / lrObs.

func sfBrain(k float64, latched bool) *Brain {
	c := lrConfig()
	c.Mech.RearMult, c.Mech.FrontArcDeg = 1.5, 180
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: c}, lrTestWorld())
	b.P.SmartFire = k
	b.notPractice = latched
	return b
}

// sfFoe: mech 3 at (d, 0) with hp; facingUs = its aim points at us (front arc), else away (we are in its rear arc).
func sfFoe(d, hp float64, facingUs bool) MechSeen {
	yaw := 0.0
	if facingUs {
		yaw = math.Pi
	}
	return MechSeen{ID: 3, Kind: "mech", Pos: Vec3{d, 0, 0}, Aim: [2]float64{yaw, 0}, HP: hp, MaxHP: 1400, Weapons: []string{"gun", "laser"}}
}

// sfObs: our laser holds `charge` s of charge (0 = idle, cooldown 0); our HP myHP; we aim straight at the foe.
func sfObs(t, myHP, charge float64, foe MechSeen) *Obs {
	o := lrObs(t, foe)
	o.Self.HP, o.Self.Vel = myHP, Vec3{0, 11.7, 0}
	o.Self.Weapons[1] = &WeaponState{Kind: "laser", Charge: charge, Stats: lrLaserStats}
	return o
}

func sfAct(t *testing.T, b *Brain, o *Obs) Action {
	t.Helper()
	act := b.Act(o)
	if b.Panics != 0 {
		t.Fatalf("step panicked (%d)", b.Panics)
	}
	return act
}

func TestSmartFireDamageModel(t *testing.T) {
	b := sfBrain(0.92, false)
	sfAct(t, b, sfObs(30, 1400, 0, sfFoe(100, 1000, true)))
	tr := b.tracks[3]
	me := &sfObs(30, 1400, 0, sfFoe(100, 1000, true)).Self
	w := me.Weapons[1]
	near := func(a, want float64) bool { return math.Abs(a-want) < 0.01 }
	if d := b.laserDmg(me, w, tr, 0, 0, 0); !near(d, 315*0.35) {
		t.Fatalf("zero-charge damage %.2f, want %.2f", d, 315*0.35)
	}
	if d := b.laserDmg(me, w, tr, 0.75, 0, 0); !near(d, 315) {
		t.Fatalf("full-charge damage %.2f, want 315", d)
	}
	me.Cubes = 5 // +4% per cube
	if d := b.laserDmg(me, w, tr, 0.75, 0, 0); !near(d, 315*1.2) {
		t.Fatalf("5-cube damage %.2f, want %.2f", d, 315*1.2)
	}
	me.Cubes = 0
	tr.Weapons = []string{"gun", "mortar"} // cook-off +60%
	if d := b.laserDmg(me, w, tr, 0.75, 0, 0); !near(d, 315*1.6) {
		t.Fatalf("cook-off damage %.2f, want %.2f", d, 315*1.6)
	}
	tr.Weapons = []string{"gun"}
	tr.Aim[0] = 0 // facing away from us: rear arc x1.5
	if d := b.laserDmg(me, w, tr, 0.75, 0, 0); !near(d, 315*1.5) {
		t.Fatalf("rear-arc damage %.2f, want %.2f", d, 315*1.5)
	}
}

// Squad (practice) play: with 0.3 s of charge (192 damage) on a 170 HP mech facing us, giddy's Lethal (x0.7 = 134)
// keeps charging; smartfire (x0.92 = 177) releases now.
func TestSmartFireEarlyRelease(t *testing.T) {
	plain, smart := sfBrain(0, false), sfBrain(0.92, false)
	a := sfAct(t, plain, sfObs(30, 1400, 0.3, sfFoe(100, 170, true)))
	c := sfAct(t, smart, sfObs(30, 1400, 0.3, sfFoe(100, 170, true)))
	if plain.tracks[3] == nil || !plain.tracks[3].Engage {
		t.Fatalf("test setup: target not engaged")
	}
	if !a.Fire[1] {
		t.Fatalf("test setup: giddy's Lethal already releases at 0.3 s on 170 HP")
	}
	if c.Fire[1] {
		t.Fatalf("smartfire did not release a lethal 0.3 s charge")
	}
	// 250 HP facing us: not lethal (177 < 250), keep charging; facing away (rear x1.5 = 265): release
	s2 := sfBrain(0.92, false)
	if act := sfAct(t, s2, sfObs(30, 1400, 0.3, sfFoe(100, 250, true))); !act.Fire[1] {
		t.Fatalf("released a non-lethal charge")
	}
	s3 := sfBrain(0.92, false)
	if act := sfAct(t, s3, sfObs(30, 1400, 0.3, sfFoe(100, 250, false))); act.Fire[1] {
		t.Fatalf("did not count the rear arc (x1.5) for a lethal release")
	}
}

// Base params (before PracAt): at 36% HP giddy retreats from a 300 HP mech at 50 m; a ready laser into its back
// kills it (315 x 1.5 x 0.92 = 435), so smartfire commits (finishing) and keeps fighting.
func TestSmartFireCommitsToKill(t *testing.T) {
	plain, smart := sfBrain(0, false), sfBrain(0.92, false)
	for i := 0; i < 3; i++ {
		tt := 10 + 0.1*float64(i)
		sfAct(t, plain, sfObs(tt, 500, 0, sfFoe(50, 300, false)))
		sfAct(t, smart, sfObs(tt, 500, 0, sfFoe(50, 300, false)))
	}
	if strings.HasPrefix(plain.Mode, "fight") {
		t.Fatalf("test setup: giddy keeps fighting at 36%% HP (mode %s)", plain.Mode)
	}
	if !strings.HasPrefix(smart.Mode, "fight") {
		t.Fatalf("smartfire did not commit to a certain kill (mode %s, plain %s)", smart.Mode, plain.Mode)
	}
	me := &sfObs(10, 500, 0, sfFoe(50, 300, false)).Self
	if !smart.canKill(me, smart.tracks[3], me.Weapons[0], me.Weapons[1], 0, 0) {
		t.Fatalf("canKill false for a ready laser into a 300 HP back")
	}
	smart.tracks[3].Aim[0] = math.Pi // facing us: 290 < 300, and the gun burst needs 8 shots (1.9 s)
	if smart.canKill(me, smart.tracks[3], me.Weapons[0], me.Weapons[1], 0, 0) {
		t.Fatalf("canKill true without a lethal burst")
	}
}

func TestSmartFireInertAfterFFAProof(t *testing.T) {
	plain, smart := sfBrain(0, true), sfBrain(0.92, true)
	for _, tt := range []float64{30, 30.1, 30.2} {
		a, c := sfAct(t, plain, sfObs(tt, 500, 0.3, sfFoe(50, 170, false))), sfAct(t, smart, sfObs(tt, 500, 0.3, sfFoe(50, 170, false)))
		if !reflect.DeepEqual(a, c) {
			t.Fatalf("smartfire changed post-latch play at t=%v", tt)
		}
	}
}

func TestSmartFireResetByBreaker(t *testing.T) {
	b := sfBrain(0.92, false)
	sfAct(t, b, sfObs(30, 1400, 0, sfFoe(100, 1000, true)))
	b.safeFallback()
	if b.P.SmartFire != 0 || b.base.SmartFire != 0 {
		t.Fatalf("safeFallback leaves smartfire on")
	}
}

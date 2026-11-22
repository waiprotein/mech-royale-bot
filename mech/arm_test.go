package mech

import (
	"math"
	"testing"
)

// Tests for the opening family ported from lab/opening (arm, armstrict, armwin, armprac, armuntil, scan, cubepush).
// They reuse lrConfig / lrTestWorld / lrObs from lrlaser_test.go.

func armBrain(flags string, latched bool) *Brain {
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: lrConfig()}, lrTestWorld())
	applyFlags(&b.P, flags)
	b.notPractice = latched
	return b
}

// armObs: we are gun-only at the origin, walking; one lightning pickup (8001) at (0, d).
func armObs(t, d float64, mechs ...MechSeen) *Obs {
	o := lrObs(t, mechs...)
	o.Self.Vel = Vec3{0, 11.7, 0}
	o.Self.Weapons = []*WeaponState{{Kind: "gun", Ammo: 12, Stats: lrGun}, nil}
	if d > 0 {
		o.Pickups = []Item{{ID: 8001, Kind: "lightning", Pos: Vec3{0, d, 0}}}
	}
	return o
}

func armAct(t *testing.T, b *Brain, o *Obs) Action {
	t.Helper()
	act := b.Act(o)
	if b.Panics != 0 {
		t.Fatalf("step panicked (%d)", b.Panics)
	}
	return act
}

func TestArmSeeksFarWeapon(t *testing.T) {
	// plain giddy (LootRadius 90) ignores a lightning 110 m away; arm (ArmR 120 / ArmRFar 160) walks to it
	plain, arm := armBrain("", false), armBrain("arm=1", false)
	armAct(t, plain, armObs(5, 110))
	act := armAct(t, arm, armObs(5, 110))
	if plain.lootID == 8001 {
		t.Fatalf("test setup: plain build already goes for a pickup 110 m away")
	}
	if arm.lootID != 8001 || act.Move[1] < 0.5 {
		t.Fatalf("arm did not go for the lightning 110 m away (lootID %d, move %v)", arm.lootID, act.Move)
	}
	// a mech tracked near the route blocks the candidate
	b := armBrain("arm=1", false)
	bystander := MechSeen{ID: 5, Kind: "mech", Pos: Vec3{30, 55, 0}, Aim: [2]float64{0, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
	armAct(t, b, armObs(5, 110, bystander))
	if b.lootID == 8001 {
		t.Fatalf("arm went for a pickup with a mech 30 m off the route")
	}
	// after ArmUntil (default 30 s) the recipe is off
	b = armBrain("arm=1", true)
	armAct(t, b, armObs(31, 110))
	if b.lootID == 8001 {
		t.Fatalf("arm active after armuntil")
	}
}

func TestArmActiveGates(t *testing.T) {
	me := &armObs(5, 0).Self
	cases := []struct {
		flags   string
		latched bool
		tt      float64
		want    bool
	}{
		{"arm=1", false, 5, true},
		{"arm=1,armuntil=60", false, 45, true},
		{"arm=1", false, 31, false},
		{"arm=1,armstrict=1", false, 5, false},
		{"arm=1,armstrict=1", true, 5, true},
		{"arm=1,armwin=1", false, 5, false},
		{"", false, 5, false},
	}
	for _, c := range cases {
		b := armBrain(c.flags, c.latched)
		b.now = c.tt
		if got := b.armActive(me); got != c.want {
			t.Fatalf("armActive(%q, latched %v, t %v) = %v, want %v", c.flags, c.latched, c.tt, got, c.want)
		}
	}
	// holding a second weapon ends it
	b := armBrain("arm=1", false)
	b.now = 5
	armed := &lrObs(5).Self
	if b.armActive(armed) {
		t.Fatalf("armActive while holding a laser")
	}
	// squad params drop Arm unless armprac or armwin
	p := DefaultParams()
	applyFlags(&p, "arm=1")
	if squadParams(p).Arm {
		t.Fatalf("squadParams keeps arm without armprac")
	}
	applyFlags(&p, "armprac=1")
	if !squadParams(p).Arm {
		t.Fatalf("squadParams drops arm with armprac")
	}
}

func TestArmHoldsLongGunFire(t *testing.T) {
	// a gun-only target at 70 m that has not hit us: plain giddy (FFA ghost, engaged when aimed at) may shoot; arm holds
	foe := MechSeen{ID: 3, Kind: "mech", Pos: Vec3{70, 0, 0}, Aim: [2]float64{math.Pi, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
	b := armBrain("arm=1", false)
	for i := 0; i < 3; i++ {
		act := armAct(t, b, armObs(5+0.1*float64(i), 0, foe))
		if act.Fire[0] {
			t.Fatalf("arm opened gun fire at 70 m while gun-only")
		}
	}
}

func TestScanSpins(t *testing.T) {
	b := armBrain("scan=1.6", false)
	o := armObs(0.5, 0)
	act := armAct(t, b, o)
	if act.Aim == nil || math.Abs(angDiff(act.Aim[0], o.Self.Aim[0]+1.2)) > 1e-6 {
		t.Fatalf("scan did not spin the torso: aim %v", act.Aim)
	}
}

func TestArmResetByBreaker(t *testing.T) {
	b := armBrain("arm=1,scan=1.6,cubepush=60", false)
	armAct(t, b, armObs(5, 110))
	b.safeFallback()
	if b.P.Arm || b.base.Arm || b.base.Scan != 0 || b.base.CubePush != 0 {
		t.Fatalf("safeFallback leaves the opening flags on")
	}
}

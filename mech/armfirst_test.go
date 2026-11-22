package mech

import "testing"

// Tests for armfirst (weapon first in the first 20 s, on top of arm). Reuse armBrain / armObs / armAct (arm_test.go).

func TestArmFirstRadiusWithMechNear(t *testing.T) {
	// a mech 140 m away (not near the route) shrinks arm's reach to 120 m; armfirst keeps 160 m
	far := MechSeen{ID: 5, Kind: "mech", Pos: Vec3{140, -20, 0}, Aim: [2]float64{0, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
	arm, first := armBrain("arm=1", false), armBrain("arm=1,armfirst=1", false)
	armAct(t, arm, armObs(5, 140, far))
	armAct(t, first, armObs(5, 140, far))
	if arm.lootID == 8001 {
		t.Fatalf("test setup: arm alone goes 140 m with a mech within 150 m")
	}
	if first.lootID != 8001 {
		t.Fatalf("armfirst did not keep the 160 m reach (lootID %d)", first.lootID)
	}
}

func TestArmFirstSkipsCloseCube(t *testing.T) {
	o := func() *Obs {
		o := armObs(5, 100)
		o.Cubes = []Item{{ID: 5001, Pos: Vec3{4, 0, 0}}}
		return o
	}
	arm, first := armBrain("arm=1", false), armBrain("arm=1,armfirst=1", false)
	armAct(t, arm, o())
	armAct(t, first, o())
	if arm.lootID != 5001 {
		t.Fatalf("test setup: arm alone does not take the cube 4 m away first (lootID %d)", arm.lootID)
	}
	if first.lootID != 8001 {
		t.Fatalf("armfirst detoured for a cube (lootID %d)", first.lootID)
	}
	// from 20 s (PracAt) armfirst is off: arm's own choice again
	late := armBrain("arm=1,armfirst=1", true)
	o2 := o()
	o2.T, o2.Tick = 21, 420
	armAct(t, late, o2)
	if late.lootID != 5001 {
		t.Fatalf("armfirst still active at 21 s (lootID %d)", late.lootID)
	}
}

func TestArmFirstPrefersUnseenPickup(t *testing.T) {
	// lightning 80 m east (seen by a mech at (250, 60)) and laser... both lightning: 90 m west, unseen
	watcher := MechSeen{ID: 5, Kind: "mech", Pos: Vec3{250, 60, 0}, Aim: [2]float64{0, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
	o := func() *Obs {
		o := armObs(5, 0, watcher)
		o.Pickups = []Item{{ID: 8001, Kind: "lightning", Pos: Vec3{80, 0, 0}}, {ID: 8002, Kind: "lightning", Pos: Vec3{-90, 0, 0}}}
		return o
	}
	arm, first := armBrain("arm=1", false), armBrain("arm=1,armfirst=1", false)
	armAct(t, arm, o())
	armAct(t, first, o())
	if arm.lootID != 8001 {
		t.Fatalf("test setup: arm alone does not take the nearest pickup (lootID %d)", arm.lootID)
	}
	if first.lootID != 8002 {
		t.Fatalf("armfirst did not prefer the pickup no mech can see (lootID %d)", first.lootID)
	}
}

func TestArmFirstResetByBreaker(t *testing.T) {
	b := armBrain("arm=1,armfirst=1", false)
	armAct(t, b, armObs(5, 100))
	b.safeFallback()
	if b.base.ArmFirst || b.P.ArmFirst {
		t.Fatalf("safeFallback leaves armfirst on")
	}
}

// Under fire from a foe 100 m away, keep running to a weapon 60 m away; with the foe at 60 m, fight as before.
func TestArmFirstRunsUnderFire(t *testing.T) {
	shot := func(foeDist float64) *Obs {
		foe := MechSeen{ID: 3, Kind: "mech", Pos: Vec3{foeDist, 0, 0}, Aim: [2]float64{3.14159, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
		o := armObs(5, 60, foe)
		by, br := 3, 0.0
		o.Events = []Event{{Type: "damaged", Amount: 45, Weapon: "gun", By: &by, Bearing: &br}}
		return o
	}
	arm, first := armBrain("arm=1", false), armBrain("arm=1,armfirst=1", false)
	armAct(t, arm, shot(100))
	armAct(t, first, shot(100))
	if arm.Mode == "arm:underfire" {
		t.Fatalf("test setup: arm alone runs under fire")
	}
	if first.Mode != "arm:underfire" {
		t.Fatalf("armfirst did not keep arming under fire from 100 m (mode %s, arm alone %s)", first.Mode, arm.Mode)
	}
	close := armBrain("arm=1,armfirst=1", false)
	armAct(t, close, shot(60))
	if close.Mode == "arm:underfire" {
		t.Fatalf("armfirst kept arming with the shooter at 60 m")
	}
}

// A pickup another mech is close to (contest x0.3, and it sees it: /2) loses to an unseen, uncontested one a
// little farther away; arm alone takes the nearest.
func TestArmFirstContestedPickup(t *testing.T) {
	rival := MechSeen{ID: 5, Kind: "mech", Pos: Vec3{0, 230, 0}, Aim: [2]float64{0, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
	o := func() *Obs {
		o := armObs(5, 0, rival)
		o.Pickups = []Item{{ID: 8001, Kind: "lightning", Pos: Vec3{0, 150, 0}}, {ID: 8002, Kind: "lightning", Pos: Vec3{0, -155, 0}}}
		return o
	}
	arm, first := armBrain("arm=1,armrfar=160", false), armBrain("arm=1,armfirst=1", false)
	armAct(t, arm, o())
	armAct(t, first, o())
	if first.lootID != 8002 {
		t.Fatalf("armfirst went for the contested pickup (lootID %d)", first.lootID)
	}
}

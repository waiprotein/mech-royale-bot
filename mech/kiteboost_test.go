package mech

import (
	"reflect"
	"testing"
)

// Tests for kiteboost (practice-side: boost while backing off to our range). They reuse lrConfig / lrTestWorld /
// lrObs.

func kbBrain(e float64, latched bool) *Brain {
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: lrConfig()}, lrTestWorld())
	b.P.KiteBoost = e
	b.notPractice = latched
	return b
}

// kbObs: a full-HP gun mech at d m on +x aiming at us; we hold a gun and a laser (not charging) with energy en.
func kbObs(t, d, en float64) *Obs {
	foe := MechSeen{ID: 3, Kind: "mech", Pos: Vec3{d, 0, 0}, Aim: [2]float64{3.14159, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
	o := lrObs(t, foe)
	o.Self.Energy = en
	return o
}

func kbAct(t *testing.T, b *Brain, o *Obs) Action {
	t.Helper()
	act := b.Act(o)
	if b.Panics != 0 {
		t.Fatalf("step panicked (%d)", b.Panics)
	}
	return act
}

func TestKiteBoostPractice(t *testing.T) {
	// squad play, target at 30 m (preferred laser range 120): plain giddy backs off at walk speed
	plain := kbBrain(0, false)
	a := kbAct(t, plain, kbObs(30, 30, 100))
	if !plain.pracOn || plain.Mode != "fight" || a.Boost || a.Move[0] >= 0 {
		t.Fatalf("test setup: want squad fight backing off without boost (pracOn %v, mode %s, boost %v, move %v)",
			plain.pracOn, plain.Mode, a.Boost, a.Move)
	}
	b := kbBrain(30, false)
	a = kbAct(t, b, kbObs(30, 30, 100))
	if !a.Boost || a.Move[0] >= 0 {
		t.Fatalf("kiteboost did not boost away (boost %v, move %v)", a.Boost, a.Move)
	}
	// low energy: no boost
	b = kbBrain(30, false)
	if a = kbAct(t, b, kbObs(30, 30, 25)); a.Boost {
		t.Fatalf("kiteboost boosted below its energy floor")
	}
	// already near our range (110 m vs 120): no boost
	b = kbBrain(30, false)
	if a = kbAct(t, b, kbObs(30, 110, 100)); a.Boost {
		t.Fatalf("kiteboost boosted outside the 15 m margin")
	}
	// laser charging (half speed): no boost
	b = kbBrain(30, false)
	o := kbObs(30, 30, 100)
	o.Self.Weapons[1].Charge = 0.3
	if a = kbAct(t, b, o); a.Boost {
		t.Fatalf("kiteboost boosted while charging the laser")
	}
}

// After the FFA proof (ffaOn) kiteboost does nothing.
func TestKiteBoostInertAfterFFAProof(t *testing.T) {
	plain, kb := kbBrain(0, true), kbBrain(30, true)
	for _, tm := range []float64{30, 30.1, 30.2, 30.5} {
		a, c := kbAct(t, plain, kbObs(tm, 40, 100)), kbAct(t, kb, kbObs(tm, 40, 100))
		if !reflect.DeepEqual(a, c) {
			t.Fatalf("kiteboost changed post-latch play at t=%v", tm)
		}
	}
}

func TestKiteBoostResetByBreaker(t *testing.T) {
	b := kbBrain(30, false)
	kbAct(t, b, kbObs(30, 30, 100))
	b.safeFallback()
	if b.P.KiteBoost != 0 || b.base.KiteBoost != 0 {
		t.Fatalf("safeFallback leaves kiteboost on")
	}
}

// kitew: with a target well inside our range, fightMove backs off more directly than giddy's strafe-heavy step.
func TestKiteWBacksOff(t *testing.T) {
	plain := kbBrain(0, false)
	a := kbAct(t, plain, kbObs(30, 30, 100))
	b := kbBrain(0, false)
	b.P.KiteW = 2.5
	c := kbAct(t, b, kbObs(30, 30, 100))
	if !(-c.Move[0] > -a.Move[0]+0.1) {
		t.Fatalf("kitew did not back off more directly: plain move %v, kitew move %v", a.Move, c.Move)
	}
	// not when the target is near our range (110 m vs 120)
	plain, b = kbBrain(0, false), kbBrain(0, false)
	b.P.KiteW = 2.5
	if a, c = kbAct(t, plain, kbObs(30, 110, 100)), kbAct(t, b, kbObs(30, 110, 100)); !reflect.DeepEqual(a, c) {
		t.Fatalf("kitew changed the move outside the 15 m margin")
	}
	// inert after the FFA proof
	plain, b = kbBrain(0, true), kbBrain(0, true)
	b.P.KiteW = 2.5
	if a, c = kbAct(t, plain, kbObs(30, 30, 100)), kbAct(t, b, kbObs(30, 30, 100)); !reflect.DeepEqual(a, c) {
		t.Fatalf("kitew changed post-latch play")
	}
}

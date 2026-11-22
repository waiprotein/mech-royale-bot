package mech

import (
	"reflect"
	"testing"
)

// Tests for pracregen (practice-side regen-hide latch). They reuse lrConfig / lrTestWorld / lrObs.

func prBrain(frac float64, latched bool) *Brain {
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: lrConfig()}, lrTestWorld())
	b.P.PracRegen = frac
	b.notPractice = latched
	return b
}

// prObs: we have hp HP; mech 3 is 170 m away on +x, full HP, aiming away and not closing (not engaged by squad
// rules), so there is no target or foe.
func prObs(t, hp float64) *Obs {
	far := MechSeen{ID: 3, Kind: "mech", Pos: Vec3{170, 0, 0}, Aim: [2]float64{0, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
	o := lrObs(t, far)
	o.Self.HP = hp
	return o
}

func prAct(t *testing.T, b *Brain, o *Obs) Action {
	t.Helper()
	act := b.Act(o)
	if b.Panics != 0 {
		t.Fatalf("step panicked (%d)", b.Panics)
	}
	return act
}

func TestPracRegenLatch(t *testing.T) {
	plain := prBrain(0, false)
	prAct(t, plain, prObs(30, 600))
	if !plain.pracOn || plain.regenOn {
		t.Fatalf("test setup: want squad params and no regen latch (pracOn %v, regenOn %v)", plain.pracOn, plain.regenOn)
	}
	b := prBrain(0.6, false)
	prAct(t, b, prObs(30, 600))
	if !b.regenOn {
		t.Fatalf("pracregen did not latch at 43%% HP with no foe")
	}
	// stays latched while regenerating, releases at 90%
	prAct(t, b, prObs(35, 1000))
	if !b.regenOn {
		t.Fatalf("pracregen released at 71%% HP")
	}
	prAct(t, b, prObs(40, 1270))
	if b.regenOn {
		t.Fatalf("pracregen still latched at 91%% HP")
	}
	// above the threshold: no latch
	b = prBrain(0.6, false)
	prAct(t, b, prObs(30, 900))
	if b.regenOn {
		t.Fatalf("pracregen latched at 64%% HP")
	}
	// with an engaged mech (within 60 m) it does not latch
	b = prBrain(0.6, false)
	o := prObs(30, 600)
	o.Mechs[0].Pos = Vec3{50, 0, 0}
	prAct(t, b, o)
	if b.regenOn {
		t.Fatalf("pracregen latched with an engaged mech at 50 m")
	}
}

// After the FFA proof (ffaOn) pracregen does nothing (regenhide is the FFA version).
func TestPracRegenInertAfterFFAProof(t *testing.T) {
	plain, pr := prBrain(0, true), prBrain(0.6, true)
	for _, tm := range []float64{30, 30.1, 30.5, 31} {
		a, c := prAct(t, plain, prObs(tm, 600)), prAct(t, pr, prObs(tm, 600))
		if !reflect.DeepEqual(a, c) || pr.regenOn {
			t.Fatalf("pracregen changed post-latch play at t=%v", tm)
		}
	}
}

func TestPracRegenResetByBreaker(t *testing.T) {
	b := prBrain(0.6, false)
	prAct(t, b, prObs(30, 600))
	b.safeFallback()
	if b.P.PracRegen != 0 || b.base.PracRegen != 0 || b.regenOn {
		t.Fatalf("safeFallback leaves pracregen on")
	}
}

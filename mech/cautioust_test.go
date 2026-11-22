package mech

import (
	"reflect"
	"testing"
)

// Tests for cautioust (late practice play: squad hunt -> giddy's cautious ghost play). Reuse lrConfig / lrObs.

func cautBrain(t0 float64, latched bool) *Brain {
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: lrConfig()}, lrTestWorld())
	b.P.CautiousT = t0
	b.notPractice = latched
	return b
}

func cautObs(t, nextR float64) *Obs {
	o := lrObs(t)
	o.Self.Vel = Vec3{0, 11.7, 0}
	o.Zone.NextRadius = nextR
	return o
}

func cautAct(t *testing.T, b *Brain, o *Obs) Action {
	t.Helper()
	act := b.Act(o)
	if b.Panics != 0 {
		t.Fatalf("step panicked (%d)", b.Panics)
	}
	return act
}

func TestCautiousTSwitch(t *testing.T) {
	plain, caut := cautBrain(0, false), cautBrain(265, false)
	// before 265 s: identical play, squad params on after PracAt
	for _, tt := range []float64{5, 19.9, 20.5, 100, 200, 264.9} {
		a, c := cautAct(t, plain, cautObs(tt, 240)), cautAct(t, caut, cautObs(tt, 240))
		if !reflect.DeepEqual(a, c) {
			t.Fatalf("cautioust changed play before 265 s (t=%v)", tt)
		}
	}
	if !caut.pracOn || caut.P.Ghost {
		t.Fatalf("test setup: squad params not on at 264.9 s")
	}
	cautAct(t, plain, cautObs(265.1, 240))
	cautAct(t, caut, cautObs(265.1, 240))
	if !plain.pracOn {
		t.Fatalf("test setup: plain build left squad params at 265 s")
	}
	if caut.pracOn || !caut.P.Ghost {
		t.Fatalf("cautioust=265 still hunting at 265.1 s (pracOn %v, Ghost %v)", caut.pracOn, caut.P.Ghost)
	}
	// it stays cautious
	cautAct(t, caut, cautObs(300, 240))
	if caut.pracOn {
		t.Fatalf("squad params came back after the cautious switch")
	}
	// next circle <= 72 m switches earlier
	b := cautBrain(265, false)
	cautAct(t, b, cautObs(100, 240))
	cautAct(t, b, cautObs(200, 70))
	if b.pracOn {
		t.Fatalf("still hunting with the next circle at 70 m")
	}
}

// After a proven FFA squad params are already off: cautioust changes nothing.
func TestCautiousTInertAfterFFAProof(t *testing.T) {
	plain, caut := cautBrain(0, true), cautBrain(265, true)
	for _, tt := range []float64{100, 264, 266, 300} {
		a, c := cautAct(t, plain, cautObs(tt, 70)), cautAct(t, caut, cautObs(tt, 70))
		if !reflect.DeepEqual(a, c) {
			t.Fatalf("cautioust changed post-latch play at t=%v", tt)
		}
	}
}

func TestCautiousTResetByBreaker(t *testing.T) {
	b := cautBrain(265, false)
	cautAct(t, b, cautObs(100, 240))
	b.safeFallback()
	if b.base.CautiousT != 0 || b.P.CautiousT != 0 {
		t.Fatalf("safeFallback leaves cautioust on")
	}
}

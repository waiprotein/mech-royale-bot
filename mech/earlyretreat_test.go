package mech

import (
	"reflect"
	"testing"
)

// Tests for earlyretreat. They reuse lrConfig / lrTestWorld / lrObs from lrlaser_test.go.

func erBrain(window float64, latched bool) *Brain {
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: lrConfig()}, lrTestWorld())
	b.P.EarlyRetreat = window
	b.notPractice = latched
	return b
}

// erObs: we have myHP of 1400; mech 3, 50 m east and aimed at us, with hp/kit as given, has just hit us.
func erObs(t, myHP, itsHP float64, gunOnly bool, kit ...string) *Obs {
	o := lrObs(t, MechSeen{ID: 3, Kind: "mech", Pos: Vec3{50, 0, 0}, Aim: [2]float64{3.14159, 0}, HP: itsHP, MaxHP: 1400, Weapons: kit})
	o.Self.HP = myHP
	o.Self.Vel = Vec3{0, 11.7, 0}
	if gunOnly {
		o.Self.Weapons = o.Self.Weapons[:1]
		o.Self.Weapons = append(o.Self.Weapons, nil)
	}
	by, br := 3, 0.0
	o.Events = []Event{{Type: "damaged", Amount: 45, Weapon: "gun", By: &by, Bearing: &br}}
	return o
}

func erAct(t *testing.T, b *Brain, o *Obs) {
	t.Helper()
	b.Act(o)
	if b.Panics != 0 {
		t.Fatalf("step panicked (%d)", b.Panics)
	}
}

func TestEarlyRetreatInertWithoutLatch(t *testing.T) {
	plain, strict := erBrain(0, false), erBrain(60, false)
	for _, tt := range []float64{5, 10, 19.9, 20.5, 30, 45} {
		o1, o2 := erObs(tt, 1000, 1400, false, "gun", "laser"), erObs(tt, 1000, 1400, false, "gun", "laser")
		a, c := plain.Act(o1), strict.Act(o2)
		if !reflect.DeepEqual(a, c) {
			t.Fatalf("earlyretreat without the latch changed play at t=%v", tt)
		}
	}
}

func TestEarlyRetreatTriggers(t *testing.T) {
	cases := []struct {
		name        string
		t, my, its  float64
		gunOnly     bool
		kit         []string
		window      float64
		wantLatched bool
	}{
		{"losing trade at 30 s", 30, 1000, 1400, false, []string{"gun", "laser"}, 60, true},
		{"even trade", 30, 1000, 1050, false, []string{"gun", "laser"}, 60, false},
		{"we are above 80% HP", 30, 1150, 1400, false, []string{"gun", "laser"}, 60, false},
		{"gun-only vs lightning, full HP", 30, 1400, 1400, true, []string{"gun", "lightning"}, 60, true},
		{"gun-only vs gun-only, full HP", 30, 1400, 1400, true, []string{"gun"}, 60, false},
		{"losing trade after the window", 70, 1000, 1400, false, []string{"gun", "laser"}, 60, false},
		{"gun-only vs armed but finishing it (150 HP at 50 m)", 30, 1400, 150, true, []string{"gun", "laser"}, 60, false},
		{"flag off", 30, 1000, 1400, false, []string{"gun", "laser"}, 0, false},
	}
	for _, c := range cases {
		b := erBrain(c.window, true)
		erAct(t, b, erObs(c.t, c.my, c.its, c.gunOnly, c.kit...))
		got := b.oddsFleeUntil > c.t
		if got != c.wantLatched {
			t.Fatalf("%s: flee latched = %v, want %v (until %v, mode %s)", c.name, got, c.wantLatched, b.oddsFleeUntil, b.Mode)
		}
		if got && (b.oddsFleeUntil < c.t+5.4 || !b.retreat) {
			t.Fatalf("%s: latch %v s / retreat %v, want 5.5 s and retreat", c.name, b.oddsFleeUntil-c.t, b.retreat)
		}
	}
}

func TestEarlyRetreatResetBySquadAndBreaker(t *testing.T) {
	p := DefaultParams()
	p.EarlyRetreat = 60
	if squadParams(p).EarlyRetreat != 0 {
		t.Fatalf("squadParams keeps earlyretreat on")
	}
	b := erBrain(60, true)
	erAct(t, b, erObs(30, 1400, 1400, false, "gun"))
	b.safeFallback()
	if b.P.EarlyRetreat != 0 || b.base.EarlyRetreat != 0 {
		t.Fatalf("safeFallback leaves earlyretreat on")
	}
}

package mech

import (
	"reflect"
	"testing"
)

// Tests for lcdpunish (practice-side laser-cooldown punish). They reuse lrConfig / lrTestWorld / lrObs.

func lcdBrain(window float64, latched bool) *Brain {
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: lrConfig()}, lrTestWorld())
	b.P.LCDPunish = window
	b.notPractice = latched
	return b
}

// lcdFoe: a laser holder at (d, 0) with the given HP, aimed away from us (yaw 0) unless aimedAtUs.
func lcdFoe(d, hp float64, aimedAtUs bool) MechSeen {
	yaw := 0.0
	if aimedAtUs {
		yaw = 3.14159
	}
	return MechSeen{ID: 3, Kind: "mech", Pos: Vec3{d, 0, 0}, Aim: [2]float64{yaw, 0}, HP: hp, MaxHP: 1400, Weapons: []string{"gun", "laser"}}
}

// lcdObs at time t; fired adds a laser beam from mech 3 this tick (aimed elsewhere).
func lcdObs(t float64, foe MechSeen, fired bool) *Obs {
	o := lrObs(t, foe)
	o.Self.Vel = Vec3{0, 11.7, 0}
	if fired {
		o.Beams = []Beam{{By: 3, Kind: "laser", Path: []Vec3{{foe.Pos[0], foe.Pos[1], 7}, {foe.Pos[0] + 100, 50, 4}}, Age: 0.05}}
	}
	return o
}

func lcdAct(t *testing.T, b *Brain, o *Obs) Action {
	t.Helper()
	act := b.Act(o)
	if b.Panics != 0 {
		t.Fatalf("step panicked (%d)", b.Panics)
	}
	return act
}

// After the FFA proof (ffaOn) lcdpunish does nothing: C1's FFA brain is unchanged.
func TestLCDPunishInertAfterFFAProof(t *testing.T) {
	plain, lcd := lcdBrain(0, true), lcdBrain(1.5, true)
	for i, tt := range []float64{30, 30.1, 30.2, 30.3, 31, 32} {
		fired := i == 1
		a, c := lcdAct(t, plain, lcdObs(tt, lcdFoe(55, 1000, false), fired)), lcdAct(t, lcd, lcdObs(tt, lcdFoe(55, 1000, false), fired))
		if !reflect.DeepEqual(a, c) {
			t.Fatalf("lcdpunish changed post-latch play at t=%v", tt)
		}
	}
}

func TestLCDPunishPractice(t *testing.T) {
	// plain squad play engages a laser holder at 55 m (inside EngageDist)
	plain := lcdBrain(0, false)
	lcdAct(t, plain, lcdObs(30, lcdFoe(55, 1000, false), false))
	if !plain.tracks[3].Engage {
		t.Fatalf("test setup: plain squad play does not engage the mech at 55 m")
	}
	// lcdpunish: a fresh laser holder whose laser is ready, not aiming at us: not opened on
	b := lcdBrain(1.5, false)
	lcdAct(t, b, lcdObs(30, lcdFoe(55, 1000, false), false))
	if b.tracks[3].Engage {
		t.Fatalf("opened on a laser holder with a ready laser")
	}
	// ... it fires (beam): inside the window we engage it and close to <= 50 m
	lcdAct(t, b, lcdObs(30.1, lcdFoe(55, 1000, false), true))
	if !b.tracks[3].Engage {
		t.Fatalf("did not engage a laser holder in its cooldown window")
	}
	if r := b.prefRange(&lcdObs(30.1, lcdFoe(55, 1000, false), false).Self, b.tracks[3]); r > 50 {
		t.Fatalf("preferred range %.0f m in the cooldown window, want <= 50", r)
	}
	// ... window over (1.5 s later), still no hits either way: back to not opening
	lcdAct(t, b, lcdObs(31.8, lcdFoe(55, 1000, false), false))
	if b.tracks[3].Engage {
		t.Fatalf("still engaged after the window, with no exchange")
	}
	// in the window but it has more HP than us: no forced engagement at 150 m
	b = lcdBrain(1.5, false)
	o := lcdObs(30, lcdFoe(150, 1400, false), true)
	o.Self.HP = 1000
	lcdAct(t, b, o)
	if b.tracks[3].Engage {
		t.Fatalf("forced an engagement on a laser holder with more HP than us")
	}
	// ... with our HP >= its HP the window engages it at 150 m too
	b = lcdBrain(1.5, false)
	lcdAct(t, b, lcdObs(30, lcdFoe(150, 1000, false), true))
	if !b.tracks[3].Engage {
		t.Fatalf("did not engage a weaker laser holder at 150 m in its cooldown window")
	}
	// a laser holder aiming at us is not suppressed (the fight is on)
	b = lcdBrain(1.5, false)
	lcdAct(t, b, lcdObs(30, lcdFoe(55, 1000, true), false))
	if !b.tracks[3].Engage {
		t.Fatalf("suppressed a laser holder that is aiming at us")
	}
	// a laser sound on its bearing (no beam) also opens the window
	b = lcdBrain(1.5, false)
	o = lcdObs(30, lcdFoe(55, 1000, false), false)
	o.Sounds = []Sound{{Kind: "laser", Bearing: 0.05, Dist: "near", Age: 0.05}}
	lcdAct(t, b, o)
	if !b.lcdWindow(b.tracks[3]) || !b.tracks[3].Engage {
		t.Fatalf("laser sound on its bearing did not open the window (LaserAt %v)", b.tracks[3].LaserAt)
	}
}

func TestLCDPunishResetByBreaker(t *testing.T) {
	b := lcdBrain(1.5, false)
	lcdAct(t, b, lcdObs(30, lcdFoe(55, 1000, false), false))
	b.safeFallback()
	if b.P.LCDPunish != 0 || b.base.LCDPunish != 0 {
		t.Fatalf("safeFallback leaves lcdpunish on")
	}
}

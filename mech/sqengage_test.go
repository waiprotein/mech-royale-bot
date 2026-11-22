package mech

import (
	"reflect"
	"testing"
)

// Tests for sqengage (squad-mode EngageDist override). Reuse sqBrain / sqAct (squadrim_test.go) and lrObs.

// seObs: a calm full-HP gun mech at d m on +x, aiming away from us and standing still.
func seObs(t, d float64) *Obs {
	m := MechSeen{ID: 3, Kind: "mech", Pos: Vec3{d, 0, 0}, Aim: [2]float64{0, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
	return lrObs(t, m)
}

func TestSqEngageParams(t *testing.T) {
	p := DefaultParams()
	if q := squadParams(p); q.EngageDist != p.EngageDist {
		t.Fatalf("squadParams changed EngageDist without sqengage (%v)", q.EngageDist)
	}
	applyFlags(&p, "sqengage=80")
	if q := squadParams(p); q.EngageDist != 80 || p.EngageDist != 60 {
		t.Fatalf("sqengage=80: squad %v (want 80), base %v (want 60)", q.EngageDist, p.EngageDist)
	}
	b := sqBrain("sqengage=80", false)
	sqAct(t, b, seObs(30, 70))
	b.safeFallback()
	if b.P.SqEngage != 0 || b.base.SqEngage != 0 || b.P.EngageDist != 60 {
		t.Fatalf("safeFallback leaves sqengage on (EngageDist %v)", b.P.EngageDist)
	}
}

func TestSqEngageSquadOnly(t *testing.T) {
	plain, se := sqBrain("", false), sqBrain("sqengage=80", false)
	sqAct(t, plain, seObs(30, 70))
	sqAct(t, se, seObs(30, 70))
	if !plain.pracOn || plain.tracks[3] == nil || plain.tracks[3].Engage {
		t.Fatalf("test setup: want squad play not engaging a calm mech at 70 m")
	}
	if !se.tracks[3].Engage {
		t.Fatalf("sqengage=80 did not engage a calm mech at 70 m in squad play")
	}
	// beyond the override: still not engaged
	se = sqBrain("sqengage=80", false)
	if sqAct(t, se, seObs(30, 90)); se.tracks[3].Engage {
		t.Fatalf("sqengage=80 engaged a calm mech at 90 m")
	}
	// ghost play before PracAt and FFA play after the latch: identical to plain
	for _, c := range []struct {
		latched bool
		tt      float64
	}{{false, 5}, {true, 30}, {true, 120}} {
		a, b := sqBrain("", c.latched), sqBrain("sqengage=80", c.latched)
		for k := 0; k < 5; k++ {
			tt := c.tt + float64(k)*0.05
			if x, y := sqAct(t, a, seObs(tt, 70)), sqAct(t, b, seObs(tt, 70)); !reflect.DeepEqual(x, y) {
				t.Fatalf("sqengage changed non-squad play (latched %v, t %v)", c.latched, tt)
			}
		}
		if b.pracOn || b.P.EngageDist != 60 {
			t.Fatalf("test setup: squad params %v, EngageDist %v (latched %v, t %v)", b.pracOn, b.P.EngageDist, c.latched, c.tt)
		}
	}
}

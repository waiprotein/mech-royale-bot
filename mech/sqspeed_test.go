package mech

import (
	"reflect"
	"testing"
)

// Tests for sqspeed / sqdodge (speed upgrade order and dodge boost while !ffaOn). Reuse lrConfig / lrTestWorld /
// lrObs and sqBrain / sqAct from squadrim_test.go.

// ssObs: no mechs at t; optionally an upgrade offer (laser_damage, servos, gun_damage) and a gun bullet 20 m out
// on +x flying straight at us.
func ssObs(t float64, offer, shot bool, energy float64) *Obs {
	o := lrObs(t)
	o.Self.Energy = energy
	if offer {
		o.Offer = &Offer{ID: 1, Expires: t + 10, Choices: []OfferChoice{{ID: "laser_damage"}, {ID: "servos"}, {ID: "gun_damage"}}}
	}
	if shot {
		o.Projectiles = []Proj{{ID: 9, Kind: "gun", Pos: Vec3{20, 0, 5}, Vel: Vec3{-250, 0, 0}}}
	}
	return o
}

func TestSqSpeedFlags(t *testing.T) {
	var p Params
	applyFlags(&p, "sqspeed=1,sqdodge=25")
	if !p.SqSpeed || p.SqDodge != 25 {
		t.Fatalf("flags not parsed: %v %v", p.SqSpeed, p.SqDodge)
	}
	b := sqBrain("sqspeed=1,sqdodge=25", false)
	sqAct(t, b, ssObs(30, false, false, 100))
	b.safeFallback()
	if b.P.SqSpeed || b.base.SqSpeed || b.P.SqDodge != 0 || b.base.SqDodge != 0 {
		t.Fatalf("safeFallback leaves sqspeed/sqdodge on")
	}
}

func TestSqSpeedUpgradeOrder(t *testing.T) {
	pick := func(flags string, latched bool, tt float64) int {
		b := sqBrain(flags, latched)
		act := sqAct(t, b, ssObs(tt, true, false, 100))
		if act.Pick == nil {
			t.Fatalf("no pick (%s, latched %v, t %v)", flags, latched, tt)
		}
		return *act.Pick
	}
	// squad play (laser order): laser_damage; with sqspeed: servos
	if p, q := pick("", false, 30), pick("sqspeed=1", false, 30); p != 0 || q != 1 {
		t.Fatalf("squad play: plain pick %d (want 0), sqspeed pick %d (want 1)", p, q)
	}
	// pre-latch base play (t < PracAt): giddy's base order picks gun_damage; with sqspeed: servos
	if p, q := pick("", false, 5), pick("sqspeed=1", false, 5); p != 2 || q != 1 {
		t.Fatalf("pre-latch: plain pick %d (want 2), sqspeed pick %d (want 1)", p, q)
	}
	// after the FFA proof: unchanged
	if p, q := pick("", true, 30), pick("sqspeed=1", true, 30); p != q {
		t.Fatalf("sqspeed changed the FFA pick (%d vs %d)", p, q)
	}
}

func TestSqDodgeBoost(t *testing.T) {
	act := func(flags string, latched bool, tt, energy float64) Action {
		b := sqBrain(flags, latched)
		return sqAct(t, b, ssObs(tt, false, true, energy))
	}
	a := act("", false, 30, 100)
	if a.Boost || (a.Move[0] == 0 && a.Move[1] == 0) {
		t.Fatalf("test setup: want a dodge step without boost (boost %v, move %v)", a.Boost, a.Move)
	}
	for _, tt := range []float64{5, 30} { // pre-latch base play and squad play
		if c := act("sqdodge=25", false, tt, 100); !c.Boost {
			t.Fatalf("sqdodge did not boost the dodge at t=%v", tt)
		}
	}
	if c := act("sqdodge=25", false, 30, 20); c.Boost {
		t.Fatalf("sqdodge boosted below its energy floor")
	}
	// no shot: no boost
	b := sqBrain("sqdodge=25", false)
	if c := sqAct(t, b, ssObs(30, false, false, 100)); c.Boost {
		t.Fatalf("sqdodge boosted with nothing to dodge")
	}
	// after the FFA proof: identical to plain
	if x, y := act("", true, 30, 100), act("sqdodge=25", true, 30, 100); !reflect.DeepEqual(x, y) {
		t.Fatalf("sqdodge changed FFA play")
	}
}

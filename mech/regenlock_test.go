package mech

import (
	"reflect"
	"testing"
)

// Tests for regenlock (squad mode: stay on, and chase, a mech we hurt until its regen window runs out). Reuse
// sqBrain / sqAct (squadrim_test.go) and lrObs.

// rlA: mech 3 at 130 m on +x, aiming at us; rlB: mech 4 at 70 m on +y, aiming at us, full HP.
func rlA(hp float64) MechSeen {
	return MechSeen{ID: 3, Kind: "mech", Pos: Vec3{130, 0, 0}, Aim: [2]float64{3.14159, 0}, HP: hp, MaxHP: 1400, Weapons: []string{"gun"}}
}

func rlB() MechSeen {
	return MechSeen{ID: 4, Kind: "mech", Pos: Vec3{0, 70, 0}, Aim: [2]float64{-1.5708, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
}

func rlHit(o *Obs, id int, amount float64) *Obs {
	o.Events = append(o.Events, Event{Type: "hit", Target: id, Amount: amount, Weapon: "gun"})
	return o
}

func TestRegenLockFlags(t *testing.T) {
	var p Params
	applyFlags(&p, "regenlock=1,regenlockhp=0.5")
	if !p.RegenLock || p.RegenLockHP != 0.5 {
		t.Fatalf("flags not parsed")
	}
	b := sqBrain("regenlock=1", false)
	sqAct(t, b, rlHit(lrObs(30, rlA(1300)), 3, 100))
	b.safeFallback()
	if b.P.RegenLock || b.base.RegenLock {
		t.Fatalf("safeFallback leaves regenlock on")
	}
}

func TestRegenLockWindow(t *testing.T) {
	b := sqBrain("regenlock=1", false)
	sqAct(t, b, rlHit(lrObs(30, rlA(1300)), 3, 100))
	sqAct(t, b, rlHit(lrObs(32, rlA(1200)), 3, 100))
	if w := b.regenLive(3); w == nil || w.dmg != 200 || w.start != 30 {
		t.Fatalf("window after two hits 2 s apart: %+v", w)
	}
	sqAct(t, b, lrObs(36.9, rlA(1200)))
	if b.regenLive(3) == nil {
		t.Fatalf("window lapsed 4.9 s after the last hit")
	}
	sqAct(t, b, lrObs(37.1, rlA(1200)))
	if b.regenLive(3) != nil {
		t.Fatalf("window still live 5.1 s after the last hit")
	}
	sqAct(t, b, rlHit(lrObs(38, rlA(1200)), 3, 50))
	if w := b.regenLive(3); w == nil || w.dmg != 50 || w.start != 38 {
		t.Fatalf("a hit after the lapse did not start a new window: %+v", w)
	}
}

func TestRegenLockSticksToHurtTarget(t *testing.T) {
	// plain squad play prefers the nearer fresh mech B; with regenlock, the mech we just hurt (A) stays the target
	plain, rl := sqBrain("", false), sqBrain("regenlock=1", false)
	for _, b := range []*Brain{plain, rl} {
		sqAct(t, b, rlHit(lrObs(30, rlA(1300), rlB()), 3, 300))
	}
	if !plain.pracOn || plain.stickID != 4 {
		t.Fatalf("test setup: plain squad play should pick B (picked %d)", plain.stickID)
	}
	if rl.stickID != 3 {
		t.Fatalf("regenlock did not stay on the hurt mech (picked %d)", rl.stickID)
	}
}

func TestRegenLockChase(t *testing.T) {
	start := func(flags string, latched bool, tt float64) *Brain {
		b := sqBrain(flags, latched)
		sqAct(t, b, rlHit(lrObs(tt, rlA(1000)), 3, 400))
		return b
	}
	// A breaks line of sight 1 s later: walk to where it was
	b := start("regenlock=1", false, 30)
	act := sqAct(t, b, lrObs(31))
	if b.Mode != "regenchase" || act.Move[0] < 0.5 {
		t.Fatalf("no chase after the hurt mech left sight (mode %s, move %v)", b.Mode, act.Move)
	}
	plain := start("", false, 30)
	if sqAct(t, plain, lrObs(31)); plain.Mode == "regenchase" {
		t.Fatalf("plain play chased")
	}
	// window lapsed: no chase
	b = start("regenlock=1", false, 30)
	if sqAct(t, b, lrObs(35.5)); b.Mode == "regenchase" {
		t.Fatalf("chased after the regen window ran out")
	}
	// our HP below 40%: no chase
	b = start("regenlock=1", false, 30)
	o := lrObs(31)
	o.Self.HP = 500
	if sqAct(t, b, o); b.Mode == "regenchase" {
		t.Fatalf("chased at 36%% HP")
	}
	// another mech seen within 60 m: no chase
	b = start("regenlock=1", false, 30)
	near := MechSeen{ID: 5, Kind: "mech", Pos: Vec3{-50, 0, 0}, Aim: [2]float64{3.14159, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
	if sqAct(t, b, lrObs(31, near)); b.Mode == "regenchase" {
		t.Fatalf("chased with another mech at 50 m")
	}
	// too far to reach weapon range before the window ends (gun-only reach 130 m; A last seen 600 m away)
	b = sqBrain("regenlock=1", false)
	far := rlA(1000)
	far.Pos = Vec3{600, 0, 0}
	o = rlHit(lrObs(30, far), 3, 400)
	o.Self.Weapons = o.Self.Weapons[:1]
	sqAct(t, b, o)
	o = lrObs(31)
	o.Self.Weapons = o.Self.Weapons[:1]
	if sqAct(t, b, o); b.Mode == "regenchase" {
		t.Fatalf("chased a mech it cannot reach inside the window")
	}
}

// Before PracAt (ghost play) and after the FFA proof, regenlock changes nothing.
func TestRegenLockInertOutsideSquad(t *testing.T) {
	for _, c := range []struct {
		latched bool
		tt      float64
	}{{false, 5}, {true, 30}, {true, 120}} {
		a, b := sqBrain("", c.latched), sqBrain("regenlock=1", c.latched)
		script := []*Obs{
			rlHit(lrObs(c.tt, rlA(1300), rlB()), 3, 300),
			lrObs(c.tt+0.5, rlA(1250), rlB()),
			lrObs(c.tt+1, rlB()),
			lrObs(c.tt + 1.5),
			lrObs(c.tt + 2),
		}
		for i, o := range script {
			o2 := *o
			o2.Events = append([]Event(nil), o.Events...)
			o2.Mechs = append([]MechSeen(nil), o.Mechs...)
			if x, y := sqAct(t, a, o), sqAct(t, b, &o2); !reflect.DeepEqual(x, y) {
				t.Fatalf("regenlock changed non-squad play (latched %v, t %v, step %d)", c.latched, c.tt, i)
			}
		}
		if b.pracOn {
			t.Fatalf("test setup: squad params on (latched %v, t %v)", c.latched, c.tt)
		}
	}
}

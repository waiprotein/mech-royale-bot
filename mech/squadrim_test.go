package mech

import (
	"math"
	"reflect"
	"testing"
)

// Tests for the squadrim family (sqrim / sqloot / sqband / sqret). Reuse lrConfig / lrTestWorld / lrObs.

func sqBrain(flags string, latched bool) *Brain {
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: lrConfig()}, lrTestWorld())
	applyFlags(&b.P, flags)
	b.notPractice = latched
	return b
}

func sqAct(t *testing.T, b *Brain, o *Obs) Action {
	t.Helper()
	act := b.Act(o)
	if b.Panics != 0 {
		t.Fatalf("step panicked (%d)", b.Panics)
	}
	return act
}

// sqIdleObs: no mechs; we stand at (px, py); the next circle is centred on the origin with radius nextR.
func sqIdleObs(t, px, py, nextR float64, mechs ...MechSeen) *Obs {
	o := lrObs(t, mechs...)
	o.Self.Pos, o.Self.Vel = Vec3{px, py, 0}, Vec3{0, 0, 0}
	o.Zone.NextRadius = nextR
	return o
}

// sqScript is a 3 s scene that walks through every squadrim part: a cube deep inside the circle at 70 m, then a
// gun mech at 100 m, then that mech hitting us while we are at 25% HP.
func sqScript(t0 float64) []*Obs {
	var out []*Obs
	by := 3
	for k := 0; k < 60; k++ {
		t := t0 + float64(k)*0.05
		o := sqIdleObs(t, 10, 0, 380)
		o.Tick = int(t*20) + k
		o.Cubes = []Item{{ID: 50, Kind: "cube", Pos: Vec3{60, 50, 0}}}
		o.Pickups = []Item{{ID: 60, Kind: "laser", Pos: Vec3{-80, 60, 0}}}
		if k >= 20 {
			o.Mechs = []MechSeen{{ID: 3, Kind: "mech", Pos: Vec3{110, 5, 0}, Aim: [2]float64{math.Pi, 0}, HP: 1300, MaxHP: 1400, Weapons: []string{"gun"}}}
		}
		if k >= 40 {
			o.Self.HP = 350
			b := math.Pi / 2
			o.Events = []Event{{Type: "damaged", Amount: 45, Weapon: "gun", By: &by, Bearing: &b}}
		}
		out = append(out, o)
	}
	return out
}

func runScript(t *testing.T, b *Brain, obs []*Obs) []Action {
	t.Helper()
	var acts []Action
	for _, o := range obs {
		acts = append(acts, sqAct(t, b, o))
	}
	return acts
}

func TestSquadRimFlags(t *testing.T) {
	var p Params
	applyFlags(&p, "squadrim=1")
	if p.SqRim != 0.9 || p.SqLootIn != 50 || !p.SqBand || p.SqRet != 0.30 {
		t.Fatalf("squadrim=1 set %v %v %v %v", p.SqRim, p.SqLootIn, p.SqBand, p.SqRet)
	}
	applyFlags(&p, "sqband=0,sqret=0")
	if p.SqBand || p.SqRet != 0 || p.SqRim != 0.9 || p.SqLootIn != 50 {
		t.Fatalf("sub-flags did not drop parts independently")
	}
	q := DefaultParams()
	applyFlags(&q, "squadrim=0")
	if !reflect.DeepEqual(q, DefaultParams()) {
		t.Fatalf("squadrim=0 changed the default params")
	}
	b := sqBrain("squadrim=1", false)
	sqAct(t, b, sqIdleObs(30, 10, 0, 380))
	b.safeFallback()
	if b.base.SqRim != 0 || b.P.SqRim != 0 || b.P.SqBand || b.P.SqRet != 0 || b.P.SqLootIn != 0 || b.sqRetOn {
		t.Fatalf("safeFallback leaves squadrim on")
	}
}

// Flag off = identical play; flag on outside squad mode (before PracAt, after the FFA proof) = identical play.
func TestSquadRimInertWhenOffOrOutsideSquad(t *testing.T) {
	cases := []struct {
		flags   string
		latched bool
		t0      float64
		squad   bool
	}{
		{"squadrim=0", false, 30, true}, {"squadrim=0", true, 30, false}, // off
		{"squadrim=1", false, 5, false}, {"squadrim=1", true, 30, false}, {"squadrim=1", true, 120, false}, // on, not squad mode
	}
	for _, c := range cases {
		a, b := sqBrain("", c.latched), sqBrain(c.flags, c.latched)
		if x, y := runScript(t, a, sqScript(c.t0)), runScript(t, b, sqScript(c.t0)); !reflect.DeepEqual(x, y) {
			t.Fatalf("squadrim changed play (%s, latched %v, t0 %v)", c.flags, c.latched, c.t0)
		}
		if b.pracOn != c.squad {
			t.Fatalf("test setup: squad params %v (%s, latched %v, t0 %v)", b.pracOn, c.flags, c.latched, c.t0)
		}
	}
	// and in squad mode the same script does change play
	a, b := sqBrain("", false), sqBrain("squadrim=1", false)
	if x, y := runScript(t, a, sqScript(30)), runScript(t, b, sqScript(30)); reflect.DeepEqual(x, y) {
		t.Fatalf("squadrim=1 changed nothing in squad mode")
	}
}

func TestSquadRimSpot(t *testing.T) {
	b := sqBrain("sqrim=0.9", false)
	z := &sqIdleObs(30, 10, 0, 380).Zone
	if rx, ry := b.squadRimSpot(z, 10, 0); math.Abs(rx-342) > 0.5 || math.Abs(ry) > 0.5 {
		t.Fatalf("rim spot (%.1f, %.1f), want (342, 0)", rx, ry)
	}
	for _, c := range []struct{ r, want float64 }{{150, 90}, {40, 14}, {9, 1}} {
		z.NextRadius = c.r
		if rx, ry := b.squadRimSpot(z, 0, 10); math.Abs(ry-c.want) > 0.5 || math.Abs(rx) > 0.5 {
			t.Fatalf("radius %v: rim spot (%.1f, %.1f), want (0, %v)", c.r, rx, ry, c.want)
		}
	}
	// a mech seen at a slightly larger bearing pushes us the other way along the rim (clockwise: y < 0)
	z.NextRadius = 380
	b.now = 30
	b.tracks[3] = &Track{MechSeen: MechSeen{ID: 3, Kind: "mech", Pos: Vec3{300, 40, 0}}, Seen: 25}
	rx, ry := b.squadRimSpot(z, 340, 0)
	if ry > -100 || math.Abs(hyp(rx, ry)-342) > 0.5 {
		t.Fatalf("no drift away from the mech: (%.1f, %.1f)", rx, ry)
	}
	// a mech in the middle of the circle does not push
	b.tracks[3].Pos = Vec3{0.5, 0, 0}
	if _, ry := b.squadRimSpot(z, 340, 0); math.Abs(ry) > 0.5 {
		t.Fatalf("a mech at the centre pushed us (y %.1f)", ry)
	}
}

func TestSquadRimWalksAndHolds(t *testing.T) {
	plain, rim := sqBrain("", false), sqBrain("sqrim=0.9", false)
	a := sqAct(t, plain, sqIdleObs(30, 10, 0, 380))
	c := sqAct(t, rim, sqIdleObs(30, 10, 0, 380))
	if !plain.pracOn || hyp(a.Move[0], a.Move[1]) > 0.1 {
		t.Fatalf("test setup: plain squad play not idle (squad %v, mode %s, move %v)", plain.pracOn, plain.Mode, a.Move)
	}
	if rim.Mode != "sqrim" || c.Move[0] < 0.5 {
		t.Fatalf("sqrim did not walk out to the rim (mode %s, move %v)", rim.Mode, c.Move)
	}
	b := sqBrain("sqrim=0.9", false)
	if act := sqAct(t, b, sqIdleObs(30, 340, 0, 380)); b.Mode == "sqrim" || hyp(act.Move[0], act.Move[1]) > 0.1 {
		t.Fatalf("kept walking at the rim spot (mode %s, move %v)", b.Mode, act.Move)
	}
	// outside the next circle's walk-in radius, sqrim takes us in to the rim spot (not giddy's zone walk)
	b = sqBrain("sqrim=0.9", false)
	if act := sqAct(t, b, sqIdleObs(30, 0, 200, 150)); b.Mode != "sqrim" || act.Move[1] > -0.5 {
		t.Fatalf("not walking in to the 90 m rim spot (mode %s, move %v)", b.Mode, act.Move)
	}
}

func TestSquadLootInside(t *testing.T) {
	for _, c := range []struct {
		name string
		item Item
		cube bool
		want bool
	}{
		{"deep cube at 78 m", Item{ID: 50, Kind: "cube", Pos: Vec3{60, 50, 0}}, true, false},
		{"deep cube at 36 m", Item{ID: 50, Kind: "cube", Pos: Vec3{40, 20, 0}}, true, true},
		{"rim cube at 80 m", Item{ID: 50, Kind: "cube", Pos: Vec3{330, 0, 0}}, true, true},
		{"deep laser at 120 m", Item{ID: 60, Kind: "laser", Pos: Vec3{-100, 60, 0}}, false, true},
	} {
		plain, lim := sqBrain("", false), sqBrain("sqloot=50", false)
		px := 10.0
		if c.name == "rim cube at 80 m" {
			px = 250
		}
		for _, b := range []*Brain{plain, lim} {
			o := sqIdleObs(30, px, 0, 380)
			o.Self.Weapons = o.Self.Weapons[:1]
			o.Self.Weapons = append(o.Self.Weapons, nil)
			if c.cube {
				o.Cubes = []Item{c.item}
			} else {
				o.Pickups = []Item{c.item}
			}
			sqAct(t, b, o)
		}
		if plain.lootID != c.item.ID {
			t.Fatalf("%s: test setup: plain squad play does not go for it (mode %s)", c.name, plain.Mode)
		}
		if got := lim.lootID == c.item.ID; got != c.want {
			t.Fatalf("%s: sqloot loots it = %v, want %v (mode %s)", c.name, got, c.want, lim.Mode)
		}
	}
}

func TestSquadBand(t *testing.T) {
	b := sqBrain("sqband=1", false)
	sqAct(t, b, sqIdleObs(30, 10, 0, 380))
	if !b.pracOn {
		t.Fatalf("test setup: squad params off")
	}
	mech := &Track{MechSeen: MechSeen{ID: 3, Kind: "mech"}}
	npc := &Track{MechSeen: MechSeen{ID: 9, Kind: "npc"}}
	o := sqIdleObs(30, 10, 0, 380)
	me := &o.Self
	if lo, hi := b.squadBand(me, mech, b.prefRange(me, mech)); lo != 110 || hi != 175 {
		t.Fatalf("laser band %v-%v, want 110-175", lo, hi)
	}
	me.Weapons = me.Weapons[:1]
	if lo, hi := b.squadBand(me, mech, b.prefRange(me, mech)); lo != 85 || hi != 125 {
		t.Fatalf("gun band %v-%v, want 85-125", lo, hi)
	}
	if lo, hi := b.squadBand(me, npc, b.prefRange(me, npc)); lo != hi {
		t.Fatalf("band against an NPC")
	}
	if lo, hi := b.squadBand(me, mech, 50); lo != 50 || hi != 50 {
		t.Fatalf("band ignored a moved range (lcdpunish-style 50 m): %v-%v", lo, hi)
	}
	me.Weapons = []*WeaponState{{Kind: "gun", Ammo: 12, Stats: lrGun}, {Kind: "lightning", Stats: WeaponStats{Range: 45}}}
	if lo, hi := b.squadBand(me, mech, b.prefRange(me, mech)); lo != hi {
		t.Fatalf("band with lightning in use")
	}
	// at 115 m from a gun mech with only a gun, giddy's single 85 m range pulls us in; the band does not
	g := MechSeen{ID: 3, Kind: "mech", Pos: Vec3{125, 0, 0}, Aim: [2]float64{math.Pi, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun"}}
	pull := func(flags string) float64 {
		b := sqBrain(flags, false)
		o := sqIdleObs(30, 10, 0, 380, g)
		o.Self.Weapons = o.Self.Weapons[:1]
		sqAct(t, b, o)
		mx, _ := b.fightMove(o, b.tracks[3], []*Track{b.tracks[3]}, 1)
		return mx
	}
	if p, q := pull(""), pull("sqband=1"); !(p > 0.3 && q < p-0.3) {
		t.Fatalf("band did not stop the pull to 85 m: plain %.2f, band %.2f", p, q)
	}
}

func TestSquadRetreat(t *testing.T) {
	by := 7
	hurt := func(flags string, tt, hp float64) *Brain {
		b := sqBrain(flags, false)
		o := sqIdleObs(tt, 10, 0, 380)
		o.Self.HP = hp
		bb := 1.0
		o.Events = []Event{{Type: "damaged", Amount: 45, Weapon: "gun", By: &by, Bearing: &bb}}
		sqAct(t, b, o)
		return b
	}
	if b := hurt("", 30, 350); b.retreat {
		t.Fatalf("test setup: plain squad play retreats with no mech in sight")
	}
	if b := hurt("sqret=0.3", 30, 350); !b.retreat || !b.sqRetOn {
		t.Fatalf("sqret did not retreat at 25%% HP under fire (mode %s)", b.Mode)
	}
	if b := hurt("sqret=0.3", 30, 490); b.retreat {
		t.Fatalf("sqret retreated at 35%% HP")
	}
	// latch holds while hurt and calm < 7 s, releases at RecoverHP
	b := hurt("sqret=0.3", 30, 350)
	o := sqIdleObs(33, 10, 0, 380)
	o.Self.HP = 600
	sqAct(t, b, o)
	if !b.retreat {
		t.Fatalf("sqret latch dropped at 43%% HP 3 s after the hit")
	}
	o = sqIdleObs(34, 10, 0, 380)
	o.Self.HP = 1200
	sqAct(t, b, o)
	if b.retreat || b.sqRetOn {
		t.Fatalf("sqret latch held at 86%% HP")
	}
}

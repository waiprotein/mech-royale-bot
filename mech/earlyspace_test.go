package mech

import (
	"math"
	"testing"
)

func TestEarlySpaceParse(t *testing.T) {
	p := DefaultParams()
	if p.EarlySpace != 0 {
		t.Fatal("earlyspace must default off")
	}
	applyFlags(&p, "earlyspace=40")
	if p.EarlySpace != 40 {
		t.Fatalf("parse: %v", p.EarlySpace)
	}
}

func TestEarlySpaceCost(t *testing.T) {
	b := &Brain{tracks: map[int]*Track{}}
	b.P.EarlySpace = 40
	b.now = 10
	if c := b.earlySpaceCost(0, 0); c != 0 {
		t.Fatalf("no mechs: cost %v", c)
	}
	// one mech 60 m away (inside 85 m): 40 * (1 + 60/120) = 60
	b.tracks[1] = &Track{MechSeen: MechSeen{ID: 1, Pos: Vec3{60, 0, 0}}, Seen: 9}
	if c := b.earlySpaceCost(0, 0); math.Abs(c-60) > 1e-9 {
		t.Fatalf("one mech at 60 m: cost %v, want 60", c)
	}
	// a second mech at 100 m: still nearest 60 m, 100 m is outside 85 m -> unchanged
	b.tracks[2] = &Track{MechSeen: MechSeen{ID: 2, Pos: Vec3{0, 100, 0}}, Seen: 9}
	if c := b.earlySpaceCost(0, 0); math.Abs(c-60) > 1e-9 {
		t.Fatalf("plus a mech at 100 m: cost %v, want 60", c)
	}
	// a spot 150 m from both: no cost
	if c := b.earlySpaceCost(-150, -150); c != 0 {
		t.Fatalf("far spot: cost %v", c)
	}
	// stale tracks (seen > 5 s ago) are ignored
	b.now = 20.5
	if c := b.earlySpaceCost(0, 0); c != 0 {
		t.Fatalf("stale tracks: cost %v", c)
	}
}

func TestEarlySpaceRange(t *testing.T) {
	b := &Brain{tracks: map[int]*Track{}}
	b.P.EarlySpace, b.P.EarlySpaceR = 40, 160
	b.now = 10
	b.tracks[1] = &Track{MechSeen: MechSeen{ID: 1, Pos: Vec3{140, 0, 0}}, Seen: 9}
	// 140 m: outside 85 m, closeness (160-140)/160 = 0.125 -> 5
	if c := b.earlySpaceCost(0, 0); math.Abs(c-5) > 1e-9 {
		t.Fatalf("range 160, mech at 140 m: cost %v, want 5", c)
	}
	b.P.EarlySpaceR = 0 // default 120: no cost at 140 m
	if c := b.earlySpaceCost(0, 0); c != 0 {
		t.Fatalf("default range, mech at 140 m: cost %v", c)
	}
}

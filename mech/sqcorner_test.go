package mech

import "testing"

// TestSqRetUpCorner checks the K3 squad retreat additions: sqretup ends the sqret latch at its own HP share, and
// sqcorner turns on a lone close chaser after 2 s of retreat (but not with two mechs close, and not before 2 s).
func TestSqRetUpCorner(t *testing.T) {
	by := 3
	bb := 1.0
	hit := []Event{{Type: "damaged", Amount: 45, Weapon: "gun", By: &by, Bearing: &bb}}
	start := func(flags string, mechs ...MechSeen) *Brain {
		b := sqBrain(flags, false)
		o := sqIdleObs(30, 10, 0, 380, mechs...)
		o.Self.HP = 350 // 25%
		o.Events = hit
		sqAct(t, b, o)
		if !b.pracOn || !b.sqRetOn || !b.retreat {
			t.Fatalf("%s: test setup: sqret latch not on at 25%% HP under fire (prac %v, mode %s)", flags, b.pracOn, b.Mode)
		}
		return b
	}
	// sqretup: release at 68% with sqretup=0.65, hold with the default RecoverHP 0.8
	for _, c := range []struct {
		flags string
		held  bool
	}{{"sqret=0.3", true}, {"sqret=0.3,sqretup=0.65", false}} {
		b := start(c.flags)
		o := sqIdleObs(31, 10, 0, 380)
		o.Self.HP = 950 // 68%
		o.Events = hit
		sqAct(t, b, o)
		if b.sqRetOn != c.held {
			t.Fatalf("%s: latch at 68%% HP = %v, want %v", c.flags, b.sqRetOn, c.held)
		}
	}

	// sqcorner: a weak lone mech 25 m away, aiming at us, keeps hitting us while we retreat
	lone := MechSeen{ID: 3, Kind: "mech", Pos: Vec3{35, 0, 0}, Aim: [2]float64{3.14159, 0}, HP: 150, MaxHP: 1400, Weapons: []string{"gun"}}
	step := func(b *Brain, tt float64, mechs ...MechSeen) {
		o := sqIdleObs(tt, 10, 0, 380, mechs...)
		o.Self.HP = 380
		o.Events = hit
		sqAct(t, b, o)
	}
	b := start("sqret=0.3,sqcorner=1", lone)
	step(b, 31, lone) // 1 s into the retreat: still running
	if !b.retreat {
		t.Fatalf("sqcorner turned before 2 s of retreat (mode %s)", b.Mode)
	}
	step(b, 32.5, lone)
	if b.retreat || !b.sqRetOn {
		t.Fatalf("sqcorner did not turn on a weak lone chaser at 25 m after 2.5 s (retreat %v, latch %v, mode %s)", b.retreat, b.sqRetOn, b.Mode)
	}
	// two mechs close: keep retreating
	second := lone
	second.ID, second.Pos = 4, Vec3{10, 30, 0}
	b = start("sqret=0.3,sqcorner=1", lone, second)
	step(b, 32.5, lone, second)
	if !b.retreat {
		t.Fatalf("sqcorner turned with two mechs close (mode %s)", b.Mode)
	}
	// flag off: keep retreating from the lone chaser
	b = start("sqret=0.3", lone)
	step(b, 32.5, lone)
	if !b.retreat {
		t.Fatalf("test setup: plain sqret stopped retreating from the lone chaser")
	}
	// crash breaker resets both
	b = start("sqret=0.3,sqretup=0.65,sqcorner=1")
	b.base = b.P
	b.Panics = 3
	b.safeFallback()
	if b.P.SqRetUp != 0 || b.P.SqCorner || b.base.SqRetUp != 0 || b.base.SqCorner {
		t.Fatalf("crash breaker kept sqretup/sqcorner")
	}
}

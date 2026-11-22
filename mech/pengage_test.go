package mech

import "testing"

// TestPEngage checks pengage: EngageDist = PEngage while giddy's FFA latch has not fired (ghost window before
// PracAt and the squad window), sqengage keeps priority in squad mode, and after the latch the base 60 applies.
func TestPEngage(t *testing.T) {
	eng := func(flags string, latched bool, tt float64) float64 {
		b := sqBrain(flags, latched)
		sqAct(t, b, sqIdleObs(tt, 10, 0, 380))
		return b.P.EngageDist
	}
	cases := []struct {
		name    string
		flags   string
		latched bool
		tt      float64
		want    float64
	}{
		{"off, ghost window", "", false, 10, 60},
		{"ghost window (before PracAt)", "pengage=80", false, 10, 80},
		{"squad window", "pengage=80", false, 30, 80},
		{"sqengage wins in squad mode", "pengage=80,sqengage=100", false, 30, 100},
		{"after the FFA latch", "pengage=80", true, 30, 60},
		{"after the FFA latch, early", "pengage=80", true, 10, 60},
	}
	for _, c := range cases {
		if got := eng(c.flags, c.latched, c.tt); got != c.want {
			t.Fatalf("%s: EngageDist %v, want %v", c.name, got, c.want)
		}
	}
	// latch firing mid-match restores the base value
	b := sqBrain("pengage=80", false)
	sqAct(t, b, sqIdleObs(10, 10, 0, 380))
	if b.P.EngageDist != 80 {
		t.Fatalf("test setup: pengage not applied")
	}
	b.notPractice = true
	sqAct(t, b, sqIdleObs(10.1, 10, 0, 380))
	if b.P.EngageDist != 60 {
		t.Fatalf("EngageDist %v after the latch fired, want 60", b.P.EngageDist)
	}
	// crash breaker resets it
	b = sqBrain("pengage=80", false)
	b.base = b.P
	b.Panics = 3
	b.safeFallback()
	if b.P.PEngage != 0 || b.base.PEngage != 0 {
		t.Fatalf("crash breaker kept pengage")
	}
}

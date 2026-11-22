package mech

import (
	"math"
	"os"
	"reflect"
	"testing"
)

// Tests for cornercover (practice-side: when no spot hides us from every mech, hide from the worst attackers).
// They need walls, so they use the real arena from the organiser's kit (skipped if it is not there).

func ccWorld(t *testing.T) *World {
	t.Helper()
	dir := "../../kit/mechfight-kit/arena"
	if _, err := os.Stat(dir); err != nil {
		t.Skip("kit arena not found")
	}
	w, err := LoadWorld(dir)
	if err != nil {
		t.Skipf("arena: %v", err)
	}
	return w
}

func ccBrain(w *World, n int, latched bool) *Brain {
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: lrConfig()}, w)
	b.P.CornerCover = n
	b.notPractice = latched
	return b
}

// ccObs: we stand at p with 600 HP (LoseRule retreat with 3 mechs near), mechs 3, 4, 5 at q[0..2] (full HP, so we
// are not finishing anyone); mech 4 hits us 4 times and mech 5 twice this tick, mech 3 not at all.
func ccObs(w *World, tm float64, p Vec3, q [3]Vec3) *Obs {
	var ms []MechSeen
	for i, qq := range q {
		ms = append(ms, MechSeen{ID: 3 + i, Kind: "mech", Pos: qq, Aim: [2]float64{math.Atan2(p[1]-qq[1], p[0]-qq[0]), 0}, HP: 1400,
			MaxHP: 1400, Weapons: []string{"gun"}})
	}
	o := lrObs(tm, ms...)
	o.Self.Pos, o.Self.HP = p, 600
	for k, by := range []int{4, 4, 4, 4, 5, 5} {
		id := by
		br := math.Atan2(q[by-3][1]-p[1], q[by-3][0]-p[0])
		_ = k
		o.Events = append(o.Events, Event{Type: "damaged", Amount: 45, Weapon: "gun", By: &id, Bearing: &br})
	}
	return o
}

// ccFind searches the arena for a spot where plain play is cornered (no full cover) and cornercover=2 finds
// partial cover; it returns the brains' modes for the first such layout.
func ccFind(t *testing.T, w *World) (Vec3, [3]Vec3, bool) {
	t.Helper()
	for gx := -240.0; gx <= 240; gx += 24 {
		for gy := -240.0; gy <= 240; gy += 24 {
			if !w.Free(gx, gy) {
				continue
			}
			p := Vec3{gx, gy, w.WalkAt(gx, gy)}
			eye := Vec3{gx, gy, p[2] + 8}
			var q [3]Vec3
			ok := true
			for i := 0; i < 3 && ok; i++ {
				ok = false
				for _, d := range []float64{70, 90, 110} {
					a := float64(i)*2*math.Pi/3 + 0.3
					x, y := gx+d*math.Cos(a), gy+d*math.Sin(a)
					if w.Free(x, y) {
						z := w.WalkAt(x, y)
						if w.Clear(Vec3{x, y, z + 8}, eye) {
							q[i], ok = Vec3{x, y, z}, true
							break
						}
					}
				}
			}
			if !ok {
				continue
			}
			plain, cc := ccBrain(w, 0, false), ccBrain(w, 2, false)
			plain.Act(ccObs(w, 30, p, q))
			cc.Act(ccObs(w, 30, p, q))
			if plain.Panics != 0 || cc.Panics != 0 {
				t.Fatalf("step panicked")
			}
			if plain.Mode == "cornered" && cc.Mode == "partcover" {
				return p, q, true
			}
		}
	}
	return Vec3{}, [3]Vec3{}, false
}

func TestCornerCoverPractice(t *testing.T) {
	w := ccWorld(t)
	p, q, ok := ccFind(t, w)
	if !ok {
		t.Fatalf("no layout found where plain play is cornered and cornercover finds partial cover")
	}
	b := ccBrain(w, 2, false)
	b.Act(ccObs(w, 30, p, q))
	if !b.pracOn || !b.retreat {
		t.Fatalf("test setup: want squad params and a retreat (pracOn %v, retreat %v)", b.pracOn, b.retreat)
	}
	// the threats it hides from are the two attackers, worst first
	th := b.cornerThreats(losNow(func() []*Track {
		var s []*Track
		for _, tr := range b.tracks {
			s = append(s, tr)
		}
		return s
	}(), b.now), 2)
	if len(th) != 2 || th[0].ID != 4 || th[1].ID != 5 {
		ids := []int{}
		for _, tr := range th {
			ids = append(ids, tr.ID)
		}
		t.Fatalf("cornerThreats = %v, want [4 5]", ids)
	}
	// the chosen spot is hidden from mechs 4 and 5
	g := b.coverGoal
	gz := w.WalkAt(g[0], g[1])
	for _, tr := range th {
		if w.Clear(b.eye(tr), Vec3{g[0], g[1], gz + 9}) {
			t.Fatalf("partial cover spot (%.0f, %.0f) is seen by mech %d", g[0], g[1], tr.ID)
		}
	}
}

// After the FFA proof (ffaOn) cornercover does nothing.
func TestCornerCoverInertAfterFFAProof(t *testing.T) {
	w := ccWorld(t)
	p, q, ok := ccFind(t, w)
	if !ok {
		t.Skip("no cornered layout found")
	}
	plain, cc := ccBrain(w, 0, true), ccBrain(w, 2, true)
	for _, tm := range []float64{30, 30.1, 30.2, 30.3} {
		a, c := plain.Act(ccObs(w, tm, p, q)), cc.Act(ccObs(w, tm, p, q))
		if !reflect.DeepEqual(a, c) {
			t.Fatalf("cornercover changed post-latch play at t=%v (modes %s / %s)", tm, plain.Mode, cc.Mode)
		}
	}
}

func TestCornerCoverResetByBreaker(t *testing.T) {
	b := ccBrain(lrTestWorld(), 2, false)
	b.Act(lrObs(30))
	b.safeFallback()
	if b.P.CornerCover != 0 || b.base.CornerCover != 0 {
		t.Fatalf("safeFallback leaves cornercover on")
	}
}

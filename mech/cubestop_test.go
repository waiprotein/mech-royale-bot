package mech

import (
	"reflect"
	"testing"
)

// Tests for cubestop (no cube/crate detours at the cube stat cap). They reuse lrConfig / lrTestWorld / lrObs.

func cubeBrain(cubestop, latched bool) *Brain {
	c := lrConfig()
	c.Cubes.UpgradeThresholds = []int{1, 3, 6, 10, 15, 21}
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: c}, lrTestWorld())
	b.P.CubeStop = cubestop
	b.notPractice = latched
	return b
}

// cubeObs: we hold `cubes` cubes, walking north; one cube (id 5001) d m east of us. Optional crate (id 1001).
func cubeObs(t float64, cubes int, d float64, crate bool) *Obs {
	o := lrObs(t)
	o.Self.Cubes = cubes
	o.Self.Vel = Vec3{0, 11.7, 0}
	if d > 0 {
		o.Cubes = []Item{{ID: 5001, Pos: Vec3{d, 0, 0}}}
	}
	if crate {
		o.Crates = []Item{{ID: 1001, Pos: Vec3{-30, 0, 0}, HP: 300}}
	}
	return o
}

func cubeAct(t *testing.T, b *Brain, o *Obs) Action {
	t.Helper()
	act := b.Act(o)
	if b.Panics != 0 {
		t.Fatalf("step panicked (%d)", b.Panics)
	}
	return act
}

// Flag on without the FFA latch: identical to flag off, before PracAt (base params) and after (squad params).
func TestCubeStopInertWithoutLatch(t *testing.T) {
	plain, strict := cubeBrain(false, false), cubeBrain(true, false)
	for _, tt := range []float64{5, 10, 19.9, 20.5, 40, 80} {
		a, c := cubeAct(t, plain, cubeObs(tt, 12, 30, true)), cubeAct(t, strict, cubeObs(tt, 12, 30, true))
		if !reflect.DeepEqual(a, c) {
			t.Fatalf("cubestop=1 without the latch changed play at t=%v", tt)
		}
	}
}

func TestCubeStopAfterLatch(t *testing.T) {
	cases := []struct {
		name  string
		cubes int
		d     float64
		want  bool // still goes for the cube
	}{
		{"below the cap", 9, 30, true},
		{"at the cap, next threshold 5 away", 10, 30, false},
		{"12 cubes, next threshold 3 away", 12, 30, false},
		{"13 cubes, threshold 2 away, cube at 30 m", 13, 30, true},
		{"13 cubes, threshold 2 away, cube at 55 m", 13, 55, false},
		{"20 cubes, threshold 1 away, cube at 35 m", 20, 35, true},
		{"21 cubes, no threshold listed above", 21, 20, false},
	}
	for _, c := range cases {
		off, on := cubeBrain(false, true), cubeBrain(true, true)
		cubeAct(t, off, cubeObs(80, c.cubes, c.d, false))
		cubeAct(t, on, cubeObs(80, c.cubes, c.d, false))
		if off.Mode != "loot:cube" {
			t.Fatalf("%s: plain build does not go for the cube (mode %s); test setup", c.name, off.Mode)
		}
		if got := on.Mode == "loot:cube"; got != c.want {
			t.Fatalf("%s: goes for cube = %v, want %v (mode %s)", c.name, got, c.want, on.Mode)
		}
	}
	// crates: skipped at 12 cubes, kept below the cap
	on := cubeBrain(true, true)
	cubeAct(t, on, cubeObs(80, 12, 0, true))
	if on.Mode == "loot:crate" {
		t.Fatalf("went for a crate at 12 cubes")
	}
	on = cubeBrain(true, true)
	cubeAct(t, on, cubeObs(80, 5, 0, true))
	if on.Mode != "loot:crate" {
		t.Fatalf("skipped a crate below the cap (mode %s)", on.Mode)
	}
}

func TestCubeStopResetBySquadAndBreaker(t *testing.T) {
	p := DefaultParams()
	p.CubeStop = true
	if squadParams(p).CubeStop {
		t.Fatalf("squadParams keeps cubestop on")
	}
	b := cubeBrain(true, true)
	cubeAct(t, b, cubeObs(80, 12, 30, false))
	b.safeFallback()
	if b.P.CubeStop || b.base.CubeStop {
		t.Fatalf("safeFallback leaves cubestop on")
	}
}

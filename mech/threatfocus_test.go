package mech

import (
	"reflect"
	"testing"
)

// Tests for threatfocus (practice-side: target whoever deals us the most damage). They reuse lrConfig /
// lrTestWorld / lrObs.

func tfBrain(w float64, latched bool) *Brain {
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: lrConfig()}, lrTestWorld())
	b.P.ThreatFocus = w
	b.notPractice = latched
	return b
}

// tfObs at time t: mech 3 (near, 50 m, not shooting) and mech 4 (farther, 120 m on -x); hits adds that many gun
// hits from mech 4 this tick.
func tfObs(t float64, hits int) *Obs {
	near := MechSeen{ID: 3, Kind: "mech", Pos: Vec3{50, 0, 0}, Aim: [2]float64{0, 0}, HP: 1000, MaxHP: 1400, Weapons: []string{"gun"}}
	far := MechSeen{ID: 4, Kind: "mech", Pos: Vec3{-120, 0, 0}, Aim: [2]float64{0, 0}, HP: 1000, MaxHP: 1400, Weapons: []string{"gun"}}
	o := lrObs(t, near, far)
	by, bearing := 4, 3.14159
	for i := 0; i < hits; i++ {
		o.Events = append(o.Events, Event{Type: "damaged", Amount: 45, Weapon: "gun", By: &by, Bearing: &bearing})
	}
	return o
}

func tfAct(t *testing.T, b *Brain, o *Obs) Action {
	t.Helper()
	act := b.Act(o)
	if b.Panics != 0 {
		t.Fatalf("step panicked (%d)", b.Panics)
	}
	return act
}

// In practice (squad params after 20 s) plain giddy scoring takes the near mech; threatfocus takes the one that
// has been hitting us (6 hits = 270 HP within the decay window, ~45 HP/s).
func TestThreatFocusPractice(t *testing.T) {
	plain := tfBrain(0, false)
	tfAct(t, plain, tfObs(30, 6))
	if !plain.pracOn || plain.stickID != 3 {
		t.Fatalf("test setup: plain squad play should target the near mech 3 (pracOn %v, target %d)", plain.pracOn, plain.stickID)
	}
	b := tfBrain(1, false)
	tfAct(t, b, tfObs(30, 6))
	if b.stickID != 4 {
		t.Fatalf("threatfocus targeted %d, want the attacker 4", b.stickID)
	}
	if r := b.threatRate(4); r < 40 || r > 46 {
		t.Fatalf("threat rate %.1f HP/s, want ~45", r)
	}
	// the rate decays: 12 s later it is e^-2 of that
	b.now = 42
	if r := b.threatRate(4); r < 5 || r > 7 {
		t.Fatalf("decayed threat rate %.1f HP/s, want ~6.1", r)
	}
	// non-weapon damage (zone, NPC drones) does not count
	b = tfBrain(1, false)
	o := tfObs(30, 0)
	npc := 104
	o.Events = append(o.Events, Event{Type: "damaged", Amount: 500, Weapon: "npc", By: &npc}, Event{Type: "damaged", Amount: 300, Weapon: "zone"})
	tfAct(t, b, o)
	if b.threatRate(104) != 0 {
		t.Fatalf("NPC damage counted as a mech threat")
	}
}

// After the FFA proof (ffaOn) threatfocus does nothing: the post-latch FFA brain is unchanged.
func TestThreatFocusInertAfterFFAProof(t *testing.T) {
	plain, tf := tfBrain(0, true), tfBrain(2, true)
	for i, tt := range []float64{30, 30.1, 30.2, 30.5, 31, 32} {
		hits := 0
		if i < 3 {
			hits = 4
		}
		a, c := tfAct(t, plain, tfObs(tt, hits)), tfAct(t, tf, tfObs(tt, hits))
		if !reflect.DeepEqual(a, c) {
			t.Fatalf("threatfocus changed post-latch play at t=%v", tt)
		}
	}
}

func TestThreatFocusResetByBreaker(t *testing.T) {
	b := tfBrain(1, false)
	b.base.ThreatFocus = 1
	tfAct(t, b, tfObs(30, 2))
	b.safeFallback()
	if b.P.ThreatFocus != 0 || b.base.ThreatFocus != 0 {
		t.Fatalf("safeFallback leaves threatfocus on")
	}
}

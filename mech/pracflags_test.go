package mech

import "testing"

func TestPracFlagsParse(t *testing.T) {
	p := DefaultParams()
	if p.RearAvoid != 0 || p.CoverReload {
		t.Fatalf("defaults must be off: %v %v", p.RearAvoid, p.CoverReload)
	}
	applyFlags(&p, "rearavoid=25,coverreload=1")
	if p.RearAvoid != 25 || !p.CoverReload {
		t.Fatalf("parse failed: %v %v", p.RearAvoid, p.CoverReload)
	}
	// practice params keep them (they are practice features); FFA-only flags are the ones squadParams zeroes
	q := squadParams(p)
	if q.RearAvoid != 25 || !q.CoverReload {
		t.Fatalf("squadParams must keep practice flags: %v %v", q.RearAvoid, q.CoverReload)
	}
}

func TestCanFire(t *testing.T) {
	b := &Brain{}
	me := &Self{Energy: 100}
	gunOK := &WeaponState{Kind: "gun", Ammo: 5}
	gunRel := &WeaponState{Kind: "gun", Ammo: 0, Reload: 1.5}
	laserCool := &WeaponState{Kind: "laser", Cooldown: 1.0}
	laserOK := &WeaponState{Kind: "laser"}
	light := &WeaponState{Kind: "lightning", Stats: WeaponStats{Energy: 3, Range: 45}}
	far := &Track{Dist: 120}
	if !b.canFire(me, gunOK, nil, nil, far) {
		t.Fatal("gun with ammo can fire")
	}
	if b.canFire(me, gunRel, laserCool, nil, far) {
		t.Fatal("reloading gun + cooling laser cannot fire")
	}
	if !b.canFire(me, gunRel, laserOK, nil, far) {
		t.Fatal("ready laser can fire")
	}
	if b.canFire(me, gunRel, nil, light, far) {
		t.Fatal("lightning cannot reach a 120 m target")
	}
	if !b.canFire(me, gunRel, nil, light, &Track{Dist: 30}) {
		t.Fatal("lightning reaches a 30 m target")
	}
	me.Energy = 5
	if b.canFire(me, gunRel, nil, light, &Track{Dist: 30}) {
		t.Fatal("lightning without energy cannot fire")
	}
}

func TestLosNow(t *testing.T) {
	a := &Track{Seen: 10, LOS: true}
	b := &Track{Seen: 10, LOS: false}
	c := &Track{Seen: 9, LOS: true}
	got := losNow([]*Track{a, b, c}, 10)
	if len(got) != 1 || got[0] != a {
		t.Fatalf("losNow = %v", got)
	}
}

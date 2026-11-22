package mech

import "testing"

func TestRescanSpin(t *testing.T) {
	p := DefaultParams()
	if p.Rescan {
		t.Fatal("rescan must default off")
	}
	applyFlags(&p, "rescan=1")
	if !p.Rescan {
		t.Fatal("parse")
	}
	b := &Brain{lastHurt: -100, rsAt: -100}
	o := &Obs{}
	o.Self.Stats.TurnRate = 4
	b.now = 10
	if !b.rescanSpin(o, nil, false) {
		t.Fatal("calm and due: spin")
	}
	b.now = 11 // still inside the 1.67 s spin
	if !b.rescanSpin(o, nil, false) {
		t.Fatal("spin continues")
	}
	b.now = 13 // spin over, gun-only period 6 s not elapsed
	if b.rescanSpin(o, nil, false) {
		t.Fatal("not due yet")
	}
	b.now = 16.5 // 6.5 s after start: due again gun-only
	if !b.rescanSpin(o, nil, false) {
		t.Fatal("gun-only: due after 6 s")
	}
	b.now = 25 // armed: 8.5 s since last start < 15 s
	if b.rescanSpin(o, nil, true) {
		t.Fatal("armed: not due before 15 s")
	}
	b.now, b.lastHurt = 40, 38.5 // hurt 1.5 s ago: not calm
	if b.rescanSpin(o, nil, true) {
		t.Fatal("not calm after damage")
	}
	b.lastHurt = -100
	near := &Track{MechSeen: MechSeen{Pos: Vec3{30, 0, 0}}}
	if b.rescanSpin(o, []*Track{near}, true) {
		t.Fatal("not calm with a mech within 50 m")
	}
	las := &Track{MechSeen: MechSeen{Pos: Vec3{120, 0, 0}, Weapons: []string{"gun", "laser"}}, HitMe: -100}
	if b.rescanSpin(o, []*Track{las}, true) {
		t.Fatal("not calm with a laser holder within 150 m")
	}
	if !b.rescanSpin(o, nil, true) {
		t.Fatal("armed, calm, due")
	}
}

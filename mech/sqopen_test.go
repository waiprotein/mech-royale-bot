package mech

import "testing"

func TestSqFlagsParse(t *testing.T) {
	p := DefaultParams()
	if p.SqBackOpen || p.SqLRLaser != 0 {
		t.Fatal("defaults must be off")
	}
	applyFlags(&p, "sqbackopen=1,sqlrlaser=180")
	if !p.SqBackOpen || p.SqLRLaser != 180 {
		t.Fatalf("parse: %v %v", p.SqBackOpen, p.SqLRLaser)
	}
	q := squadParams(p)
	if !q.SqBackOpen || q.SqLRLaser != 180 {
		t.Fatal("squadParams must keep squad flags")
	}
}

func TestSqLROpen(t *testing.T) {
	b := &Brain{tracks: map[int]*Track{}, now: 50, oddsFleeUntil: -1}
	b.P.EngageDist, b.P.SqLRLaser = 60, 180
	me := &Self{HP: 1400}
	laser := &WeaponState{Kind: "laser"}
	tg := &Track{MechSeen: MechSeen{ID: 3, HP: 1200}, Seen: 50, LOS: true, Dist: 120}
	b.tracks[3] = tg
	o := &Obs{}
	if !b.sqLROpen(o, me, tg, laser, 2.0, 240) {
		t.Fatal("unaware, weaker, in range: open")
	}
	if b.sqLROpen(o, me, tg, laser, 0.1, 240) {
		t.Fatal("facing us and not busy: no open")
	}
	tg.Dist = 200
	if b.sqLROpen(o, me, tg, laser, 2.0, 240) {
		t.Fatal("beyond sqlrlaser range: no open")
	}
	tg.Dist, tg.HP = 120, 1500
	if b.sqLROpen(o, me, tg, laser, 2.0, 240) {
		t.Fatal("stronger target: no open")
	}
	tg.HP = 1200
	laser.Cooldown = 1
	if b.sqLROpen(o, me, tg, laser, 2.0, 240) {
		t.Fatal("laser cooling: no open")
	}
}

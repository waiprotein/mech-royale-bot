package mech

import "testing"

func TestSqHuntOK(t *testing.T) {
	p := DefaultParams()
	if p.SqHunt != 0 {
		t.Fatal("default off")
	}
	applyFlags(&p, "sqhunt=130")
	if p.SqHunt != 130 || squadParams(p).SqHunt != 130 {
		t.Fatal("parse / squadParams must keep it")
	}
	b := &Brain{now: 60, pracOn: true}
	b.P.SqHunt = 130
	b.Cfg.Cubes.DamagePerCube, b.Cfg.Cubes.StatCap = 0.04, 10
	me := &Self{HP: 1200, Weapons: []*WeaponState{{Kind: "gun"}}}
	tg := &Track{MechSeen: MechSeen{Kind: "mech", Pos: Vec3{100, 0, 0}, HP: 1400, Weapons: []string{"gun"}}, Seen: 59}
	if !b.sqHuntOK(me, tg, 0.86) {
		t.Fatal("1200 vs 1400 gun/gun (0.86 >= 0.7), 100 m: hunt")
	}
	tg.Weapons = []string{"gun", "laser"}
	if b.sqHuntOK(me, tg, 0.86) {
		t.Fatal("vs an armed mech: odds 1200/(1400*1.5) = 0.57 < 0.7")
	}
	tg.Weapons = []string{"gun"}
	if b.sqHuntOK(me, tg, 0.55) {
		t.Fatal("our HP <= 60%: no hunt")
	}
	tg.Pos = Vec3{140, 0, 0}
	if b.sqHuntOK(me, tg, 0.86) {
		t.Fatal("beyond 130 m: no hunt")
	}
	tg.Pos = Vec3{100, 0, 0}
	b.pracOn = false
	if b.sqHuntOK(me, tg, 0.86) {
		t.Fatal("not squad mode: no hunt")
	}
}

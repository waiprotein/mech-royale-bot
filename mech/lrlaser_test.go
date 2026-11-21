package mech

import (
	"math"
	"sync"
	"testing"
)

var (
	lrWorldOnce sync.Once
	lrWorld     *World
)

func lrTestWorld() *World {
	lrWorldOnce.Do(func() { lrWorld = FlatWorld() })
	return lrWorld
}

// lrConfig is the kit hello's config, trimmed to what step reads.
func lrConfig() Config {
	var c Config
	c.Mech = MechStats{Radius: 2.5, Height: 9, EyeHeight: 8, MuzzleHeight: 7, HP: 1400, WalkSpeed: 11.7, BoostSpeed: 19.5,
		GroundAccel: 58.5, AirAccel: 12, Gravity: 22, JumpSpeed: 11, JetAccel: 34, MaxRiseSpeed: 10, StepUp: 1.2,
		MaxSlopeDeg: 50, Energy: 100, EnergyRegen: 18, EnergyRegenDelay: 0.6, BoostDrain: 17, JetDrain: 15, TurnRate: 4,
		PitchRate: 3, MinPitch: -0.7, MaxPitch: 1.3, RegenDelay: 5, RegenPerS: 45, ChargeMoveMult: 0.5, RearMult: 1.5,
		FrontArcDeg: 180, Hearing: 1}
	c.Vision.Range, c.Vision.FovDeg, c.Vision.NearRange, c.Vision.MemoryS = 200, 120, 45, 5
	c.Weapons = map[string]WeaponStats{"gun": lrGun, "laser": lrLaserStats}
	c.Cubes.PickupRadius, c.Cubes.DamagePerCube, c.Cubes.StatCap, c.Cubes.SizePerCube = 3.5, 0.04, 10, 0.05
	c.Crates.Radius, c.Crates.Height = 1.5, 2
	c.Loot.PickupRadius = 4
	c.Npc.Radius, c.Npc.Height, c.Npc.HP = 1.8, 3.5, 350
	return c
}

var (
	lrGun        = WeaponStats{Damage: 45, Cooldown: 0.24, Speed: 252, Range: 230, SpreadDeg: 0.6, Mag: 12, Reload: 2.4}
	lrLaserStats = WeaponStats{Damage: 315, Cooldown: 1.65, Range: 280, Charge: 0.75, MinCharge: 0.15, MinFrac: 0.35, HoldS: 2, Cookoff: 0.6}
)

// lrBrain builds a brain with lrlaser=180, lrkeep=110; latched = giddy's FFA detector has fired (notPractice).
func lrBrain(latched bool) *Brain {
	b := NewBrain(&Hello{Slot: 1, Seed: 7, Config: lrConfig()}, lrTestWorld())
	b.P.LRLaser, b.P.LRKeep = 180, 110
	b.notPractice = latched
	return b
}

// lrObs: we stand at the origin, full HP, holding a gun and a ready laser, aimed along +x. tgt is mech 3.
func lrObs(t float64, mechs ...MechSeen) *Obs {
	return &Obs{Type: "obs", Tick: int(t * 20), T: t, Alive: 10,
		Self: Self{ID: 1, Aim: [2]float64{0, 0}, HP: 1400, MaxHP: 1400, Energy: 100, MaxEnergy: 100, Grounded: true, InZone: true,
			Weapons: []*WeaponState{{Kind: "gun", Ammo: 12, Stats: lrGun}, {Kind: "laser", Stats: lrLaserStats}}},
		Mechs: mechs,
		Zone:  Zone{Radius: 560, NextRadius: 380, ShrinkStart: 60, ShrinkEnd: 110, Dps: 20},
	}
}

// lrTarget: mech 3 at 140 m on +x with less HP than us, aim yaw 0 (facing away from us, so it has not noticed us).
func lrTarget() MechSeen {
	return MechSeen{ID: 3, Kind: "mech", Pos: Vec3{140, 0, 0}, Aim: [2]float64{0, 0}, HP: 900, MaxHP: 1400, Weapons: []string{"gun", "laser"}}
}

func lrAct(t *testing.T, b *Brain, o *Obs) Action {
	t.Helper()
	act := b.Act(o)
	if b.Panics != 0 {
		t.Fatalf("step panicked (%d)", b.Panics)
	}
	return act
}

func TestLRLaserOpensOnUnawareWeakerMech(t *testing.T) {
	b := lrBrain(true)
	act := lrAct(t, b, lrObs(50, lrTarget()))
	tr := b.tracks[3]
	if tr == nil || !tr.Engage {
		t.Fatalf("lrlaser did not engage an unaware, weaker mech at 140 m")
	}
	if !act.Fire[1] {
		t.Fatalf("laser is not charging on the long-range opener: %+v", act)
	}
	if !act.Fire[0] {
		t.Fatalf("gun did not open at once alongside the laser charge: %+v", act)
	}
	if b.Mode != "fight" {
		t.Fatalf("mode %q, want fight", b.Mode)
	}
	if b.P.LaserRange != 85 || b.base.LaserRange != 85 {
		t.Fatalf("lrkeep leaked into params: P %.0f base %.0f", b.P.LaserRange, b.base.LaserRange)
	}

	// Follow-through: 2 s later (past the 1.5 s hold) it has turned to aim at us. It stays engaged while our
	// hits keep landing and we still have more HP.
	o := lrObs(52, lrTarget())
	o.Mechs[0].Aim[0], o.Mechs[0].HP = math.Pi, 600
	o.Events = []Event{{Type: "hit", Target: 3, Amount: 300, Weapon: "laser"}}
	lrAct(t, b, o)
	if !b.tracks[3].Engage {
		t.Fatalf("lrlaser dropped the fight it opened right after hitting it")
	}
}

func TestLRLaserDropsWithoutFollowThrough(t *testing.T) {
	b := lrBrain(true)
	lrAct(t, b, lrObs(50, lrTarget()))
	o := lrObs(52, lrTarget())
	o.Mechs[0].Aim[0] = math.Pi // it noticed us, and we have not hit it
	lrAct(t, b, o)
	if b.tracks[3].Engage {
		t.Fatalf("lrlaser kept a fight with no hit of ours after the 1.5 s hold")
	}
}

func TestLRLaserOpensOnBusyMech(t *testing.T) {
	b := lrBrain(true)
	tgt := lrTarget()
	tgt.Aim[0] = math.Pi // aims at us, but another mech's laser is hitting it
	other := MechSeen{ID: 4, Kind: "mech", Pos: Vec3{140, 100, 0}, Aim: [2]float64{-math.Pi / 2, 0}, HP: 1400, MaxHP: 1400, Weapons: []string{"gun", "laser"}}
	o := lrObs(50, tgt, other)
	o.Beams = []Beam{{By: 4, Kind: "laser", Path: []Vec3{{140, 100, 7}, {140, 0, 4.5}}, Age: 0.05}}
	lrAct(t, b, o)
	if !b.tracks[3].Engage {
		t.Fatalf("lrlaser did not engage a mech busy with another mech")
	}
}

func TestLRLaserHoldsBack(t *testing.T) {
	cases := []struct {
		name    string
		latched bool
		now     float64
		edit    func(o *Obs)
	}{
		{"target has more HP", true, 50, func(o *Obs) { o.Mechs[0].HP, o.Mechs[0].MaxHP = 1500, 1600 }},
		{"target aims at us", true, 50, func(o *Obs) { o.Mechs[0].Aim[0] = math.Pi }},
		{"target charging at us", true, 50, func(o *Obs) { o.Mechs[0].Aim[0], o.Mechs[0].Charging = math.Pi-0.4, true }},
		{"laser cooling down", true, 50, func(o *Obs) { o.Self.Weapons[1].Cooldown = 1.0 }},
		{"no laser", true, 50, func(o *Obs) { o.Self.Weapons = o.Self.Weapons[:1] }},
		{"beyond lrlaser", true, 50, func(o *Obs) { o.Mechs[0].Pos[0] = 190 }},
		{"crowd", true, 50, func(o *Obs) {
			o.Mechs = append(o.Mechs,
				MechSeen{ID: 4, Kind: "mech", Pos: Vec3{0, 100, 0}, Aim: [2]float64{math.Pi / 2, 0}, HP: 1400, MaxHP: 1400},
				MechSeen{ID: 5, Kind: "mech", Pos: Vec3{0, -100, 0}, Aim: [2]float64{-math.Pi / 2, 0}, HP: 1400, MaxHP: 1400})
		}},
		{"not latched, squad params", false, 50, nil},
		{"not latched, before PracAt", false, 10, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := lrBrain(c.latched)
			o := lrObs(c.now, lrTarget())
			if c.edit != nil {
				c.edit(o)
			}
			act := lrAct(t, b, o)
			if tr := b.tracks[3]; tr == nil || tr.Engage {
				t.Fatalf("lrlaser engaged (track %+v)", tr)
			}
			if len(o.Self.Weapons) > 1 && act.Fire[1] {
				t.Fatalf("laser charging with no engagement: %+v", act)
			}
		})
	}
}

func TestLRLaserOffInSquadParams(t *testing.T) {
	p := DefaultParams()
	p.LRLaser, p.LRHP, p.LRKeep = 180, 1.2, 110
	if q := squadParams(p); q.LRLaser != 0 || q.LRHP != 0 || q.LRKeep != 0 {
		t.Fatalf("squadParams kept lrlaser: %+v", q)
	}
	applyFlags(&p, "lrlaser=150,lrhp=1.1,lrkeep=100")
	if p.LRLaser != 150 || p.LRHP != 1.1 || p.LRKeep != 100 {
		t.Fatalf("applyFlags: %v %v %v", p.LRLaser, p.LRHP, p.LRKeep)
	}
}

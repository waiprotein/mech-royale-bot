package mech

import "testing"

func TestArmBoostSqJinkParse(t *testing.T) {
	p := DefaultParams()
	if p.ArmBoost || p.SqJink {
		t.Fatal("defaults must be off")
	}
	applyFlags(&p, "armboost=1,sqjink=1")
	if !p.ArmBoost || !p.SqJink {
		t.Fatal("parse")
	}
	q := squadParams(p)
	if !q.SqJink {
		t.Fatal("squadParams must keep sqjink")
	}
}

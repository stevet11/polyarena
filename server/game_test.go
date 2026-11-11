package main

import (
	"math"
	"testing"
)

func newTestWorld() *World {
	return &World{players: map[uint32]*Player{}}
}

func addTank(w *World, name string, x, y float64) *Player {
	p := &Player{ID: w.newID(), Level: 1, Name: name}
	w.players[p.ID] = p
	w.Join(p, name)
	p.Ent.X, p.Ent.Y, p.Ent.Shield = x, y, 0
	return p
}

func TestBulletKillsTankAndCreditsKiller(t *testing.T) {
	w := newTestWorld()
	killer := addTank(w, "killer", 100, 100)
	victim := addTank(w, "victim", 300, 100)
	killer.Score = 900
	victim.Score = 300
	victim.Ent.HP = 1
	killer.In = Input{Aim: 0, Fire: true}
	for i := 0; i < TickRate*2 && victim.Alive; i++ {
		w.step(Dt)
	}
	if victim.Alive || victim.Ent != nil {
		t.Fatalf("victim should be dead")
	}
	if killer.Kills != 1 {
		t.Fatalf("killer kills = %d, want 1", killer.Kills)
	}
	if killer.Score <= 900 {
		t.Fatalf("killer should have gained xp, score=%d", killer.Score)
	}
	for _, e := range w.ents {
		if e.Kind == KindTank && e.Player == victim {
			t.Fatalf("dead tank still in world")
		}
	}
}

func TestBulletsDoNotBounce(t *testing.T) {
	w := newTestWorld()
	p := addTank(w, "p", 100, 100)
	p.Stats[StatBulletPen] = MaxStat // enough hp to survive a square
	p.In = Input{Aim: 0, Fire: true}
	w.step(Dt) // fires one bullet
	var bul *Entity
	for _, e := range w.ents {
		if e.Kind == KindBullet {
			bul = e
		}
	}
	if bul == nil {
		t.Fatal("no bullet fired")
	}
	sq := &Entity{ID: w.newID(), Kind: KindShape, Sides: 4, X: bul.X + 60, Y: bul.Y, Radius: 22, HP: 10, MaxHP: 10, BodyDmg: 8, Mass: 0.6}
	w.ents = append(w.ents, sq)
	p.In.Fire = false
	vx0 := bul.VX
	for i := 0; i < TickRate/2; i++ {
		w.step(Dt)
	}
	if sq.HP >= sq.MaxHP {
		t.Fatalf("square should have been hit, hp=%v", sq.HP)
	}
	if bul.Dead || bul.HP <= 0 {
		t.Fatalf("bullet should have survived the square")
	}
	if bul.VX < vx0*0.99 {
		t.Fatalf("bullet slowed or reversed: vx %v -> %v", vx0, bul.VX)
	}
	// one pass = one shove, regardless of how many ticks the bullet overlapped
	maxPush := 200 * bul.Mass / (bul.Mass + sq.Mass)
	if sq.VX > maxPush*1.01 {
		t.Fatalf("square shoved %v px/s, more than a single hit's %v", sq.VX, maxPush)
	}
}

func TestShapesDoNotDamageEachOther(t *testing.T) {
	w := newTestWorld()
	w.spawnShape()
	w.spawnShape()
	a, b := w.ents[0], w.ents[1]
	b.X, b.Y = a.X+a.Radius*0.5, a.Y
	for i := 0; i < TickRate; i++ {
		w.step(Dt)
	}
	for _, e := range w.ents {
		if e.HP < e.MaxHP {
			t.Fatalf("shape lost hp from a shape collision: %v/%v", e.HP, e.MaxHP)
		}
	}
	if math.Hypot(a.X-b.X, a.Y-b.Y) < a.Radius+b.Radius-1 {
		t.Fatalf("shapes were not pushed apart")
	}
}

func TestLevelingAndClassUpgrade(t *testing.T) {
	w := newTestWorld()
	p := addTank(w, "p", 100, 100)
	w.addXP(p, 100000)
	if p.Level != MaxLevel {
		t.Fatalf("level = %d, want %d", p.Level, MaxLevel)
	}
	// 27 points for levels 2..28, then one each at 31, 34, 37, 40, 43, 45
	if p.Points != 33 {
		t.Fatalf("points = %d, want 33", p.Points)
	}
	w.UpgradeClass(p, ClsRailgun) // not reachable directly from Scout
	if p.Class != ClsScout {
		t.Fatalf("illegal class jump allowed")
	}
	w.UpgradeClass(p, ClsMarksman)
	w.UpgradeClass(p, ClsRailgun)
	w.UpgradeClass(p, ClsLongbow)
	if p.Class != ClsLongbow {
		t.Fatalf("class = %d, want Longbow", p.Class)
	}
	for i := 0; i < 10; i++ {
		w.UpgradeStat(p, StatReload)
	}
	if p.Stats[StatReload] != MaxStat || p.Points != 33-MaxStat {
		t.Fatalf("stat cap not enforced: %v points=%d", p.Stats, p.Points)
	}
}

func TestClassTreeIsConsistent(t *testing.T) {
	for i, c := range classes {
		if c.ID != i {
			t.Fatalf("class %q has id %d at index %d", c.Name, c.ID, i)
		}
		for _, u := range c.Upgrades {
			if classes[u].Level <= c.Level {
				t.Fatalf("%s -> %s does not raise the level requirement", c.Name, classes[u].Name)
			}
		}
	}
}

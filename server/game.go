package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

const (
	WorldW         = 6000.0
	WorldH         = 6000.0
	TickRate       = 60
	SendEvery      = 3 // broadcast every N ticks -> 20 Hz
	Dt             = 1.0 / TickRate
	TankBaseRadius = 26.0
	MaxLevel       = 45
	MaxStat        = 7
	NumStats       = 8
	TargetShapes   = 500
	HitCooldown    = 0.25 // seconds between damage events for one pair
	NestRadius     = 800.0
	MaxAlphas      = 4
	SpawnShield    = 4.0 // seconds of invulnerability after spawn
)

const (
	StatRegen = iota
	StatMaxHP
	StatBody
	StatBulletSpeed
	StatBulletPen
	StatBulletDmg
	StatReload
	StatSpeed
)

var statNames = []string{
	"Health Regen", "Max Health", "Body Damage", "Bullet Speed",
	"Bullet Penetration", "Bullet Damage", "Reload", "Movement Speed",
}

type Kind uint8

const (
	KindTank Kind = iota
	KindBullet
	KindShape
)

type Entity struct {
	ID        uint32
	Kind      Kind
	X, Y      float64
	VX, VY    float64
	Radius    float64
	Rot       float64
	AngVel    float64
	HP, MaxHP float64
	BodyDmg   float64 // damage per second of contact
	Mass      float64
	Sides     int     // shapes
	Owner     uint32  // player id for tanks and bullets
	Damage    float64 // bullets: damage per hit
	Life      float64 // bullets: seconds remaining
	XP        int     // shapes: reward
	Shield    float64 // tanks: spawn protection remaining
	DriftX    float64 // shapes: wander velocity
	DriftY    float64
	Dead      bool
	LastHitBy uint32 // player id
	hits      map[uint32]float64
	Player    *Player
}

type Input struct {
	DX, DY float64
	Aim    float64
	Fire   bool
}

type Player struct {
	ID        uint32
	Name      string
	Client    *Client
	Ent       *Entity
	XP        int
	Score     int
	Level     int
	Points    int
	Stats     [NumStats]int
	Class     int
	In        Input
	Reloads   []float64
	NoDmgTime float64
	Alive     bool
	Joined    bool
	Kills     int
}

var xpToNext [MaxLevel + 1]int

func init() {
	for l := 1; l < MaxLevel; l++ {
		xpToNext[l] = int(12 + 6*float64(l) + 1.2*float64(l*l))
	}
}

// points granted for reaching level l
func pointsForLevel(l int) int {
	if l <= 28 {
		return 1
	}
	switch l {
	case 31, 34, 37, 40, 43, 45:
		return 1
	}
	return 0
}

// ---------- player derived stats ----------

func (p *Player) maxHP() float64 {
	return 50 + 2*float64(p.Level) + 20*float64(p.Stats[StatMaxHP])
}
func (p *Player) bodyDmg() float64 { return 20 + 6*float64(p.Stats[StatBody]) }
func (p *Player) moveSpeed() float64 {
	return 250 + 24*float64(p.Stats[StatSpeed]) - 0.8*float64(p.Level)
}
func (p *Player) radius() float64 { return TankBaseRadius * (1 + 0.012*float64(p.Level-1)) }
func (p *Player) fov() float64 {
	return classes[p.Class].FOV * (1 + 0.004*float64(p.Level-1))
}
func (p *Player) regenPerSec() float64 {
	return p.maxHP() * (0.004 + 0.003*float64(p.Stats[StatRegen]))
}

// ---------- world ----------

type World struct {
	mu           sync.Mutex
	ents         []*Entity
	players      map[uint32]*Player
	nextID       uint32
	tick         uint64
	targetShapes int // shapes are respawned to maintain this count
}

func NewWorld() *World {
	w := &World{players: map[uint32]*Player{}, targetShapes: TargetShapes}
	for i := 0; i < w.targetShapes; i++ {
		w.spawnShape()
	}
	return w
}

func (w *World) newID() uint32 {
	w.nextID++
	return w.nextID
}

func (w *World) Run() {
	t := time.NewTicker(time.Second / TickRate)
	defer t.Stop()
	for range t.C {
		w.mu.Lock()
		w.step(Dt)
		w.tick++
		if w.tick%SendEvery == 0 {
			w.broadcast()
		}
		w.mu.Unlock()
	}
}

// ---------- players ----------

func (w *World) AddPlayer(c *Client) *Player {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := &Player{ID: w.newID(), Client: c, Level: 1, Name: "Unnamed"}
	w.players[p.ID] = p
	return p
}

func (w *World) RemovePlayer(p *Player) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p.Ent != nil {
		p.Ent.Dead = true
		p.Ent = nil
	}
	p.Alive = false
	delete(w.players, p.ID)
}

func (w *World) Join(p *Player, name string) {
	if p.Alive {
		return // no renaming mid-life: clients cache tank names by entity id
	}
	p.Name = sanitizeName(name)
	p.Joined = true
	// reset progression on each spawn
	p.XP, p.Score, p.Level, p.Points, p.Kills = 0, 0, 1, 0, 0
	p.Stats = [NumStats]int{}
	p.Class = ClsScout
	p.Reloads = make([]float64, len(classes[p.Class].Barrels))
	p.In = Input{}

	e := &Entity{
		ID: w.newID(), Kind: KindTank, Owner: p.ID, Player: p,
		Radius: p.radius(), Mass: 1, Shield: SpawnShield,
	}
	e.MaxHP = p.maxHP()
	e.HP = e.MaxHP
	e.BodyDmg = p.bodyDmg()
	e.X, e.Y = w.findSpawn()
	w.ents = append(w.ents, e)
	p.Ent = e
	p.Alive = true
}

// pick a spot away from other tanks and outside the nest
func (w *World) findSpawn() (float64, float64) {
	best, bx, by := -1.0, 0.0, 0.0
	for try := 0; try < 12; try++ {
		x := 200 + rand.Float64()*(WorldW-400)
		y := 200 + rand.Float64()*(WorldH-400)
		if math.Hypot(x-WorldW/2, y-WorldH/2) < NestRadius+300 {
			continue
		}
		minD := math.Inf(1)
		for _, e := range w.ents {
			if e.Kind == KindTank {
				minD = math.Min(minD, math.Hypot(e.X-x, e.Y-y))
			}
		}
		if minD > best {
			best, bx, by = minD, x, y
		}
	}
	if best < 0 {
		return WorldW / 2, 300
	}
	return bx, by
}

func (w *World) UpgradeStat(p *Player, i int) {
	if !p.Alive || i < 0 || i >= NumStats || p.Points <= 0 || p.Stats[i] >= MaxStat {
		return
	}
	p.Points--
	p.Stats[i]++
	w.refreshTank(p)
}

func (w *World) UpgradeClass(p *Player, cls int) {
	if !p.Alive || cls < 0 || cls >= len(classes) {
		return
	}
	cur := classes[p.Class]
	ok := false
	for _, u := range cur.Upgrades {
		if u == cls {
			ok = true
		}
	}
	if !ok || p.Level < classes[cls].Level {
		return
	}
	p.Class = cls
	p.Reloads = make([]float64, len(classes[cls].Barrels))
	w.refreshTank(p)
}

func (w *World) refreshTank(p *Player) {
	e := p.Ent
	if e == nil {
		return
	}
	nm := p.maxHP()
	if nm > e.MaxHP {
		e.HP += nm - e.MaxHP
	}
	e.MaxHP = nm
	if e.HP > e.MaxHP {
		e.HP = e.MaxHP
	}
	e.BodyDmg = p.bodyDmg()
	e.Radius = p.radius()
	e.Mass = 1 + 0.03*float64(p.Level)
}

func (w *World) addXP(p *Player, n int) {
	if !p.Alive || n <= 0 {
		return
	}
	p.XP += n
	p.Score += n
	for p.Level < MaxLevel && p.XP >= xpToNext[p.Level] {
		p.XP -= xpToNext[p.Level]
		p.Level++
		p.Points += pointsForLevel(p.Level)
	}
	w.refreshTank(p)
}

// ---------- shapes ----------

func (w *World) countAlphas() int {
	n := 0
	for _, e := range w.ents {
		if e.Kind == KindShape && e.Sides == 5 && e.Radius > 80 {
			n++
		}
	}
	return n
}

func (w *World) spawnShape() {
	x := 60 + rand.Float64()*(WorldW-120)
	y := 60 + rand.Float64()*(WorldH-120)
	inNest := math.Hypot(x-WorldW/2, y-WorldH/2) < NestRadius
	e := &Entity{ID: w.newID(), Kind: KindShape, X: x, Y: y, Rot: rand.Float64() * math.Pi * 2}
	r := rand.Float64()
	switch {
	case inNest && r < 0.1 && w.countAlphas() < MaxAlphas:
		e.Sides, e.Radius, e.MaxHP, e.BodyDmg, e.XP, e.Mass = 5, 110, 3000, 40, 3000, 40
	case inNest:
		e.Sides, e.Radius, e.MaxHP, e.BodyDmg, e.XP, e.Mass = 5, 36, 100, 12, 130, 2
	case r < 0.5:
		e.Sides, e.Radius, e.MaxHP, e.BodyDmg, e.XP, e.Mass = 4, 22, 10, 8, 10, 0.6
	case r < 0.85:
		e.Sides, e.Radius, e.MaxHP, e.BodyDmg, e.XP, e.Mass = 3, 20, 30, 8, 25, 0.5
	default:
		e.Sides, e.Radius, e.MaxHP, e.BodyDmg, e.XP, e.Mass = 5, 36, 100, 12, 130, 2
	}
	e.HP = e.MaxHP
	a := rand.Float64() * math.Pi * 2
	e.DriftX, e.DriftY = math.Cos(a)*12, math.Sin(a)*12
	e.AngVel = (rand.Float64() - 0.5) * 0.6
	w.ents = append(w.ents, e)
}

// ---------- simulation ----------

func (w *World) step(dt float64) {
	// player control
	for _, p := range w.players {
		e := p.Ent
		if e == nil {
			continue
		}
		e.Rot = p.In.Aim
		dx, dy := p.In.DX, p.In.DY
		if l := math.Hypot(dx, dy); l > 1 {
			dx, dy = dx/l, dy/l
		}
		sp := p.moveSpeed()
		k := 1 - math.Exp(-7*dt)
		e.VX += (dx*sp - e.VX) * k
		e.VY += (dy*sp - e.VY) * k

		if p.In.Fire && e.Shield > 0 {
			e.Shield = 0
		}
		if e.Shield > 0 {
			e.Shield -= dt
		}
		p.NoDmgTime += dt
		regen := p.regenPerSec()
		if p.NoDmgTime > 6 {
			regen *= 8
		}
		e.HP = math.Min(e.MaxHP, e.HP+regen*dt)
		w.shoot(p, dt)
	}

	// movement
	for _, e := range w.ents {
		switch e.Kind {
		case KindShape:
			k := 1 - math.Exp(-1.5*dt)
			e.VX += (e.DriftX - e.VX) * k
			e.VY += (e.DriftY - e.VY) * k
			e.Rot += e.AngVel * dt
		case KindBullet:
			e.Life -= dt
			if e.Life <= 0 {
				e.Dead = true
			}
		}
		e.X += e.VX * dt
		e.Y += e.VY * dt
		if e.Kind == KindBullet {
			if e.X < -100 || e.Y < -100 || e.X > WorldW+100 || e.Y > WorldH+100 {
				e.Dead = true
			}
		} else {
			if e.X < e.Radius {
				e.X, e.VX = e.Radius, math.Abs(e.VX)*0.3
				e.DriftX = math.Abs(e.DriftX)
			} else if e.X > WorldW-e.Radius {
				e.X, e.VX = WorldW-e.Radius, -math.Abs(e.VX)*0.3
				e.DriftX = -math.Abs(e.DriftX)
			}
			if e.Y < e.Radius {
				e.Y, e.VY = e.Radius, math.Abs(e.VY)*0.3
				e.DriftY = math.Abs(e.DriftY)
			} else if e.Y > WorldH-e.Radius {
				e.Y, e.VY = WorldH-e.Radius, -math.Abs(e.VY)*0.3
				e.DriftY = -math.Abs(e.DriftY)
			}
		}
		// hit cooldowns
		for id, t := range e.hits {
			t -= dt
			if t <= 0 {
				delete(e.hits, id)
			} else {
				e.hits[id] = t
			}
		}
	}

	w.collisions()

	// deaths & cleanup
	alive := w.ents[:0]
	shapes := 0
	for _, e := range w.ents {
		if !e.Dead && e.HP <= 0 {
			e.Dead = true
			w.onDeath(e)
		}
		if e.Dead {
			continue
		}
		if e.Kind == KindShape {
			shapes++
		}
		alive = append(alive, e)
	}
	for i := len(alive); i < len(w.ents); i++ {
		w.ents[i] = nil
	}
	w.ents = alive
	for i := 0; i < 3 && shapes < w.targetShapes; i++ {
		w.spawnShape()
		shapes++
	}
}

func (w *World) onDeath(e *Entity) {
	killer := w.players[e.LastHitBy]
	switch e.Kind {
	case KindShape:
		if killer != nil {
			w.addXP(killer, e.XP)
		}
	case KindTank:
		p := e.Player
		if p == nil {
			return
		}
		p.Alive = false
		p.Ent = nil
		kname := "the arena"
		if killer != nil && killer != p {
			kname = killer.Name
			killer.Kills++
			w.addXP(killer, p.Score/3+20)
		}
		if p.Client != nil {
			p.Client.Send(encodeDead(p.Score, p.Level, kname, p.Kills))
		}
	}
}

func (w *World) shoot(p *Player, dt float64) {
	e := p.Ent
	c := classes[p.Class]
	if len(p.Reloads) != len(c.Barrels) {
		p.Reloads = make([]float64, len(c.Barrels))
	}
	reloadBase := 0.62 * (1 - 0.08*float64(p.Stats[StatReload]))
	for i := range c.Barrels {
		b := &c.Barrels[i]
		rl := reloadBase * b.Reload
		floor := rl * b.Delay
		if p.Reloads[i] > 0 {
			p.Reloads[i] -= dt
		}
		if !p.In.Fire {
			if p.Reloads[i] < floor {
				p.Reloads[i] = floor
			}
			continue
		}
		if p.Reloads[i] <= 0 {
			p.Reloads[i] = rl
			w.fireBarrel(p, e, b)
		}
	}
}

func (w *World) fireBarrel(p *Player, e *Entity, b *Barrel) {
	dir := e.Rot + b.Angle
	cos, sin := math.Cos(dir), math.Sin(dir)
	mx := e.X + cos*b.Length*e.Radius - sin*b.OffX*e.Radius
	my := e.Y + sin*b.Length*e.Radius + cos*b.OffX*e.Radius
	ang := dir + (rand.Float64()*2-1)*b.Spread
	spd := (560 + 55*float64(p.Stats[StatBulletSpeed])) * b.Speed
	rad := e.Radius * 0.5 * b.Width * b.Size
	dmg := (7 + 3*float64(p.Stats[StatBulletDmg])) * b.Damage
	pen := (8 + 4*float64(p.Stats[StatBulletPen])) * b.Pen
	bul := &Entity{
		ID: w.newID(), Kind: KindBullet, Owner: p.ID,
		X: mx, Y: my, Rot: ang,
		VX:     math.Cos(ang)*spd + e.VX*0.2,
		VY:     math.Sin(ang)*spd + e.VY*0.2,
		Radius: rad, HP: pen, MaxHP: pen, Damage: dmg,
		Mass: 0.12 * (rad / 11) * (rad / 11), Life: 2.0 * b.Life,
	}
	w.ents = append(w.ents, bul)
	recoil := 22 * b.Recoil
	e.VX -= cos * recoil
	e.VY -= sin * recoil
}

// sort-and-sweep broadphase on X
func (w *World) collisions() {
	es := w.ents
	sort.Slice(es, func(i, j int) bool { return es[i].X-es[i].Radius < es[j].X-es[j].Radius })
	for i, a := range es {
		if a.Dead {
			continue
		}
		maxX := a.X + a.Radius
		for j := i + 1; j < len(es); j++ {
			b := es[j]
			if b.X-b.Radius > maxX {
				break
			}
			if b.Dead {
				continue
			}
			w.collide(a, b)
		}
	}
}

func (w *World) collide(a, b *Entity) {
	if a.Kind == KindBullet && b.Kind == KindBullet && a.Owner == b.Owner {
		return
	}
	if (a.Kind == KindBullet && b.Kind == KindTank || a.Kind == KindTank && b.Kind == KindBullet) && a.Owner == b.Owner {
		return
	}
	dx, dy := b.X-a.X, b.Y-a.Y
	rr := a.Radius + b.Radius
	d2 := dx*dx + dy*dy
	if d2 >= rr*rr {
		return
	}
	d := math.Sqrt(d2)
	if d < 1e-6 {
		dx, dy, d = 1, 0, 1
	}
	nx, ny := dx/d, dy/d
	aB, bB := a.Kind == KindBullet, b.Kind == KindBullet

	switch {
	case !aB && !bB:
		// body vs body: separate, bounce, and shove
		tot := a.Mass + b.Mass
		wa, wb := b.Mass/tot, a.Mass/tot
		over := rr - d
		a.X -= nx * over * wa
		a.Y -= ny * over * wa
		b.X += nx * over * wb
		b.Y += ny * over * wb
		vn := (b.VX-a.VX)*nx + (b.VY-a.VY)*ny
		if vn < 0 {
			j := -(1 + 0.4) * vn / (1/a.Mass + 1/b.Mass)
			a.VX -= nx * j / a.Mass
			a.VY -= ny * j / a.Mass
			b.VX += nx * j / b.Mass
			b.VY += ny * j / b.Mass
		}
		push := 90.0
		a.VX -= nx * push * wa
		a.VY -= ny * push * wa
		b.VX += nx * push * wb
		b.VY += ny * push * wb
	}
	// bullets never receive impulses; bullet-on-body knockback is applied once
	// per damage event below so it doesn't scale with overlap time

	// damage (rate limited per pair); shapes only bump each other
	if a.Kind == KindShape && b.Kind == KindShape {
		return
	}
	if a.hits[b.ID] > 0 || b.hits[a.ID] > 0 {
		return
	}
	if a.hits == nil {
		a.hits = map[uint32]float64{}
	}
	a.hits[b.ID] = HitCooldown
	w.damage(a, b)
	w.damage(b, a)
	if aB != bB {
		bul, body := a, b
		if bB {
			bul, body = b, a
		}
		if sp := math.Hypot(bul.VX, bul.VY); sp > 0 {
			push := 200 * bul.Mass / (bul.Mass + body.Mass)
			body.VX += bul.VX / sp * push
			body.VY += bul.VY / sp * push
		}
	}
}

// src hits dst
func (w *World) damage(src, dst *Entity) {
	if dst.Kind == KindTank && dst.Shield > 0 {
		return
	}
	var dmg float64
	switch {
	case src.Kind == KindBullet:
		dmg = src.Damage
	case dst.Kind == KindBullet:
		dmg = src.BodyDmg // bullets lose full body damage of what they hit
	default:
		dmg = src.BodyDmg * HitCooldown
	}
	dst.HP -= dmg
	if dst.Kind == KindTank && dst.Player != nil {
		dst.Player.NoDmgTime = 0
	}
	if src.Owner != 0 && (src.Kind == KindBullet || src.Kind == KindTank) {
		dst.LastHitBy = src.Owner
	}
}

// ---------- snapshots ----------

func (w *World) broadcast() {
	lb := make([]*Player, 0, len(w.players))
	for _, p := range w.players {
		if p.Alive {
			lb = append(lb, p)
		}
	}
	sort.Slice(lb, func(i, j int) bool {
		if lb[i].Score != lb[j].Score {
			return lb[i].Score > lb[j].Score
		}
		return lb[i].ID < lb[j].ID // stable order so ties don't resend the board
	})
	if len(lb) > 10 {
		lb = lb[:10]
	}
	for _, p := range w.players {
		if p.Client == nil || !p.Joined {
			continue
		}
		w.sendState(p, lb)
	}
}

func (w *World) sendState(p *Player, lb []*Player) {
	c := p.Client
	cx, cy, fov := c.camX, c.camY, 1.0
	if p.Ent != nil {
		cx, cy, fov = p.Ent.X, p.Ent.Y, p.fov()
		c.camX, c.camY = cx, cy
	}
	hw, hh := 1150*fov+150, 700*fov+150

	buf := wbuf{b: c.scratch[:0]}
	buf.u8(MsgState)

	// me
	if p.Ent != nil {
		buf.u32(p.Ent.ID)
	} else {
		buf.u32(0)
	}
	buf.f32(cx)
	buf.f32(cy)
	if p.Ent != nil {
		buf.f32(p.Ent.HP)
		buf.f32(p.Ent.MaxHP)
	} else {
		buf.f32(0)
		buf.f32(0)
	}
	buf.u8c(p.Level)
	buf.u32c(p.XP)
	buf.u32c(xpToNext[p.Level])
	buf.u32c(p.Score)
	buf.u8c(p.Points)
	for _, s := range p.Stats {
		buf.u8c(s)
	}
	buf.u8c(p.Class)
	buf.f32(fov)
	if p.Alive {
		buf.u8(1)
	} else {
		buf.u8(0)
	}
	upAt := len(buf.b)
	buf.u8(0)
	if p.Ent != nil {
		n := 0
		for _, u := range classes[p.Class].Upgrades {
			if p.Level >= classes[u].Level {
				buf.u8c(u)
				n++
			}
		}
		buf.b[upAt] = uint8(n)
	}

	// entities
	countAt := len(buf.b)
	buf.u16(0)
	n := 0
	c.newNames = c.newNames[:0]
	for _, e := range w.ents {
		if n == 65535 {
			break
		}
		if e.X < cx-hw-e.Radius || e.X > cx+hw+e.Radius || e.Y < cy-hh-e.Radius || e.Y > cy+hh+e.Radius {
			continue
		}
		n++
		buf.u32(e.ID)
		buf.u8(uint8(e.Kind))
		buf.pos(e.X)
		buf.pos(e.Y)
		buf.u16(quant16(e.Radius * 10))
		switch e.Kind {
		case KindTank:
			var flags uint8
			if e.Owner == p.ID {
				flags |= FlagMine
			}
			if e.Shield > 0 {
				flags |= FlagShield
			}
			_, known := c.knownNames[e.ID]
			if !known {
				flags |= FlagName
			}
			buf.u8(flags)
			buf.angle(e.Rot)
			buf.frac(e.HP, e.MaxHP)
			cls, lvl, name := 0, 1, ""
			if e.Player != nil {
				cls, lvl, name = e.Player.Class, e.Player.Level, e.Player.Name
			}
			buf.u8c(cls)
			buf.u8c(lvl)
			if !known {
				buf.str(name)
				c.newNames = append(c.newNames, e.ID)
			}
		case KindBullet:
			var flags uint8
			if e.Owner == p.ID {
				flags |= FlagMine
			}
			buf.u8(flags)
		case KindShape:
			buf.u8c(e.Sides)
			buf.angle(e.Rot)
			buf.frac(e.HP, e.MaxHP)
		}
	}
	binary.LittleEndian.PutUint16(buf.b[countAt:], uint16(n))

	// leaderboard, only when it changed for this viewer
	lbBytes := encodeLeaderboard(lb, p)
	lbChanged := !bytes.Equal(lbBytes, c.lastLB)
	if lbChanged {
		buf.u8(1)
		buf.b = append(buf.b, lbBytes...)
	} else {
		buf.u8(0)
	}

	c.scratch = buf.b
	// The frame is copied because scratch is reused next tick. Names and the
	// leaderboard only count as delivered once the frame is actually queued;
	// a dropped frame means they are resent next time.
	if c.Send(append([]byte(nil), buf.b...)) {
		for _, id := range c.newNames {
			c.knownNames[id] = struct{}{}
		}
		if lbChanged {
			c.lastLB = lbBytes
		}
	}
}

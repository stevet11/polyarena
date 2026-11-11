package main

import "math"

// Barrel describes one gun on a tank. Geometry fields are in units of the
// tank's radius and are sent to the client for rendering; the remaining
// fields are gameplay multipliers that stay on the server.
type Barrel struct {
	Angle    float64 `json:"a"`  // radians, relative to the aim direction
	OffX     float64 `json:"x"`  // lateral offset from the tank centre line
	Length   float64 `json:"l"`  // distance from tank centre to muzzle
	Width    float64 `json:"w"`  // width at the base
	WidthEnd float64 `json:"we"` // width at the muzzle (trapezoid barrels)

	Delay  float64 `json:"-"` // fraction of reload before first shot
	Reload float64 `json:"-"`
	Damage float64 `json:"-"`
	Pen    float64 `json:"-"`
	Speed  float64 `json:"-"`
	Size   float64 `json:"-"`
	Spread float64 `json:"-"` // max random deviation in radians
	Recoil float64 `json:"-"`
	Life   float64 `json:"-"`
}

type Class struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Level    int      `json:"level"` // level required to pick this class
	FOV      float64  `json:"fov"`
	Barrels  []Barrel `json:"barrels"`
	Upgrades []int    `json:"upgrades"`
}

type bopt func(*Barrel)

func dl(v float64) bopt  { return func(b *Barrel) { b.Delay = v } }
func rl(v float64) bopt  { return func(b *Barrel) { b.Reload = v } }
func dm(v float64) bopt  { return func(b *Barrel) { b.Damage = v } }
func pn(v float64) bopt  { return func(b *Barrel) { b.Pen = v } }
func sp(v float64) bopt  { return func(b *Barrel) { b.Speed = v } }
func sz(v float64) bopt  { return func(b *Barrel) { b.Size = v } }
func spr(v float64) bopt { return func(b *Barrel) { b.Spread = v } }
func rc(v float64) bopt  { return func(b *Barrel) { b.Recoil = v } }
func lf(v float64) bopt  { return func(b *Barrel) { b.Life = v } }
func trap(we float64) bopt {
	return func(b *Barrel) { b.WidthEnd = we }
}

func barrel(angleDeg, offX, length, width float64, opts ...bopt) Barrel {
	b := Barrel{
		Angle: angleDeg * math.Pi / 180, OffX: offX, Length: length, Width: width, WidthEnd: width,
		Reload: 1, Damage: 1, Pen: 1, Speed: 1, Size: 1, Recoil: 1, Life: 1,
	}
	for _, o := range opts {
		o(&b)
	}
	return b
}

const (
	ClsScout = iota
	ClsTwin
	ClsMarksman
	ClsSpray
	ClsFlanker
	ClsTrident
	ClsCross
	ClsTwinFlank
	ClsRailgun
	ClsCannon
	ClsHailstorm
	ClsThruster
	ClsFan
	ClsStar
	ClsLongbow
	ClsSiege
)

var classes = buildClasses()

func buildClasses() []Class {
	cs := []Class{
		{ID: ClsScout, Name: "Scout", Level: 1, FOV: 1,
			Barrels:  []Barrel{barrel(0, 0, 1.9, 0.84)},
			Upgrades: []int{ClsTwin, ClsMarksman, ClsSpray, ClsFlanker}},

		// ---- tier 2 (level 15) ----
		{ID: ClsTwin, Name: "Twin", Level: 15, FOV: 1,
			Barrels: []Barrel{
				barrel(0, -0.48, 1.85, 0.76, dm(0.65), pn(0.9)),
				barrel(0, 0.48, 1.85, 0.76, dm(0.65), pn(0.9), dl(0.5)),
			},
			Upgrades: []int{ClsTrident, ClsCross, ClsTwinFlank}},
		{ID: ClsMarksman, Name: "Marksman", Level: 15, FOV: 1.18,
			Barrels:  []Barrel{barrel(0, 0, 2.4, 0.84, rl(1.5), sp(1.5), pn(1.2), lf(1.3), rc(1.5))},
			Upgrades: []int{ClsRailgun}},
		{ID: ClsSpray, Name: "Spray", Level: 15, FOV: 1,
			Barrels:  []Barrel{barrel(0, 0, 1.75, 0.84, trap(1.45), rl(0.5), dm(0.7), spr(0.18), sp(0.95))},
			Upgrades: []int{ClsCannon, ClsHailstorm}},
		{ID: ClsFlanker, Name: "Flanker", Level: 15, FOV: 1,
			Barrels: []Barrel{
				barrel(0, 0, 1.9, 0.84),
				barrel(180, 0, 1.6, 0.84, dl(0.5)),
			},
			Upgrades: []int{ClsCross, ClsTwinFlank, ClsThruster}},

		// ---- tier 3 (level 30) ----
		{ID: ClsTrident, Name: "Trident", Level: 30, FOV: 1,
			Barrels: []Barrel{
				barrel(-28, 0, 1.75, 0.76, dm(0.7), dl(0.5)),
				barrel(28, 0, 1.75, 0.76, dm(0.7), dl(0.5)),
				barrel(0, 0, 2.0, 0.8, dm(0.7)),
			},
			Upgrades: []int{ClsFan}},
		{ID: ClsCross, Name: "Cross", Level: 30, FOV: 1,
			Barrels: []Barrel{
				barrel(0, 0, 1.85, 0.8, dm(0.75)),
				barrel(90, 0, 1.85, 0.8, dm(0.75), dl(0.5)),
				barrel(180, 0, 1.85, 0.8, dm(0.75)),
				barrel(270, 0, 1.85, 0.8, dm(0.75), dl(0.5)),
			},
			Upgrades: []int{ClsStar}},
		{ID: ClsTwinFlank, Name: "Twin Flank", Level: 30, FOV: 1,
			Barrels: []Barrel{
				barrel(0, -0.48, 1.85, 0.76, dm(0.55), pn(0.9)),
				barrel(0, 0.48, 1.85, 0.76, dm(0.55), pn(0.9), dl(0.5)),
				barrel(180, -0.48, 1.85, 0.76, dm(0.55), pn(0.9)),
				barrel(180, 0.48, 1.85, 0.76, dm(0.55), pn(0.9), dl(0.5)),
			}},
		{ID: ClsRailgun, Name: "Railgun", Level: 30, FOV: 1.38,
			Barrels:  []Barrel{barrel(0, 0, 2.8, 0.84, rl(2.0), sp(1.9), dm(1.35), pn(1.6), lf(1.4), rc(2))},
			Upgrades: []int{ClsLongbow}},
		{ID: ClsCannon, Name: "Cannon", Level: 30, FOV: 1.05,
			Barrels:  []Barrel{barrel(0, 0, 1.85, 1.45, rl(3.5), dm(3.0), pn(2.5), sp(0.7), rc(14), lf(1.2))},
			Upgrades: []int{ClsSiege}},
		{ID: ClsHailstorm, Name: "Hailstorm", Level: 30, FOV: 1,
			Barrels: []Barrel{
				barrel(0, -0.45, 1.65, 0.42, rl(0.6), dm(0.42), pn(0.6), spr(0.08)),
				barrel(0, 0.45, 1.65, 0.42, rl(0.6), dm(0.42), pn(0.6), spr(0.08), dl(0.66)),
				barrel(0, 0, 1.95, 0.42, rl(0.6), dm(0.42), pn(0.6), spr(0.08), dl(0.33)),
			}},
		{ID: ClsThruster, Name: "Thruster", Level: 30, FOV: 1,
			Barrels: []Barrel{
				barrel(0, 0, 1.9, 0.84),
				barrel(150, 0, 1.6, 0.84, dm(0.3), pn(0.6), rc(3.5), dl(0.5), lf(0.6)),
				barrel(210, 0, 1.6, 0.84, dm(0.3), pn(0.6), rc(3.5), dl(0.5), lf(0.6)),
			}},

		// ---- tier 4 (level 45) ----
		{ID: ClsFan, Name: "Fan", Level: 45, FOV: 1,
			Barrels: []Barrel{
				barrel(-44, 0, 1.6, 0.72, dm(0.6), dl(0.66)),
				barrel(44, 0, 1.6, 0.72, dm(0.6), dl(0.66)),
				barrel(-22, 0, 1.8, 0.72, dm(0.6), dl(0.33)),
				barrel(22, 0, 1.8, 0.72, dm(0.6), dl(0.33)),
				barrel(0, 0, 2.0, 0.76, dm(0.6)),
			}},
		{ID: ClsStar, Name: "Star", Level: 45, FOV: 1,
			Barrels: func() []Barrel {
				bs := make([]Barrel, 8)
				for i := range bs {
					bs[i] = barrel(float64(i)*45, 0, 1.8, 0.72, dm(0.65), dl(float64(i%2)*0.5))
				}
				return bs
			}()},
		{ID: ClsLongbow, Name: "Longbow", Level: 45, FOV: 1.6,
			Barrels: []Barrel{barrel(0, 0, 3.1, 0.84, rl(2.2), sp(2.15), dm(1.6), pn(1.9), lf(1.5), rc(2.5))}},
		{ID: ClsSiege, Name: "Siege", Level: 45, FOV: 1.08,
			Barrels: []Barrel{barrel(0, 0, 1.85, 1.9, rl(4.6), dm(4.6), pn(3.5), sp(0.62), rc(24), lf(1.3))}},
	}
	for i := range cs {
		if cs[i].Upgrades == nil {
			cs[i].Upgrades = []int{}
		}
	}
	return cs
}

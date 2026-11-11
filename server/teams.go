package main

import (
	"math"
	"math/rand"
)

// Game modes. In FFA every tank is hostile to every other tank. In team mode
// players are assigned to the smallest team on join; teammates cannot damage
// each other and their bullets pass through one another.
const (
	ModeFFA   = 0
	ModeTeams = 1
	MaxTeams  = 4
)

// Team ids are 1-based on the wire; 0 means "no team" (FFA).
var teamNames = [MaxTeams]string{"Blue", "Red", "Green", "Purple"}

// teamBase is the corner each team spawns near, as a fraction of the world.
var teamBase = [MaxTeams][2]float64{{0, 0}, {1, 1}, {1, 0}, {0, 1}}

const TeamBaseRadius = 700.0

// sameTeam reports whether two entities are allied: owned by the same player
// or, in team mode, by players on the same team.
func sameTeam(a, b *Entity) bool {
	if a.Owner != 0 && a.Owner == b.Owner {
		return true
	}
	return a.Team != 0 && a.Team == b.Team
}

// pickTeam returns the team with the fewest joined players (ties go to the
// lowest id), or 0 in FFA.
func (w *World) pickTeam() uint8 {
	if w.mode != ModeTeams {
		return 0
	}
	var count [MaxTeams + 1]int
	for _, p := range w.players {
		if p.Joined && p.Team != 0 {
			count[p.Team]++
		}
	}
	best := uint8(1)
	for t := uint8(2); t <= uint8(w.nTeams); t++ {
		if count[t] < count[best] {
			best = t
		}
	}
	return best
}

// teamSpawn picks a spot inside the team's base corner, away from enemies.
func (w *World) teamSpawn(team uint8) (float64, float64) {
	bx := teamBase[team-1][0]*(WorldW-2*TeamBaseRadius) + TeamBaseRadius
	by := teamBase[team-1][1]*(WorldH-2*TeamBaseRadius) + TeamBaseRadius
	best, sx, sy := -1.0, bx, by
	for try := 0; try < 12; try++ {
		a := rand.Float64() * math.Pi * 2
		r := rand.Float64() * (TeamBaseRadius - 100)
		x, y := bx+math.Cos(a)*r, by+math.Sin(a)*r
		minD := math.Inf(1)
		for _, e := range w.ents {
			if e.Kind == KindTank && e.Team != team {
				minD = math.Min(minD, math.Hypot(e.X-x, e.Y-y))
			}
		}
		if minD > best {
			best, sx, sy = minD, x, y
		}
	}
	return sx, sy
}

// teamScores sums the score of every living player per team, indexed by
// team id (index 0 unused). Returns nil in FFA.
func (w *World) teamScores() []int {
	if w.mode != ModeTeams {
		return nil
	}
	s := make([]int, w.nTeams+1)
	for _, p := range w.players {
		if p.Alive && p.Team != 0 && int(p.Team) <= w.nTeams {
			s[p.Team] += p.Score
		}
	}
	return s
}

package main

// Binary wire protocol. All messages are websocket binary frames, little-endian,
// starting with a one-byte type tag. Strings are a u8 byte length followed by
// UTF-8 bytes. Keep in sync with client/src/protocol.ts.
//
// client -> server
//   Join    u8 1, str name
//   Input   u8 2, i8 dx*127, i8 dy*127, u16 aim (angle), u8 flags (bit0 fire)
//   Stat    u8 3, u8 stat index
//   Upgrade u8 4, u8 class id
//
// server -> client
//   Init  u8 1, u32 playerId, f32 worldW, f32 worldH,
//         u8 mode (0 ffa, 1 teams), u8 nTeams, nTeams×str team name,
//         u8 nStats, nStats×str,
//         u8 nClasses, per class: str name, u8 level, f32 fov,
//           u8 nBarrels, per barrel: f32 angle, offX, length, width, widthEnd,
//           u8 nUpgrades, nUpgrades×u8
//   State u8 2, me, u16 nEnts, nEnts×entity, u8 hasLB, [leaderboard]
//     me:     u32 entId, f32 x, f32 y, f32 hp, f32 maxHp, u8 level, u32 xp,
//             u32 xpNext, u32 score, u8 points, 8×u8 stats, u8 class,
//             f32 fov, u8 alive, u8 team, u8 nUpg, nUpg×u8
//     entity: u32 id, u8 kind, u16 x (pos), u16 y (pos), u16 radius*10, then
//       tank:   u8 flags, u16 angle, u8 hp frac, u8 class, u8 level, u8 team,
//               [str name if FlagName]
//       bullet: u8 flags, u8 team
//       shape:  u8 sides, u16 angle, u8 hp frac
//     leaderboard: u8 n, per row: str name, u32 score, u8 class, u8 team, u8 isMe,
//                  then u8 nTeams, nTeams×u32 team score
//   Teams are 1-based; team 0 means none (FFA).
//   Dead  u8 3, u32 score, u8 level, str killer, u16 kills
//
// pos   = round((v + PosOffset) * PosScale) clamped to u16
// angle = round(a mod 2π / 2π * 65535)
// frac  = round(hp / maxHp * 255)

import (
	"encoding/binary"
	"math"
	"strings"
	"unicode"
)

const (
	MsgJoin    = 1
	MsgInput   = 2
	MsgStat    = 3
	MsgUpgrade = 4

	MsgInit  = 1
	MsgState = 2
	MsgDead  = 3

	FlagMine   = 1 << 0
	FlagShield = 1 << 1
	FlagName   = 1 << 2

	InputFire = 1 << 0

	PosOffset = 200.0
	PosScale  = 10.0
)

// ---------- writer ----------

type wbuf struct{ b []byte }

func (w *wbuf) u8(v uint8)   { w.b = append(w.b, v) }
func (w *wbuf) u16(v uint16) { w.b = binary.LittleEndian.AppendUint16(w.b, v) }
func (w *wbuf) u32(v uint32) { w.b = binary.LittleEndian.AppendUint32(w.b, v) }
func (w *wbuf) f32(v float64) {
	w.b = binary.LittleEndian.AppendUint32(w.b, math.Float32bits(float32(v)))
}
func (w *wbuf) str(s string) {
	if len(s) > 255 {
		s = s[:255]
	}
	w.u8(uint8(len(s)))
	w.b = append(w.b, s...)
}
func (w *wbuf) pos(v float64)   { w.u16(quant16((v + PosOffset) * PosScale)) }
func (w *wbuf) angle(a float64) { w.u16(encodeAngle(a)) }
func (w *wbuf) frac(v, max float64) {
	if max <= 0 {
		w.u8(0)
		return
	}
	w.u8(uint8(math.Round(clamp(v/max, 0, 1) * 255)))
}
func (w *wbuf) u8c(v int) { w.u8(uint8(clampInt(v, 0, 255))) }
func (w *wbuf) u32c(v int) {
	if v < 0 {
		v = 0
	}
	w.u32(uint32(v))
}

func quant16(v float64) uint16 { return uint16(math.Round(clamp(v, 0, 65535))) }

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func encodeAngle(a float64) uint16 {
	a = math.Mod(a, 2*math.Pi)
	if a < 0 {
		a += 2 * math.Pi
	}
	return uint16(math.Round(a / (2 * math.Pi) * 65535)) // a < 2π so this fits
}

func decodeAngle(v uint16) float64 { return float64(v) / 65535 * 2 * math.Pi }

// ---------- reader ----------

type rbuf struct {
	b   []byte
	i   int
	bad bool
}

func (r *rbuf) u8() uint8 {
	if r.i+1 > len(r.b) {
		r.bad = true
		return 0
	}
	v := r.b[r.i]
	r.i++
	return v
}

func (r *rbuf) u16() uint16 {
	if r.i+2 > len(r.b) {
		r.bad = true
		return 0
	}
	v := binary.LittleEndian.Uint16(r.b[r.i:])
	r.i += 2
	return v
}

func (r *rbuf) str() string {
	n := int(r.u8())
	if r.bad || r.i+n > len(r.b) {
		r.bad = true
		return ""
	}
	s := string(r.b[r.i : r.i+n])
	r.i += n
	return s
}

// ---------- client messages ----------

type clientMsg struct {
	Type  uint8
	Name  string
	Input Input
	Index int
}

func decodeClientMsg(data []byte) (clientMsg, bool) {
	r := rbuf{b: data}
	m := clientMsg{Type: r.u8()}
	switch m.Type {
	case MsgJoin:
		m.Name = r.str()
	case MsgInput:
		dx, dy := int8(r.u8()), int8(r.u8())
		aim := r.u16()
		flags := r.u8()
		m.Input = Input{
			DX:   clamp(float64(dx)/127, -1, 1),
			DY:   clamp(float64(dy)/127, -1, 1),
			Aim:  decodeAngle(aim),
			Fire: flags&InputFire != 0,
		}
	case MsgStat, MsgUpgrade:
		m.Index = int(r.u8())
	default:
		return m, false
	}
	return m, !r.bad
}

// sanitizeName keeps at most 16 printable runes of valid UTF-8.
func sanitizeName(s string) string {
	s = strings.ToValidUTF8(s, "")
	out := make([]rune, 0, 16)
	for _, c := range s {
		if len(out) == 16 {
			break
		}
		if unicode.IsPrint(c) {
			out = append(out, c)
		}
	}
	name := strings.TrimSpace(string(out))
	if name == "" {
		return "Unnamed"
	}
	return name
}

// ---------- server messages ----------

func encodeInit(playerID uint32, mode, nTeams int) []byte {
	w := wbuf{b: make([]byte, 0, 1024)}
	w.u8(MsgInit)
	w.u32(playerID)
	w.f32(WorldW)
	w.f32(WorldH)
	w.u8c(mode)
	w.u8c(nTeams)
	for i := 0; i < nTeams; i++ {
		w.str(teamNames[i])
	}
	w.u8(uint8(len(statNames)))
	for _, s := range statNames {
		w.str(s)
	}
	w.u8(uint8(len(classes)))
	for _, c := range classes {
		w.str(c.Name)
		w.u8c(c.Level)
		w.f32(c.FOV)
		w.u8(uint8(len(c.Barrels)))
		for _, b := range c.Barrels {
			w.f32(b.Angle)
			w.f32(b.OffX)
			w.f32(b.Length)
			w.f32(b.Width)
			w.f32(b.WidthEnd)
		}
		w.u8(uint8(len(c.Upgrades)))
		for _, u := range c.Upgrades {
			w.u8c(u)
		}
	}
	return w.b
}

func encodeDead(score, level int, killer string, kills int) []byte {
	w := wbuf{b: make([]byte, 0, 32)}
	w.u8(MsgDead)
	w.u32c(score)
	w.u8c(level)
	w.str(killer)
	w.u16(uint16(clampInt(kills, 0, 65535)))
	return w.b
}

// teamScores is indexed by team id (index 0 unused) or nil in FFA.
func encodeLeaderboard(lb []*Player, viewer *Player, teamScores []int) []byte {
	w := wbuf{b: make([]byte, 0, 192)}
	w.u8(uint8(len(lb)))
	for _, q := range lb {
		w.str(q.Name)
		w.u32c(q.Score)
		w.u8c(q.Class)
		w.u8(q.Team)
		if q == viewer {
			w.u8(1)
		} else {
			w.u8(0)
		}
	}
	if len(teamScores) == 0 {
		w.u8(0)
		return w.b
	}
	w.u8(uint8(len(teamScores) - 1))
	for _, s := range teamScores[1:] {
		w.u32c(s)
	}
	return w.b
}

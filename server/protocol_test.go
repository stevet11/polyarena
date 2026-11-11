package main

import (
	"math"
	"testing"
)

func TestDecodeInput(t *testing.T) {
	w := wbuf{}
	w.u8(MsgInput)
	dx, dy := int8(-127), int8(64)
	w.u8(uint8(dx))
	w.u8(uint8(dy))
	w.angle(-math.Pi / 2)
	w.u8(InputFire)
	m, ok := decodeClientMsg(w.b)
	if !ok || m.Type != MsgInput {
		t.Fatalf("decode failed: %+v ok=%v", m, ok)
	}
	if m.Input.DX != -1 || math.Abs(m.Input.DY-64.0/127) > 1e-9 || !m.Input.Fire {
		t.Fatalf("bad input: %+v", m.Input)
	}
	if d := math.Abs(m.Input.Aim - 3*math.Pi/2); d > 1e-3 {
		t.Fatalf("aim %v, want 3π/2", m.Input.Aim)
	}
}

func TestDecodeRejectsMalformed(t *testing.T) {
	cases := map[string][]byte{
		"empty":            {},
		"unknown type":     {99},
		"short input":      {MsgInput, 1, 2},
		"short stat":       {MsgStat},
		"name past end":    {MsgJoin, 10, 'a', 'b'},
		"missing name len": {MsgJoin},
	}
	for name, b := range cases {
		if _, ok := decodeClientMsg(b); ok {
			t.Errorf("%s: accepted %v", name, b)
		}
	}
}

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"":                       "Unnamed",
		"   ":                    "Unnamed",
		"bob":                    "bob",
		"a\x00b\nc":              "abc",
		"ééééééééééééééééééé":    "éééééééééééééééé", // 16 runes, not 16 bytes
		"\xff\xfeok":             "ok",
		"twelve chars and more!": "twelve chars and",
	}
	for in, want := range cases {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAngleRoundTrip(t *testing.T) {
	for _, a := range []float64{0, 1, math.Pi, -0.5, 2*math.Pi - 1e-9, 7} {
		got := decodeAngle(encodeAngle(a))
		want := math.Mod(math.Mod(a, 2*math.Pi)+2*math.Pi, 2*math.Pi)
		d := math.Abs(got - want)
		if d > math.Pi {
			d = 2*math.Pi - d
		}
		if d > 2*math.Pi/65535 {
			t.Errorf("angle %v -> %v", a, got)
		}
	}
}

// Names and the leaderboard go out once, then are omitted until something changes.
func TestStateSendsNamesAndLeaderboardOnce(t *testing.T) {
	w := newTestWorld()
	c := &Client{send: make(chan []byte, 4), knownNames: map[uint32]struct{}{}}
	p := addTank(w, "viewer", 500, 500)
	p.Client = c
	addTank(w, "other", 700, 500)

	w.broadcast()
	first := <-c.send
	w.broadcast()
	second := <-c.send

	if len(second) >= len(first) {
		t.Fatalf("second frame (%d B) should be smaller than first (%d B)", len(second), len(first))
	}
	if !containsStr(first, "other") || containsStr(second, "other") {
		t.Fatalf("name 'other' should be in first frame only")
	}
	if second[len(second)-1] != 0 {
		t.Fatalf("leaderboard should be omitted when unchanged")
	}
}

// A frame dropped because the client is slow must not mark names as delivered.
func TestDroppedFrameResendsNames(t *testing.T) {
	w := newTestWorld()
	c := &Client{send: make(chan []byte), knownNames: map[uint32]struct{}{}} // unbuffered: every Send drops
	p := addTank(w, "viewer", 500, 500)
	p.Client = c
	addTank(w, "other", 700, 500)
	w.broadcast()
	if len(c.knownNames) != 0 || c.lastLB != nil {
		t.Fatalf("dropped frame marked state as delivered")
	}
}

func containsStr(b []byte, s string) bool {
	for i := 0; i+len(s) <= len(b); i++ {
		if string(b[i:i+len(s)]) == s {
			return true
		}
	}
	return false
}

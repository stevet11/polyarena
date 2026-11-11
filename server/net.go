package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1 << 16,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Client struct {
	conn       *websocket.Conn
	send       chan []byte
	player     *Player
	world      *World
	camX, camY float64
}

type clientMsg struct {
	T    string  `json:"t"`
	Name string  `json:"name"`
	DX   float64 `json:"dx"`
	DY   float64 `json:"dy"`
	Aim  float64 `json:"aim"`
	Fire bool    `json:"fire"`
	I    int     `json:"i"`
	C    int     `json:"c"`
}

func (c *Client) SendJSON(v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Println("marshal:", err)
		return
	}
	select {
	case c.send <- b:
	default: // slow client: drop the frame
	}
}

func serveWS(w *World) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(rw, r, nil)
		if err != nil {
			log.Println("upgrade:", err)
			return
		}
		c := &Client{conn: conn, send: make(chan []byte, 16), world: w, camX: WorldW / 2, camY: WorldH / 2}
		c.player = w.AddPlayer(c)
		c.SendJSON(initMsg{T: "init", ID: c.player.ID, W: WorldW, H: WorldH, Classes: classes, Stats: statNames})
		go c.writePump()
		c.readPump()
	}
}

func (c *Client) readPump() {
	defer func() {
		c.world.RemovePlayer(c.player)
		close(c.send)
	}()
	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		var m clientMsg
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		c.handle(&m)
	}
}

func (c *Client) handle(m *clientMsg) {
	w, p := c.world, c.player
	w.mu.Lock()
	defer w.mu.Unlock()
	switch m.T {
	case "join":
		w.Join(p, m.Name)
	case "in":
		p.In = Input{DX: clamp(m.DX, -1, 1), DY: clamp(m.DY, -1, 1), Aim: m.Aim, Fire: m.Fire}
	case "stat":
		w.UpgradeStat(p, m.I)
	case "up":
		w.UpgradeClass(p, m.C)
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (c *Client) writePump() {
	ping := time.NewTicker(25 * time.Second)
	defer func() {
		ping.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ping.C:
			c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

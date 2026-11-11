package main

import (
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

	// Owned by the world goroutine (accessed under World.mu).
	knownNames map[uint32]struct{} // tank entity ids whose name this client has received
	newNames   []uint32
	lastLB     []byte
	scratch    []byte
}

// Send queues a binary frame. It never blocks: if the client is too slow the
// frame is dropped and false is returned.
func (c *Client) Send(b []byte) bool {
	select {
	case c.send <- b:
		return true
	default:
		return false
	}
}

func serveWS(w *World) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(rw, r, nil)
		if err != nil {
			log.Println("upgrade:", err)
			return
		}
		c := &Client{
			conn: conn, send: make(chan []byte, 16), world: w,
			camX: WorldW / 2, camY: WorldH / 2,
			knownNames: map[uint32]struct{}{},
		}
		c.player = w.AddPlayer(c)
		c.Send(encodeInit(c.player.ID))
		go c.writePump()
		c.readPump()
	}
}

func (c *Client) readPump() {
	defer func() {
		c.world.RemovePlayer(c.player)
		close(c.send)
	}()
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		mt, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		if mt != websocket.BinaryMessage {
			continue
		}
		m, ok := decodeClientMsg(data)
		if !ok {
			continue
		}
		c.handle(&m)
	}
}

func (c *Client) handle(m *clientMsg) {
	w, p := c.world, c.player
	w.mu.Lock()
	defer w.mu.Unlock()
	switch m.Type {
	case MsgJoin:
		w.Join(p, m.Name)
	case MsgInput:
		p.In = m.Input
	case MsgStat:
		w.UpgradeStat(p, m.Index)
	case MsgUpgrade:
		w.UpgradeClass(p, m.Index)
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
			if err := c.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
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

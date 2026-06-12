package api

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/jodacame/fluxtorrent/internal/engine"
)

// event is a server→client WebSocket message (SPEC §7).
type event struct {
	Type     string        `json:"type"` // stats|added|dropped|warning
	Hash     string        `json:"hash,omitempty"`
	Name     string        `json:"name,omitempty"`
	Torrents []engine.Info `json:"torrents,omitempty"`
}

// hub fans out events to all connected WebSocket clients.
type hub struct {
	mu       sync.RWMutex
	clients  map[*client]struct{}
	upgrader websocket.Upgrader
	register chan *client
	unreg    chan *client
	out      chan event
}

type client struct {
	conn *websocket.Conn
	send chan event
}

func newHub() *hub {
	return &hub{
		clients:  map[*client]struct{}{},
		upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
		register: make(chan *client),
		unreg:    make(chan *client),
		out:      make(chan event, 64),
	}
}

func (h *hub) run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.mu.Unlock()
		case c := <-h.unreg:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()
		case ev := <-h.out:
			h.mu.RLock()
			for c := range h.clients {
				select {
				case c.send <- ev:
				default: // drop for slow clients; never block the engine
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *hub) broadcast(ev event) {
	select {
	case h.out <- ev:
	default:
	}
}

func (h *hub) count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *hub) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{conn: conn, send: make(chan event, 32)}
	h.register <- c

	go c.writePump()
	c.readPump(h)
}

func (c *client) readPump(h *hub) {
	defer func() {
		h.unreg <- c
		_ = c.conn.Close()
	}()
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *client) writePump() {
	for ev := range c.send {
		if err := c.conn.WriteJSON(ev); err != nil {
			return
		}
	}
}

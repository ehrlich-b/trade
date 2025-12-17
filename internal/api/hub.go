package api

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

// Hub maintains active WebSocket connections and broadcasts messages
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
	stopCh  chan struct{}
}

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	lastPong time.Time
}

func NewHub() *Hub {
	h := &Hub{
		clients: make(map[*Client]bool),
		stopCh:  make(chan struct{}),
	}
	go h.cleanupLoop()
	return h
}

// Stop stops the hub's cleanup goroutine
func (h *Hub) Stop() {
	close(h.stopCh)
}

// cleanupLoop periodically checks for stale connections
func (h *Hub) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.pruneStaleClients()
		case <-h.stopCh:
			return
		}
	}
}

// pruneStaleClients removes clients that haven't responded to pings
func (h *Hub) pruneStaleClients() {
	h.mu.Lock()
	defer h.mu.Unlock()

	staleThreshold := time.Now().Add(-pongWait - 10*time.Second)
	for client := range h.clients {
		if client.lastPong.Before(staleThreshold) {
			delete(h.clients, client)
			close(client.send)
			client.conn.Close()
		}
	}
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
	h.mu.Unlock()
}

func (h *Hub) Broadcast(message interface{}) {
	data, err := json.Marshal(message)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- data:
		default:
			// Client buffer full, skip
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.lastPong = time.Now()
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		// For now, we don't process incoming messages from clients
		// Future: could handle subscriptions, order submissions via WS
	}
}

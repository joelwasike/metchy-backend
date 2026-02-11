package ws

import (
	"encoding/json"
	"sync"
)

// Client represents a single WebSocket connection with user context.
type Client struct {
	UserID   uint
	Role     string
	Send     chan []byte
	conn     interface{ SendMessage([]byte) error }
	Hub      *Hub // set so Close() can unregister; may be nil for chat/video rooms
	mu       sync.Mutex
	closed   bool
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	close(c.Send)
	if c.Hub != nil {
		c.Hub.unregister(c)
	}
}

// Hub maintains the set of active clients and broadcasts to them.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
	// userID -> clients (one user can have multiple connections)
	byUser map[uint]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[*Client]struct{}),
		byUser:  make(map[uint]map[*Client]struct{}),
	}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c.Hub = h
	h.clients[c] = struct{}{}
	if h.byUser[c.UserID] == nil {
		h.byUser[c.UserID] = make(map[*Client]struct{})
	}
	h.byUser[c.UserID][c] = struct{}{}
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	if m := h.byUser[c.UserID]; m != nil {
		delete(m, c)
		if len(m) == 0 {
			delete(h.byUser, c.UserID)
		}
	}
}

func (h *Hub) BroadcastToUser(userID uint, payload interface{}) {
	data, _ := json.Marshal(payload)
	h.mu.RLock()
	m := h.byUser[userID]
	if m == nil {
		h.mu.RUnlock()
		return
	}
	clients := make([]*Client, 0, len(m))
	for c := range m {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, c := range clients {
		select {
		case c.Send <- data:
		default:
		}
	}
}

func (h *Hub) BroadcastAll(payload interface{}) {
	data, _ := json.Marshal(payload)
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, c := range clients {
		select {
		case c.Send <- data:
		default:
		}
	}
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

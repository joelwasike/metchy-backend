package ws

import (
	"encoding/json"
	"sync"
)

// ChatRoom is one room per interaction (client + companion).
type ChatRoom struct {
	InteractionID uint
	ClientID      uint
	CompanionID   uint
	clients       map[*Client]struct{}
	mu            sync.RWMutex
}

func NewChatRoom(interactionID, clientID, companionID uint) *ChatRoom {
	return &ChatRoom{
		InteractionID: interactionID,
		ClientID:      clientID,
		CompanionID:   companionID,
		clients:       make(map[*Client]struct{}),
	}
}

func (r *ChatRoom) Join(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[c] = struct{}{}
}

func (r *ChatRoom) Leave(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, c)
}

func (r *ChatRoom) ClientCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

func (r *ChatRoom) Broadcast(from *Client, payload interface{}) {
	data, _ := json.Marshal(payload)
	r.mu.RLock()
	clients := make([]*Client, 0, len(r.clients))
	for c := range r.clients {
		if c != from {
			clients = append(clients, c)
		}
	}
	r.mu.RUnlock()
	for _, c := range clients {
		select {
		case c.Send <- data:
		default:
		}
	}
}

// ChatHub holds all chat rooms by interaction ID.
type ChatHub struct {
	mu    sync.RWMutex
	rooms map[uint]*ChatRoom
}

func NewChatHub() *ChatHub {
	return &ChatHub{rooms: make(map[uint]*ChatRoom)}
}

func (h *ChatHub) GetOrCreateRoom(interactionID, clientID, companionID uint) *ChatRoom {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[interactionID]; ok {
		return r
	}
	r := NewChatRoom(interactionID, clientID, companionID)
	h.rooms[interactionID] = r
	return r
}

func (h *ChatHub) GetRoom(interactionID uint) *ChatRoom {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rooms[interactionID]
}

func (h *ChatHub) RemoveRoom(interactionID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms, interactionID)
}

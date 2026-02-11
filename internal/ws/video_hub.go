package ws

import (
	"encoding/json"
	"sync"
)

// VideoRoom has exactly two peers (client and companion) for WebRTC signaling.
type VideoRoom struct {
	InteractionID uint
	peers        map[uint]*Client // userID -> client
	mu           sync.RWMutex
}

func NewVideoRoom(interactionID uint) *VideoRoom {
	return &VideoRoom{InteractionID: interactionID, peers: make(map[uint]*Client)}
}

func (r *VideoRoom) Join(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.peers[c.UserID] = c
}

func (r *VideoRoom) Leave(userID uint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.peers, userID)
}

func (r *VideoRoom) SendToOther(senderUserID uint, payload interface{}) {
	data, _ := json.Marshal(payload)
	r.mu.RLock()
	defer r.mu.RUnlock()
	for uid, c := range r.peers {
		if uid != senderUserID {
			select {
			case c.Send <- data:
			default:
			}
			break
		}
	}
}

type VideoHub struct {
	mu    sync.RWMutex
	rooms map[uint]*VideoRoom
}

func NewVideoHub() *VideoHub {
	return &VideoHub{rooms: make(map[uint]*VideoRoom)}
}

func (h *VideoHub) GetOrCreateRoom(interactionID uint) *VideoRoom {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[interactionID]; ok {
		return r
	}
	r := NewVideoRoom(interactionID)
	h.rooms[interactionID] = r
	return r
}

func (h *VideoHub) GetRoom(interactionID uint) *VideoRoom {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rooms[interactionID]
}

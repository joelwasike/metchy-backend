package ws

import (
	"encoding/json"
	"sync"
	"time"
)

// MapMarker represents a fuzzed companion location for the live map (Uber-style).
type MapMarker struct {
	CompanionID uint    `json:"companion_id"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	IsOnline    bool    `json:"is_online"`
	UpdatedAt   int64   `json:"updated_at"`
}

// MapHub streams fuzzed companion locations to clients; companions push their location.
type MapHub struct {
	*Hub
	// companionID -> last fuzzed location (so we can broadcast to map viewers)
	mu      sync.RWMutex
	markers map[uint]MapMarker
}

func NewMapHub() *MapHub {
	return &MapHub{
		Hub:     NewHub(),
		markers: make(map[uint]MapMarker),
	}
}

// UpdateLocation is called when a companion's location updates (with fuzzed coords).
func (m *MapHub) UpdateLocation(companionID uint, lat, lng float64, isOnline bool) {
	marker := MapMarker{
		CompanionID: companionID,
		Lat:         lat,
		Lng:         lng,
		IsOnline:    isOnline,
		UpdatedAt:   time.Now().Unix(),
	}
	m.mu.Lock()
	m.markers[companionID] = marker
	m.mu.Unlock()
	m.BroadcastAll(marker)
}

// GetMarkers returns current markers for all online companions (for initial map load).
func (m *MapHub) GetMarkers() []MapMarker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]MapMarker, 0, len(m.markers))
	for _, v := range m.markers {
		if v.IsOnline {
			list = append(list, v)
		}
	}
	return list
}

// MapClient extends Client for map channel (client receives markers, companions send location).
type MapClient struct {
	*Client
	MapHub *MapHub
}

func (c *MapClient) SendMarkers(markers []MapMarker) {
	data, _ := json.Marshal(map[string]interface{}{"type": "markers", "markers": markers})
	select {
	case c.Send <- data:
	default:
	}
}

package ws

import (
	"encoding/json"
	"net/http"
	"time"

	"lusty/config"
	"lusty/internal/auth"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// UpgradeMapWS upgrades connection for map channel; client sends optional auth, server sends markers.
func UpgradeMapWS(cfg *config.JWTConfig, mapHub *MapHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// Expect query or first message with token
		token := c.Query("token")
		if token == "" {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"token required"}`))
			return
		}
		claims, err := auth.ParseAccessToken(cfg, token)
		if err != nil {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"invalid token"}`))
			return
		}
		client := &Client{
			UserID: claims.UserID,
			Role:   claims.Role,
			Send:   make(chan []byte, 256),
			Hub:    mapHub.Hub,
		}
		client.conn = &wsConn{conn: conn}
		mapHub.Register(client)
		defer client.Close()
		// Send initial markers
		markers := mapHub.GetMarkers()
		data, _ := json.Marshal(map[string]interface{}{"type": "markers", "markers": markers})
		client.Send <- data
		go writePump(client, conn)
		readPump(conn)
	}
}

// writePump copies messages from client.Send to the connection.
func writePump(c *Client, conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func readPump(conn *websocket.Conn) {
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

type wsConn struct {
	conn *websocket.Conn
}

func (w *wsConn) SendMessage(data []byte) error {
	return w.conn.WriteMessage(websocket.TextMessage, data)
}

// Location updates from companions are done via HTTP PATCH /api/v1/me/location;
// the handler updates DB and calls MapHub.UpdateLocation with fuzzed coords.

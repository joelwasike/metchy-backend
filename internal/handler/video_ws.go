package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"lusty/config"
	"lusty/internal/auth"
	"lusty/internal/domain"
	"lusty/internal/repository"
	"lusty/internal/ws"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var videoUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// UpgradeVideoWS handles WebRTC signaling: offer, answer, ice. Query: token, interaction_id.
func UpgradeVideoWS(cfg *config.JWTConfig, videoHub *ws.VideoHub, interactionRepo *repository.InteractionRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		interactionIDStr := c.Query("interaction_id")
		if token == "" || interactionIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "token and interaction_id required"})
			return
		}
		claims, err := auth.ParseAccessToken(cfg, token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		var interactionID uint
		if _, err := fmt.Sscanf(interactionIDStr, "%d", &interactionID); err != nil || interactionID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid interaction_id"})
			return
		}
		ir, err := interactionRepo.GetByID(interactionID)
		if err != nil || ir == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "interaction not found"})
			return
		}
		if ir.Status != domain.RequestStatusAccepted {
			c.JSON(http.StatusForbidden, gin.H{"error": "interaction not accepted"})
			return
		}
		companionUserID := ir.Companion.UserID
		if claims.UserID != ir.ClientID && claims.UserID != companionUserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "not part of this interaction"})
			return
		}
		conn, err := videoUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		client := &ws.Client{
			UserID: claims.UserID,
			Role:   claims.Role,
			Send:   make(chan []byte, 64),
			Hub:    ws.NewHub(),
		}
		room := videoHub.GetOrCreateRoom(interactionID)
		room.Join(client)
		defer func() {
			room.Leave(claims.UserID)
			client.Close()
		}()
		go func() {
			for msg := range client.Send {
				_ = conn.WriteMessage(websocket.TextMessage, msg)
			}
		}()
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var msg struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}
			if json.Unmarshal(raw, &msg) != nil {
				continue
			}
			switch msg.Type {
			case "offer", "answer", "ice":
				room.SendToOther(claims.UserID, map[string]interface{}{"type": msg.Type, "payload": msg.Payload})
			}
		}
	}
}

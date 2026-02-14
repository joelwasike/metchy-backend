package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"lusty/config"
	"lusty/internal/auth"
	"lusty/internal/domain"
	"lusty/internal/models"
	"lusty/internal/repository"
	"lusty/internal/ws"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	chatWriteWait  = 10 * time.Second
	chatPongWait   = 60 * time.Second
	chatPingPeriod = (chatPongWait * 9) / 10
)

var chatUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// UpgradeChatWS upgrades to WebSocket for chat; query: token, interaction_id. User must be client or companion of that interaction; request must be accepted.
func UpgradeChatWS(cfg *config.JWTConfig, chatHub *ws.ChatHub, interactionRepo *repository.InteractionRepository) gin.HandlerFunc {
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
		clientID := ir.ClientID
		companionUserID := ir.Companion.UserID
		if claims.UserID != clientID && claims.UserID != companionUserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "not part of this interaction"})
			return
		}
		session, err := interactionRepo.GetChatSessionByInteractionID(interactionID)
		if err != nil || session == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "chat session not found"})
			return
		}
		if session.EndsAt.Before(time.Now()) {
			c.JSON(http.StatusForbidden, gin.H{"error": "chat access expired"})
			return
		}
		conn, err := chatUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		client := &ws.Client{
			UserID: claims.UserID,
			Role:   claims.Role,
			Send:   make(chan []byte, 256),
			Hub:    ws.NewHub(),
		}
		room := chatHub.GetOrCreateRoom(interactionID, clientID, ir.CompanionID)
		room.Join(client)
		defer func() {
			room.Leave(client)
			client.Close()
		}()
		conn.SetReadDeadline(time.Now().Add(chatPongWait))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(chatPongWait))
			return nil
		})
		go func() {
			ticker := time.NewTicker(chatPingPeriod)
			defer ticker.Stop()
			for {
				select {
				case msg, ok := <-client.Send:
					if !ok {
						return
					}
					conn.SetWriteDeadline(time.Now().Add(chatWriteWait))
					if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
						return
					}
				case <-ticker.C:
					conn.SetWriteDeadline(time.Now().Add(chatWriteWait))
					if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						return
					}
				}
			}
		}()
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var msg struct {
				Type     string `json:"type"`
				Content  string `json:"content"`
				MediaURL string `json:"media_url"`
			}
			if json.Unmarshal(raw, &msg) != nil || msg.Type != "message" {
				continue
			}
			cm := &models.ChatMessage{
				SessionID: session.ID,
				SenderID:  claims.UserID,
				Content:   msg.Content,
				MediaURL:  msg.MediaURL,
			}
			if err := interactionRepo.CreateMessage(cm); err != nil {
				continue
			}
			payload := map[string]interface{}{
				"type":       "message",
				"id":         cm.ID,
				"sender_id":  cm.SenderID,
				"content":    cm.Content,
				"media_url":  cm.MediaURL,
				"created_at": cm.CreatedAt,
			}
			room.Broadcast(client, payload)
		}
	}
}

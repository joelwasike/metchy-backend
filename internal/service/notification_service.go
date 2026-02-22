package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"lusty/internal/models"
	"lusty/internal/repository"
)

type NotificationService struct {
	repo     *repository.NotificationRepository
	userRepo *repository.UserRepository
	fcm      *FCMService
}

func NewNotificationService(repo *repository.NotificationRepository, userRepo *repository.UserRepository, fcm *FCMService) *NotificationService {
	return &NotificationService{repo: repo, userRepo: userRepo, fcm: fcm}
}

func (s *NotificationService) Notify(userID uint, notifType, title, body string, data map[string]interface{}) error {
	var dataJSON string
	if data != nil {
		b, _ := json.Marshal(data)
		dataJSON = string(b)
	}
	err := s.repo.Create(&models.Notification{
		UserID: userID,
		Type:   notifType,
		Title:  title,
		Body:   body,
		Data:   dataJSON,
	})
	if err != nil {
		return err
	}
	// Push via FCM
	s.sendPush(userID, notifType, title, body, data)
	return nil
}

func (s *NotificationService) sendPush(userID uint, notifType, title, body string, data map[string]interface{}) {
	if s.fcm == nil || s.userRepo == nil {
		return
	}
	u, err := s.userRepo.GetByID(userID)
	if err != nil || u == nil || u.FCMToken == "" {
		return
	}
	_ = s.fcm.SendToUser(context.Background(), u.FCMToken, notifType, title, body, data)
}

func (s *NotificationService) NotifyNewRequest(companionUserID uint, requestID uint, clientName string) error {
	return s.Notify(companionUserID, "NEW_REQUEST", "New request", clientName+" sent you an interaction request", map[string]interface{}{"request_id": requestID, "interaction_id": requestID})
}

// NotifyPaidRequest notifies the companion that a client has paid for a service. They should accept or deny.
func (s *NotificationService) NotifyPaidRequest(companionUserID uint, requestID uint, clientName string, serviceType string) error {
	svc := serviceType
	if svc == "" {
		svc = "chat"
	}
	return s.Notify(companionUserID, "PAID_REQUEST", "Paid request", clientName+" has paid for "+svc+". Accept or Deny.", map[string]interface{}{"request_id": requestID, "interaction_id": requestID})
}

func (s *NotificationService) NotifyAccepted(clientUserID uint, companionName string, interactionID uint) error {
	return s.Notify(clientUserID, "REQUEST_ACCEPTED", "Request accepted", companionName+" accepted your request", map[string]interface{}{"interaction_id": interactionID})
}

func (s *NotificationService) NotifyRejected(clientUserID uint, companionName string) error {
	return s.Notify(clientUserID, "REQUEST_REJECTED", "Request declined", companionName+" declined your request", nil)
}

func (s *NotificationService) NotifyPaymentConfirmed(userID uint, amountCents int64, reference string) error {
	return s.Notify(userID, "PAYMENT_CONFIRMED", "Payment confirmed", "Your payment was successful.", map[string]interface{}{"amount_cents": amountCents, "reference": reference})
}

func (s *NotificationService) NotifyFavoriteOnline(clientUserID uint, companionName string, companionID uint) error {
	return s.Notify(clientUserID, "FAVORITE_ONLINE", "Favorite online", companionName+" is now online", map[string]interface{}{"companion_id": companionID})
}

func (s *NotificationService) NotifyBoostExpiry(companionUserID uint) error {
	return s.Notify(companionUserID, "BOOST_EXPIRY", "Boost ending", "Your profile boost is about to expire", nil)
}

func (s *NotificationService) NotifySessionEnding(userID uint, minutesLeft int) error {
	return s.Notify(userID, "SESSION_ENDING", "Session ending", "Your session ends in a few minutes", map[string]interface{}{"minutes_left": minutesLeft})
}

// NotifyNewChatMessage sends push when the recipient is not in the chat. Skips if recipient has no FCM token.
func (s *NotificationService) NotifyNewChatMessage(recipientUserID uint, senderName string, interactionID uint, contentPreview string) {
	if recipientUserID == 0 {
		return
	}
	body := senderName + ": " + contentPreview
	if len(body) > 100 {
		body = body[:97] + "..."
	}
	s.sendPush(recipientUserID, "NEW_MESSAGE", "New message", body, map[string]interface{}{
		"interaction_id": interactionID,
		"sender_name":    senderName,
	})
}

// NotifyVideoCall sends a data-only push for incoming video call (does not save to notifications table).
// Data-only ensures the Android background handler fires even when the app is killed,
// allowing flutter_callkit_incoming to show the native call UI with ring + vibration.
func (s *NotificationService) NotifyVideoCall(calleeUserID uint, callerName string, interactionID uint) {
	if s.fcm == nil || s.userRepo == nil {
		log.Printf("[VideoCall] FCM not configured, skipping push calleeUserID=%d", calleeUserID)
		return
	}
	u, err := s.userRepo.GetByID(calleeUserID)
	if err != nil || u == nil {
		log.Printf("[VideoCall] callee user not found calleeUserID=%d err=%v", calleeUserID, err)
		return
	}
	if u.FCMToken == "" {
		log.Printf("[VideoCall] callee has no FCM token calleeUserID=%d — push not sent", calleeUserID)
		return
	}
	log.Printf("[VideoCall] sending FCM push calleeUserID=%d interactionID=%d callerName=%q token=%s...", calleeUserID, interactionID, callerName, u.FCMToken[:8])
	err = s.fcm.SendDataOnly(context.Background(), u.FCMToken, map[string]string{
		"type":           "VIDEO_CALL",
		"interaction_id": fmt.Sprintf("%d", interactionID),
		"caller_name":    callerName,
	})
	if err != nil {
		log.Printf("[VideoCall] FCM send error calleeUserID=%d interactionID=%d: %v", calleeUserID, interactionID, err)
	} else {
		log.Printf("[VideoCall] FCM push sent OK calleeUserID=%d interactionID=%d", calleeUserID, interactionID)
	}
}

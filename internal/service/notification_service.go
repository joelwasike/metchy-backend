package service

import (
	"encoding/json"

	"lusty/internal/models"
	"lusty/internal/repository"
)

type NotificationService struct {
	repo *repository.NotificationRepository
}

func NewNotificationService(repo *repository.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

func (s *NotificationService) Notify(userID uint, notifType, title, body string, data map[string]interface{}) error {
	var dataJSON string
	if data != nil {
		b, _ := json.Marshal(data)
		dataJSON = string(b)
	}
	return s.repo.Create(&models.Notification{
		UserID: userID,
		Type:   notifType,
		Title:  title,
		Body:   body,
		Data:   dataJSON,
	})
}

func (s *NotificationService) NotifyNewRequest(companionUserID uint, requestID uint, clientName string) error {
	return s.Notify(companionUserID, "NEW_REQUEST", "New request", clientName+" sent you an interaction request", map[string]interface{}{"request_id": requestID})
}

// NotifyPaidRequest notifies the companion that a client has paid for a service. They should accept or deny.
func (s *NotificationService) NotifyPaidRequest(companionUserID uint, requestID uint, clientName string, serviceType string) error {
	svc := serviceType
	if svc == "" {
		svc = "chat"
	}
	return s.Notify(companionUserID, "PAID_REQUEST", "Paid request", clientName+" has paid for "+svc+". Accept or Deny.", map[string]interface{}{"request_id": requestID})
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

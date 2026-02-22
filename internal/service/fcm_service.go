package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// FCMService sends push notifications via Firebase Cloud Messaging.
type FCMService struct {
	client *messaging.Client
}

// NewFCMService creates an FCM service. Returns nil if Firebase is not configured.
func NewFCMService(serviceAccountPath string) *FCMService {
	if serviceAccountPath == "" {
		return nil
	}
	ctx := context.Background()
	opt := option.WithCredentialsFile(serviceAccountPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		log.Printf("[FCM] Failed to init Firebase app: %v", err)
		return nil
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		log.Printf("[FCM] Failed to get Messaging client: %v", err)
		return nil
	}
	return &FCMService{client: client}
}

// Send sends a push notification to the given FCM token.
func (s *FCMService) Send(ctx context.Context, token string, title, body string, data map[string]string) error {
	if s == nil || token == "" {
		return nil
	}
	msg := &messaging.Message{
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data:  data,
		Token: token,
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Sound: "default",
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound: "default",
				},
			},
			FCMOptions: &messaging.APNSFCMOptions{
				ImageURL: "",
			},
		},
	}
	_, err := s.client.Send(ctx, msg)
	if err != nil {
		log.Printf("[FCM] Send error: %v", err)
		return err
	}
	return nil
}

// SendDataOnly sends a silent, data-only FCM push. No notification payload is included,
// which causes Android's background message handler to fire even when the app is killed.
// Used for VIDEO_CALL so flutter_callkit_incoming can show the native call UI.
func (s *FCMService) SendDataOnly(ctx context.Context, token string, data map[string]string) error {
	if s == nil || token == "" {
		return nil
	}
	msg := &messaging.Message{
		Data:  data,
		Token: token,
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-priority": "10",
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					ContentAvailable: true,
				},
			},
		},
	}
	_, err := s.client.Send(ctx, msg)
	if err != nil {
		log.Printf("[FCM] SendDataOnly error: %v", err)
		return err
	}
	return nil
}

// SendToUser sends a push to a user by their FCM token. Token is fetched by the caller.
// All data values are converted to strings (FCM requires string values).
func (s *FCMService) SendToUser(ctx context.Context, fcmToken string, notifType, title, body string, data map[string]interface{}) error {
	if s == nil || fcmToken == "" {
		return nil
	}
	dataStr := make(map[string]string)
	dataStr["type"] = notifType
	if data != nil {
		for k, v := range data {
			switch val := v.(type) {
			case string:
				dataStr[k] = val
			case uint:
				dataStr[k] = fmt.Sprintf("%d", val)
			case int:
				dataStr[k] = fmt.Sprintf("%d", val)
			case float64:
				dataStr[k] = fmt.Sprintf("%.0f", val)
			default:
				b, _ := json.Marshal(v)
				dataStr[k] = string(b)
			}
		}
	}
	return s.Send(ctx, fcmToken, title, body, dataStr)
}

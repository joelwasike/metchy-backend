package payment

import (
	"context"
	"time"
)

type PaymentRequest struct {
	UserID          uint
	AmountCents     int64
	Currency        string
	IdempotencyKey  string
	Metadata        map[string]interface{}
	ExpiresIn       time.Duration
	Description     string
	// M-Pesa (Liberec) fields
	OrderID           string // unique order_id; used as merchant_order_id in callback
	CustomerPhone     string // e.g. 254112299271
	CustomerFirstName string
	CustomerLastName  string
	CustomerEmail     string
	CallbackURL       string
}

type PaymentResponse struct {
	Reference        string
	Status           string
	CheckoutURL      string
	ExpiresAt        time.Time
	CheckoutRequestID string // M-Pesa STK checkout request ID
}

type Provider interface {
	InitiatePayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error)
	VerifyPayment(ctx context.Context, reference string) (bool, error)
}

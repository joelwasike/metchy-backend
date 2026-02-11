package payment

import (
	"context"
	"fmt"
	"time"
)

// StubProvider is a no-op provider for development; replace with Stripe/PayPal etc.
type StubProvider struct{}

func (s *StubProvider) InitiatePayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	ref := fmt.Sprintf("stub_%d_%d", time.Now().UnixNano(), req.UserID)
	return &PaymentResponse{
		Reference:   ref,
		Status:      "PENDING",
		CheckoutURL: "",
		ExpiresAt:   time.Now().Add(req.ExpiresIn),
	}, nil
}

func (s *StubProvider) VerifyPayment(ctx context.Context, reference string) (bool, error) {
	return len(reference) > 0 && reference[:5] == "stub_", nil
}

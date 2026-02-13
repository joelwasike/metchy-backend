package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

// B2CRequest is the request for M-Pesa B2C (Business to Customer) withdrawal.
type B2CRequest struct {
	Amount      int64  // in KES (whole units)
	PhoneNumber string // e.g. 254112299271
	Description string
	Remarks     string
	OrderID     string // unique
	CallbackURL string
}

// B2CResponse is the response from the B2C API.
type B2CResponse struct {
	UUID                     string `json:"uuid"`
	OrderID                  string `json:"order_id"`
	OriginatorConversationID string `json:"originator_conversation_id"`
	ConversationID           string `json:"conversation_id"`
	Amount                   int    `json:"amount"`
	PhoneNumber              string `json:"phone_number"`
	Status                   string `json:"status"`
	ResponseCode             string `json:"response_code"`
	ResponseDescription      string `json:"response_description"`
	CreatedAt                string `json:"created_at"`
}

// InitiateB2C calls the M-Pesa B2C API to send money to a phone number.
func (p *LiberecMpesaProvider) InitiateB2C(ctx context.Context, req B2CRequest) (*B2CResponse, error) {
	token, err := p.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("b2c login: %w", err)
	}
	orderID := req.OrderID
	if orderID == "" {
		orderID = fmt.Sprintf("wd-%s", uuid.New().String())
	}
	callbackURL := req.CallbackURL
	if callbackURL == "" && p.WebhookBase != "" {
		base := p.WebhookBase
		if len(base) > 0 && base[0] != 'h' {
			base = "https://" + base
		}
		callbackURL = base + "/api/v1/webhooks/withdrawal"
	}
	body := map[string]string{
		"amount":        strconv.FormatInt(req.Amount, 10),
		"phone_number":  req.PhoneNumber,
		"description":  req.Description,
		"remarks":      req.Remarks,
		"order_id":     orderID,
		"callback_url": callbackURL,
	}
	if body["description"] == "" {
		body["description"] = "B2C Payment to customer"
	}
	if body["remarks"] == "" {
		body["remarks"] = "Withdrawal payment"
	}
	bodyBytes, _ := json.Marshal(body)
	apiReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/api/v1/transactions/mpesa/b2c", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	apiReq.Header.Set("Content-Type", "application/json")
	apiReq.Header.Set("Authorization", "Bearer "+token)
	log.Printf("[MPESA B2C] POST %s/transactions/mpesa/b2c order_id=%s amount=%d phone=%s", p.BaseURL, orderID, req.Amount, req.PhoneNumber)
	resp, err := p.client.Do(apiReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[MPESA B2C] response status=%d body=%s", resp.StatusCode, string(respBody))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("b2c api: %d %s", resp.StatusCode, string(respBody))
	}
	var out B2CResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

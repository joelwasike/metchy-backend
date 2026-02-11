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
	"time"
)

// LiberecMpesaProvider implements M-Pesa STK push via TheLiberec Card API.
type LiberecMpesaProvider struct {
	BaseURL   string
	Email     string
	Password  string
	WebhookBase string
	client    *http.Client
}

func NewLiberecMpesaProvider(baseURL, email, password, webhookBase string) *LiberecMpesaProvider {
	if baseURL == "" {
		baseURL = "https://card-api.theliberec.com"
	}
	return &LiberecMpesaProvider{
		BaseURL:     baseURL,
		Email:       email,
		Password:    password,
		WebhookBase: webhookBase,
		client:      &http.Client{Timeout: 30 * time.Second},
	}
}

type liberecLoginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type liberecLoginResp struct {
	Token string `json:"token"`
}

// getToken logs in and returns a fresh token (per transaction as recommended).
func (p *LiberecMpesaProvider) getToken(ctx context.Context) (string, error) {
	body, _ := json.Marshal(liberecLoginReq{Email: p.Email, Password: p.Password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/api/v1/merchants/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login failed: %d", resp.StatusCode)
	}
	var out liberecLoginResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Token, nil
}

type mpesaSTKReq struct {
	Amount          string `json:"amount"`
	Currency        string `json:"currency"`
	Description     string `json:"description"`
	CustomerPhone   string `json:"customer_phone"`
	CustomerFirstName string `json:"customer_first_name"`
	CustomerLastName  string `json:"customer_last_name"`
	CustomerEmail    string `json:"customer_email"`
	CallbackURL     string `json:"callback_url"`
	OrderID         string `json:"order_id"`
}

type mpesaSTKResp struct {
	UUID               string `json:"uuid"`
	OrderID            string `json:"order_id"`
	MerchantOrderID    string `json:"merchant_order_id"`
	CheckoutRequestID  string `json:"checkout_request_id"`
	Amount             int    `json:"amount"`
	Currency           string `json:"currency"`
	Status             string `json:"status"`
	ResponseCode       string `json:"response_code"`
	ResponseDescription string `json:"response_description"`
	CustomerMessage   string `json:"customer_message"`
	CreatedAt         string `json:"created_at"`
}

func (p *LiberecMpesaProvider) InitiatePayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	token, err := p.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("mpesa login: %w", err)
	}
	orderID := req.OrderID
	if orderID == "" {
		orderID = req.IdempotencyKey
	}
	if orderID == "" {
		orderID = fmt.Sprintf("lusty-%d", time.Now().UnixNano())
	}
	callbackURL := req.CallbackURL
	if callbackURL == "" && p.WebhookBase != "" {
		callbackURL = p.WebhookBase + "/api/v1/webhooks/mpesa"
	}
	// KES: amount_cents/100 = whole KES (e.g. 1000 cents = 10 KES); other currencies similar
	amountStr := strconv.FormatInt(req.AmountCents/100, 10)
	if req.AmountCents < 100 && req.AmountCents > 0 {
		amountStr = "1"
	}
	payload := mpesaSTKReq{
		Amount:            amountStr,
		Currency:           "KES",
		Description:        req.Description,
		CustomerPhone:      req.CustomerPhone,
		CustomerFirstName:  req.CustomerFirstName,
		CustomerLastName:   req.CustomerLastName,
		CustomerEmail:      req.CustomerEmail,
		CallbackURL:        callbackURL,
		OrderID:            orderID,
	}
	if req.Currency != "" {
		payload.Currency = req.Currency
	}
	body, _ := json.Marshal(payload)
	apiReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/api/v1/transactions/mpesa", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	apiReq.Header.Set("Content-Type", "application/json")
	apiReq.Header.Set("Authorization", "Bearer "+token)
	log.Printf("[MPESA Liberec] POST %s/transactions/mpesa order_id=%s callback=%s", p.BaseURL, orderID, callbackURL)
	resp, err := p.client.Do(apiReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[MPESA Liberec] response status=%d body=%s", resp.StatusCode, string(respBody))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("mpesa stk: %d", resp.StatusCode)
	}
	var out mpesaSTKResp
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	log.Printf("[MPESA Liberec] parsed: merchant_order_id=%s order_id=%s status=%s checkout_request_id=%s", out.MerchantOrderID, out.OrderID, out.Status, out.CheckoutRequestID)
	return &PaymentResponse{
		Reference:         orderID,
		Status:            out.Status,
		CheckoutURL:       "",
		ExpiresAt:         time.Now().Add(10 * time.Minute),
		CheckoutRequestID: out.CheckoutRequestID,
	}, nil
}

func (p *LiberecMpesaProvider) VerifyPayment(ctx context.Context, reference string) (bool, error) {
	return false, nil
}

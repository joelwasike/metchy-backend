package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// SwapuziProvider handles Solana/USDT deposits via the Swapuzi merchant API.
type SwapuziProvider struct {
	BaseURL  string
	Email    string
	Password string
	client   *http.Client
}

func NewSwapuziProvider(baseURL, email, password string) *SwapuziProvider {
	if baseURL == "" {
		baseURL = "https://api.swapuzi.com"
	}
	return &SwapuziProvider{
		BaseURL:  baseURL,
		Email:    email,
		Password: password,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

type swapuziLoginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type swapuziLoginResp struct {
	Token string `json:"token"`
}

// getToken authenticates with the Swapuzi merchant API and returns a fresh token.
func (p *SwapuziProvider) getToken(ctx context.Context) (string, error) {
	body, _ := json.Marshal(swapuziLoginReq{Email: p.Email, Password: p.Password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/merchants/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("swapuzi login failed: %d %s", resp.StatusCode, string(respBody))
	}
	var out swapuziLoginResp
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", fmt.Errorf("swapuzi: login returned empty token")
	}
	return out.Token, nil
}

// SwapuziRates holds the current USDT exchange rates.
type SwapuziRates struct {
	UsdtBuyingRate  float64 `json:"usdt_buying_rate"`
	UsdtSellingRate float64 `json:"usdt_selling_rate"`
}

// GetRates fetches the current USDT/KES exchange rates.
func (p *SwapuziProvider) GetRates(ctx context.Context) (*SwapuziRates, error) {
	token, err := p.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("swapuzi rates auth: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.BaseURL+"/merchants/rates", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[Swapuzi] rates response: %s", string(respBody))
	var out SwapuziRates
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type swapuziDepositReq struct {
	ExpectedAmount float64 `json:"expected_amount"`
	WebhookURL     string  `json:"webhook_url"`
	Notes          string  `json:"notes"`
	DepositID      string  `json:"deposit_id"`
}

// SwapuziDepositResp is the response from the Swapuzi deposit initiation endpoint.
type SwapuziDepositResp struct {
	DepositID         int     `json:"deposit_id"`
	MerchantDepositID string  `json:"merchant_deposit_id"`
	Status            string  `json:"status"`
	Message           string  `json:"message"`
	PageURL           string  `json:"page_url"`
	ExpectedAmount    float64 `json:"expected_amount"`
	ExpiresAt         string  `json:"expires_at"`
	CreatedAt         string  `json:"created_at"`
}

// InitiateDeposit creates a Solana deposit request and returns the page URL for the client.
// depositID is our internal order ID (stored as merchant_deposit_id at Swapuzi).
func (p *SwapuziProvider) InitiateDeposit(ctx context.Context, depositID, webhookURL, notes string, expectedAmountUSDT float64) (*SwapuziDepositResp, error) {
	token, err := p.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("swapuzi deposit auth: %w", err)
	}
	payload := swapuziDepositReq{
		ExpectedAmount: expectedAmountUSDT,
		WebhookURL:     webhookURL,
		Notes:          notes,
		DepositID:      depositID,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/merchants/solana/deposit/initiate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	log.Printf("[Swapuzi] POST %s/merchants/solana/deposit/initiate deposit_id=%s amount_usdt=%.4f webhook=%s",
		p.BaseURL, depositID, expectedAmountUSDT, webhookURL)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[Swapuzi] deposit response status=%d body=%s", resp.StatusCode, string(respBody))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("swapuzi deposit: %d %s", resp.StatusCode, string(respBody))
	}
	var out SwapuziDepositResp
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

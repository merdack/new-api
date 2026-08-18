package service

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type ZarinpalClient struct {
	MerchantID string
	BaseURL    string
	HTTPClient *http.Client
}

type zarinpalResponse struct {
	Data struct {
		Code      int    `json:"code"`
		Authority string `json:"authority"`
		RefID     int64  `json:"ref_id"`
	} `json:"data"`
	Errors any `json:"errors"`
}

func NewZarinpalClient(merchantID string, sandbox bool) *ZarinpalClient {
	baseURL := "https://payment.zarinpal.com"
	if sandbox {
		baseURL = "https://sandbox.zarinpal.com"
	}
	return &ZarinpalClient{MerchantID: strings.TrimSpace(merchantID), BaseURL: baseURL, HTTPClient: &http.Client{Timeout: 15 * time.Second}}
}

func (z *ZarinpalClient) post(path string, payload any) (*zarinpalResponse, error) {
	if z == nil || z.MerchantID == "" || z.HTTPClient == nil {
		return nil, errors.New("zarinpal client is not configured")
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(z.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := z.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("zarinpal returned HTTP %d", resp.StatusCode)
	}
	var result zarinpalResponse
	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (z *ZarinpalClient) Request(amountIRR int64, callbackURL, description, orderID string) (string, error) {
	result, err := z.post("/pg/v4/payment/request.json", map[string]any{
		"merchant_id": z.MerchantID, "amount": amountIRR, "currency": "IRR",
		"callback_url": callbackURL, "description": description,
		"metadata": map[string]string{"order_id": orderID},
	})
	if err != nil {
		return "", err
	}
	if result.Data.Code != 100 || result.Data.Authority == "" {
		return "", fmt.Errorf("zarinpal request rejected with code %d", result.Data.Code)
	}
	return result.Data.Authority, nil
}

func (z *ZarinpalClient) Verify(amountIRR int64, authority string) (string, error) {
	result, err := z.post("/pg/v4/payment/verify.json", map[string]any{
		"merchant_id": z.MerchantID, "amount": amountIRR, "authority": authority,
	})
	if err != nil {
		return "", err
	}
	if result.Data.Code != 100 && result.Data.Code != 101 {
		return "", fmt.Errorf("zarinpal verification rejected with code %d", result.Data.Code)
	}
	return fmt.Sprintf("%d", result.Data.RefID), nil
}

func (z *ZarinpalClient) PaymentURL(authority string) string {
	return strings.TrimRight(z.BaseURL, "/") + "/pg/StartPay/" + authority
}

package service

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type ZibalClient struct {
	Merchant   string
	BaseURL    string
	HTTPClient *http.Client
}

type zibalResponse struct {
	Result    int    `json:"result"`
	TrackID   int64  `json:"trackId"`
	RefNumber int64  `json:"refNumber"`
	Amount    int64  `json:"amount"`
	Message   string `json:"message"`
}

func NewZibalClient(merchant string) *ZibalClient {
	return &ZibalClient{
		Merchant: strings.TrimSpace(merchant), BaseURL: "https://gateway.zibal.ir/v1",
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (z *ZibalClient) post(path string, payload any) (*zibalResponse, error) {
	if z == nil || z.Merchant == "" || z.HTTPClient == nil {
		return nil, errors.New("zibal client is not configured")
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
		return nil, fmt.Errorf("zibal returned HTTP %d", resp.StatusCode)
	}
	var result zibalResponse
	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (z *ZibalClient) Request(amountIRR int64, callbackURL, description, orderID string) (string, error) {
	result, err := z.post("/request", map[string]any{
		"merchant": z.Merchant, "amount": amountIRR, "callbackUrl": callbackURL,
		"description": description, "orderId": orderID,
	})
	if err != nil {
		return "", err
	}
	if result.Result != 100 || result.TrackID <= 0 {
		return "", fmt.Errorf("zibal request rejected with result %d", result.Result)
	}
	return fmt.Sprintf("%d", result.TrackID), nil
}

func (z *ZibalClient) Verify(reference string, expectedAmountIRR int64) (string, error) {
	trackID, err := strconv.ParseInt(reference, 10, 64)
	if err != nil || trackID <= 0 {
		return "", errors.New("invalid zibal track id")
	}
	result, err := z.post("/verify", map[string]any{"merchant": z.Merchant, "trackId": trackID})
	if err != nil {
		return "", err
	}
	if result.Result != 100 && result.Result != 201 {
		return "", fmt.Errorf("zibal verification rejected with result %d", result.Result)
	}
	if result.Amount != expectedAmountIRR {
		return "", errors.New("zibal verified amount mismatch")
	}
	return fmt.Sprintf("%d", result.RefNumber), nil
}

func (z *ZibalClient) PaymentURL(reference string) string {
	baseURL := strings.TrimRight(z.BaseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	return baseURL + "/start/" + reference
}

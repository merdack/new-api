package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZarinpalRequestAndVerifyContracts(t *testing.T) {
	var requestAmount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &body))
		requestAmount = int64(body["amount"].(float64))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/pg/v4/payment/request.json" {
			_, _ = w.Write([]byte(`{"data":{"code":100,"authority":"S0001"},"errors":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"code":100,"ref_id":42},"errors":[]}`))
	}))
	defer server.Close()

	client := NewZarinpalClient("merchant", false)
	client.BaseURL = server.URL
	authority, err := client.Request(12345, "https://example.com/callback", "test", "order-1")
	require.NoError(t, err)
	assert.Equal(t, "S0001", authority)
	assert.Equal(t, int64(12345), requestAmount)
	receipt, err := client.Verify(12345, authority)
	require.NoError(t, err)
	assert.Equal(t, "42", receipt)
}

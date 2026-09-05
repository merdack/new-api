package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZibalRequestAndVerifyContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &body))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/request" {
			assert.Equal(t, "order-1", body["orderId"])
			_, _ = w.Write([]byte(`{"result":100,"trackId":123}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":100,"refNumber":456,"amount":12000}`))
	}))
	defer server.Close()

	client := NewZibalClient("zibal")
	client.BaseURL = server.URL
	reference, err := client.Request(12000, "https://example.com/callback", "test", "order-1")
	require.NoError(t, err)
	assert.Equal(t, "123", reference)
	receipt, err := client.Verify(reference, 12000)
	require.NoError(t, err)
	assert.Equal(t, "456", receipt)
	client.BaseURL = "https://gateway.zibal.ir/v1"
	assert.Equal(t, "https://gateway.zibal.ir/start/123", client.PaymentURL(reference))
}

func TestZibalVerifyRejectsAmountMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":100,"refNumber":456,"amount":11999}`))
	}))
	defer server.Close()
	client := NewZibalClient("zibal")
	client.BaseURL = server.URL
	_, err := client.Verify("123", 12000)
	require.EqualError(t, err, "zibal verified amount mismatch")
}

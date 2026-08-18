package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZarinpalQuoteSnapshotsRateAndMargin(t *testing.T) {
	oldRate, oldMargin, oldMin := setting.ZarinpalIRRPerUSD, setting.ZarinpalMarginBPS, setting.ZarinpalMinTopUpUSD
	t.Cleanup(func() {
		setting.ZarinpalIRRPerUSD, setting.ZarinpalMarginBPS, setting.ZarinpalMinTopUpUSD = oldRate, oldMargin, oldMin
	})
	setting.ZarinpalIRRPerUSD = 1_000_000
	setting.ZarinpalMarginBPS = 1000
	setting.ZarinpalMinTopUpUSD = 1

	amountIRR, quota, err := zarinpalQuote(10)
	require.NoError(t, err)
	assert.Equal(t, int64(11_000_000), amountIRR)
	expectedQuota, err := common.QuotaFromDecimalStrict(decimal.NewFromInt(10).Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
	require.NoError(t, err)
	assert.Equal(t, expectedQuota, quota)
}

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

	amountIRR, quota, err := iranianPaymentQuote(10)
	require.NoError(t, err)
	assert.Equal(t, int64(11_000_000), amountIRR)
	expectedQuota, err := common.QuotaFromDecimalStrict(decimal.NewFromInt(10).Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
	require.NoError(t, err)
	assert.Equal(t, expectedQuota, quota)
}

func configureIranianProviderOrderTest(t *testing.T) {
	t.Helper()
	confirmPaymentComplianceForTest(t)
	oldDefault := setting.IranianPaymentDefault
	oldFailover := setting.IranianPaymentAutoFailover
	oldZarinpalMerchant := setting.ZarinpalMerchantID
	oldZibalMerchant := setting.ZibalMerchant
	oldRate := setting.ZarinpalIRRPerUSD
	oldMargin := setting.ZarinpalMarginBPS
	t.Cleanup(func() {
		setting.IranianPaymentDefault = oldDefault
		setting.IranianPaymentAutoFailover = oldFailover
		setting.ZarinpalMerchantID = oldZarinpalMerchant
		setting.ZibalMerchant = oldZibalMerchant
		setting.ZarinpalIRRPerUSD = oldRate
		setting.ZarinpalMarginBPS = oldMargin
	})
	setting.IranianPaymentDefault = "zarinpal"
	setting.ZarinpalMerchantID = ""
	setting.ZibalMerchant = "zibal"
	setting.ZarinpalIRRPerUSD = 1_000_000
	setting.ZarinpalMarginBPS = 1000
}

func TestIranianProviderOrderFailoverDisabledDoesNotSubstituteDefault(t *testing.T) {
	configureIranianProviderOrderTest(t)
	setting.IranianPaymentAutoFailover = false

	providers, err := iranianProviderOrder("")

	require.Error(t, err)
	assert.Nil(t, providers)
}

func TestIranianProviderOrderFailoverEnabledUsesConfiguredBackup(t *testing.T) {
	configureIranianProviderOrderTest(t)
	setting.IranianPaymentAutoFailover = true

	providers, err := iranianProviderOrder("")

	require.NoError(t, err)
	assert.Equal(t, []string{"zibal"}, providers)
}

func TestIranianProviderOrderExplicitUnavailableProviderNeverFallsBack(t *testing.T) {
	configureIranianProviderOrderTest(t)
	setting.IranianPaymentAutoFailover = true

	providers, err := iranianProviderOrder("zarinpal")

	require.Error(t, err)
	assert.Nil(t, providers)
}

func TestIranianProviderOrderExplicitConfiguredProviderSucceeds(t *testing.T) {
	configureIranianProviderOrderTest(t)
	setting.IranianPaymentAutoFailover = false

	providers, err := iranianProviderOrder("zibal")

	require.NoError(t, err)
	assert.Equal(t, []string{"zibal"}, providers)
}

func TestIranianPaymentExclusiveUIDefaultsOff(t *testing.T) {
	oldExclusive := setting.IranianPaymentExclusiveUI
	t.Cleanup(func() {
		setting.IranianPaymentExclusiveUI = oldExclusive
	})

	setting.IranianPaymentExclusiveUI = false
	assert.False(t, setting.IranianPaymentExclusiveUI)

	setting.IranianPaymentExclusiveUI = true
	assert.True(t, setting.IranianPaymentExclusiveUI)
}

package controller

import (
	"testing"
	"time"

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

func configureIranianFXRateGuardTest(t *testing.T) {
	t.Helper()
	oldRate := setting.ZarinpalIRRPerUSD
	oldSource := setting.IranianFXRateSource
	oldUpdatedAt := setting.IranianFXRateUpdatedAt
	oldMaxAge := setting.IranianFXRateMaxAgeSeconds
	oldGuard := setting.IranianFXRateGuardEnabled
	oldMin := setting.ZarinpalMinTopUpUSD
	oldMargin := setting.ZarinpalMarginBPS
	t.Cleanup(func() {
		setting.ZarinpalIRRPerUSD = oldRate
		setting.IranianFXRateSource = oldSource
		setting.IranianFXRateUpdatedAt = oldUpdatedAt
		setting.IranianFXRateMaxAgeSeconds = oldMaxAge
		setting.IranianFXRateGuardEnabled = oldGuard
		setting.ZarinpalMinTopUpUSD = oldMin
		setting.ZarinpalMarginBPS = oldMargin
	})
	setting.ZarinpalIRRPerUSD = 1_000_000
	setting.IranianFXRateSource = "operator-test"
	setting.IranianFXRateMaxAgeSeconds = 900
	setting.IranianFXRateGuardEnabled = true
	setting.ZarinpalMinTopUpUSD = 1
	setting.ZarinpalMarginBPS = 1000
}

func TestIranianFXRateGuardAcceptsFreshRate(t *testing.T) {
	configureIranianFXRateGuardTest(t)
	now := time.Unix(2_000_000_000, 0)
	setting.IranianFXRateUpdatedAt = now.Add(-5 * time.Minute).Unix()

	amountIRR, _, err := iranianPaymentQuoteAt(1, now)

	require.NoError(t, err)
	assert.Equal(t, int64(1_100_000), amountIRR)
}

func TestIranianFXRateGuardRejectsStaleRate(t *testing.T) {
	configureIranianFXRateGuardTest(t)
	now := time.Unix(2_000_000_000, 0)
	setting.IranianFXRateUpdatedAt = now.Add(-16 * time.Minute).Unix()

	_, _, err := iranianPaymentQuoteAt(1, now)

	require.ErrorContains(t, err, "stale")
}

func TestIranianFXRateGuardRejectsFutureTimestamp(t *testing.T) {
	configureIranianFXRateGuardTest(t)
	now := time.Unix(2_000_000_000, 0)
	setting.IranianFXRateUpdatedAt = now.Add(2 * time.Minute).Unix()

	_, _, err := iranianPaymentQuoteAt(1, now)

	require.ErrorContains(t, err, "future")
}

func TestIranianFXRateGuardRequiresMetadata(t *testing.T) {
	configureIranianFXRateGuardTest(t)
	setting.IranianFXRateSource = ""
	setting.IranianFXRateUpdatedAt = 0

	_, _, err := iranianPaymentQuoteAt(1, time.Unix(2_000_000_000, 0))

	require.ErrorContains(t, err, "metadata")
}

func TestIranianFXRateGuardDisabledAllowsLegacyManualRate(t *testing.T) {
	configureIranianFXRateGuardTest(t)
	setting.IranianFXRateGuardEnabled = false
	setting.IranianFXRateSource = ""
	setting.IranianFXRateUpdatedAt = 0

	amountIRR, _, err := iranianPaymentQuoteAt(1, time.Unix(2_000_000_000, 0))

	require.NoError(t, err)
	assert.Equal(t, int64(1_100_000), amountIRR)
}

func TestCurrentIranianFXRateSnapshotCapturesAuditMetadata(t *testing.T) {
	configureIranianFXRateGuardTest(t)
	now := time.Unix(2_000_000_000, 0)
	setting.IranianFXRateUpdatedAt = now.Add(-time.Minute).Unix()

	snapshot, err := currentIranianFXRateSnapshot(now)

	require.NoError(t, err)
	assert.Equal(t, int64(1_000_000), snapshot.Rate)
	assert.Equal(t, 1000, snapshot.MarginBPS)
	assert.Equal(t, "operator-test", snapshot.Source)
	assert.Equal(t, setting.IranianFXRateUpdatedAt, snapshot.UpdatedAt)
	assert.Equal(t, setting.IranianFXRateUpdatedAt+900, snapshot.ExpiresAt)
}

func TestIranianPaymentQuoteWithRateEnforcesMinimum(t *testing.T) {
	configureIranianFXRateGuardTest(t)
	setting.ZarinpalMinTopUpUSD = 2
	rate := iranianFXRateSnapshot{Rate: 1_000_000, MarginBPS: 1000}

	_, _, err := iranianPaymentQuoteWithRate(1, rate)

	require.ErrorContains(t, err, "top-up amount")
}

package controller

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type IranianPayRequest struct {
	AmountUSD int64  `json:"amount_usd"`
	Provider  string `json:"provider"`
}

type iranianPaymentResult struct {
	Provider  string
	PayLink   string
	AmountIRR int64
}

func RequestZarinpalAmount(c *gin.Context) { requestIranianAmount(c) }
func RequestIranianAmount(c *gin.Context)  { requestIranianAmount(c) }

func requestIranianAmount(c *gin.Context) {
	var req IranianPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "invalid payment request")
		return
	}
	amountIRR, _, err := iranianPaymentQuote(req.AmountUSD)
	if err != nil || amountIRR < 10000 {
		common.ApiErrorMsg(c, "invalid payment quote")
		return
	}
	common.ApiSuccess(c, gin.H{
		"amount_irr": amountIRR, "currency": "IRR",
		"providers": enabledIranianProviders(), "default_provider": normalizedIranianDefault(),
	})
}

func iranianPaymentQuote(amountUSD int64) (amountIRR int64, creditedQuota int, err error) {
	if amountUSD < int64(setting.ZarinpalMinTopUpUSD) || setting.ZarinpalIRRPerUSD <= 0 {
		return 0, 0, fmt.Errorf("invalid top-up amount or exchange rate")
	}
	if setting.ZarinpalMarginBPS < 0 || setting.ZarinpalMarginBPS > 10000 {
		return 0, 0, fmt.Errorf("invalid pricing margin")
	}
	quoted := decimal.NewFromInt(amountUSD).
		Mul(decimal.NewFromInt(setting.ZarinpalIRRPerUSD)).
		Mul(decimal.NewFromInt(int64(10000 + setting.ZarinpalMarginBPS))).
		Div(decimal.NewFromInt(10000)).Ceil()
	if quoted.GreaterThan(decimal.NewFromInt(math.MaxInt64)) {
		return 0, 0, model.ErrInvalidTopUpQuota
	}
	quota, quotaErr := common.QuotaFromDecimalStrict(decimal.NewFromInt(amountUSD).Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
	if quotaErr != nil || quota <= 0 {
		return 0, 0, model.ErrInvalidTopUpQuota
	}
	return quoted.IntPart(), quota, nil
}

func normalizedIranianDefault() string {
	provider := strings.ToLower(strings.TrimSpace(setting.IranianPaymentDefault))
	if provider != model.PaymentProviderZibal {
		return model.PaymentProviderZarinpal
	}
	return provider
}

func enabledIranianProviders() []string {
	providers := make([]string, 0, 2)
	for _, provider := range []string{model.PaymentProviderZarinpal, model.PaymentProviderZibal} {
		if isIranianGatewayEnabled(provider) {
			providers = append(providers, provider)
		}
	}
	return providers
}

func iranianProviderOrder(requested string) ([]string, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested != "" {
		if requested != model.PaymentProviderZarinpal && requested != model.PaymentProviderZibal {
			return nil, fmt.Errorf("unsupported payment provider")
		}
		if !isIranianGatewayEnabled(requested) {
			return nil, fmt.Errorf("payment gateway is not configured")
		}
		return []string{requested}, nil
	}
	first := normalizedIranianDefault()
	second := model.PaymentProviderZibal
	if first == second {
		second = model.PaymentProviderZarinpal
	}
	providers := make([]string, 0, 2)
	for _, provider := range []string{first, second} {
		if isIranianGatewayEnabled(provider) {
			providers = append(providers, provider)
		}
	}
	if len(providers) == 0 {
		return nil, fmt.Errorf("payment gateway is not configured")
	}
	if !setting.IranianPaymentAutoFailover && len(providers) > 1 {
		providers = providers[:1]
	}
	return providers, nil
}

func createIranianPayment(c *gin.Context, amountUSD int64, provider string) (*iranianPaymentResult, bool, error) {
	amountIRR, creditedQuota, err := iranianPaymentQuote(amountUSD)
	if err != nil || amountIRR < 10000 {
		return nil, false, fmt.Errorf("invalid payment quote")
	}
	userID := c.GetInt("id")
	if err := model.ValidateTopUpQuotaCapacity(userID, creditedQuota); err != nil {
		return nil, false, err
	}
	orderID := fmt.Sprintf("na-%d-%s", time.Now().UnixMilli(), common.GetRandomString(10))
	description := fmt.Sprintf("AI API credit for user %d", userID)
	var reference, payLink string
	switch provider {
	case model.PaymentProviderZarinpal:
		client := service.NewZarinpalClient(setting.ZarinpalMerchantID, setting.ZarinpalSandbox)
		reference, err = client.Request(amountIRR, service.GetCallbackAddress()+"/api/user/zarinpal/return", description, orderID)
		if err == nil {
			payLink = client.PaymentURL(reference)
		}
	case model.PaymentProviderZibal:
		client := service.NewZibalClient(setting.ZibalMerchant)
		reference, err = client.Request(amountIRR, service.GetCallbackAddress()+"/api/user/zibal/return", description, orderID)
		if err == nil {
			payLink = client.PaymentURL(reference)
		}
	default:
		return nil, false, fmt.Errorf("unsupported payment provider")
	}
	if err != nil {
		return nil, true, err
	}
	topUp := &model.TopUp{
		UserId: userID, Amount: amountUSD, Money: float64(amountIRR), TradeNo: provider + "_" + reference,
		PaymentMethod: provider, PaymentProvider: provider, CreateTime: time.Now().Unix(), Status: common.TopUpStatusPending,
		CreditedQuota: creditedQuota, PaidAmountMinor: amountIRR, Currency: "IRR",
		ExchangeRate: setting.ZarinpalIRRPerUSD, PricingMarginBPS: setting.ZarinpalMarginBPS, ProviderReference: reference,
	}
	if err := topUp.Insert(); err != nil {
		// A remote identifier now exists. Switching providers could create two
		// payable orders for one user action, so this error is not retryable.
		return nil, false, fmt.Errorf("payment order persistence failed: %w", err)
	}
	return &iranianPaymentResult{Provider: provider, PayLink: payLink, AmountIRR: amountIRR}, false, nil
}

func RequestIranianPay(c *gin.Context)  { requestIranianPay(c, "") }
func RequestZarinpalPay(c *gin.Context) { requestIranianPay(c, model.PaymentProviderZarinpal) }
func RequestZibalPay(c *gin.Context)    { requestIranianPay(c, model.PaymentProviderZibal) }

func requestIranianPay(c *gin.Context, forcedProvider string) {
	var req IranianPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "invalid payment request")
		return
	}
	if forcedProvider != "" {
		req.Provider = forcedProvider
	}
	providers, err := iranianProviderOrder(req.Provider)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	for index, provider := range providers {
		result, retryable, createErr := createIranianPayment(c, req.AmountUSD, provider)
		if createErr == nil {
			common.ApiSuccess(c, gin.H{"provider": result.Provider, "pay_link": result.PayLink, "amount_irr": result.AmountIRR, "currency": "IRR"})
			return
		}
		logger.LogError(c.Request.Context(), fmt.Sprintf("Iranian payment request failed provider=%s user_id=%d error=%q", provider, c.GetInt("id"), createErr.Error()))
		if !retryable || index == len(providers)-1 {
			if createErr == model.ErrTopUpQuotaLimitExceeded {
				common.ApiError(c, createErr)
			} else {
				common.ApiErrorMsg(c, "payment gateway request failed")
			}
			return
		}
	}
}

func ZarinpalReturn(c *gin.Context) {
	reference := strings.TrimSpace(c.Query("Authority"))
	settleIranianReturn(c, model.PaymentProviderZarinpal, reference, strings.TrimSpace(c.Query("Status")) == "OK")
}

func ZibalReturn(c *gin.Context) {
	reference := strings.TrimSpace(c.Query("trackId"))
	settleIranianReturn(c, model.PaymentProviderZibal, reference, strings.TrimSpace(c.Query("success")) == "1")
}

func settleIranianReturn(c *gin.Context, provider, reference string, callbackSucceeded bool) {
	topUp := model.GetTopUpByProviderReference(provider, reference)
	if topUp == nil {
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?payment="+provider+"_not_found"))
		return
	}
	if topUp.Status == common.TopUpStatusSuccess {
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?payment=success"))
		return
	}
	if !callbackSucceeded {
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?payment=cancelled"))
		return
	}
	var receipt string
	var err error
	if provider == model.PaymentProviderZarinpal {
		receipt, err = service.NewZarinpalClient(setting.ZarinpalMerchantID, setting.ZarinpalSandbox).Verify(topUp.PaidAmountMinor, reference)
	} else {
		receipt, err = service.NewZibalClient(setting.ZibalMerchant).Verify(reference, topUp.PaidAmountMinor)
	}
	if err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Iranian payment verification failed provider=%s trade_no=%s error=%q", provider, topUp.TradeNo, err.Error()))
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?payment=verification_failed"))
		return
	}
	if _, err := model.RechargeIranianTopUp(provider, reference, receipt, c.ClientIP()); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Iranian payment settlement failed provider=%s trade_no=%s error=%q", provider, topUp.TradeNo, err.Error()))
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?payment=settlement_failed"))
		return
	}
	c.Redirect(http.StatusFound, paymentReturnPath("/wallet?payment=success"))
}

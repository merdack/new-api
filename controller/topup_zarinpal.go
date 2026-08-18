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

type ZarinpalPayRequest struct {
	AmountUSD int64 `json:"amount_usd"`
}

func RequestZarinpalAmount(c *gin.Context) {
	var req ZarinpalPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "invalid payment request")
		return
	}
	amountIRR, _, err := zarinpalQuote(req.AmountUSD)
	if err != nil || amountIRR < 10000 {
		common.ApiErrorMsg(c, "invalid payment quote")
		return
	}
	common.ApiSuccess(c, gin.H{"amount_irr": amountIRR, "currency": "IRR"})
}

func zarinpalQuote(amountUSD int64) (amountIRR int64, creditedQuota int, err error) {
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

func RequestZarinpalPay(c *gin.Context) {
	if !isZarinpalTopUpEnabled() {
		common.ApiErrorMsg(c, "payment gateway is not configured")
		return
	}
	var req ZarinpalPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "invalid payment request")
		return
	}
	amountIRR, creditedQuota, err := zarinpalQuote(req.AmountUSD)
	if err != nil || amountIRR < 10000 {
		common.ApiErrorMsg(c, "invalid payment quote")
		return
	}
	userID := c.GetInt("id")
	if err := model.ValidateTopUpQuotaCapacity(userID, creditedQuota); err != nil {
		common.ApiError(c, err)
		return
	}
	client := service.NewZarinpalClient(setting.ZarinpalMerchantID, setting.ZarinpalSandbox)
	callbackURL := service.GetCallbackAddress() + "/api/user/zarinpal/return"
	authority, err := client.Request(amountIRR, callbackURL, fmt.Sprintf("AI API credit for user %d", userID))
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Zarinpal payment request failed user_id=%d error=%q", userID, err.Error()))
		common.ApiErrorMsg(c, "payment gateway request failed")
		return
	}
	topUp := &model.TopUp{
		UserId: userID, Amount: req.AmountUSD, Money: float64(amountIRR),
		TradeNo: "zarinpal_" + authority, PaymentMethod: model.PaymentMethodZarinpal,
		PaymentProvider: model.PaymentProviderZarinpal, CreateTime: time.Now().Unix(),
		Status: common.TopUpStatusPending, CreditedQuota: creditedQuota,
		PaidAmountMinor: amountIRR, Currency: "IRR", ExchangeRate: setting.ZarinpalIRRPerUSD,
		PricingMarginBPS: setting.ZarinpalMarginBPS, ProviderReference: authority,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Zarinpal order persistence failed user_id=%d error=%q", userID, err.Error()))
		common.ApiErrorMsg(c, "payment order creation failed")
		return
	}
	common.ApiSuccess(c, gin.H{"pay_link": client.PaymentURL(authority), "amount_irr": amountIRR, "currency": "IRR"})
}

func ZarinpalReturn(c *gin.Context) {
	authority := strings.TrimSpace(c.Query("Authority"))
	status := strings.TrimSpace(c.Query("Status"))
	topUp := model.GetTopUpByProviderReference(model.PaymentProviderZarinpal, authority)
	if topUp == nil {
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?payment=zarinpal_not_found"))
		return
	}
	if topUp.Status == common.TopUpStatusSuccess {
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?payment=success"))
		return
	}
	if status != "OK" {
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?payment=cancelled"))
		return
	}
	client := service.NewZarinpalClient(setting.ZarinpalMerchantID, setting.ZarinpalSandbox)
	receipt, err := client.Verify(topUp.PaidAmountMinor, authority)
	if err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Zarinpal verification failed trade_no=%s error=%q", topUp.TradeNo, err.Error()))
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?payment=verification_failed"))
		return
	}
	if _, err := model.RechargeZarinpal(authority, receipt, c.ClientIP()); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Zarinpal settlement failed trade_no=%s error=%q", topUp.TradeNo, err.Error()))
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?payment=settlement_failed"))
		return
	}
	c.Redirect(http.StatusFound, paymentReturnPath("/wallet?payment=success"))
}

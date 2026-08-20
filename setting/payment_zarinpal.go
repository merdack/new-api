package setting

var (
	ZarinpalMerchantID               = ""
	ZarinpalSandbox                  = false
	ZarinpalIRRPerUSD          int64 = 0
	ZarinpalMarginBPS                = 1000
	ZarinpalMinTopUpUSD              = 1
	ZibalMerchant                    = ""
	IranianPaymentDefault            = "zarinpal"
	IranianPaymentAutoFailover       = true
	IranianPaymentExclusiveUI         = false
	IranianFXRateGuardEnabled          = false
	IranianFXRateSource                = "manual"
	IranianFXRateUpdatedAt       int64 = 0
	IranianFXRateMaxAgeSeconds   int64 = 900
)

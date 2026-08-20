# تست محافظ تازگی نرخ ارز ایران برای Hermes

این Runbook روی branch `agent/iranian-fx-rate-guard` و فقط در staging اجرا می‌شود.

## ۱. گیت خودکار

```bash
git fetch origin
git switch agent/iranian-fx-rate-guard
git pull --ff-only origin agent/iranian-fx-rate-guard

go test ./controller ./model
```

## ۲. تنظیم نرخ تازه

با helper احراز هویت‌شده `set_option`:

```bash
NOW="$(date +%s)"
set_option IranianFXRateGuardEnabled false
set_option ZarinpalIRRPerUSD 1000000
set_option IranianFXRateSource hermes-manual-test
set_option IranianFXRateMaxAgeSeconds 900
set_option IranianFXRateUpdatedAt "$NOW"
set_option IranianFXRateGuardEnabled true
```

quote یک دلار باید موفق باشد و پاسخ شامل source، updated_at، expires_at و guard=true باشد.

## ۳. نرخ منقضی

```bash
STALE="$(( $(date +%s) - 901 ))"
set_option IranianFXRateUpdatedAt "$STALE"
```

quote و checkout باید fail شوند و هیچ سفارش یا pay_link ساخته نشود.

## ۴. timestamp آینده

```bash
FUTURE="$(( $(date +%s) + 120 ))"
set_option IranianFXRateUpdatedAt "$FUTURE"
```

quote و checkout باید fail شوند. tolerance فقط ۶۰ ثانیه است.

## ۵. سازگاری guard خاموش

```bash
set_option IranianFXRateGuardEnabled false
set_option IranianFXRateUpdatedAt 0
```

quote با نرخ دستی موجود باید موفق شود.

## ۶. restore امن

```bash
NOW="$(date +%s)"
set_option ZarinpalIRRPerUSD 1000000
set_option IranianFXRateSource hermes-manual-test
set_option IranianFXRateMaxAgeSeconds 900
set_option IranianFXRateUpdatedAt "$NOW"
set_option IranianFXRateGuardEnabled true
```

در پایان checkout زیبال، callback idempotency و دو probe failover را دوباره اجرا کن. artifactها باید redacted باشند و شامل مقدار rate/source/timestamp، نتیجه stale/future و تأیید عدم ساخت سفارش باشند.

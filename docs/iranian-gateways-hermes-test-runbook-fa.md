# دستورالعمل تست MVP درگاه‌های ایرانی برای Hermes

این Runbook برای آزمایش اتصال New API به **زرین‌پال Sandbox** و **زیبال Test Merchant**، رابط فارسی، تسویه idempotent و failover ایمن نوشته شده است. تمام مراحل باید روی محیط آزمایشی انجام شوند؛ استفاده از Merchant تولیدی یا پرداخت با پول واقعی ممنوع است.

## ۱. خروجی مورد انتظار

Hermes باید در پایان یک پوشه با ساختار زیر تحویل دهد:

```text
artifacts/iranian-gateways-<UTC_TIMESTAMP>/
├── environment.txt
├── preflight.json
├── quote-auto.json
├── quote-zarinpal.json
├── quote-zibal.json
├── checkout-auto.json
├── checkout-zarinpal.json
├── checkout-zibal.json
├── failover.json
├── settlement-before.json
├── settlement-after.json
├── idempotency.json
├── ui-fa-wallet.png
├── ui-fa-confirm.png
└── report.md
```

هر فایل باید قبل از تحویل redacted شود. Cookie، Merchant ID، ایمیل، IP، Authority، TrackID، RefID و شماره کامل سفارش نباید در گزارش عمومی دیده شوند. برای شناسه‌ها فقط ۶ نویسه اول و ۴ نویسه آخر نگه داشته شود.

## ۲. قواعد ایمنی

1. فقط از branch زیر استفاده کن:

   ```text
   agent/iranian-fiat-billing
   ```

2. `ZarinpalSandbox=true` الزامی است.
3. برای زیبال فقط از Merchant آزمایشی `zibal` استفاده کن.
4. هیچ تستی نباید `close`، `withdraw`، عملیات AntSeed یا تراکنش Mainnet اجرا کند.
5. مقدار تست را روی حداقل مجاز نگه دار؛ پیشنهاد این Runbook برابر ۱ دلار اعتبار است.
6. Secretها را در command line، shell history، Git، screenshot یا گزارش ننویس. آن‌ها فقط باید در متغیر محیطی قرار گیرند.
7. callback باید HTTPS و از اینترنت عمومی قابل دسترس باشد. localhost برای تست بازگشت درگاه کافی نیست.
8. اگر هر پاسخ به دامنه تولیدی درگاه یا Merchant تولیدی اشاره کرد، تست را متوقف کن.

## ۳. پیش‌نیازها

- Docker و Docker Compose یا build فعال New API
- `curl`, `jq`, `git`, `sed`, `sha256sum`
- یک کاربر Root و یک کاربر تست معمولی
- session آزمایشی Root و session کاربر تست
- آدرس عمومی HTTPS محیط تست
- Merchant آزمایشی زرین‌پال

Secretها را بدون نمایش مقدار تنظیم کن:

```bash
export NEW_API_BASE_URL='https://YOUR-STAGING-DOMAIN'
export NEW_API_CALLBACK_URL='https://YOUR-STAGING-DOMAIN'
export NEW_API_ROOT_ACCESS_TOKEN='ROOT_ACCESS_TOKEN_FROM_STAGING'
export NEW_API_USER_COOKIE='TEST_USER_SESSION_COOKIE'
export ZARINPAL_SANDBOX_MERCHANT='SANDBOX_MERCHANT_ID'
export TEST_TOPUP_USD='1'
export TEST_IRR_PER_USD='1000000'
export TEST_MARGIN_BPS='1000'
```

بررسی کن مقدارها وجود دارند، اما خود مقدار Secretها را چاپ نکن:

```bash
test -n "$NEW_API_BASE_URL"
test -n "$NEW_API_CALLBACK_URL"
test -n "$NEW_API_ROOT_ACCESS_TOKEN"
test -n "$NEW_API_USER_COOKIE"
test -n "$ZARINPAL_SANDBOX_MERCHANT"
```

## ۴. آماده‌سازی کد و artifacts

```bash
git fetch origin
git switch agent/iranian-fiat-billing
git pull --ff-only origin agent/iranian-fiat-billing

export RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
export ARTIFACT_DIR="artifacts/iranian-gateways-${RUN_ID}"
mkdir -p "$ARTIFACT_DIR"

{
  git rev-parse HEAD
  git status --short
  docker --version
  docker compose version
  uname -a
} > "$ARTIFACT_DIR/environment.txt"
```

اگر `git status --short` قبل از تست خالی نیست، توقف کن و تغییرات موجود را گزارش بده؛ فایل‌های کاربر را پاک یا reset نکن.

## ۵. گیت تست خودکار قبل از sandbox

از ابزارهای نصب‌شده پروژه استفاده کن:

```bash
go test ./service ./controller ./model

cd web
bun install
bun run typecheck
bun run test -- src/features/wallet/hooks/use-payment.test.ts src/features/wallet/lib/payment.test.ts
bun run lint
bun run build
cd ..
```

معیار قبولی:

- تست‌های Go پاس شوند.
- تست‌های wallet پاس شوند.
- Typecheck و lint بدون error باشند.
- build پوشه `web/dist` را تولید کند.

در صورت شکست هرکدام، تست sandbox را ادامه نده و log خطا را بدون Secret در `report.md` ثبت کن.

## ۶. Health check و پیکربندی آزمایشی

Health check:

```bash
curl --fail --silent --show-error \
  "$NEW_API_BASE_URL/api/status" \
  | jq '{success, message}' \
  > "$ARTIFACT_DIR/preflight.json"
```

تابع زیر تنظیمات را با access token کاربر Root به API می‌فرستد. Cookie مربوط به refresh به مسیر `/api/user/auth` محدود است و برای `/api/option/` قابل استفاده نیست. نام واقعی Cookie نباید در فایل گزارش ثبت شود:

```bash
set_option() {
  local key="$1"
  local value="$2"
  curl --fail --silent --show-error \
    -X PUT "$NEW_API_BASE_URL/api/option/" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer $NEW_API_ROOT_ACCESS_TOKEN" \
    --data "$(jq -nc --arg key "$key" --arg value "$value" '{key:$key,value:$value}')" \
    | jq -e '.success == true' >/dev/null
}
```

ابتدا compliance را با access token کاربر Root تأیید کن:

```bash
curl --fail --silent --show-error \
  -X POST "$NEW_API_BASE_URL/api/option/payment_compliance" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $NEW_API_ROOT_ACCESS_TOKEN" \
  --data '{"confirmed":true}' \
  | jq -e '.success == true' >/dev/null
```

سپس تنظیمات sandbox را اعمال کن:

```bash
set_option CustomCallbackAddress "$NEW_API_CALLBACK_URL"
set_option ZarinpalSandbox true
set_option ZarinpalMerchantID "$ZARINPAL_SANDBOX_MERCHANT"
set_option ZibalMerchant zibal
set_option ZarinpalIRRPerUSD "$TEST_IRR_PER_USD"
set_option ZarinpalMarginBPS "$TEST_MARGIN_BPS"
set_option ZarinpalMinTopUpUSD 1
set_option IranianPaymentDefault zarinpal
set_option IranianPaymentAutoFailover true
```

پس از تنظیمات، سرویس را یک‌بار restart کن و دوباره health check بگیر. روش restart باید مطابق deployment موجود باشد؛ برای Docker Compose معمولاً:

```bash
docker compose restart new-api
```

## ۷. تست quote و snapshot نرخ

تابع quote:

```bash
quote() {
  curl --fail --silent --show-error \
    -X POST "$NEW_API_BASE_URL/api/user/iranian/amount" \
    -H 'Content-Type: application/json' \
    -H "Cookie: $NEW_API_USER_COOKIE" \
    --data "$(jq -nc --argjson amount "$TEST_TOPUP_USD" '{amount_usd:$amount}')"
}
```

```bash
quote | jq . > "$ARTIFACT_DIR/quote-auto.json"
```

مبلغ مورد انتظار را محاسبه و مقایسه کن:

```bash
QUOTE_NUMERATOR=$(( TEST_TOPUP_USD * TEST_IRR_PER_USD * (10000 + TEST_MARGIN_BPS) ))
EXPECTED_IRR=$(( (QUOTE_NUMERATOR + 9999) / 10000 ))
ACTUAL_IRR="$(jq -r '.data.amount_irr' "$ARTIFACT_DIR/quote-auto.json")"
test "$ACTUAL_IRR" -eq "$EXPECTED_IRR"
jq -e '.data.currency == "IRR"' "$ARTIFACT_DIR/quote-auto.json" >/dev/null
jq -e '.data.providers | index("zarinpal") != null' "$ARTIFACT_DIR/quote-auto.json" >/dev/null
jq -e '.data.providers | index("zibal") != null' "$ARTIFACT_DIR/quote-auto.json" >/dev/null
```

نرخ را موقتاً تغییر بده، quote دوم بگیر و سپس نرخ را برگردان. سفارش قبلی یا رکوردهای قبلی نباید تغییر کنند:

```bash
set_option ZarinpalIRRPerUSD 1100000
quote | jq . > "$ARTIFACT_DIR/quote-rate-changed.json"
set_option ZarinpalIRRPerUSD "$TEST_IRR_PER_USD"
```

## ۸. تست UI فارسی و RTL

1. وارد داشبورد کاربر تست شو.
2. زبان را از منوی زبان روی `فارسی` قرار بده.
3. به صفحه کیف پول برو.
4. بررسی کن `dir="rtl"` روی عنصر `<html>` قرار گرفته است.
5. بررسی کن این گزینه‌ها دیده می‌شوند:
   - انتخاب خودکار درگاه (پیشنهادی)
   - زرین‌پال
   - زیبال
6. مبلغ را وارد کن و هر سه گزینه را جداگانه انتخاب کن.
7. در پنجره تأیید، مبلغ باید با پسوند `IRR` نمایش داده شود.
8. screenshotهای زیر را ذخیره کن:
   - `ui-fa-wallet.png`
   - `ui-fa-confirm.png`

در screenshot، نام کاربری، ایمیل، موجودی حساس و شناسه سفارش را blur یا crop کن.

## ۹. تست ایجاد checkout هر درگاه

تابع checkout:

```bash
checkout() {
  local provider="${1:-}"
  local payload
  if [ -n "$provider" ]; then
    payload="$(jq -nc --argjson amount "$TEST_TOPUP_USD" --arg provider "$provider" '{amount_usd:$amount,provider:$provider}')"
  else
    payload="$(jq -nc --argjson amount "$TEST_TOPUP_USD" '{amount_usd:$amount}')"
  fi
  curl --fail --silent --show-error \
    -X POST "$NEW_API_BASE_URL/api/user/iranian/pay" \
    -H 'Content-Type: application/json' \
    -H "Cookie: $NEW_API_USER_COOKIE" \
    --data "$payload"
}
```

ابتدا checkout صریح هر درگاه را بساز:

```bash
checkout zarinpal | jq . > "$ARTIFACT_DIR/checkout-zarinpal.json"
checkout zibal | jq . > "$ARTIFACT_DIR/checkout-zibal.json"
checkout | jq . > "$ARTIFACT_DIR/checkout-auto.json"
```

معیار قبولی:

```bash
jq -e '.success == true and .data.provider == "zarinpal" and (.data.pay_link | length > 0)' \
  "$ARTIFACT_DIR/checkout-zarinpal.json" >/dev/null
jq -e '.success == true and .data.provider == "zibal" and (.data.pay_link | length > 0)' \
  "$ARTIFACT_DIR/checkout-zibal.json" >/dev/null
jq -e '.success == true and (.data.provider == "zarinpal" or .data.provider == "zibal")' \
  "$ARTIFACT_DIR/checkout-auto.json" >/dev/null
```

قبل از ذخیره artifact، `pay_link` را redacted کن یا فقط hostname آن را نگه دار. انتظار hostname:

- زرین‌پال Sandbox: `sandbox.zarinpal.com`
- زیبال: `gateway.zibal.ir`

## ۱۰. تست failover ایمن

این تست فقط خطای **قبل از صدور Authority** را شبیه‌سازی می‌کند:

```bash
set_option IranianPaymentDefault zarinpal
set_option IranianPaymentAutoFailover true
set_option ZarinpalMerchantID invalid-sandbox-merchant

checkout | jq . > "$ARTIFACT_DIR/failover.json"

set_option ZarinpalMerchantID "$ZARINPAL_SANDBOX_MERCHANT"
```

معیار قبولی:

```bash
jq -e '.success == true and .data.provider == "zibal"' \
  "$ARTIFACT_DIR/failover.json" >/dev/null
```

سپس failover را خاموش کن و دوباره امتحان کن؛ درخواست باید fail شود و نباید checkout زیبال بسازد:

```bash
set_option IranianPaymentAutoFailover false
set_option ZarinpalMerchantID invalid-sandbox-merchant

checkout | jq . > "$ARTIFACT_DIR/failover-disabled.json"
jq -e '.success != true' "$ARTIFACT_DIR/failover-disabled.json" >/dev/null

set_option ZarinpalMerchantID "$ZARINPAL_SANDBOX_MERCHANT"
set_option IranianPaymentAutoFailover true
```

اگر restore تنظیمات شکست خورد، تست را متوقف و به اپراتور اطلاع بده.

## ۱۱. تست callback، تسویه و idempotency

این مرحله را برای هر درگاه جداگانه انجام بده:

1. quota و آخرین سفارش کاربر را پیش از پرداخت ذخیره کن.
2. از `pay_link` redacted‌نشده فقط در همان session تست استفاده کن.
3. در صفحه Sandbox/Test پرداخت را موفق کن.
4. اجازه بده مرورگر به callback عمومی New API برگردد.
5. quota و سفارش را دوباره بخوان.
6. URL callback را یک بار دیگر باز کن.
7. quota را برای بار سوم بخوان.

معیار قبولی:

- callback به `/wallet?payment=success` ختم شود.
- وضعیت سفارش از `pending` به `success` تغییر کند.
- `payment_provider` مطابق درگاه انتخابی باشد.
- `currency=IRR` و `paid_amount_minor` برابر quote ذخیره‌شده باشد.
- `provider_receipt` پس از verify خالی نباشد.
- quota فقط یک بار افزایش پیدا کند.
- اجرای دوباره همان callback، quota را دوباره افزایش ندهد.
- تغییر نرخ بعد از ایجاد سفارش، `credited_quota` یا `paid_amount_minor` همان سفارش را تغییر ندهد.

مقادیر قبل/بعد را به صورت redacted در فایل‌های زیر ذخیره کن:

```text
settlement-before.json
settlement-after.json
idempotency.json
```

برای callback لغوشده نیز یک checkout جدید بساز و عملیات را لغو کن. معیار قبولی این است که سفارش `pending` بماند و quota افزایش پیدا نکند.

## ۱۲. تست restart بین checkout و callback

1. یک checkout آزمایشی جدید بساز، ولی هنوز پرداخت را کامل نکن.
2. New API را restart کن.
3. health check را تا سالم‌شدن سرویس تکرار کن.
4. همان پرداخت Sandbox/Test را کامل کن.
5. بررسی کن callback، verify و credit پس از restart موفق هستند.

این تست ثابت می‌کند اطلاعات تسویه فقط در حافظه نگهداری نمی‌شوند و snapshot سفارش از database بازیابی می‌شود.

## ۱۳. پاک‌سازی و restore

در پایان تنظیمات سالم آزمایشی را restore کن:

```bash
set_option ZarinpalMerchantID "$ZARINPAL_SANDBOX_MERCHANT"
set_option ZarinpalSandbox true
set_option ZibalMerchant zibal
set_option ZarinpalIRRPerUSD "$TEST_IRR_PER_USD"
set_option ZarinpalMarginBPS "$TEST_MARGIN_BPS"
set_option ZarinpalMinTopUpUSD 1
set_option IranianPaymentDefault zarinpal
set_option IranianPaymentAutoFailover true
```

سفارش‌های آزمایشی را حذف نکن. آن‌ها برای audit و reconciliation لازم هستند. فقط در گزارش مشخص کن کدام سفارش‌ها آزمایشی بوده‌اند.

## ۱۴. Redaction اجباری

قبل از تحویل، فایل‌ها را بررسی کن:

```bash
rg -n --hidden \
  'Cookie:|session=|merchant|Authority|trackId|refNumber|pay_link|wallet|private|secret|token' \
  "$ARTIFACT_DIR"
```

هر مورد را بررسی و Secret یا شناسه کامل را ماسک کن. سپس checksum بساز:

```bash
find "$ARTIFACT_DIR" -type f ! -name SHA256SUMS -print0 \
  | sort -z \
  | xargs -0 sha256sum \
  > "$ARTIFACT_DIR/SHA256SUMS"
```

## ۱۵. قالب گزارش نهایی Hermes

فایل `report.md` باید این قالب را داشته باشد:

```markdown
# Iranian Gateways MVP Test Report

- Date (UTC):
- Commit SHA:
- Environment:
- Public callback hostname:
- Tester: Hermes

## Final decision

GO / NO-GO

## Test matrix

| Test | Zarinpal | Zibal | Auto/failover | Evidence | Result |
|---|---:|---:|---:|---|---|
| Unit/type/lint/build | N/A | N/A | N/A | ... | PASS/FAIL |
| Quote and IRR rounding | ... | ... | ... | ... | PASS/FAIL |
| Persian UI and RTL | N/A | N/A | ... | ... | PASS/FAIL |
| Checkout creation | ... | ... | ... | ... | PASS/FAIL |
| Successful callback | ... | ... | N/A | ... | PASS/FAIL |
| Amount verification | ... | ... | N/A | ... | PASS/FAIL |
| Callback idempotency | ... | ... | N/A | ... | PASS/FAIL |
| Cancelled callback | ... | ... | N/A | ... | PASS/FAIL |
| Restart recovery | ... | ... | N/A | ... | PASS/FAIL |
| Default gateway failure | N/A | N/A | ... | ... | PASS/FAIL |
| Failover disabled | N/A | N/A | ... | ... | PASS/FAIL |

## Accounting deltas

- Quota before:
- Quota after first callback:
- Quota after repeated callback:
- Credited quota:
- Stored IRR amount:
- Stored exchange rate:
- Stored margin BPS:

## Findings

1. ...

## Blockers

1. ...

## Redaction confirmation

- [ ] No cookies or tokens
- [ ] No merchant secrets
- [ ] No complete Authority/TrackID/receipt
- [ ] No private wallet or AntSeed data
```

تصمیم `GO` فقط زمانی مجاز است که هر دو درگاه checkout معتبر بسازند، حداقل یک callback موفق برای هر درگاه ثبت شود، اختلاف مبلغ وجود نداشته باشد، callback تکراری دوباره اعتبار ندهد و failover فقط پیش از دریافت شناسه درگاه رخ دهد.

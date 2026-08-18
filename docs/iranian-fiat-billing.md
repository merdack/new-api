# Iranian fiat billing MVP

This integration adds Zarinpal as an IRR top-up provider while keeping New API's existing quota ledger as the customer balance source of truth. AntSeed remains an upstream channel and never receives customer payment details.

## Price model

An administrator configures:

| Option | Meaning |
|---|---|
| `ZarinpalMerchantID` | Zarinpal merchant identifier |
| `ZarinpalSandbox` | Use the Zarinpal sandbox endpoints |
| `ZarinpalIRRPerUSD` | Current integer IRR sale rate for one USD of API credit |
| `ZarinpalMarginBPS` | Safety/commercial margin in basis points (`1000` = 10%) |
| `ZarinpalMinTopUpUSD` | Minimum whole-dollar credit purchase |

The charged amount is rounded up:

```text
amount_irr = ceil(amount_usd * irr_per_usd * (10000 + margin_bps) / 10000)
```

The exact IRR amount, exchange rate, margin, and credited quota are persisted on the order. A later rate or model-price change never changes a pending or completed order.

Model token prices remain in New API's billing configuration. Prefer tiered billing expressions because settlement captures the expression used for the request. Update expressions when the AntSeed retail allowlist changes; do not recalculate historical usage.

## HTTP flow

1. An authenticated user previews a quote with `POST /api/user/zarinpal/amount` and `{ "amount_usd": 10 }`.
2. The user creates the payment with `POST /api/user/zarinpal/pay` using the same body.
3. New API requests an authority from Zarinpal, persists the immutable quote, and returns `pay_link`.
4. Zarinpal redirects to `GET /api/user/zarinpal/return?Authority=...&Status=OK`.
5. New API loads the amount from its database, calls Zarinpal verification, and credits the snapshotted quota in one database transaction.

The settlement operation is idempotent. Repeated callbacks cannot credit the same order twice. A cancelled browser return leaves the order pending because a forged or out-of-order cancellation must not invalidate a later verified payment.

## Initial deployment

Start with sandbox mode and a dedicated test user. Set the public callback address, confirm payment compliance, configure all five options, then test:

- quote rounding and minimum amount;
- successful payment and quota credit;
- repeated callback idempotency;
- cancelled return without credit;
- rate change after order creation;
- database restart between payment creation and callback.

Do not enable production mode until callback TLS, database backups, order monitoring, and a daily reconciliation of Zarinpal receipts against successful top-ups are in place.

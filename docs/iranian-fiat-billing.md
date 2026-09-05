# Iranian fiat billing MVP

This integration adds Zarinpal and Zibal as IRR top-up providers while keeping New API's quota ledger as the customer balance source of truth. AntSeed remains an upstream channel and never receives customer payment details.

For staging verification and the required evidence bundle, follow the [Persian Hermes test runbook](./iranian-gateways-hermes-test-runbook-fa.md).

## Configuration

| Option | Meaning |
|---|---|
| `ZarinpalMerchantID` | Zarinpal merchant identifier |
| `ZarinpalSandbox` | Use Zarinpal sandbox endpoints |
| `ZibalMerchant` | Zibal merchant identifier (`zibal` is available for provider testing) |
| `IranianPaymentDefault` | Default provider: `zarinpal` or `zibal` |
| `IranianPaymentAutoFailover` | Try the other configured provider if checkout creation fails before an identifier is issued |
| `IranianPaymentExclusiveUI` | Hide non-Iranian payment methods in the wallet UI without disabling their backend endpoints |
| `ZarinpalIRRPerUSD` | Current integer IRR sale rate for one USD of API credit, shared by both providers |
| `ZarinpalMarginBPS` | Safety/commercial margin in basis points (`1000` = 10%) |
| `ZarinpalMinTopUpUSD` | Minimum whole-dollar credit purchase |

For an Iranian deployment, set `IranianPaymentExclusiveUI=true`. This is a presentation control: it removes Epay, Stripe, Creem, and Waffo choices from the wallet response while preserving their configuration and API endpoints for mixed or future deployments.

The charged amount is rounded up:

```text
amount_irr = ceil(amount_usd * irr_per_usd * (10000 + margin_bps) / 10000)
```

The exact IRR amount, exchange rate, margin, provider, provider reference, and credited quota are persisted on the order. A later exchange-rate or model-price change never changes a pending or completed order.

Model token prices remain in New API's billing configuration. Prefer tiered billing expressions because settlement captures the expression used for the request. Update expressions when the AntSeed retail allowlist changes; do not recalculate historical usage.

## HTTP flow

The preferred provider-neutral API is:

1. Preview with `POST /api/user/iranian/amount` and `{ "amount_usd": 10 }`. The response includes enabled providers and the configured default.
2. Create checkout with `POST /api/user/iranian/pay` and `{ "amount_usd": 10 }`. An optional `provider` selects `zarinpal` or `zibal` explicitly.
3. New API creates a provider request, persists the immutable quote, and returns `provider` and `pay_link`.
4. The provider redirects to its dedicated public return URL.
5. New API loads the stored amount, verifies it server-to-server, and credits the snapshotted quota in one database transaction.

Compatibility endpoints remain available:

| Provider | Create checkout | Return URL |
|---|---|---|
| Zarinpal | `POST /api/user/zarinpal/pay` | `GET /api/user/zarinpal/return` |
| Zibal | `POST /api/user/zibal/pay` | `GET /api/user/zibal/return` |

Failover is deliberately narrow. It occurs only when the default provider fails before returning an Authority/TrackID. Once any provider identifier exists, New API never switches providers for that request—even if local persistence fails—because creating a second payable checkout could cause duplicate payment. Explicit provider selection does not fail over.

Settlement is idempotent: repeated callbacks cannot credit an order twice. A cancelled browser return leaves the order pending because a forged or out-of-order cancellation must not invalidate a later verified payment. Zibal and Zarinpal verification responses are matched to the immutable stored amount before crediting.

## Initial deployment

Start with Zarinpal sandbox and Zibal's test merchant using a dedicated user. Set the public callback address, confirm payment compliance, configure rate and margin, then test:

- each gateway independently;
- default-provider failure before checkout creation and automatic fallback;
- no fallback after a provider reference exists;
- callback amount mismatch rejection;
- repeated callback idempotency;
- cancelled return without credit;
- rate change after order creation;
- database restart between payment creation and callback.

Do not enable production merchants until callback TLS, database backups, pending-order monitoring, and daily per-provider reconciliation against successful top-ups are in place. Treat gateway availability and exchange-rate freshness as separate operational health signals.

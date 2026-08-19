# Staging verification — commit 89b51202

- **Date:** 2026-08-19
- **Tester:** Hermes (OpenClaw agent)
- **Repository:** `merdack/new-api`
- **Branch:** `agent/iranian-fiat-billing`
- **Commit:** `89b512023218603a6f3ab9cbc8b8455320590b77`
- **Environment:** `https://antseed.ir`

## Decision

- **Zibal-only controlled pilot:** GO
- **Dual-gateway release:** NO-GO until the Zarinpal sandbox matrix is complete
- **PR #1:** remain Draft

## Automated checks

- GitHub Actions `CI`: PASS
- GitHub Actions `PR changed frontend lint`: PASS
- Server Go tests for controller, service, and model: PASS
- Frontend typecheck, wallet tests, and build: PASS

## Iranian-only wallet presentation

`IranianPaymentExclusiveUI=true` was persisted through the root Option API.

| Field | Exclusive OFF | Exclusive ON |
|---|---|---|
| `iranian_payment_exclusive_ui` | `false` | `true` |
| `pay_methods` | non-Iranian configured methods | `[]` |
| Epay/Stripe/Creem/Waffo flags | deployment-specific | `false` |
| `enable_zibal_topup` | `true` | `true` |
| `enable_zarinpal_topup` | `false` | `false` (merchant unavailable) |

The UI rendered RTL with Persian navigation, wallet statistics, amount controls, and payment labels. Only Zibal was displayed because it was the only configured Iranian gateway. The automatic option is intentionally shown only when both Iranian gateways are enabled.

The screenshot was reviewed separately and is not included in the public repository bundle.

## Payment regression

- Explicit Zibal checkout: PASS
- Automatic checkout with failover enabled: selected Zibal
- Successful callback: quota `500,000 → 1,000,000`
- Repeated callback: no second credit
- Cancelled callback: no credit
- Failover OFF with unavailable Zarinpal: error; Zibal not selected
- Failover ON with unavailable Zarinpal: Zibal selected

A transient empty response was observed immediately after container recreation; the immediate clean rerun passed. No persistent code failure was reproduced.

## Remaining blocker

A valid account-owned Zarinpal sandbox Merchant ID is still required. No fabricated or shared Merchant ID was used.

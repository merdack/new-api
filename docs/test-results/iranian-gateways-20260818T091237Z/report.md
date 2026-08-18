# Iranian Gateways MVP Test Report

- **Tester:** Hermes (OpenClaw agent)
- **Reviewed by:** Codex
- **Date:** 2026-08-18
- **Repository:** `merdack/new-api`
- **Branch:** `agent/iranian-fiat-billing`
- **Tested commit:** `d33713e93d8afe14be4ffc486213dee6ceb445a4`
- **Environment:** `https://antseed.ir`
- **Evidence bundle:** `iranian-gateways-20260818T091237Z`

## Final decision

**NO-GO (BLOCKED).**

The Zibal sandbox path passed quote, checkout, successful callback, immutable accounting, cancellation, restart recovery, and callback idempotency. A final two-provider GO remains blocked until a valid Zarinpal sandbox Merchant ID is supplied and the Zarinpal checkout/callback path is completed.

Two additional release findings remain:

1. the Persian wallet payment flow is usable, but the whole dashboard is not yet fully translated;
2. when Zarinpal is configured as the default but unavailable, the provider-neutral endpoint selects Zibal even when `IranianPaymentAutoFailover=false`.

## Test matrix

| Test | Provider | Result | Evidence |
|---|---|---|---|
| Go tests (`service`, `controller`, `model`) | — | PASS | Hermes report |
| Web install and typecheck | — | PASS | Hermes report |
| Wallet Vitest suite | — | PASS (7/7) | Hermes report |
| Production web build | — | PASS | Hermes report |
| Full-repository web lint | — | FAIL (364 errors, 78 warnings) | Hermes report |
| Health check | — | PASS | `preflight.json` |
| Quote: 1 USD | shared | PASS: 1,100,000 IRR | `quote-auto.json` |
| Rate-change quote | shared | PASS: 1,210,000 IRR | `quote-rate-changed.json` |
| Explicit checkout | Zibal | PASS | `checkout-zibal.json` |
| Provider-neutral checkout | automatic | PASS: selected Zibal | `checkout-auto.json` |
| Explicit checkout | Zarinpal | BLOCKED: no valid sandbox Merchant ID | — |
| Successful callback and settlement | Zibal | PASS | `settlement-before.json`, `settlement-after.json` |
| Repeated callback | Zibal | PASS: no double credit | `idempotency.json` |
| Cancelled callback | Zibal | PASS: no credit; order remained pending | `idempotency.json` |
| Restart between checkout and callback | Zibal | PASS | Hermes report |
| Persian payment UI and RTL | — | PARTIAL PASS | Screenshots reviewed; not included in the public bundle |
| Failover after request failure | automatic | BLOCKED: Zarinpal merchant unavailable | — |
| Failover disabled with unavailable default | automatic | FAIL: Zibal checkout was still created | `failover-disabled.json` |

## Verified accounting deltas

| Metric | Before | After first callback | After repeated callback |
|---|---:|---:|---:|
| Test-user quota | 0 | 500,000 | 500,000 |

- Stored payment provider: `zibal`
- Stored currency: `IRR`
- Stored paid amount: `1,100,000`
- Stored credited quota: `500,000`
- Provider receipt: present in the database and omitted from public evidence
- Amount discrepancy: none
- Double credit: none

## Findings

### F1 — Zibal settlement path passed

The explicit Zibal checkout returned a redacted `gateway.zibal.ir` payment URL. A successful test callback changed the order from pending to success and credited exactly 500,000 quota. Replaying the callback did not credit the account again.

### F2 — Zarinpal remains a hard GO blocker

No shared or invented Zarinpal Merchant ID was used. Official Zarinpal sandbox operation requires a Merchant ID from an account. Checkout, callback, amount verification, idempotency, restart recovery, and request-level failover involving Zarinpal therefore remain untested.

### F3 — Disabled failover still changes provider at configuration level

Observed configuration:

- default provider: `zarinpal`;
- Zarinpal unavailable because its Merchant ID was unset;
- Zibal available;
- `IranianPaymentAutoFailover=false`.

Observed result: the provider-neutral checkout succeeded through Zibal.

This does not exercise request-level failover because Zarinpal was filtered out before a request was attempted. It nevertheless conflicts with the operator-facing meaning of disabling automatic failover: an unavailable configured default should produce a gateway-unavailable error rather than silently select another provider. This requires a code fix and regression test.

### F4 — Persian UI is incomplete

The evidence confirms:

- `<html dir="rtl">`;
- the Zibal option rendered in Persian;
- the confirmation amount rendered as `1,100,000 ریال`;
- the Zibal test-payment page opened correctly.

However, the wallet screenshot also contains English navigation/stat labels and Chinese payment-method labels. Therefore this is a **partial Persian payment-flow pass**, not a fully Persian dashboard pass. Unused payment methods should be disabled for the Iranian deployment and the remaining dashboard translation keys should be completed separately.

### F5 — Lint result needs baseline separation

Hermes recorded 364 errors and 78 warnings from the full frontend lint run. The report characterizes these as stylistic, but the submitted bundle does not contain the lint log needed to independently classify every finding. This should be tracked separately as a merge gate: compare the branch against the base branch and require zero new lint errors in changed files.

### F6 — Runbook authentication needs correction

In this deployment, admin option updates were authenticated with `Authorization: Bearer <access_token>`. The refresh cookie is scoped to `/api/user/auth` and cannot be reused for `/api/option/`. The runbook's Cookie-based option helper should be updated before the next Hermes run.

## Remaining blockers

1. Obtain a valid Zarinpal sandbox Merchant ID and run the complete Zarinpal matrix.
2. Correct provider selection when failover is disabled and the configured default is unavailable.
3. Separate full-repository lint debt from errors introduced by this PR.
4. Treat full-dashboard Persian localization as a separate completion gate if it is required for MVP launch.

## Evidence integrity and redaction

- Checkout URLs contain only the provider hostname plus `<REDACTED>`.
- No cookies, bearer tokens, Merchant IDs, Authority values, TrackIDs, RefNumbers, or provider receipts are included.
- Screenshots were reviewed separately and intentionally omitted from the public repository bundle.
- Test orders were retained for audit and reconciliation.

## GO criteria for the follow-up run

The decision may move to GO only after:

- Zarinpal explicit checkout, callback, verification, cancellation, idempotency, and restart recovery pass;
- real request-level failover from Zarinpal to Zibal is demonstrated before an Authority is issued;
- disabling failover prevents automatic provider substitution;
- no amount or quota reconciliation difference exists for either provider;
- no new lint errors are introduced by the PR.

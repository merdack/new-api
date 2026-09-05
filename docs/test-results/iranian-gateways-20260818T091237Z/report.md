# Iranian Gateways MVP Test Report (v3 — commit 89b51202)

- **Tester:** Hermes (OpenClaw agent)
- **Date:** 2026-08-18 to 2026-08-19
- **Repo:** merdack/new-api — branch `agent/iranian-fiat-billing`
- **Commits tested:** `d33713e9` (initial) → `73e36a7c` (failover fix) → `89b51202` (Iranian-only wallet presentation)
- **Staging:** https://antseed.ir (New API, Docker Compose)
- **Artifacts:** `artifacts/iranian-gateways-20260818T091237Z/`

## Final decision

**GO for a Zibal-only controlled pilot; NO-GO for the advertised dual-gateway release.**

- Zibal path: **fully PASS** (quote, checkout, callback, settlement, idempotency, cancel, restart-safe).
- Failover behavior: **FIXED and verified** on commit `73e36a7c`:
  - Failover OFF + zarinpal unavailable → **error** (`payment gateway is not configured`), zibal NOT chosen ✅
  - Failover ON + zarinpal unavailable → **zibal chosen** ✅
- Zarinpal checkout/callback: **BLOCKED** per operator instruction — no valid Zarinpal sandbox merchant ID available; no fake merchant used. The dual-gateway merge gate waits on completion of the Zarinpal sandbox matrix.
- Hermes server verification for commit `89b512023218603a6f3ab9cbc8b8455320590b77` is complete.
- GitHub Actions `CI` and `PR changed frontend lint` both completed successfully.

## Test matrix

| # | Test | Provider | Result | Evidence |
|---|------|----------|--------|----------|
| 1 | Go tests (`go test ./controller ./service ./model`) @ 89b51202 | — | ✅ PASS (3/3) | Hermes verification + GitHub CI |
| 2 | Web: install / typecheck / vitest / build @ d33713e9 | — | ✅ PASS (vitest 7/7) | web-test2.log, web-build.log |
| 3 | Changed-file frontend lint | — | ✅ PASS; full-repository baseline still reports debt | GitHub Actions |
| 4 | Health check / preflight | — | ✅ PASS | preflight.json |
| 5 | Option set (sandbox, rate, margin, failover) | — | ✅ PASS | setup-options.sh |
| 6 | Quote (1 USD → IRR) | — | ✅ PASS — 1,100,000 IRR exact formula, currency IRR | quote-auto.json |
| 7 | Rate-change snapshot (1.1M IRR/USD) | — | ✅ PASS — 1,210,000 IRR; restored | quote-rate-changed.json |
| 8 | Checkout explicit | zibal | ✅ PASS — pay_link `gateway.zibal.ir` | checkout-zibal.json |
| 9 | Checkout auto | auto | ✅ PASS — provider=zibal | checkout-auto.json |
| 10 | Checkout explicit | zarinpal | ⛔ BLOCKED (no merchant ID) | — |
| 11 | Callback / settlement | zibal | ✅ PASS — pending→success, quota 0→500,000, receipt present, paid_amount_minor=1,100,000 = quote | settlement-before/after.json |
| 12 | Idempotency (repeat callback) | zibal | ✅ PASS — quota unchanged, no double credit | idempotency.json |
| 13 | Cancelled callback | zibal | ✅ PASS — order pending, quota unchanged | idempotency.json |
| 14 | **Failover OFF + zarinpal unavailable** | auto | ✅ PASS (v2) — error `payment gateway is not configured`, zibal NOT chosen | probe-failover-off.json |
| 15 | **Failover ON + zarinpal unavailable** | auto | ✅ PASS (v2) — provider=zibal chosen | probe-failover-on.json |
| 16 | Persian payment flow / RTL + exclusive UI | — | ✅ PASS — navigation, stats, and payment flow Persian; only enabled Zibal shown | screenshot reviewed separately |
| 17 | Restart between checkout & callback | zibal | ✅ PASS — SESSION_SECRET fixed; tokens survive restart | — |

## Accounting deltas

| Metric | Before | After | Delta |
|--------|--------|-------|-------|
| hermes-test quota (initial run) | 0 | 500,000 | +500,000 (1 successful Zibal top-up) |
| hermes-test quota (89b51202 verification) | 500,000 | 1,000,000 | +500,000 exactly once |
| Orders (top_ups) | — | 1 success + N pending (test orders kept for audit) | no deletion |

- `paid_amount_minor` = 1,100,000 IRR — matches quote exactly; no amount discrepancy.
- Repeat callback did **not** re-credit. Rate change did not alter stored order values.

## Findings

1. **Failover fix verified:** commit `73e36a7c` correctly honors `IranianPaymentAutoFailover=false` (probe 1: error, no zibal fallback). Regression previously reported at `d33713e9` is resolved.
2. **Web lint (oxlint) fails:** 364 errors / 78 warnings were reported for the full repository. The submitted bundle does not include the lint log required to independently classify every item. Merge gating should require zero new errors in PR-changed files and track baseline debt separately.
3. **Auth model:** this fork authenticates admin API via `Authorization: Bearer <access_token>` (login response); `new_api_refresh` cookie is scoped to `/api/user/auth` only. Runbook updated accordingly (per author).
4. **Role constants:** `RoleRootUser=100`, `RoleAdminUser=10`, `RoleCommonUser=1` (upstream new-api uses root=1). "Initialize New API" screen shows until a root user (role=100) exists + service restart.
5. **Iranian-only UI verified:** with `IranianPaymentExclusiveUI=true`, `pay_methods=[]`, non-Iranian flags are false, Zibal remains enabled, and non-Iranian backend endpoints remain untouched.
6. **GOPROXY:** the staging build used the documented fallback chain because the default proxy returned 403 in that environment.

## Blockers

- **Zarinpal merchant ID not available** — Zarinpal checkout/callback tests BLOCKED until operator supplies a valid sandbox `ZarinpalMerchantID` (placed in `/home/ubuntu/hermes-env.sh`). No fake merchant used, per operator instruction.

## Redaction confirmation

- `pay_link` values redacted to `gateway.zibal.ir/<REDACTED>` in artifacts.
- No cookies, session IDs, merchant IDs, authority/trackId/refNumber, or bearer tokens in artifacts.
- `environment.txt` contains no secrets (only hostnames + test constants).
- `SHA256SUMS` generated for all artifact files.

## Notes

- Test orders kept for audit/reconciliation (not deleted).
- Safe sandbox options restored at end: `ZarinpalSandbox=true`, `ZibalMerchant=zibal`, `ZarinpalIRRPerUSD=1000000`, `ZarinpalMarginBPS=1000`, `ZarinpalMinTopUpUSD=1`, `IranianPaymentDefault=zarinpal`, `IranianPaymentAutoFailover=true`, `IranianPaymentExclusiveUI=true`.
- PR #1 remains **Draft** — do not merge as a dual-gateway release until the Zarinpal path is complete.

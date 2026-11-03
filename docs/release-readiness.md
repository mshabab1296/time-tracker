# TimeTracker V1 — Release Readiness

Last reviewed: 2026-09-27. This is evidence, not a production launch approval.

## Phase 5A — Verified

- Ticket creator versus other Member and Admin edit/delete permissions.
- Revoked membership and organization isolation.
- Case-insensitive reference uniqueness, bounded search, pagination, invalid limits.
- Multiple tickets on manual entries and live timers, active retrieval after refresh, ticket removal.
- Invalid duplicate/cross-organization selections rejected.
- Historical reference rename visible in entry details and CSV exports.
- Linked-ticket deletion protection; entry deletion cascades links and makes tickets deletable.
- Personal/organization report permissions, ticket grouping (including No ticket), any-ticket filtering, detailed totals counted once, detailed and summary CSV output.
- Migration 8 → 9 preserves the original nullable ticket link, duration, and deletion protection.

Evidence: `internal/tickets/tickets_integration_test.go`, `internal/reporting/report_test.go`, and `scripts/verify-release.ps1`, executed against disposable PostgreSQL 16 databases. No application database was migrated or restored by these checks.

## Phase 6 — Implemented and locally verified

- Database-backed API tests and `go vet` pass.
- Linux `go test -race -count=1 ./...` passes in the official Go 1.25 container against a disposable PostgreSQL 16 database. GitHub CI execution is still pending a push.
- Frontend formatting, TypeScript, lint, production build, and production-dependency audit pass.
- Dependency scan reports zero reachable vulnerabilities; it identifies 22 advisories in required modules not reached by the application, which remain an upgrade-review item rather than a claim that every dependency is vulnerability-free.
- Regression: completed timer reclassification preserves duration excluding pauses.
- Ticket mutation and audit record commit atomically; injected audit failures roll back create/update/delete.
- Ticket JSON rejects unknown fields and multiple objects; release rehearsal asserts exact remaining synthetic fixture counts after test cleanup.
- Detailed and summary CSV neutralize spreadsheet formulas in user-controlled text.
- Structured request logs include HTTP status, duration, and validated correlation IDs without cookies/query values. Safe panic recovery does not disclose panic values.
- Internal `/metrics` exposes request counts by status class and latency histogram. Labels have bounded cardinality.
- `docs/openapi.json` covers all registered routes, typed response payloads, body schemas, authentication, CSRF, query parameters, and envelope conventions. Tests enforce route coverage, resolve references, compile every JSON response schema, and reject anonymous requests at every session-protected operation. Database-backed ticket, timer, manual-entry, and report integration responses are checked against the schemas; this is critical-flow coverage, not every possible payload on every endpoint.
- Summary generation walks tag × ticket combinations incrementally, capped at 10,000 distinct groups and 1,000,000 contributions. Boundary, excessive repeated-work, and multi-ticket/tag duration tests pass. Excessive grouping fails with 422 rather than truncation or partial CSV.
- Invitation decline response/audit regression fixed: `DECLINED`, not `DECLINEED`.
- Shared modal focus trapping, focus restoration, background inertness, body-scroll locking, and dropdown arrow/Enter/Escape controls implemented. Browser verification used real shared components with synthetic data in `apps/web/tests/harness.html`, including React StrictMode. Keyboard checks and 390 × 844 mobile modal fit/scrolling passed in the available in-app Chromium browser; all four corners were 16px and no console errors were observed. This is not a screen-reader or four-browser sign-off.
- `scripts/runtime-grants.sql` grants domain DML and audit SELECT/INSERT only. A temporary non-superuser/non-owner role successfully updates entries but cannot create schema objects or update/delete/truncate audit records. No production credentials were changed.
- Operational runbooks exist in `docs/operations.md`.
- Local migration/backup/restore rehearsal passed with 10,001 synthetic entries, 20,002 ticket links, 10,001 tag links, and 8,000 timer events. Restored counts, preserved original migration link/duration, total duration (22,803,600 seconds), and timer-history counts are asserted. Final backup plus restore plus invariant checks took 3.2 seconds. This does not establish production-host RTO or encrypted off-server backup compliance.
- Local read-heavy capacity scenario: 1,000 authenticated users, one request per user every 5 seconds, 60-second measurement, 12,000 requests, approximately 200 requests/second, zero failures, pool limit 10. Dataset: 1,000 users, 1,000 time entries, and 1,000 tickets.
- Per-endpoint p95: auth/me 12.2 ms; active timer 18.8 ms; selected-day entries 24.8 ms; paginated ticket list 25.3 ms; personal report 24.9 ms. End-to-end localhost HTTP timings include session validation and database queries, but exclude public-network latency, ingress, TLS, production contention, and browser rendering.
- Additional mixed local scenario passed with 1,000 authenticated users and 100,000 historical entries: 60 seconds, 12,000 requests (~200/s), pool limit 10, zero failed requests, all ten tested endpoints p95 <= 300 ms. 600 users read; 200 cycle timer start/pause/resume/stop; 200 create non-overlapping manual entries. The passing test asserts the latency threshold; the non-verbose rehearsal output does not retain exact per-endpoint percentile numbers. Use the verbose command below for fresh measurements.

## Required before Phase 6 can be marked complete

- GitHub Actions must run and pass on the final changes. Linux race detection has passed locally; this is not a claim of remotely passing CI.
- Browser/keyboard/screen-reader validation on Chrome, Edge, Firefox, and Safari, plus mobile layouts. Automated backend checks do not establish this.
- Repeat capacity tests on the intended release host with representative historical data and concurrent timer/manual-entry mutations; verify p95 <= 300 ms per normal endpoint. CSV generation remains excluded from the latency target.
- Verify least-privilege runtime credentials, append-only audit grants, secret handling, HTTPS, and internal-only metrics in the production-shaped environment.
- Perform a restore rehearsal with production-sized non-production data and record RPO/RTO evidence. The synthetic local check is only functional evidence.
- Review actual deployment/rollback/backup/alert configuration when Phase 7 provisions it. Do not expose the current local development defaults publicly.

## Repeating the checks

From the repository root in PowerShell:

```powershell
./scripts/verify-release.ps1 -LinuxRace
```

`-LinuxRace` uses the official `golang:1.25-bookworm` image with a read-only repository mount and the PostgreSQL container's network. Native Linux/CGO-capable environments can omit the switch. Windows without a C compiler/CGO can explicitly use `-SkipRace`; this prints a warning and is not race evidence. The script creates and removes only uniquely named synthetic databases and a temporary no-login runtime role. It also removes its synthetic dump. It never drops the application database. Docker Desktop must be running.

For the optional load scenario, use a disposable, migrated database:

```powershell
$env:DATABASE_URL = '<disposable PostgreSQL URL>'
$env:RUN_CAPACITY_TEST = '1'
$env:CAPACITY_DURATION_SECONDS = '60'
$env:CAPACITY_MIXED = '1' # Optional: 100,000 historical entries and concurrent mutations.
cd apps/api
go test -v -run '^TestReleaseCapacity$' -count=1 ./internal/platform/httpserver
```

The scenario seeds and removes its own UUID-scoped records. Do not benchmark against production. The test fails on any response/read error or per-endpoint p95 above 300 ms. It is opt-in and is skipped by normal CI.

For repeatable browser component checks, start the web dev server and open `/tests/harness.html`. This page uses synthetic state only and is not included in the production bundle. Check dropdown keyboard selection, Escape ordering, Shift+Tab/Tab wrapping, focus restoration, modal scrolling, and mobile fit. Full authenticated workflow/browser and screen-reader checks remain required.

## Phase 7 boundary

Production infrastructure, domain/HTTPS, real SES delivery, ECR publishing, daily encrypted S3 backups, alerts, and automatic deployment/rollback are not created by Phase 6. No server or cloud resources have been purchased or provisioned.

# TimeTracker V1 — Operational Runbooks

These procedures define release expectations. Production infrastructure is still pending Phase 7. Commands use placeholders intentionally; never substitute the current local Docker database for a production restore target.

## Deployment and preflight

1. Select an immutable image tag or digest for the API and web app. Require passing CI, integration/race tests, frontend checks, and vulnerability scans.
2. Record the previous compatible API/web images and current migration version. Confirm tested backup availability and database free space before any migration.
3. Provision separate migration-owner and least-privilege application database credentials via deployment Secrets, not images or Git. The app needs DML only on its domain tables; audit grants must be SELECT/INSERT only, never UPDATE/DELETE/TRUNCATE. It must not own tables, create schemas, or bypass permissions as a superuser. Verify these grants with the real application account.
4. Configure HTTPS, `COOKIE_SECURE=true`, the actual HTTPS `WEB_BASE_URL`, SES region/from address, and database TLS appropriate to the deployment network. Never print environment variables or credentials into CI logs.
5. Run forward migrations once, using the migration owner. Do not start a new app version if migrations fail or leave a dirty migration state. Diagnose and repair intentionally; never blindly force the migration version.
6. Roll out the API and web images. Validate `/health/live`, `/health/ready`, login/session refresh, organization selection, timer state transitions, manual-entry creation, and report/CSV download with a dedicated smoke-test account.
7. Observe error rate, latency, disk space, database pool use, and email delivery. Store image digests, migration version, results, and operator/time in release notes.

`/metrics` is an internal endpoint without application authentication. Restrict it to monitoring/network access; exclude it from the public Ingress. It exposes no user identifiers or ticket references. Compute request/error rates from counters and latency percentiles from histogram buckets. CloudWatch alert wiring belongs to Phase 7.

## Application rollback

- Roll back API and web to the last **schema-compatible** immutable release if rollout/readiness/smoke checks fail. Recheck health and critical workflows.
- Do not automatically run down migrations or restore a database simply to roll back an app: both can destroy writes received since deployment.
- Migration 9 removes `time_entries.ticket_id`. Builds that still use that column are not valid rollback targets after this migration. Establish a multi-ticket-capable rollback baseline, or use an expand/contract transition before any production upgrade across this boundary.
- If no compatible image exists, stop the rollout and prefer an audited forward fix. Database restore requires incident approval and an explicit data-loss decision.

## Backup

Production requirements: daily encrypted logical backups, off-server S3 storage in Mumbai, 30-day retention, RPO <= 24 hours, RTO <= 4 hours.

1. Use a dedicated backup identity and secret-supplied connection details. Do not put passwords in command-line arguments or logs. Use `PGPASSFILE` with restrictive permissions or equivalent secret delivery.
2. Generate a custom-format dump to a private filesystem directory:

```sh
pg_dump --format=custom --no-owner --no-acl --file=<private-backup-file> <database-name>
pg_restore --list <private-backup-file>
```

3. Calculate and retain an integrity checksum and record schema version, PostgreSQL version, timestamp, dump size, and object key. A readable archive is not proof of a successful restore.
4. Upload using the backup-only AWS identity to the approved private S3 bucket with server-side encryption, TLS, public access blocked, and 30-day lifecycle rules. Enforce encryption in bucket policy. Keep the application identity unable to delete backups.
5. Mark success only after upload and integrity checks. Alert the owner when the last successful backup is older than 24 hours or a scheduled backup fails.
6. Remove the local backup only after verified off-server storage and according to the local retention policy. Do not leave dumps in a web-served directory.

These steps describe policy; they are not yet an installed production scheduler or S3 configuration.

## Restore and rehearsal

1. Confirm the exact incident/rehearsal scope, selected backup time, expected data loss, and target database with the owner. Start the RTO clock.
2. Download the encrypted backup using an authorized identity, verify its checksum, and check PostgreSQL compatibility. Do not restore onto the live app database.
3. Create a **new empty database** with an explicit rehearsal/recovery name and restore there using the migration/recovery identity:

```sh
createdb <new-recovery-database>
pg_restore --exit-on-error --single-transaction --no-owner --no-acl --dbname=<new-recovery-database> <verified-backup-file>
```

4. Reapply least-privilege grants separately. Check schema version, expected row counts, entry durations, ticket/tag links, timer events, foreign keys, and audit records. Compare representative records with the backup evidence.
5. Run the API and critical smoke workflows against the recovered database in an isolated environment. Keep real outbound email disabled during rehearsals.
6. For a real recovery, approve a controlled database connection switch, verify health/workflows, then reopen traffic. Preserve the old database for investigation until retention/cleanup is approved.
7. Record elapsed recovery time and backup age. Confirm RTO <= 4 hours and RPO <= 24 hours at production-representative scale. Cleanup only explicitly identified rehearsal databases/artifacts.

`scripts/verify-release.ps1` automates a synthetic local rehearsal and migration-preservation check. It does not restore production data or prove production RTO by itself.

The rehearsal also applies `scripts/runtime-grants.sql` to a disposable no-login role and checks domain updates, denied schema creation, and denied audit update/delete/truncate. To apply this template in deployment, provision the existing non-owner runtime identity securely, review inherited/PUBLIC privileges, then run it as the migration owner with `psql -v application_role=<runtime-role>`. Reapply after new domain tables are introduced. This template does not create a production login or store its password.

## Incident response

- Identify scope using correlated request logs, readiness, error-rate/latency metrics, disk usage, database connectivity, and backup status. Do not log raw passwords, cookies, action tokens, or connection strings.
- For availability failures, restrict changes, preserve logs, check recent deployments/migrations, and choose compatible rollback versus forward fix. Avoid repeated destructive retries.
- For credential exposure, restrict access, rotate the affected secret, revoke affected sessions, review audit records, and assess backup access. Preserve evidence and notify the owner.
- For data corruption, stop affected writes before approving recovery; document the backup timestamp and expected lost writes. Never silently restore over current data.
- After recovery, verify health and critical paths, record a timeline and root cause, and add regression tests or alerts for the failure mode.

# TimeTracker V1 — Implementation Plan

## 1. Purpose

This plan turns the agreed functional requirements, non-functional requirements, high-level design, and low-level design into small, verifiable delivery phases.

The implementation uses a **vertical-slice** approach: each early milestone delivers a working user workflow across the database, Go API, React UI, and automated tests. This avoids building isolated layers that cannot yet be used.

## 2. Target Repository Layout

```text
time-tracker/
├── apps/
│   ├── api/                    # Go modular-monolith API
│   └── web/                    # React + TypeScript frontend
├── db/
│   └── migrations/             # Version-controlled PostgreSQL migrations
├── deploy/
│   ├── compose/                # Local Docker Compose configuration
│   └── kubernetes/             # K3s/Kubernetes manifests
├── docs/                       # Requirements, design, and runbooks
├── .github/workflows/          # CI/CD workflows
└── README.md
```

Within `apps/api`, code shall be organized by domain rather than technical layer alone:

```text
internal/
├── auth/
├── users/
├── organizations/
├── projects/
├── tags/
├── tickets/                # Provider-neutral work references
├── timeentries/
├── reports/
├── invitations/
├── audit/
└── platform/                   # HTTP, database, configuration, email, storage
```

## 3. Delivery Principles

- PostgreSQL migrations are the only mechanism for schema changes.
- Every protected API endpoint validates the database-backed session and server-side authorization.
- API responses use the agreed camelCase universal envelope.
- Each feature includes API tests and UI coverage proportionate to its risk.
- No secrets are committed. Local development uses a `.env.example`; production uses manually provisioned Kubernetes Secrets.
- The `main` branch remains deployable. Features are integrated only after CI passes.

## 4. Phase 0 — Repository and Local Foundation

### Scope

- Create the monorepo structure, Go module, React/TypeScript application, and root documentation.
- Add Docker Compose for the local PostgreSQL database and local API/web development.
- Establish configuration loading, structured logging, correlation IDs, health endpoints, database connection pooling, and a standard API error/response package.
- Configure formatting, linting, unit-test commands, dependency-vulnerability scanning, and GitHub Actions CI.
- Add the first empty migration and migration runner.

### Done when

- A new developer can follow the README to run the web app, API, and PostgreSQL locally.
- `GET /health/live` and `GET /health/ready` work.
- Pull requests run the baseline CI checks successfully.

## 5. Phase 1 — Account, Session, and Organization Slice

### Scope

- Create migrations for users, hashed database sessions, email-action tokens, organizations, and memberships.
- Implement registration, email verification, login, logout, `GET /auth/me`, password reset, profile name update, and organization creation/selection.
- Use Argon2id password hashing, hashed session and action tokens, Secure/HttpOnly/SameSite cookies, and CSRF protection.
- Create the initial React authentication screens and authenticated organization selector.
- Make the organization creator an `ADMIN` transactionally.

### Done when

A newly registered user can verify their email, log in, create an organization, refresh the browser without losing their authenticated state, and log out. Authorization and session-expiry tests cover the workflow.

## 6. Phase 2 — Members, Invitations, Projects, and Tags Slice

### Scope

- Add invitations, membership management, projects, project assignments, tags, and audit-log migrations.
- Implement Admin-only member, invitation, project, assignment, and tag endpoints.
- Integrate Amazon SES behind the email-sender interface; retain a local development/test sender.
- Build the React organization-management views.
- Enforce project visibility, organization scoping, case-insensitive name uniqueness, invitation lifecycle rules, and deletion restrictions.

### Done when

An Admin can invite a user, manage Members, create projects/tags, and assign a project. A Member sees only their assigned projects and cannot perform administrative actions.

## 7. Phase 3 — Timer Slice

### Scope

- Add time entries, timer events, and entry-tag links.
- Implement start, pause, resume, stop, active-entry update, and active-timer retrieval endpoints.
- Enforce one global active timer with the PostgreSQL partial unique index and transaction-safe state changes.
- Implement timer-event ordering and duration calculation in the Go domain service.
- Build the primary timer UI, active timer state, project/tag selection, and organization-aware timer display.

### Done when

A user can start, pause, resume, stop, and revisit a timer after refreshing the browser. Concurrent/retried actions remain safe and cannot create two active timers.

## 8. Phase 4 — Manual and Completed Entry Slice

### Scope

- Implement manual-entry creation using Start + End or Start + Duration.
- Implement completed-entry lists, detail, edit, reclassification, hard deletion, and authorized Admin management.
- Implement timer-event editing, valid pause/resume pair changes, recalculated duration, and audit logging.
- Enforce no-future-time and no-overlap rules with PostgreSQL constraints plus service-level transactional validation.
- Build the entry list, manual-entry form, and entry-detail/edit views.

### Done when

Members can manage their completed work; authorized Admins can manage organization members’ entries; invalid overlap, event sequence, or authorization attempts are rejected and audited.

## 9. Phase 5 — Reporting and CSV Slice

### Scope

- Implement personal and organization report queries with date, Member, project, and tag filters.
- Implement grouping by the supported dimensions and day/week/month handling.
- Allocate entries spanning midnight across report-day boundaries in the appropriate user or organization timezone.
- Implement paginated screen reports and immediate detailed/summary CSV responses.
- Enforce the 100-record page size and 10,000-entry report/export limit.

### Done when

The UI displays correctly filtered, paginated personal and Admin reports, and CSV downloads match the screen/report rules.

## 9A. Phase 5A — Ticket References Slice (design amendment)

This slice follows the already-planned reporting work and precedes release hardening. Members may manage tickets they created; Admins may manage all organization tickets. The implementation and authorization tests shall enforce that ownership rule.

### Scope

- Add a `tickets` migration with `created_by_user_id`, case-insensitive ticket-reference uniqueness, and lookup indexes. Follow it with a forward migration from the original nullable `time_entries.ticket_id` to `time_entry_tickets`, preserving existing links and enforcing same-organization referential integrity.
- Implement organization-scoped ticket create, list/search, update, and delete endpoints, including linked-entry deletion protection and audit records. Keep the domain provider-neutral and do not add Jira credentials or synchronization.
- Add a paginated ticket-management screen and bounded server-backed multi-ticket picker to timer, manual-entry, and completed-entry workflows. Display all linked tickets in entry details.
- Extend personal and organization report filters, groupings, screen rows, and CSV exports to support multiple tickets per entry; preserve behavior for entries with no ticket and avoid double-counting detailed/overall totals.
- Cover creator-versus-Admin permissions, revoked membership, organization isolation, duplicate references, entry-ticket linkage, historical renames, deletion restrictions, search/pagination, and report/export totals with tests.

### Done when

Users can attach multiple optional organization tickets to time entries and report time by ticket without creating one tag per work item. Ticket lists and pickers remain bounded as the catalog grows. Existing entries continue to work without tickets, and no external ticket-board integration is required.

## 10. Phase 6 — Hardening and Release Readiness

### Scope

- Complete integration, authorization, migration, API-contract, and load tests for critical flows.
- Validate p95 latency and the 1,000-concurrently-active-user target with repeatable load tests.
- Review accessibility, browser support, input validation, error safety, logging, auditability, and secret handling.
- Write operational runbooks: deployment, rollback, backup, restore, and incident response.
- Perform a manual database restore rehearsal against a non-production database before public launch.

### Done when

The release checklist passes and the system can be restored from a backup within the stated four-hour RTO.

## 11. Phase 7 — Production Delivery

### Scope

- Create the K3s manifests for frontend, API, migration job, Ingress, health probes, configuration, and Secrets.
- Provision the AWS Lightsail server in Mumbai, K3s, PostgreSQL persistent storage, AWS ECR Public images, Amazon SES, S3 backups, CloudWatch alerts, firewall rules, and the production domain/HTTPS certificate.
- Configure GitHub Actions to test, publish images, connect over SSH, run forward-only migrations, roll out, verify health, and automatically roll back a failed application deployment.

### Done when

Merging a tested change to `main` deploys it automatically to the single production environment. HTTPS works on the selected domain, backups complete daily, and alerts reach the owner.

## 12. Recommended Build Order Within Each Slice

1. Migration and repository/data-access tests.
2. Go domain rules and service tests.
3. HTTP handlers and API-contract tests.
4. React screen and API integration.
5. End-to-end critical-path test.
6. Documentation and release notes.

## 13. Explicitly Deferred Work

- Microservice extraction and Redis.
- Authentication and email-action rate limiting.
- Background email retry queue.
- Async/S3-persisted report exports.
- Cross-region backups, automated restore drills, paid staging, and high-availability multi-server deployment.
- Organization deletion, account deletion, project archival, ticket-board synchronization, ticket workflow management, payroll, and other features already out of V1 scope.

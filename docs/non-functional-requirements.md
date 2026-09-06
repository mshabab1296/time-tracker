# TimeTracker V1 — Non-Functional Requirements

## 1. Capacity and Performance

### NFR-1 Initial capacity

TimeTracker V1 shall be designed to support up to **1,000 concurrently active users**.

### NFR-2 API latency

Under expected load, normal API requests shall complete within **300 ms at the 95th percentile (p95)**. This target excludes CSV download generation.

### NFR-3 Load validation

The initial-capacity and API-latency targets shall be verified with repeatable load tests before release.

### NFR-4 Report and export limits

Paginated API endpoints shall enforce a maximum page size of **100 records**. A report or CSV export request shall process no more than **10,000 time entries**.

V1 reports shall render directly in the UI with pagination. Clicking Download shall generate and return the CSV immediately. V1 shall not use asynchronous export jobs, SSE export progress, or server/S3-persisted generated files. V1 does not set a separate CSV-download latency target.

## 2. Availability and Reliability

### NFR-5 Availability target

The core application shall target **99.9% monthly availability**, excluding planned maintenance.

### NFR-6 Atomic changes

Timer and time-entry changes shall be atomic: if an operation fails partway through, none of its changes shall persist. Concurrent requests shall not violate the one-active-timer or no-overlap rules.

### NFR-7 Idempotent timer actions

Timer start, pause, resume, and stop operations shall be idempotent. Retrying the same operation shall not create duplicate timer events, duplicate time entries, or an invalid timer state.

### NFR-8 Graceful shutdown

An application instance shall support graceful shutdown, allowing in-flight requests to finish safely before it stops.

### NFR-9 Health checks

The application shall expose liveness and readiness health-check endpoints. Kubernetes shall use these endpoints to restart unhealthy instances and route traffic only to ready instances.

### NFR-10 Backup and recovery

Production PostgreSQL data shall be backed up with a daily encrypted logical backup to Amazon S3 in the same AWS Mumbai region. Backups shall be stored off the application server, retained for **30 days**, and include a documented restore procedure.

The recovery-point objective (RPO) is **24 hours** and the recovery-time objective (RTO) is **4 hours**. Cross-region replication and automated restore drills are out of scope for V1.

## 3. Security

### NFR-11 Transport security

All browser-to-application traffic shall use HTTPS. Plain HTTP requests shall be redirected to HTTPS or rejected.

### NFR-12 Credential and session security

Passwords shall be stored only as salted, one-way hashes. The system shall never store passwords in plaintext or reversible form.

A logged-in session shall expire after **24 hours of inactivity**, requiring the user to authenticate again.

### NFR-13 Authorization and input protection

Every protected action shall enforce authorization on the server using the user's organization membership and role. Client-side visibility checks alone shall not grant access.

All API inputs shall be validated on the server and rejected when malformed, unauthorized, or outside defined limits.

Client-facing errors shall not expose internal implementation details; detailed diagnostics shall be available only in server logs.

### NFR-14 Secrets and database access

Credentials and other secrets shall be supplied through configuration or Kubernetes Secrets. They shall not be committed to source control, container images, or logs.

The application shall use a least-privilege database account, not a database administrator/root account.

### Deferred security work

Authentication, password-reset, email-verification, and invitation rate limiting are deferred to the next version.

## 4. Observability and Auditability

### NFR-15 Structured logging and request correlation

The application shall emit structured logs for requests, errors, and important business actions, including timer state changes and administrative updates. Logs shall include sufficient diagnostic context without recording passwords or authentication secrets.

Every request shall have a correlation ID that is included in logs and client-facing error responses.

### NFR-16 Metrics and alerts

V1 shall expose basic operational metrics for request rate, error rate, and request latency. AWS CloudWatch shall alert the owner by email when the server is unreachable, a backup fails, or disk space is critically low.

### NFR-17 Audit trail

The system shall retain an append-only, immutable audit trail for time-entry edits/deletions and Admin actions. Application users, including Admins, shall not be able to alter or delete audit records.

Audit records shall be retained for the lifetime of their organization, with no automatic expiry in V1.

## 5. Scalability and Deployment

### NFR-18 Horizontal scaling

The API shall be stateless and support multiple application instances behind a load balancer without breaking sessions or timers.

### NFR-19 Containerized, reproducible deployment

V1 shall be containerized with Docker so local development, CI, and deployment use the same runtime model.

Deployments shall be reproducible from version-controlled configuration and support rollback to the last working release. A release that fails its health check shall automatically roll back its application deployment.

All database schema changes shall be made through version-controlled, forward-only migrations rather than manual production edits. The deployment pipeline shall run migrations before application rollout; migrations shall remain compatible with the previously deployed application version.

## 6. Quality and Delivery

### NFR-20 Automated testing and CI/CD

Automated tests shall cover critical flows, including authentication, invitations, permissions, timer state transitions, and time-entry validation.

Before deployment, CI shall run formatting, linting, required automated tests, and dependency-vulnerability scanning. High-severity dependency-vulnerability findings shall fail CI.

Every merge to `main` shall automatically build, test, publish, and deploy the application to the single V1 production environment.

## 7. API and User Experience

### NFR-21 API contract

Public API endpoints shall use an explicit version prefix, such as `/api/v1`.

The API shall have versioned, machine-readable documentation (for example, an OpenAPI specification) that is kept in sync with the implementation.

### NFR-22 Browser support and accessibility

The web UI shall be responsive and usable on desktop and mobile browsers.

V1 shall support current major versions of Chrome, Edge, Firefox, and Safari.

Core workflows shall be keyboard accessible and expose accessible labels for controls and form fields to support screen readers.

## 8. Architecture Decision

### ADR-1 Modular monolith

V1 shall use a modular-monolith architecture rather than microservices. The codebase shall maintain clear domain boundaries for authentication, organizations, projects and tags, time tracking, reporting, and notifications.

This decision matches the V1 scale while avoiding unnecessary distributed-system complexity. Domain boundaries shall make future service extraction feasible when justified by operational or product needs.

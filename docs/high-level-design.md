# TimeTracker V1 — High-Level System Design

## 1. Architecture Overview

TimeTracker V1 uses a separate web frontend and backend API:

```text
React + TypeScript frontend
            |
            | HTTPS /api/v1
            v
Go modular-monolith API
            |
            v
PostgreSQL
```

## 2. Frontend

The frontend shall be a separate React and TypeScript application. It shall communicate with the backend only through the versioned `/api/v1` interface.

The frontend is responsible for user-facing workflows, including authentication screens, organization selection, timer controls, ticket search and management, time-entry management, reports, and CSV download. Ticket pickers shall query the API for a small result set rather than load the entire organization catalog.

## 3. Backend

The backend shall be a Go modular monolith. It shall expose the versioned HTTP API, enforce authentication and authorization, implement the time-tracking and ticket-reference domain rules, and provide reporting and export capabilities. Ticket records and search shall remain a provider-neutral module; Jira and other board synchronization are later integrations, not part of V1.

## 4. Primary Datastore

PostgreSQL shall be the primary relational datastore for V1. It shall store transactional application data, including users, organizations, memberships, projects, tickets, tags, time entries, timer events, invitations, and audit records. Entries and organization-owned tickets have a many-to-many relationship through an entry-ticket join table. Tickets and tags are separate dimensions in reporting.

The TimeTracker ticket ID shall be an internal identifier, independent of a Jira issue key or any other provider's identifier. A later integration may add provider connections and external-identity mappings without replacing existing ticket IDs or time-entry links.

Ticket ownership shall be recorded separately from organization membership. Members may manage tickets they created while they remain members; Admins may manage all tickets in their organization. Removing a Member shall not remove their tickets or historical entry links.

## 5. Authentication and Sessions

V1 shall use database-backed sessions rather than JWTs stored in browser storage. On successful login, the backend shall create a session record in PostgreSQL and set a random session identifier in a Secure, HttpOnly, SameSite cookie.

The frontend shall determine the authenticated state by calling `GET /api/v1/auth/me`. The browser automatically includes the session cookie. A successful response returns the current user's safe profile and context; a `401 Unauthorized` response means the user must sign in. The frontend shall keep only returned user data in its application state and shall not access or persist the session credential.

## 6. Email Delivery

Invitation, email-verification, and password-reset messages shall be sent through Amazon SES behind a Go email-sender interface. This keeps the application independent of a particular provider and permits a mock sender during local development and tests.

Email delivery is synchronous in V1. No background queue or retry worker shall be used; users and administrators may use the applicable resend flow after a temporary provider failure.

## 7. Profile-picture Storage

Profile pictures shall be stored in S3-compatible object storage. PostgreSQL shall store only the associated object metadata or URL, not image binary data.

## 8. Environments and Production Hosting

Local development shall use Docker Compose for rapid feedback. A local Kubernetes cluster shall also be used to exercise Kubernetes manifests and operational workflows before deployment. `kind` is the preferred local Kubernetes option because it runs Kubernetes nodes as Docker containers and supports local development and CI.

V1 shall have one production environment. It shall run on a 2 GB AWS Lightsail Linux instance in the AWS Mumbai region, with a low-cost VPS as a portable fallback. K3s shall host the React frontend and Go API. PostgreSQL shall run as a Docker workload on the same server with persistent storage and no public database port.

The public release requires a chosen domain and HTTPS. Until a domain is selected, external testing may occur, but the public launch shall not proceed.

## 9. Ingress and HTTPS

A Kubernetes Ingress controller shall terminate HTTPS traffic and route requests to the appropriate services. It shall route frontend traffic to the React application and `/api/v1` traffic to the Go API.

## 10. Delivery and Container Images

GitHub Actions shall run CI and automatically deploy every merge to `main`. It shall build and publish production images to a public AWS ECR repository, then connect to the production server over SSH using a deploy key to apply the release.

The server shall pull public images from ECR. Images shall not contain secrets or user data. Production configuration and secrets shall be provisioned manually as Kubernetes Secrets during server setup and shall not be stored in source control.

Schema migrations shall run as a forward-only Kubernetes job before application rollout. The deployment shall automatically roll back the application version when its health check fails.

## 11. Backups and Monitoring

A scheduled encrypted PostgreSQL backup shall be sent daily to Amazon S3 in the AWS Mumbai region and retained for 30 days. Cross-region copies are out of scope for V1.

AWS CloudWatch shall provide infrastructure monitoring and email alerts for server unavailability, backup failures, and critically low disk space.

## 12. Cache and Queueing

V1 shall not include Redis. PostgreSQL shall handle sessions and transactional data. Redis may be introduced in a later version when justified for session caching, rate limiting, caching, or background jobs.

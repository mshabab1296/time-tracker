# TimeTracker V1 — Low-Level Design

## 1. Data-Model Conventions

### LLD-1 Primary keys

All persistent entities shall use UUID primary keys rather than sequential numeric IDs. UUIDs may be safely exposed in API paths and responses without revealing record counts or relying on database-local sequences.

UUIDv7 shall be used because its roughly time-ordered values are friendlier to PostgreSQL indexes than fully random UUIDv4 values.




### LLD-2 Timestamp storage

All timestamps shall be stored in PostgreSQL as UTC `timestamptz` values. Conversion to a user's or organization's configured timezone shall occur only when interpreting input, displaying data, or generating reports.



### LLD-3 Standard audit timestamps

Every core persistent table shall include UTC `created_at` and `updated_at` timestamps for traceability and debugging.



### LLD-4 Email identity

User email addresses shall be normalized to lowercase and enforced as case-insensitively unique at the database level.



## 2. Membership Model

### LLD-5 Unique membership

The database shall enforce a unique `(organization_id, user_id)` constraint. A user may belong to many organizations, but may have only one membership—and therefore one role—within any one organization.



### LLD-6 Membership roles

The membership `role` column shall be constrained to the two V1 values: `ADMIN` and `MEMBER`.



## 3. Project Assignment Model

### LLD-7 Project assignments

Project assignments shall be stored in a separate `project_assignments` table. The database shall enforce a unique `(project_id, user_id)` constraint so a Member cannot be assigned to the same project more than once.



## 4. Time-entry Tags

### LLD-8 Tag links

A many-to-many `time_entry_tags` table shall link tags to time entries. The database shall enforce a unique `(time_entry_id, tag_id)` constraint so the same tag cannot be attached to an entry more than once.



## 5. Timer Model

### LLD-9 Time-entry statuses

The `time_entries.status` column shall be constrained to the V1 values: `RUNNING`, `PAUSED`, and `STOPPED`.



### LLD-10 Timer-event types

The `timer_events.event_type` column shall be constrained to the V1 values: `START`, `PAUSE`, `RESUME`, and `STOP`.



### LLD-11 Single active timer

PostgreSQL shall enforce the one-active-timer rule with a partial unique index on `time_entries(user_id)` for rows whose status is `RUNNING` or `PAUSED`. This database-level constraint prevents concurrent requests from creating multiple active timers for a user.



### LLD-12 Non-overlapping time

For completed time entries, PostgreSQL shall enforce non-overlap per user with an exclusion constraint over the entry's time range. Operations involving an active timer shall use transactional validation so they cannot create an overlapping manual entry or inconsistent live timer state.



## 6. Referential Integrity

### LLD-13 Foreign-key deletion behavior

Foreign keys shall prevent deletion of a project or tag while an existing time entry references it. Cascading deletion is permitted for dependent join records such as `project_assignments` and `time_entry_tags` when their parent is deleted. Hard-deleting a time entry shall cascade-delete its timer events and tag links; its audit record remains as a non-foreign-key historical record.



## 7. Audit Model

### LLD-14 Lean audit records

An append-only `audit_logs` table shall record the actor, action, target entity, target identifier, and timestamp for audited operations. For relevant edits, a `JSONB` metadata field shall store only changed fields and their before/after values. V1 shall not store complete snapshots for every action.



## 8. Session Model

### LLD-15 Session credential storage

The database shall store only a cryptographic hash of each session identifier. The raw session identifier shall exist only in the browser's Secure, HttpOnly cookie and shall not be persisted in application-readable browser storage.



## 9. Email-action Tokens

### LLD-16 Token storage and single use

Email-verification, password-reset, and invitation tokens shall be stored only as cryptographic hashes. Each token shall become invalid after successful use or expiry.



### LLD-17 Email-action expiry

Password-reset tokens shall expire after one hour. Email-verification tokens shall expire after 24 hours. Invitation expiry remains seven days as defined in the functional requirements.

### LLD-18 Timer-event sequence validation

The Go service shall transactionally validate the event sequence `START → (PAUSE → RESUME)* → STOP`. This behavior shall not rely on database triggers.

### LLD-19 Duration representation

Tracked duration shall be stored internally as integer seconds. The UI shall display duration using minute-level precision.

### LLD-20 Timezone identifiers

User and organization timezones shall use IANA timezone identifiers, such as `Asia/Kolkata`, rather than fixed UTC offsets.

### LLD-21 Name length

Organization, project, and tag names shall be limited to 100 characters.

### LLD-22 Password hashing

Passwords shall use Argon2id hashing. Its parameters shall be configuration-driven so they can be upgraded later.

### LLD-23 Session activity

Each session shall store `last_seen_at` and `expires_at` to enforce 24-hour inactivity expiry and support revocation.

### LLD-24 Organization consistency

Each time entry shall store a direct `organization_id` foreign key in addition to `user_id` and `project_id`. Before saving an entry, the Go service shall transactionally verify that its user membership, project, tags, and organization all belong to the same organization.

## 10. Invitation Model

### LLD-25 Invitation lifecycle records

Invitation records shall be retained and use the statuses `PENDING`, `ACCEPTED`, `DECLINED`, `EXPIRED`, and `CANCELLED`.

### LLD-26 Pending-invitation uniqueness and expiry

PostgreSQL shall enforce at most one pending invitation for a normalized email and organization using a partial unique index. Invitation expiry shall be evaluated lazily when an invitation is viewed, resent, or accepted; V1 requires no expiration scheduler.

## 11. Timer-entry Representation

### LLD-27 Entry source type

The `time_entries.source_type` column shall be constrained to `MANUAL` or `TIMER`.

### LLD-28 Manual and timer data

All entries shall use universal `started_at` and `ended_at` columns. A timer Stop action sets `ended_at`.

Manual entries have start/end timestamps and no timer-event rows. Timer entries also retain timer-event history. A completed timer entry must have one `START` and one `STOP` event.

### LLD-29 Event ordering

Each timer event shall include a `sequence_number` unique within its time entry, in addition to its timestamp, so ordering is unambiguous.

### LLD-30 Timer actions and retries

Timer actions shall use explicit endpoints: `POST /api/v1/timer/start`, `/pause`, `/resume`, and `/stop`.

V1 shall not use an `Idempotency-Key` header or persisted request-result table. Instead, transactional row locking and state-aware behavior shall make retries safe: a repeated action returns the existing valid outcome without creating a duplicate event.

## 12. Query and Storage Optimizations

### LLD-31 Query indexes

The `time_entries` table shall have indexes for `(organization_id, started_at)` and `(user_id, started_at)`. The `time_entry_tags` table shall have a tag-first index to support tag-filtered reports.

### LLD-32 API pagination

API lists shall use `limit`/ `offset` pagination.

## 13. Profile-picture Storage

### LLD-33 Object key and upload validation

The user record shall store only the profile-picture object-storage key, not a provider-specific URL. Uploads shall pass through the Go API for validation before they are stored.

V1 accepts only JPEG, PNG, or WebP images with a maximum size of 5 MB. Replacing a profile picture shall delete the prior stored object after the new upload succeeds.

## 14. CSRF Protection

### LLD-34 Cookie settings and CSRF tokens

Session cookies shall use `Secure`, `HttpOnly`, and `SameSite=Lax` attributes.

The backend shall issue a CSRF token with authenticated user context and require that token in an `X-CSRF-Token` header for state-changing cookie-authenticated requests.



## 15. API Representation

### LLD-35 JSON naming

Public API request and response bodies shall use camelCase JSON field names, such as `startedAt` and `organizationId`. PostgreSQL columns shall remain snake_case.



### LLD-36 Universal response envelope

Every API response shall use the same top-level JSON shape:

```json
{
  "data": {},
  "error": null,
  "meta": {}
}
```

For a successful single-resource response, `data` contains that resource. For a successful list response, `data` contains an array of resources. For a failure, `data` is `null` and `error` contains a stable machine-readable `code`, safe human-readable `message`, and optional structured `details`. Raw Go, SQL, or internal implementation errors shall never be returned to clients.



### LLD-37 Origin and CORS policy

In production, Kubernetes Ingress shall serve the frontend and `/api/v1` from the same HTTPS origin. CORS shall be enabled only for explicitly configured local-development origins.



### LLD-38 Pagination metadata

For paginated list responses, `data` shall contain the resource array and `meta` shall contain `limit`, `offset`, and `total`. For non-paginated responses, `meta` shall be an empty object.



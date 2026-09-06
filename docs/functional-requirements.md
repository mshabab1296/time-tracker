# TimeTracker V1 — Functional Requirements

## 1. Purpose and Scope

TimeTracker is an organization-based application for recording working time and producing timesheets. V1 focuses on time tracking, organization membership, projects, tags, and reports. It is not a task-management, payroll, attendance, or productivity-monitoring product.

## 2. Accounts and Authentication

### FR-1 Account access

1. The system shall let a user register, log in, and log out using an email address and password.
2. An email address shall uniquely identify a user account.
3. The system shall support a forgot-password/password-reset flow.
4. A newly registered user shall verify their email address before using the application.
5. A user shall be able to update their name and profile picture.
6. A user shall not be able to change their email address in V1.
7. A user shall not be able to delete their account in V1.

## 3. Organizations and Membership

### FR-2 Organizations

1. A user shall be able to create an organization.
2. The creator of an organization shall become an Admin of that organization.
3. A user may belong to multiple organizations and shall be able to select an organization context for time tracking and reporting.
4. A user's role is scoped to an organization, not global: the same user may be an Admin in one organization and a Member in another.
5. An Admin shall be able to edit their organization's name and timezone.
6. Organization deletion is out of scope for V1.

### FR-3 Roles and permissions

1. V1 has two organization roles: `ADMIN` and `MEMBER`.
2. An Admin can manage members, projects, tags, and organization reports in their own organization.
3. A Member can record and manage their own time, view their assigned projects, and use organization tags.
4. An Admin can view, edit, and delete time entries for members of their own organization only.
5. Members cannot manage projects, tags, or other members.

### FR-4 Invitations and removal

1. An Admin shall be able to invite a person to their organization by email.
2. If the email belongs to an existing account, the system shall create an in-app invitation. Otherwise, it shall send an email with signup/invitation instructions.
3. An invited person shall be able to accept or decline an invitation.
4. An invitation shall expire seven days after it is issued.
5. An Admin shall be able to resend a pending or expired invitation.
6. An Admin shall be able to cancel a pending invitation.
7. Resending shall refresh the existing invitation; it shall not create a second active invitation for the same organization and email.
8. The system shall reject an invitation when that email is already a member of the organization.
9. Only an Admin can remove a Member. Members and Admins cannot leave an organization in V1, and Admin-transfer is out of scope.
10. Removing a Member removes only their membership; it does not delete the user account or their memberships in other organizations.
11. A removed Member's historical time entries shall remain unchanged and available in the organization's historical reports, but the removed Member shall no longer have organization access.
12. An Admin shall not remove a Member while that Member has a `RUNNING` or `PAUSED` timer. The timer must be stopped first.

## 4. Projects and Tags

### FR-5 Projects

1. Projects are organization-owned categories for recording time; they are not a project-management or task-management feature in V1.
2. An Admin shall be able to create, rename, and delete projects, and assign or remove project assignments for Members.
3. A Member shall only see and select projects assigned to them. An Admin shall be able to view all projects in their organization.
4. Assigning a project gives the Member immediate access; no acceptance flow is required.
5. Every time entry shall have exactly one project, and the user must be assigned to that project when creating the entry.
6. A project shall not be deletable while an existing time entry references it.
7. Deleting an unused project shall also remove all of its assignments.
8. An Admin shall not remove a project assignment while the Member has a `RUNNING` or `PAUSED` timer for that project.
9. Renaming a project shall be reflected everywhere, including historical time entries and reports that reference it.
10. A project name shall be required and unique within its organization, case-insensitively.

### FR-6 Tags

1. Tags belong to an organization and are available to every Member in that organization; tags are not assigned per Member or per Project.
2. An Admin shall be able to create, rename, and delete tags.
3. Members shall be able to select zero or more tags when starting a timer or creating a manual time entry.
4. Tags are optional; a time entry may have multiple tags.
5. A tag shall not be deletable while an existing time entry references it.
6. Renaming a tag shall be reflected everywhere, including historical time entries and reports that reference it.
7. A tag name shall be required and unique within its organization, case-insensitively.

## 5. Time Tracking

### FR-7 Timers

1. A user shall be able to start, pause, resume, and stop a timer.
2. Starting a timer shall create a time entry in the selected organization with one required assigned project and optional tags.
3. A user shall have at most one active timer globally across all organizations. Both `RUNNING` and `PAUSED` timers are active.
4. A timer shall continue to run when the user's browser, application, or device is closed or disconnected.
5. Starting a timer always uses the current server time; a user cannot begin a timer at a past timestamp. Backdated work shall be recorded as a manual entry.
6. Timer statuses in V1 are `RUNNING`, `PAUSED`, and `STOPPED`.
7. A stopped timer becomes a completed time entry. Time spent while paused shall not contribute to its tracked duration.
8. The system shall retain the timer's start, pause, resume, and stop event history for a timer-generated entry.
9. A timer may be paused only while `RUNNING`, resumed only while `PAUSED`, and stopped while either `RUNNING` or `PAUSED`.
10. An active (`RUNNING` or `PAUSED`) timer shall not be deletable.
11. A user may change the project and tags of an active timer in V1, provided the user is assigned to the newly selected project.

### FR-8 Manual entries and entry validation

1. A user shall be able to create a manual time entry for any past date or the current date.
2. A manual entry may be entered as Start + End or Start + Duration; the system shall normalize both forms to one time-entry representation.
3. Manual-entry time values shall use minute-level precision in the user interface.
4. A time entry's start must be before its end, and its duration must be positive.
5. A time entry's start and end times may not be in the future.
6. A user's time entries shall not overlap. Adjacent entries are allowed.
7. This no-overlap rule applies to manual entries and timer-generated entries, including after edits.
8. A time entry may span midnight. When a report is grouped or filtered by day, its duration shall be allocated across the applicable report-day boundaries.

### FR-9 Entry management

1. A Member shall be able to view, edit, and delete their own completed entries.
2. An Admin shall be able to view, edit, and delete completed entries belonging to Members of their organization.
3. For a completed timer-generated entry, the owner and an authorized Admin shall be able to open a detail screen and edit timer event timestamps.
4. They shall be able to add or remove `PAUSE` + `RESUME` event pairs. The `START` and `STOP` events are mandatory.
5. Edited timer events shall remain chronologically ordered and form a valid event sequence. The system shall recalculate duration and revalidate the no-overlap and no-future-time rules.
6. A Member cannot view or manage another Member's entries.
7. The entry owner and an authorized Admin may reassign a completed entry to a different project and add or remove its tags. The entry owner must be assigned to the selected project, and the normal no-overlap and no-future-time rules apply.
8. Deleting a completed time entry shall permanently remove the entry and its dependent timer events and tag links. Its audit record shall remain.

## 6. Reports and Exports

### FR-10 Personal reports

1. A user shall be able to view their own completed time entries and total tracked duration.
2. The user shall be able to filter their personal report by date range, project, and tag.
3. The user shall be able to group a personal summary by any combination and order of Project, Tag, and Date.
4. The user shall be able to download detailed and summary CSV versions of the filtered personal report.
5. When filtering by multiple tags, an entry shall match if it has any selected tag.
6. When grouped by tag, an entry with multiple tags shall appear in each applicable tag group; therefore, tag-group totals are not additive.

### FR-11 Organization reports

1. An Admin shall be able to view timesheets and total tracked duration for all members of their organization.
2. An Admin shall be able to filter the report by date range, Member, Project, and Tag.
3. An Admin shall be able to group a summary by any combination and order of Member, Project, Tag, and Date.
4. Date grouping shall support Day, Week, and Month.
5. Reports shall allow arbitrary start and end dates; the end date is inclusive.
6. Reports shall include completed entries only. `RUNNING` and `PAUSED` timers shall not be included in report totals.
7. The Admin shall be able to download both detailed and summary CSV exports. Detailed CSV includes individual entries; summary CSV aggregates the filtered data using the selected grouping.
8. When filtering by multiple tags, an entry shall match if it has any selected tag.
9. When grouped by tag, an entry with multiple tags shall appear in each applicable tag group; therefore, tag-group totals are not additive.

## 7. Timezones

### FR-12 Timezone behavior

1. Each user shall have a configurable timezone.
2. Manual-entry input, timer/entry display, and personal reports shall use the user's timezone.
3. An organization shall have a configurable timezone selected by its Admin at creation and editable later.
4. Organization reports shall use the organization's current timezone for date boundaries, grouping, and day/week/month calculations.
5. Changing an organization timezone shall not alter stored timestamps. Reports generated afterwards, including for historical data, shall use the current organization timezone; V1 does not retain timezone history.

## 8. Out of Scope for V1

- Organization deletion
- Account deletion and email-address changes
- Admin transfer and self-service organization departure
- Project archival
- Tasks, task assignment, and broader project management
- Payroll, billing, invoices, attendance, screenshots, and general productivity monitoring
- PDF exports
- General notifications beyond invitation delivery and in-app invitations
- Timezone-history preservation






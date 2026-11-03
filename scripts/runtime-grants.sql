-- Run as the migration owner with psql -v application_role=<existing-runtime-role>.
-- The role must be non-superuser, must not own tables, and must not inherit
-- a privileged role. Reapply after migrations introduce new domain tables.
GRANT USAGE ON SCHEMA public TO :"application_role";
REVOKE CREATE ON SCHEMA public FROM :"application_role";
GRANT SELECT, INSERT, UPDATE, DELETE ON
  users, organizations, memberships, sessions, email_action_tokens,
  projects, project_assignments, tags, invitations, time_entries,
  timer_events, time_entry_tags, tickets, time_entry_tickets
  TO :"application_role";
REVOKE ALL ON audit_logs FROM :"application_role";
GRANT SELECT, INSERT ON audit_logs TO :"application_role";

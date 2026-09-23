DROP INDEX time_entries_ticket_started_at_idx;
ALTER TABLE time_entries DROP CONSTRAINT time_entries_ticket_same_organization_fk;
ALTER TABLE time_entries DROP COLUMN ticket_id;
DROP TABLE tickets;

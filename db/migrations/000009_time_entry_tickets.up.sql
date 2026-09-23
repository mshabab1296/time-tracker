-- Preserve links created under the original single-ticket model.
ALTER TABLE time_entries ADD CONSTRAINT time_entries_organization_id_id_unique UNIQUE (organization_id, id);

CREATE TABLE time_entry_tickets (
    organization_id UUID NOT NULL,
    time_entry_id UUID NOT NULL,
    ticket_id UUID NOT NULL,
    PRIMARY KEY (time_entry_id, ticket_id),
    FOREIGN KEY (organization_id, time_entry_id)
        REFERENCES time_entries (organization_id, id) ON DELETE CASCADE,
    FOREIGN KEY (organization_id, ticket_id)
        REFERENCES tickets (organization_id, id) ON DELETE RESTRICT
);
CREATE INDEX time_entry_tickets_ticket_id_idx ON time_entry_tickets (ticket_id, time_entry_id);

INSERT INTO time_entry_tickets (organization_id, time_entry_id, ticket_id)
SELECT organization_id, id, ticket_id FROM time_entries WHERE ticket_id IS NOT NULL;

DROP INDEX time_entries_ticket_started_at_idx;
ALTER TABLE time_entries DROP CONSTRAINT time_entries_ticket_same_organization_fk;
ALTER TABLE time_entries DROP COLUMN ticket_id;

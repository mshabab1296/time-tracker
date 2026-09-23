DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM time_entry_tickets GROUP BY time_entry_id HAVING count(*) > 1) THEN
        RAISE EXCEPTION 'Cannot restore single-ticket model while entries have multiple tickets';
    END IF;
END $$;

ALTER TABLE time_entries ADD COLUMN ticket_id UUID;
UPDATE time_entries te SET ticket_id = tet.ticket_id
FROM time_entry_tickets tet WHERE tet.time_entry_id = te.id;
ALTER TABLE time_entries ADD CONSTRAINT time_entries_ticket_same_organization_fk
    FOREIGN KEY (organization_id, ticket_id)
    REFERENCES tickets (organization_id, id) ON DELETE RESTRICT;
CREATE INDEX time_entries_ticket_started_at_idx
    ON time_entries (ticket_id, started_at DESC) WHERE ticket_id IS NOT NULL;

DROP TABLE time_entry_tickets;
ALTER TABLE time_entries DROP CONSTRAINT time_entries_organization_id_id_unique;

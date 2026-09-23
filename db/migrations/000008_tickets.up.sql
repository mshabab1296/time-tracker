CREATE TABLE tickets (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    created_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reference TEXT NOT NULL,
    title TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT tickets_reference_length CHECK (length(reference) BETWEEN 1 AND 100),
    CONSTRAINT tickets_title_length CHECK (length(title) BETWEEN 1 AND 200),
    CONSTRAINT tickets_organization_id_id_unique UNIQUE (organization_id, id)
);

CREATE UNIQUE INDEX tickets_organization_reference_unique_idx
    ON tickets (organization_id, lower(reference));
CREATE INDEX tickets_organization_created_idx
    ON tickets (organization_id, created_at DESC, id DESC);

ALTER TABLE time_entries ADD COLUMN ticket_id UUID;
ALTER TABLE time_entries ADD CONSTRAINT time_entries_ticket_same_organization_fk
    FOREIGN KEY (organization_id, ticket_id)
    REFERENCES tickets (organization_id, id) ON DELETE RESTRICT;
CREATE INDEX time_entries_ticket_started_at_idx
    ON time_entries (ticket_id, started_at DESC) WHERE ticket_id IS NOT NULL;

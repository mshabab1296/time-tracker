CREATE TABLE time_entries (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    source_type TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    duration_seconds BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT time_entries_source_type_valid CHECK (source_type IN ('MANUAL', 'TIMER')),
    CONSTRAINT time_entries_status_valid CHECK (status IN ('RUNNING', 'PAUSED', 'STOPPED')),
    CONSTRAINT time_entries_end_state_valid CHECK (
        (status = 'STOPPED' AND ended_at IS NOT NULL) OR
        (status IN ('RUNNING', 'PAUSED') AND ended_at IS NULL)
    ),
    CONSTRAINT time_entries_duration_non_negative CHECK (duration_seconds >= 0)
);

CREATE UNIQUE INDEX time_entries_one_active_timer_per_user_idx
    ON time_entries (user_id)
    WHERE status IN ('RUNNING', 'PAUSED');
CREATE INDEX time_entries_organization_started_at_idx
    ON time_entries (organization_id, started_at DESC);
CREATE INDEX time_entries_user_started_at_idx
    ON time_entries (user_id, started_at DESC);

CREATE TABLE timer_events (
    id UUID PRIMARY KEY,
    time_entry_id UUID NOT NULL REFERENCES time_entries(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    sequence_number INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT timer_events_type_valid CHECK (event_type IN ('START', 'PAUSE', 'RESUME', 'STOP')),
    CONSTRAINT timer_events_sequence_positive CHECK (sequence_number > 0),
    CONSTRAINT timer_events_entry_sequence_unique UNIQUE (time_entry_id, sequence_number)
);

CREATE INDEX timer_events_entry_sequence_idx ON timer_events (time_entry_id, sequence_number);

CREATE TABLE time_entry_tags (
    time_entry_id UUID NOT NULL REFERENCES time_entries(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (time_entry_id, tag_id)
);

CREATE INDEX time_entry_tags_tag_id_idx ON time_entry_tags (tag_id);

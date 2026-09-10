CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE time_entries
    ADD CONSTRAINT time_entries_completed_entries_do_not_overlap
    EXCLUDE USING gist (
        user_id WITH =,
        tstzrange(started_at, ended_at, '[)') WITH &&
    )
    WHERE (status = 'STOPPED');

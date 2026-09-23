ALTER TABLE time_entries ADD COLUMN description TEXT;

-- Older entries have no user-provided task description.
UPDATE time_entries SET description = 'Unspecified task';

ALTER TABLE time_entries
    ALTER COLUMN description SET NOT NULL,
    ADD CONSTRAINT time_entries_description_valid
        CHECK (char_length(btrim(description)) BETWEEN 1 AND 200);

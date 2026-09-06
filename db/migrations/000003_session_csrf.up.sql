ALTER TABLE sessions
    ADD COLUMN csrf_token_hash TEXT NOT NULL DEFAULT '';

ALTER TABLE sessions
    ADD CONSTRAINT sessions_csrf_token_hash_not_blank CHECK (csrf_token_hash <> '');

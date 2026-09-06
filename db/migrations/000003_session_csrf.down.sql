ALTER TABLE sessions DROP CONSTRAINT sessions_csrf_token_hash_not_blank;
ALTER TABLE sessions DROP COLUMN csrf_token_hash;

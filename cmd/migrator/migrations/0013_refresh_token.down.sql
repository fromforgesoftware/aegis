DROP TABLE IF EXISTS aegis.refresh_token;
ALTER TABLE aegis.authorization_code DROP COLUMN IF EXISTS session_id;

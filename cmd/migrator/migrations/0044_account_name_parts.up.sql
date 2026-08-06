-- Given and family name, stored separately.
--
-- user_account has carried only display_name, which is fine for rendering "Sanz, Domingo" in a
-- menu and useless for a settings form: a form needs two fields, and splitting a display name on
-- whitespace guesses wrongly for anyone with a compound surname or a name that does not order
-- given-then-family.
--
-- These are also the OIDC standard claim names — `given_name` and `family_name` in OIDC Core 5.1 —
-- so a relying party already knows what they mean, the same reasoning that limits which
-- preferences may map to a claim.
--
-- Nullable, and display_name is kept rather than derived. An account created before this migration
-- has a display name and no parts, and inventing parts by splitting it would put a guess in the
-- database where a person can no longer tell it from something they typed. The API returns the
-- parts when they exist and falls back to display_name when they do not.

ALTER TABLE aegis.user_account ADD COLUMN given_name  TEXT;
ALTER TABLE aegis.user_account ADD COLUMN family_name TEXT;

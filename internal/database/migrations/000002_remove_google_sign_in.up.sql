-- Continue with Google was dropped (November 2026); sign-in is email and password only.
-- A Google-only user would otherwise stay signed in with no credential left.
DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE google_sub IS NOT NULL AND password_hash IS NULL);
DROP TABLE oauth_flows;
ALTER TABLE users DROP COLUMN google_sub;

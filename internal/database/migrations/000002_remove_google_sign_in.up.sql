-- Continue with Google was dropped (November 2026); sign-in is email and password only.
DROP TABLE oauth_flows;
ALTER TABLE users DROP COLUMN google_sub;

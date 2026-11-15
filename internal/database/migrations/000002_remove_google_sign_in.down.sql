ALTER TABLE users ADD COLUMN google_sub TEXT UNIQUE;

CREATE TABLE oauth_flows (
  handle_hash   BYTEA PRIMARY KEY,
  state         TEXT NOT NULL,
  nonce         TEXT NOT NULL,
  pkce_verifier TEXT NOT NULL,
  expires_at    TIMESTAMPTZ NOT NULL,
  consumed_at   TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_oauth_flows_expires ON oauth_flows(expires_at);

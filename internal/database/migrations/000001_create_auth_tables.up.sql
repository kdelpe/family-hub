-- Phase 2 logins (architecture §3.1); users.person_id arrives with persons in Phase 3.

CREATE TABLE users (
  id                  UUID PRIMARY KEY DEFAULT uuidv7(),
  email               TEXT NOT NULL UNIQUE CHECK (email = lower(email)),
  role                TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member')),
  password_hash       TEXT,
  password_changed_at TIMESTAMPTZ,
  google_sub          TEXT UNIQUE,
  disabled_at         TIMESTAMPTZ,
  last_login_at       TIMESTAMPTZ,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((password_hash IS NULL) = (password_changed_at IS NULL))
);

CREATE TABLE auth_tokens (
  id          UUID PRIMARY KEY DEFAULT uuidv7(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  purpose     TEXT NOT NULL CHECK (purpose IN ('invite', 'password_reset')),
  token_hash  BYTEA NOT NULL UNIQUE,
  expires_at  TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_auth_tokens_user ON auth_tokens(user_id, purpose, created_at DESC);
CREATE INDEX idx_auth_tokens_expires ON auth_tokens(expires_at);

CREATE TABLE sessions (
  id_hash      BYTEA PRIMARY KEY,
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at   TIMESTAMPTZ NOT NULL,
  user_agent   TEXT
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

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

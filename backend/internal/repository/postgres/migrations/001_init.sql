-- Initial schema. Written to be safe to re-run (IF NOT EXISTS everywhere); the
-- migration runner additionally records applied versions in schema_migrations.

CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT        NOT NULL,
    email         TEXT        NOT NULL,
    password_hash TEXT        NOT NULL,
    role          TEXT        NOT NULL DEFAULT 'requester'
                  CHECK (role IN ('requester', 'agent', 'admin')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Emails are unique regardless of letter case.
CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_key ON users (lower(email));

CREATE TABLE IF NOT EXISTS tickets (
    id           BIGSERIAL PRIMARY KEY,
    title        TEXT        NOT NULL,
    description  TEXT        NOT NULL,
    category     TEXT        NOT NULL DEFAULT 'general',
    priority     TEXT        NOT NULL DEFAULT 'medium'
                 CHECK (priority IN ('low', 'medium', 'high', 'urgent')),
    status       TEXT        NOT NULL DEFAULT 'open'
                 CHECK (status IN ('open', 'in_progress', 'resolved', 'closed')),
    requester_id BIGINT      NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    assignee_id  BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS tickets_requester_idx  ON tickets (requester_id);
CREATE INDEX IF NOT EXISTS tickets_assignee_idx   ON tickets (assignee_id);
CREATE INDEX IF NOT EXISTS tickets_status_idx     ON tickets (status);
CREATE INDEX IF NOT EXISTS tickets_priority_idx   ON tickets (priority);
CREATE INDEX IF NOT EXISTS tickets_created_at_idx ON tickets (created_at DESC);

CREATE TABLE IF NOT EXISTS comments (
    id          BIGSERIAL PRIMARY KEY,
    ticket_id   BIGINT      NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
    author_id   BIGINT      NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    body        TEXT        NOT NULL,
    is_internal BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS comments_ticket_idx ON comments (ticket_id, created_at);

CREATE TABLE users (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username      varchar(64) NOT NULL,
    password_hash text        NOT NULL,
    role          varchar(16) NOT NULL CHECK (role IN ('admin', 'viewer')),
    active        boolean     NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_username_lower_key ON users (lower(username));

CREATE TABLE sessions (
    id         text        PRIMARY KEY, -- hash SHA-256 del token; el token solo vive en la cookie
    user_id    bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE TABLE audit_events (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id       bigint      REFERENCES users (id) ON DELETE SET NULL,
    action        text        NOT NULL,
    resource_type text,
    resource_id   bigint,
    detail        jsonb,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_created_at_idx ON audit_events (created_at);
CREATE INDEX audit_events_user_id_idx ON audit_events (user_id);
CREATE INDEX audit_events_action_created_at_idx ON audit_events (action, created_at);

CREATE TABLE IF NOT EXISTS mail_deliveries (
    id text PRIMARY KEY CHECK (id ~ '^eml_[0-9a-f]{32}$'),
    client_id text NOT NULL CHECK (client_id ~ '^[a-z][a-z0-9_-]{1,31}$'),
    idempotency_digest bytea NOT NULL CHECK (octet_length(idempotency_digest) = 32),
    request_digest bytea NOT NULL CHECK (octet_length(request_digest) = 32),
    recipient_digest bytea NOT NULL CHECK (octet_length(recipient_digest) = 32),
    template_id text NOT NULL CHECK (template_id ~ '^[a-z][a-z0-9.-]{2,95}$'),
    state text NOT NULL CHECK (state IN ('sending', 'delivered', 'failed')),
    attempt_count integer NOT NULL DEFAULT 1 CHECK (attempt_count BETWEEN 1 AND 20),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    delivered_at timestamptz,
    UNIQUE (client_id, idempotency_digest),
    CHECK ((state = 'delivered') = (delivered_at IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS mail_deliveries_updated_idx ON mail_deliveries (updated_at);

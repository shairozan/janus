-- +goose Up
-- +goose StatementBegin
-- Append-only audit log of every portal action. UUIDv7 keyed (time-ordered).
-- Range-partitioned by month on occurred_at so old months detach/archive cheaply.
-- The PK includes the partition key (occurred_at), as Postgres requires.
-- No FK constraints: audit rows must survive deletion of the referenced actor/org
-- (actor_label is the denormalized fallback) and never block on FK checks.
CREATE TABLE audit_events (
    id UUID NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    organization_id BIGINT,
    actor_org_user_id BIGINT,
    actor_label VARCHAR(255),
    resource VARCHAR(50) NOT NULL,
    action VARCHAR(50) NOT NULL,
    resource_id VARCHAR(255),
    request_method VARCHAR(10),
    request_path VARCHAR(1024),
    result_status INTEGER,
    body JSONB,
    PRIMARY KEY (id, occurred_at)
) PARTITION BY RANGE (occurred_at);

-- DEFAULT partition catches any row not matching a concrete monthly partition,
-- so inserts never fail before a maintenance routine pre-creates month partitions.
CREATE TABLE audit_events_default PARTITION OF audit_events DEFAULT;

-- GIN on the JSONB body for containment queries (e.g. everything touching agreement 42).
CREATE INDEX idx_audit_events_body ON audit_events USING GIN (body jsonb_path_ops);
CREATE INDEX idx_audit_events_org_time ON audit_events (organization_id, occurred_at DESC);
CREATE INDEX idx_audit_events_resource_action ON audit_events (resource, action);
CREATE INDEX idx_audit_events_actor ON audit_events (actor_org_user_id);

COMMENT ON TABLE audit_events IS 'Append-only audit of every portal action (NOT GxP/Part 11). UUIDv7 PK, month-partitioned.';
COMMENT ON COLUMN audit_events.body IS 'Redacted request payload + enrich detail.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_events;
-- +goose StatementEnd

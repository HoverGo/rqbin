CREATE TABLE IF NOT EXISTS bins (
    id         TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS requests (
    id           BIGSERIAL PRIMARY KEY,
    bin_id       TEXT NOT NULL REFERENCES bins(id) ON DELETE CASCADE,
    method       TEXT NOT NULL,
    path         TEXT NOT NULL,
    query_string TEXT NOT NULL DEFAULT '',
    headers      JSONB NOT NULL DEFAULT '{}',
    body         TEXT NOT NULL DEFAULT '',
    remote_addr  TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_requests_bin_created
    ON requests (bin_id, created_at DESC);

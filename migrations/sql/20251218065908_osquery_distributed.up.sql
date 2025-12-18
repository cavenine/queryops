CREATE TABLE IF NOT EXISTS distributed_queries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    query TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS distributed_query_targets (
    id SERIAL PRIMARY KEY,
    query_id UUID NOT NULL REFERENCES distributed_queries(id) ON DELETE CASCADE,
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending', -- pending, sent, completed, failed
    results JSONB,
    error TEXT,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_distributed_query_targets_host_id_status ON distributed_query_targets(host_id, status);

CREATE TABLE IF NOT EXISTS osquery_results (
    id SERIAL PRIMARY KEY,
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    action TEXT NOT NULL,
    columns JSONB NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS osquery_status_logs (
    id SERIAL PRIMARY KEY,
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    line INTEGER,
    message TEXT,
    severity INTEGER,
    filename TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_osquery_results_host_id ON osquery_results(host_id);
CREATE INDEX IF NOT EXISTS idx_osquery_status_logs_host_id ON osquery_status_logs(host_id);

CREATE TABLE IF NOT EXISTS hosts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    host_identifier TEXT UNIQUE NOT NULL,
    node_key TEXT UNIQUE NOT NULL,
    os_version JSONB,
    osquery_info JSONB,
    system_info JSONB,
    platform_info JSONB,
    last_enrollment_at TIMESTAMPTZ DEFAULT NOW(),
    last_config_at TIMESTAMPTZ,
    last_logger_at TIMESTAMPTZ,
    last_distributed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_hosts_node_key ON hosts(node_key);

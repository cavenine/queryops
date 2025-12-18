CREATE TABLE IF NOT EXISTS osquery_configs (
    id SERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    config JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE hosts ADD COLUMN IF NOT EXISTS config_id INTEGER REFERENCES osquery_configs(id);

INSERT INTO osquery_configs (name, config) VALUES ('default', '{
    "schedule": {
        "uptime": {
            "query": "SELECT * FROM uptime;",
            "interval": 60
        },
        "processes": {
            "query": "SELECT * FROM processes;",
            "interval": 300
        }
    }
}'::jsonb)
ON CONFLICT (name) DO UPDATE
SET config = EXCLUDED.config,
    updated_at = CURRENT_TIMESTAMP;

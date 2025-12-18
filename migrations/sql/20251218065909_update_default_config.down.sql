-- No-op or revert to previous simple config if needed
UPDATE osquery_configs 
SET config = '{
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
}'::jsonb
WHERE name = 'default';

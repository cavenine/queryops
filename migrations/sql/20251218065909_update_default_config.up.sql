UPDATE osquery_configs 
SET config = '{
    "options": {
        "pack_delimiter": "/",
        "logger_tls_period": 10,
        "distributed_plugin": "tls",
        "disable_distributed": false,
        "logger_tls_endpoint": "/osquery/logger",
        "distributed_interval": 10,
        "distributed_tls_max_attempts": 3
    },
    "schedule": {
        "uptime": {
            "query": "SELECT * FROM uptime;",
            "interval": 60
        },
        "processes": {
            "query": "SELECT * FROM processes;",
            "interval": 300
        }
    },
    "decorators": {
        "load": [
            "SELECT uuid AS host_uuid FROM system_info;",
            "SELECT hostname AS hostname FROM system_info;"
        ]
    }
}'::jsonb
WHERE name = 'default';

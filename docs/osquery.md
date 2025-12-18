# Osquery Integration in QueryOps

This document describes how osquery is integrated into QueryOps, how the TLS endpoints work, and how to set up a local development environment.

## Overview

QueryOps implements the osquery TLS remote API, allowing it to act as a management server for `osqueryd` agents. It supports:

- **Enrollment**: Securely registering new hosts using an enrollment secret.
- **Configuration**: Serving dynamic JSON configurations to hosts.
- **Logging**: Persisting both status and result logs into PostgreSQL.
- **Distributed Queries**: Executing ad-hoc SQL queries on remote hosts and retrieving results.

## Backend Implementation

The osquery logic is located in `features/osquery/`:

- `handlers.go`: HTTP handlers for the TLS endpoints.
- `routes.go`: Route registration for public and protected endpoints.
- `services/host_repository.go`: Database operations for hosts, logs, and queries.
- `pages/`: Templ components for the Hosts dashboard.

### Database Schema

The integration uses several tables:

1. `hosts`: Stores host identification, node keys, and activity timestamps.
2. `osquery_configs`: Stores JSON configuration profiles.
3. `osquery_results`: Stores scheduled and ad-hoc query results.
4. `osquery_status_logs`: Stores agent internal logs.
5. `distributed_queries`: Queues for ad-hoc queries.
6. `distributed_query_targets`: Tracks query execution status per host.

## Local Development Setup

To test the osquery integration locally, you need `osqueryd` installed and a way to expose your local server to the internet via HTTPS (as osquery requires TLS for remote endpoints).

### 1. Tunneling with Cloudflared

Osquery agents typically require TLS. For local development, you can use `cloudflared` to create a tunnel to your local machine.

1. Install `cloudflared`.
2. Set up a tunnel to point to your local dev server (default `localhost:8080`).
3. Example: `cloudflared tunnel --url http://localhost:8080` (or use a pre-configured domain like `ben.queryops.com`).

### 2. Preparing the Enrollment Secret

Create a file named `osquery.secret` containing the enrollment secret defined in your `.env` or `config/config.go` (default is `enrollment-secret`).

```bash
echo -n "enrollment-secret" > osquery.secret
```

### 3. Running the Osquery Agent

Use the following command to start `osqueryd` and connect it to your local QueryOps instance:

```bash
osqueryd \
  --tls_hostname=your-tunnel-domain.com \
  --host_identifier=uuid \
  --enroll_secret_path=osquery.secret \
  --enroll_always \
  --enroll_tls_endpoint=/osquery/enroll \
  --config_plugin=tls \
  --config_tls_endpoint=/osquery/config \
  --config_refresh=10 \
  --logger_plugin=tls \
  --logger_tls_endpoint=/osquery/logger \
  --logger_tls_period=10 \
  --disable_distributed=false \
  --distributed_plugin=tls \
  --distributed_interval=10 \
  --distributed_tls_read_endpoint=/osquery/distributed_read \
  --distributed_tls_write_endpoint=/osquery/distributed_write \
  --pidfile=osquery.pid \
  --database_path=osquery.db \
  --verbose
```

## Managing Hosts in the UI

Once the agent is running and enrolled:

1. Log in to the QueryOps dashboard.
2. Navigate to the **Hosts** section in the sidebar.
3. You should see your host listed with its current status.
4. Use the **Query** button to run ad-hoc SQL on the host.
5. Click **Details** to see the host's metadata and query results.

## Dynamic Configuration

QueryOps supports dynamic configurations. You can modify the `default` config in the `osquery_configs` table to change how agents behave (e.g., adding new scheduled queries or changing intervals).

Example configuration structure:
```json
{
  "options": {
    "logger_tls_period": 10,
    "distributed_interval": 10
  },
  "schedule": {
    "uptime": {
      "query": "SELECT * FROM uptime;",
      "interval": 60
    }
  },
  "decorators": {
    "load": [
      "SELECT uuid AS host_uuid FROM system_info;",
      "SELECT hostname AS hostname FROM system_info;"
    ]
  }
}
```

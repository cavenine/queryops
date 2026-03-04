# QUERYOPS

# Stack

- [Go](https://go.dev/doc/)
- [Datastar](https://github.com/starfederation/datastar)
- [Templ](https://templ.guide/)
  - [Tailwind](https://tailwindcss.com/) x [DaisyUI](https://daisyui.com/)

# Setup

1. Install Dependencies

```shell
go mod tidy
```

2. Create 🚀

# Development

Live Reload is set up out of the box - powered by [Air](https://github.com/air-verse/air) + [esbuild](cmd/web/build/main.go)

Use the [live task](./Taskfile.yml#L113) from the [Taskfile](https://taskfile.dev/) to start with live reload setup

```shell
go tool task live
```

Navigate to [`http://localhost:8080`](http://localhost:8080) in your favorite web browser to begin

# Starting the Server

```shell
go tool task run
```

Navigate to [`http://localhost:8080`](http://localhost:8080) in your favorite web browser

# PostgreSQL Configuration

`DATABASE_URL` is required for app and migration commands. Optional tuning env vars:

- `DATABASE_MIN_CONNS` and `DATABASE_MAX_CONNS`: pgx pool bounds.
- `DATABASE_MAX_CONN_IDLE_MS`: max idle connection lifetime in milliseconds.
- `DATABASE_MAX_CONN_LIFE_MS`: max total connection lifetime in milliseconds.
- `DATABASE_STATEMENT_TIMEOUT_MS`: per-statement timeout in milliseconds (default `15000`).
- `DATABASE_IDLE_IN_TX_TIMEOUT_MS`: idle-in-transaction timeout in milliseconds (default `10000`).
- `DATABASE_APP_NAME`: PostgreSQL `application_name` (default `queryops`).

Compatibility notes:

- `DATABASE_MAX_IDLE` is deprecated and only used as a fallback for `DATABASE_MAX_CONN_IDLE_MS`.
- `DATABASE_MAX_LIFE_MS` remains supported as fallback for `DATABASE_MAX_CONN_LIFE_MS`.

User emails are enforced as case-insensitive unique at the database level.

# Deployment

## Building an Executable

The `task build` [task](./Taskfile.yml#L37) will assemble and build a binary

[Dockerfile](./Dockerfile)

# Osquery Integration

QueryOps provides a full osquery TLS management backend.

- [🔗 Osquery Integration Guide](./docs/osquery.md)
- Remote enrollment, configuration, and logging
- Distributed "Live" queries from the dashboard

# References

## Server

- [go](https://go.dev/)
- [datastar sdk](https://github.com/starfederation/datastar/tree/develop/sdk)
- [templ](https://templ.guide/)

## Web Components x Datastar

[🔗 Vanilla Web Components Setup](./web/libs/web-components/README.md)

[🔗 Lit Web Components Setup](./web/libs/lit/README.md)

## Client

- [datastar](https://www.jsdelivr.com/package/gh/starfederation/datastar)
- [tailwindcss](https://tailwindcss.com/)
- [daisyui](https://daisyui.com/)
- [esbuild](https://esbuild.github.io/)
- [lit](https://lit.dev/)

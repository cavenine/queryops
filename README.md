# QUERYOPS

# Stack

- [Go](https://go.dev/doc/)
- [NATS](https://docs.nats.io/)
- [Datastar](https://github.com/starfederation/datastar)
- [Templ](https://templ.guide/)
  - [Tailwind](https://tailwindcss.com/) x [DaisyUI](https://daisyui.com/)

# Setup

1. Clone this repository

```shell
git clone https://github.com/yourusername/queryops.git
```

2. Install Dependencies

```shell
go mod tidy
```

3. Create 🚀

# Development

Live Reload is setup out of the box - powered by [Air](https://github.com/air-verse/air) + [esbuild](cmd/web/build/main.go)

Use the [live task](./Taskfile.yml#L76) from the [Taskfile](https://taskfile.dev/) to start with live reload setup

```shell
go tool task live
```

Navigate to [`http://localhost:8080`](http://localhost:8080) in your favorite web browser to begin

## Debugging

The [debug task](./Taskfile.yml#L42) will launch [delve](https://github.com/go-delve/delve) to begin a debugging session with your project's binary

```shell
go tool task debug
```

## IDE Support

- [Templ / TailwindCSS Support](https://templ.guide/commands-and-tools/ide-support)

### Visual Studio Code Integration

[Reference](https://code.visualstudio.com/docs/languages/go)

- [launch.json](./.vscode/launch.json)
- [settings.json](./.vscode/settings.json)

a `Debug Main` configuration has been added to the [launch.json](./.vscode/launch.json)

# Starting the Server

```shell
go tool task run
```

Navigate to [`http://localhost:8080`](http://localhost:8080) in your favorite web browser

# Deployment

## Building an Executable

The `task build` [task](./Taskfile.yml#L33) will assemble and build a binary

## Docker

```shell
# build an image
docker build -t queryops:latest .

# run the image in a container (defaults to "web" subcommand)
docker run --name queryops -p 8080:8080 queryops:latest

# run a one-off migration container
docker run --rm -e DATABASE_URL="$DATABASE_URL" queryops:latest migrate up
```

[Dockerfile](./Dockerfile)

### GitHub Container Registry (GHCR)

Image name:
- `ghcr.io/cavenine/queryops`

Local build + push (mutable tags for convenience):

```shell
# PAT (classic) recommended scopes: write:packages
export GHCR_TOKEN=YOUR_TOKEN

# optionally override (defaults in Taskfile.yml)
export GHCR_USERNAME=cavenine

# pushes ghcr.io/cavenine/queryops:latest and :dev-<git-sha>
go tool task ghcr:publish
```

CI publish (immutable, release-tagged):
- Create/publish a GitHub Release with a tag like `v1.2.3`.
- The workflow publishes `ghcr.io/cavenine/queryops:v1.2.3`.

Private image pulls (e.g. DigitalOcean VM / Kamal):
- Use a PAT (classic) with `read:packages`.
- If you also want Kamal to build+push images, use `write:packages`.

## Kamal (DigitalOcean single VM)

This repo is set up to deploy with Kamal to a single DigitalOcean droplet running:
- `web` container (behind `kamal-proxy`)
- `worker` container (River workers)
- `postgres` accessory container

### 1) Prerequisites

On your deploy machine (laptop or CI runner):
- `kamal` installed (`gem install kamal`)
- Docker installed (Kamal builds/pushes images)
- SSH access to the droplet (Kamal uses SSH)

### 2) Provision the droplet

Recommended:
- Ubuntu 24.04 LTS
- Add your SSH key
- Open inbound ports: `22`, `80`, `443`

### 3) DNS

Create an `A` record for your app hostname pointing at the droplet IP, e.g.:
- `queryops.example.com -> <droplet-ip>`

### 4) Configure Kamal

Edit `config/deploy.yml`:
- Confirm `image` (defaults to `ghcr.io/cavenine/queryops`)
- Replace `1.2.3.4` with your droplet IP
- Replace `queryops.example.com` with your real hostname
- Configure registry username/password

Healthcheck:
- Kamal proxy defaults to `GET /up` and this app implements `/up`.

### 5) Secrets

Create `.kamal/secrets` (do not commit it). Example:

```bash
# Registry
KAMAL_REGISTRY_PASSWORD=...           # ghcr/dockerhub token

# Postgres (accessory)
POSTGRES_PASSWORD=$(openssl rand -hex 32)

# App
SESSION_SECRET=$(openssl rand -hex 32)
WEBAUTHN_RP_ID=queryops.example.com
WEBAUTHN_RP_ORIGIN=https://queryops.example.com
WEBAUTHN_RP_DISPLAY_NAME=QueryOps

# Use the postgres accessory hostname (see config/deploy.yml: accessories.postgres.service)
DATABASE_URL=postgres://queryops:${POSTGRES_PASSWORD}@postgres:5432/queryops?sslmode=disable
```

### 6) First-time setup

```shell
# installs docker, boots proxy, builds/pushes image, and boots the app
kamal setup

# boot postgres (accessories are managed separately)
kamal accessory boot postgres

# deploy latest
kamal deploy
```

### 7) Migrations

Migrations run automatically during deploy via `.kamal/hooks/pre-app-boot` using:
- `/main migrate up` (runs app migrations + River migrations)

The deploy config sets `AUTO_MIGRATE=false` to avoid race conditions during boot.

### 8) Useful commands

```shell
# logs
kamal app logs
kamal app logs --primary

# run commands inside a container on the primary host
kamal app exec --primary '/main migrate version'

# maintenance mode
kamal app maintenance --message "Deploying"
kamal app live

# rollback
kamal rollback
```

# Contributing

Completely open to PR's and feature requests

# References

## Server

- [go](https://go.dev/)
- [nats](https://docs.nats.io/)
- [datastar sdk](https://github.com/starfederation/datastar/tree/develop/sdk)
- [templ](https://templ.guide/)

### Embedded NATS

The NATS server that powers the `TODO` application is [embedded into the web server](./cmd/web/main.go#L60)

To interface with it, you should install the [nats-cli](https://github.com/nats-io/natscli)

Here are some commands to inspect and make changes to the bucket backing the `TODO` app:

```shell
# list key value buckets
nats kv ls

# list keys in the `todos` bucket
nats kv ls todos

# get the value for [key]
nats kv get --raw todos [key]

# put a value into [key]
nats kv put todos [key] '{"todos":[{"text":"Hello, NATS!","completed":true}],"editingIdx":-1,"mode":0}'
```

## Web Components x Datastar

[🔗 Vanilla Web Components Setup](./web/libs/web-components/README.md)

[🔗 Lit Web Components Setup](./web/libs/lit/README.md)

## Client

- [datastar](https://www.jsdelivr.com/package/gh/starfederation/datastar)
- [tailwindcss](https://tailwindcss.com/)
- [daisyui](https://daisyui.com/)
- [esbuild](https://esbuild.github.io/)
- [lit](https://lit.dev/)

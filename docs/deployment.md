### GitHub Container Registry (GHCR)

Image name:
- `ghcr.io/<owner>/<repo>` (commonly mirrors GitHub repo name)

Local build + push (mutable tags for convenience):

```shell
# PAT (classic) recommended scopes: write:packages
export GHCR_TOKEN=YOUR_TOKEN

# Required variables (Taskfile.yml also loads .env if present)
export GHCR_USERNAME=<github-username>
export GHCR_IMAGE=ghcr.io/<owner>/<repo>

# pushes :latest and :dev-<git-sha>
go tool task ghcr:publish
```

CI publish (immutable, release-tagged):
- Create/publish a GitHub Release with a tag like `v1.2.3`.
- The workflow publishes `ghcr.io/<owner>/<repo>:v1.2.3`.

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
- Replace `1.2.3.4` with your droplet IP
- Replace `queryops.example.com` with your real hostname

This config expects image settings via env vars at runtime:
- `KAMAL_IMAGE` (example: `ghcr.io/<owner>/<repo>`)
- `KAMAL_REGISTRY_USERNAME` (the GitHub username that owns the PAT used for pulls)

Example:

```shell
export KAMAL_IMAGE=ghcr.io/<owner>/<repo>
export KAMAL_REGISTRY_USERNAME=<github-username>
```

Healthcheck:
- Kamal proxy defaults to `GET /up` and this app implements `/up`.

### 5) Secrets

Create `.kamal/secrets` (do not commit it). Example:

```bash
# Registry (for private GHCR pulls)
# PAT (classic) scope: read:packages
KAMAL_REGISTRY_PASSWORD=...

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
# installs docker, boots proxy, and boots the app
kamal setup

# boot postgres (accessories are managed separately)
kamal accessory boot postgres

# deploy a specific published release tag
kamal deploy -v v1.2.3
```

Note: if you prefer not to export env vars globally, prefix commands instead:

```shell
KAMAL_IMAGE=ghcr.io/<owner>/<repo> KAMAL_REGISTRY_USERNAME=<github-username> kamal deploy -v v1.2.3
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

# Pal-pal

Pal-pal is a small, server-rendered web interface for the [Palworld Dedicated Server REST API](https://docs.palworldgame.com/category/rest-api/).

It keeps the upstream API credentials on the server, exposes a read-only view when configured to do so, and gives authenticated administrators access to sensitive information and player controls.

## Features

- Live server status and activity
- Player list with admin-only IP addresses, kick, ban, and unban controls
- Relative coordinate plot for online player locations
- SQLite-backed metrics history with Go-emitted SVG graphs
- Optional world snapshots with in-game time and aggregate actor counts
- Admin-only full server settings
- In-memory cookie sessions with CSRF-protected administrative actions
- Server-rendered HTML with HTMX component refreshes

## Stack

- Go
- Native `http.ServeMux` routing
- `html/template`
- HTMX
- SQLite via `modernc.org/sqlite` (no cgo)
- Cobra

## Architecture

```text
cmd/pal-pal          Cobra CLI and process lifecycle
internal/config      Environment configuration
internal/palworld    Typed Palworld REST client
internal/monitor     Live snapshot polling and metric recording
internal/store       SQLite migrations and metric history
internal/web         ServeMux routes, sessions, templates, and assets
```

The monitor polls `/info`, `/players`, and `/metrics`. Successful metric samples
are recorded independently, so a temporary failure from one endpoint does not
discard previously healthy snapshot data. When enabled, `/game-data` runs on its
own slower schedule and its failures do not affect the core server status.

## Access model

Pal-pal has three access levels:

- Anonymous viewer: enabled when `PUBLIC_READ=true`
- Authenticated viewer: signs in with `PUBLIC_PASSWORD`
- Administrator: signs in with `ADMIN_PASSWORD`

Viewer responses exclude player IP addresses. Full server settings and all player mutations are restricted to administrators. Sessions are stored in memory and therefore reset when Pal-pal restarts.

The Palworld REST API is not intended to be exposed directly to the internet. Keep it on a trusted network and expose only Pal-pal through an authenticated HTTPS reverse proxy when remote access is needed.

## Configuration

Copy `.env.example` to `.env` and set:

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `API_ROOT` | yes | | Palworld server origin or full REST base URL |
| `API_PASSWORD` | yes | | Palworld REST API/admin password |
| `PUBLIC_READ` | no | `false` | Allow anonymous read-only pages |
| `PUBLIC_PASSWORD` | no | empty | Optional shared viewer login |
| `ADMIN_PASSWORD` | yes | | Pal-pal administrator login |
| `DATA_PATH` | no | `data` | Data directory or explicit SQLite `.db` path |
| `LISTEN_ADDRESS` | no | `:8080` | HTTP listen address |
| `POLL_INTERVAL` | no | `15s` | Live polling and metric sample interval |
| `HISTORY_RETENTION` | no | `720h` | Metric retention duration |
| `GAME_DATA_ENABLED` | no | `false` | Poll the optional `/game-data` world snapshot endpoint |
| `GAME_DATA_POLL_INTERVAL` | no | `1m` | World snapshot interval; must be at least 15 seconds |
| `SECURE_COOKIES` | no | `false` | Mark session cookies HTTPS-only |

Pal-pal automatically loads `.env` if present. Existing process environment variables take precedence.
When `API_ROOT` contains no path, Pal-pal automatically appends the documented `/v1/api` prefix.

World snapshots are disabled by default because `/game-data` must be enabled on
the Palworld server and may return a much larger response than the core
endpoints. When enabled, Pal-pal keeps only the fields needed for aggregate
actor counts, world time, and snapshot FPS; it does not retain actor identifiers,
IP addresses, guild names, or locations.

## Run

With Devbox:

```sh
devbox run test
devbox run run
```

Or with a compatible Go toolchain:

```sh
go test ./...
go run ./cmd/pal-pal serve
```

Then open [http://localhost:8080](http://localhost:8080).

## Container

The two-stage Docker build produces a static Go binary and runs it as the
distroless `nonroot` user. The image contains no shell or package manager.

```sh
docker build -t pal-pal .
docker run --rm \
  --env-file .env \
  -p 8080:8080 \
  -v pal-pal-data:/data \
  pal-pal
```

`DATA_PATH` defaults to `/data` in the container. For HTTPS deployments,
terminate TLS at a reverse proxy and set `SECURE_COOKIES=true`.

## Automation

GitHub Actions runs module consistency checks, `go vet`, race-enabled tests,
a static build, and a complete container build on pull requests and pushes to
`main`. Renovate tracks Go modules, GitHub Actions, and container base images;
its best-practices preset pins Actions and Docker images to immutable digests.

## Scaffold decisions and open work

The original brief did not specify several implementation details. This scaffold uses:

- A 15-second poll interval and 30-day metric retention
- Go-emitted SVG graphs rather than a browser charting dependency
- Field-level read restrictions in addition to admin-only routes
- A relative player coordinate plot with no external map asset
- One shared viewer password and one shared admin password
- In-memory 12-hour sessions

Before treating the map as geographical, select a Palworld map asset, confirm its license, and calibrate the world-coordinate bounds. Production deployments should also decide whether sessions need persistent multi-user identities, audit logging, and rate limiting at the reverse proxy.

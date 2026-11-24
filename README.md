# Bastion

**Bastion** is a self-hosted reverse proxy and security control panel. It combines TLS termination, automatic Let's Encrypt certificates, GeoIP filtering, CrowdSec integration, IP allow/deny lists, per-site Basic Auth, rate limiting and response caching behind a modern web UI.

## Features

- Reverse proxy with TLS termination (SNI certificates)
- Let's Encrypt (HTTP-01) with automatic renewal + manual certificate upload
- Multi-domain sites with per-site templates and inherit/override settings
- GeoIP filtering (DB-IP Lite) per site/template, with country multi-select
- CrowdSec integration (bouncer, alerts, good actors / whitelist)
- IP allow/deny lists (global + per site)
- Per-site Basic Auth with user management and 3-failures/5-minutes lockout
- HTTP/2 and HTTP/3 (QUIC)
- Rate limiting (token bucket, per site + client IP)
- Response cache with per-site TTL and size limits
- Multi-tenancy + RBAC (superadmin, tenant_admin, editor, viewer)
- Persistent access logs (JSONL) + audit log (logins and changes)
- Dashboard with live system metrics (CPU, RAM, disk), traffic (24h), top sites and blocked countries
- systemd service, single binary with embedded frontend
- Update check against GitHub releases

## Quick install

```sh
curl -fsSL https://raw.githubusercontent.com/Inboxes-CIG/Bastion/main/get.sh | sudo bash
```

## Manual install

1. Download the latest release tarball for your architecture (`linux_amd64` or `linux_arm64`).
2. Extract it and run:

```sh
sudo ./install.sh
```

The installer sets up PostgreSQL, the `bastion` service user, `/etc/bastion/config.yaml`, the systemd unit, CrowdSec and the GeoIP update cron job.

## Access

- GUI: `https://<server-ip>/` (accept the self-signed certificate first)
- API: `http://<server-ip>:8080/api/v1`
- Initial admin: `admin@waf.local` — password printed once in `journalctl -u bastion`

Change the admin password after the first login.

## Configuration

`/etc/bastion/config.yaml` — database, JWT secret, CrowdSec LAPI, GeoIP path, logging and cache defaults.

The GUI can issue a Let's Encrypt certificate for the admin domain itself (Settings → GUI certificate).

## Update check

The dashboard and footer show the installed version and whether a newer GitHub release is available. The check is cached for 24 hours and uses the public GitHub API.

## Build from source

Requirements: Go 1.26+, Node 22+, PostgreSQL.

```sh
cd web && npm ci && npm run build && cd ..
rm -rf internal/web/dist && cp -r web/dist internal/web/dist
go build -ldflags "-X github.com/inboxes-cig/bastion/internal/version.Version=$(cat VERSION)" -o bastion ./cmd/bastion
```

## Architecture

- Backend: Go 1.26 (Gin, GORM, PostgreSQL)
- Frontend: React + Vite + Tailwind (embedded in the binary)
- Proxy: net/http ReverseProxy with TLS SNI and HTTP/3 (quic-go)
- Certificates: certmagic (Let's Encrypt) + manual upload
- GeoIP: DB-IP Lite (MMDB)
- Security: CrowdSec bouncer, IP lists, per-site Basic Auth, rate limiting

## License

MIT — see [LICENSE](LICENSE).

# Bastion

[![Release](https://img.shields.io/github/v/release/Inboxes-CIG/Bastion)](https://github.com/Inboxes-CIG/Bastion/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Bastion** is a self-hosted reverse proxy and security control panel. It combines TLS termination, automatic Let's Encrypt certificates, GeoIP filtering, CrowdSec integration, IP allow/deny lists, per-site Basic Auth, optional TOTP two-factor authentication for the GUI, rate limiting and HTTP response caching behind a modern web UI.

## Documentation

Full documentation is available in German and English in the [Wiki](https://github.com/Inboxes-CIG/Bastion/wiki):

- [Deutsch](https://github.com/Inboxes-CIG/Bastion/wiki/DE-Start)
- [English](https://github.com/Inboxes-CIG/Bastion/wiki/EN-Home)

## Features

- Reverse proxy with TLS termination (SNI certificates)
- Let's Encrypt (HTTP-01) with automatic renewal + manual certificate upload
- Multi-domain sites with per-site templates and inherit/override settings
- GeoIP filtering (DB-IP Lite) per site/template, with country multi-select
- CrowdSec integration (bouncer, alerts, good actors / whitelist)
- IP allow/deny lists (global + per site)
- Per-site Basic Auth with user management and 3-failures/5-minutes lockout
- Path-based locations with their own upstream, auth users, IP rules, cache, rate limit and GeoIP settings
- Optional TOTP MFA for the GUI (Google Authenticator, Authy, 1Password, …)
- Automatic 404 flood ban (blocklist entry with note `404`)
- Bilingual GUI (German/English, selectable in Settings)
- HTTP/2 and HTTP/3 (QUIC)
- Rate limiting (token bucket, per site + client IP)
- HTTP response cache with per-site limits; honors backend Cache-Control (s-maxage/max-age), Expires, ETag/304, Vary and Age, with configurable TTL and size limits
- Optional TLS verification skip for self-signed HTTPS upstreams
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

## Upgrade

Re-run the installer. It replaces the binary and keeps `/etc/bastion`, `/var/lib/bastion` and the existing database:

```sh
curl -fsSL https://raw.githubusercontent.com/Inboxes-CIG/Bastion/main/get.sh | sudo bash
```

## Access

- GUI: `https://<server-ip>/` (accept the self-signed certificate first)
- API: `http://<server-ip>:8080/api/v1`
- Initial admin: `admin@bastion.local` — password printed once in `journalctl -u bastion`

Change the admin password after the first login.

## Two-factor authentication (optional)

Every user can enable TOTP under **Settings → Zwei-Faktor-Authentifizierung (MFA)**. Scan the QR code with any standard authenticator app and confirm with the generated 6-digit code. The secret is stored encrypted (AES-256-GCM) and login then requires password **and** code.

There are no recovery codes yet. If the authenticator device is lost, an administrator must reset MFA directly in the database:

```sql
UPDATE users SET mfa_enabled = false, mfa_secret = '' WHERE email = '<user-email>';
```

## Response cache

Bastion caches anonymous `GET` responses in memory. Requests with an `Authorization` header or `Cache-Control: no-cache` bypass the cache entirely. Freshness is taken from the backend:

- `Cache-Control: s-maxage=N` (preferred) or `max-age=N`
- `Expires`
- `Age` is subtracted from the freshness lifetime
- `ETag` / `Last-Modified` answer conditional requests with `304`

Responses with `Cache-Control: no-store`, `no-cache` or `private`, a `Set-Cookie` header, or a `Vary` header other than `Accept-Encoding` are never stored. The per-site TTL is only a fallback when the backend sends no freshness information. Cache entries live in memory and are cleared on restart.

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
- Security: CrowdSec bouncer, IP lists, per-site Basic Auth, TOTP MFA, rate limiting

## License

MIT — see [LICENSE](LICENSE).

# Configuration & operations

## config.yaml

Path: `/etc/bastion/config.yaml` (root:`bastion`, 0640)

```yaml
server:
  http_addr: ":80"
  https_addr: ":443"
  api_addr: "127.0.0.1:8080"  # API bind; default is loopback-only (empty = 127.0.0.1:8080)
  admin_hosts: []            # e.g. ["bastion.example.com"]; empty = localhost (IPs are always admin hosts)
  cors_origins: []           # allow CORS only for these origins (default: none)
  http3: true

database:
  url: "postgres://waf:PASSWORD@localhost:5432/waf?sslmode=disable"

auth:
  jwt_secret: "…"            # signs access/refresh tokens
  session_secret: "…"        # encrypts MFA secrets (AES-256-GCM)

certs:
  storage_dir: "/var/lib/bastion/certs"

geoip:
  db_path: "/var/lib/bastion/geoip/dbip-country-lite.mmdb"

crowdsec:
  enabled: true
  lapi_url: "http://127.0.0.1:8081"
  api_key: "…"
  refresh_seconds: 30

logging:
  dir: "/var/lib/bastion/logs"
  max_memory: 10000
  retention_days: 30

cache:
  enabled: true
  ttl_seconds: 60            # fallback when the backend sends no freshness
  max_entries: 500
  max_object_kb: 256
  max_total_mb: 64

security:
  upstream:
    allow_private: false      # allow private/loopback upstreams globally (unsafe)
    allowed_hosts: []         # optional/global; prefer per-tenant management (GUI → Tenants → upstream allowlist)
  auto_ban_404:
    enabled: true
    threshold: 50
    window_seconds: 600
    first_ban: 10m            # stage 1
    second_ban: 24h           # stage 2, permanent afterwards
```

### Notes

- **`server.api_addr`** binds the API to `127.0.0.1:8080` by default – unreachable from outside. The GUI uses `/api/` through the admin host on 443.
- **`server.admin_hosts`**: if empty, `localhost` applies; **every IP address** is also an admin host automatically.
- **`server.cors_origins`**: an empty list allows no cross-origin requests; the GUI is same-origin.
- **`cache.json`** (`/var/lib/bastion/cache.json`) overrides the cache values from `config.yaml` once you change them in the GUI.
- **`auth.jwt_secret`** signs all tokens; **`auth.session_secret`** encrypts MFA secrets. Changing `jwt_secret` logs everyone out; changing `session_secret` makes newly created MFA secrets unreadable (older secrets encrypted with `jwt_secret` remain readable).
- **Secrets** are also stored in `/etc/bastion/.secrets.env` (0600) by the installer.

## systemd

```ini
[Service]
User=bastion
Group=bastion
Environment=GIN_MODE=release
ExecStart=/opt/bastion/bin/bastion
Restart=on-failure
RestartSec=5
AmbientCapabilities=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/bastion /etc/bastion
```

```sh
systemctl status bastion
systemctl restart bastion
systemctl stop bastion
journalctl -u bastion -f
journalctl -u bastion --since "1 hour ago"
```

## Health checks

```sh
curl -s http://127.0.0.1:8080/healthz
# {"status":"ok"}

curl -sk https://<admin-host>/healthz
```

## Backup

Back up:

1. PostgreSQL database `waf`
2. `/etc/bastion` (config + secrets)
3. `/var/lib/bastion/certs` (certificates + ACME)
4. Optional: `/var/lib/bastion/geoip` and `/var/lib/bastion/logs`

```sh
sudo -u postgres pg_dump -Fc waf > /root/bastion-db-$(date +%F).dump
sudo tar -czf /root/bastion-files-$(date +%F).tar.gz /etc/bastion /var/lib/bastion/certs
```

Restore:

```sh
sudo systemctl stop bastion
sudo -u postgres pg_restore -d waf --clean --if-exists /root/bastion-db-YYYY-MM-DD.dump
sudo tar -xzf /root/bastion-files-YYYY-MM-DD.tar.gz -C /
sudo chown -R bastion:bastion /var/lib/bastion
sudo systemctl start bastion
```

## Upgrade

```sh
curl -fsSL https://raw.githubusercontent.com/Inboxes-CIG/Bastion/main/get.sh | sudo bash
```

Or copy the new binary to `/opt/bastion/bin/bastion` and `systemctl restart bastion`.

The database is migrated automatically on startup (`AutoMigrate`).

## Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `502` with `unknown_site` | `Host` matches no site/domain |
| `403` despite a valid URL | Site disabled, IP deny, CrowdSec, GeoIP or auto-ban |
| `429` | Rate limit or Basic Auth lockout (check `Retry-After`) |
| `401` | Basic Auth enabled but missing/invalid credentials |
| GUI login returns `429` | Lockout after 3 failed attempts (IP **and** email, 10 minutes); wait or change client IP |
| GUI shows `password_change_required` | The account must change its password first (mandatory dialog after login) |
| Creating a site fails: `upstream: …private…` | Upstream target is private/loopback; add host/CIDR to `security.upstream.allowed_hosts` or the tenant `allowed_upstream_hosts` |
| `502` with `block_reason=ssrf` | Upstream resolves to a private IP later on or is no longer on the allowlist |
| Error card inside the page area only | Individual GUI pages have their own error boundary: "Try again" or switch pages – no full reload needed |
| Wrong certificate | Domain not in SAN; re-issue |
| GUI unreachable | Firewall 443, admin host/IP, service status |
| GeoIP has no effect | MMDB missing/not loaded; `journalctl -u bastion | grep -i geoip` |
| CrowdSec “disconnected” | Check LAPI/key: `cscli bouncers list`, `journalctl -u crowdsec` |
| Cache does not work | Backend sends `no-store`/`private`/`Set-Cookie`/`Vary`; check TTL/enabled |
| Auto-ban too aggressive | Raise `security.auto_ban_404.threshold` or set `enabled: false` |
| `Text file busy` on upgrade | Stop the service before copying |
| Locked out of the admin UI | Use an IP/localhost; admin hosts are exempt from filters |
| White page after reload | Hard-reload once; the server now serves missing hashed assets as 404 and index.html as no-cache |

## Security

- The API binds to `127.0.0.1:8080` by default – not public; the GUI runs on the admin host over 443
- Serve the GUI only over HTTPS and preferably a dedicated admin domain (`admin_hosts`); admin HTTP always answers `301`
- Enable MFA for all GUI users; disabling it requires the password
- Login lockout (3 attempts/10 min per IP+email), refresh rotation with theft detection, re-auth for sensitive user changes
- GUI access control (dedicated IP lists + GeoIP) under Settings → GUI-Security; loopback is always allowed
- Upstreams are SSRF-validated: private targets only via the tenant allowlist (GUI → Tenants → upstream allowlist) or the optional global `security.upstream.allowed_hosts`; the check re-runs live on every request (cached DNS re-resolution) and re-reads the tenant list every 60 s, so tampered DB rows or DNS rebinding cannot slip through
- Unknown SNI names deliberately receive a self-signed default certificate (keeps the admin GUI reachable by IP)
- Back up DB, config and certificates regularly
- Keep `jwt_secret`, `session_secret` and the DB password secret
- Firewall: only 80/443 (TCP) and 443/UDP (HTTP/3)
- Behind a CDN/proxy: only trust `X-Forwarded-For` from trusted sources (Bastion trusts the header)

## Known limitations (v1.0.6)

- Some GUI options are not enforced yet: HSTS, compression, max body size, custom headers, HTTP/2 and WebSocket switches. Details: [Sites & templates](https://github.com/Inboxes-CIG/Bastion/wiki/EN-Sites-and-Templates).
- Access tokens (15 min) simply expire and cannot be individually revoked until then; refresh tokens are rotated and blacklisted server-side.
- Cache is not persistent across restarts.
- No automatic cache invalidation when deleting/changing a site (locations do clear the cache).
- MFA has no recovery codes.

Next: [API reference](https://github.com/Inboxes-CIG/Bastion/wiki/EN-API)

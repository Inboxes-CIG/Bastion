# Configuration & operations

## config.yaml

Path: `/etc/bastion/config.yaml` (root:`bastion`, 0640)

```yaml
server:
  http_addr: ":80"
  https_addr: ":443"
  api_addr: ":8080"          # API (HTTP, keep internal!)
  admin_hosts: []            # e.g. ["bastion.example.com"]; empty = waf.inboxes.de + localhost
  http3: true

database:
  url: "postgres://waf:PASSWORD@localhost:5432/waf?sslmode=disable"

auth:
  jwt_secret: "…"            # also the key for MFA secret encryption
  session_secret: "…"

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
  auto_ban_404:
    enabled: true
    threshold: 50
    window_seconds: 600
```

### Notes

- **`server.api_addr`** is HTTP and should only be reachable internally. The GUI uses `/api/` through the admin host on 443.
- **`server.admin_hosts`**: if empty, `waf.inboxes.de` and `localhost` apply; **every IP address** is also an admin host automatically.
- **`cache.json`** (`/var/lib/bastion/cache.json`) overrides the cache values from `config.yaml` once you change them in the GUI.
- **`auth.jwt_secret`** protects tokens **and** encrypts MFA secrets. Changing it logs everyone out and makes existing MFA secrets unreadable (MFA must be reset).
- **Secrets** are also stored in `/etc/bastion/.secrets.env` (0600) by the installer.

## systemd

```ini
[Service]
User=bastion
Group=bastion
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

- Serve the GUI only over HTTPS and preferably a dedicated admin domain (`admin_hosts`)
- Do not expose API port 8080 publicly
- Enable MFA for all GUI users
- Back up DB, config and certificates regularly
- Keep `jwt_secret` and the DB password secret
- Firewall: only 80/443 (TCP) and 443/UDP (HTTP/3)
- Behind a CDN/proxy: only trust `X-Forwarded-For` from trusted sources (Bastion trusts the header)

## Known limitations (v1.0.1)

- Some GUI options are not enforced yet: HSTS, compression, max body size, custom headers, force-HTTPS switch, HTTP/2 and WebSocket switches. Details: [[EN-Sites-and-Templates|Sites & templates]].
- Cache is not persistent across restarts.
- No automatic cache invalidation when deleting/changing a site (locations do clear the cache).
- MFA has no recovery codes.
- No server-side token revocation list.

Next: [[EN-API|API reference]]

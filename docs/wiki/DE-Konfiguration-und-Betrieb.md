# Konfiguration & Betrieb

## config.yaml

Pfad: `/etc/bastion/config.yaml` (root:`bastion`, 0640)

```yaml
server:
  http_addr: ":80"
  https_addr: ":443"
  api_addr: ":8080"          # API (HTTP, intern halten!)
  admin_hosts: []            # z. B. ["bastion.example.com"]; leer = waf.inboxes.de + localhost
  http3: true

database:
  url: "postgres://waf:PASSWORT@localhost:5432/waf?sslmode=disable"

auth:
  jwt_secret: "…"            # auch Key für die MFA-Verschlüsselung
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
  ttl_seconds: 60            # Fallback, wenn das Backend keine Frische sendet
  max_entries: 500
  max_object_kb: 256
  max_total_mb: 64

security:
  auto_ban_404:
    enabled: true
    threshold: 50
    window_seconds: 600
```

### Hinweise

- **`server.api_addr`** ist HTTP und sollte nur intern erreichbar sein. Die GUI nutzt `/api/` über den Admin-Host auf 443.
- **`server.admin_hosts`**: Ist die Liste leer, gelten `waf.inboxes.de` und `localhost`; **jede IP-Adresse** gilt zusätzlich automatisch als Admin-Host.
- **`cache.json`** (`/var/lib/bastion/cache.json`) überschreibt die Cache-Werte aus `config.yaml`, sobald du sie in der GUI änderst.
- **`auth.jwt_secret`** schützt Tokens **und** verschlüsselt MFA-Secrets. Änderung meldet alle Benutzer ab und macht bestehende MFA-Secrets unlesbar (MFA muss zurückgesetzt werden).
- **Secrets** werden vom Installer auch in `/etc/bastion/.secrets.env` abgelegt (0600).

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

## Health-Checks

```sh
curl -s http://127.0.0.1:8080/healthz
# {"status":"ok"}

curl -sk https://<admin-host>/healthz
```

## Backup

Sichere:

1. PostgreSQL-Datenbank `waf`
2. `/etc/bastion` (Config + Secrets)
3. `/var/lib/bastion/certs` (Zertifikate + ACME)
4. Optional: `/var/lib/bastion/geoip` und `/var/lib/bastion/logs`

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

oder manuell das neue Binary nach `/opt/bastion/bin/bastion` kopieren und `systemctl restart bastion`.

Die Datenbank wird beim Start automatisch migriert (`AutoMigrate`).

## Troubleshooting

| Symptom | Ursache / Lösung |
|---|---|
| `502` mit `unknown_site` | `Host` passt zu keiner Site/Domain |
| `403` trotz richtiger URL | Site deaktiviert, IP-Deny, CrowdSec, GeoIP oder Auto-Ban |
| `429` | Rate Limit oder Basic-Auth-Lockout (`Retry-After` beachten) |
| `401` | Basic Auth aktiv, aber keine/ungültige Zugangsdaten |
| Zertifikat falsch | Domain nicht im SAN; neu ausstellen |
| GUI nicht erreichbar | Firewall 443, Admin-Host/IP, Dienststatus |
| GeoIP wirkt nicht | MMDB fehlt/nicht geladen; `journalctl -u bastion | grep -i geoip` |
| CrowdSec „disconnected" | LAPI/Key prüfen: `cscli bouncers list`, `journalctl -u crowdsec` |
| Cache greift nicht | Backend sendet `no-store`/`private`/`Set-Cookie`/`Vary`; TTL/Enabled prüfen |
| Auto-Ban zu aggressiv | `security.auto_ban_404.threshold` erhöhen oder `enabled: false` |
| `Text file busy` beim Upgrade | Dienst vor dem Kopieren stoppen |
| Admin ausgesperrt | GUI über IP/localhost; Admin-Hosts sind von Filtern ausgenommen |

## Sicherheit

- GUI nur über HTTPS und idealerweise eine eigene Admin-Domain (`admin_hosts`)
- API-Port 8080 nicht öffentlich
- MFA für alle GUI-Benutzer aktivieren
- Regelmäßige Backups von DB, Config und Zertifikaten
- `jwt_secret` und DB-Passwort geheim halten
- Firewall: nur 80/443 (TCP) und 443/UDP (HTTP/3)
- Bei Betrieb hinter einem CDN/Proxy: `X-Forwarded-For` nur von vertrauenswürdigen Quellen akzeptieren (Bastion vertraut dem Header)

## Bekannte Einschränkungen (v1.0.1)

- Einige GUI-Optionen sind noch nicht erzwungen: HSTS, Compression, Max Body Size, Custom Headers, Force-HTTPS-Schalter, HTTP/2-/WebSocket-Schalter. Details: [[DE-Sites-und-Templates|Sites & Templates]].
- Cache ist nicht persistent über Neustarts.
- Keine automatische Cache-Invalidierung beim Löschen/Ändern einer Site (Locations leeren den Cache).
- MFA hat keine Recovery-Codes.
- Keine serverseitige Token-Sperrliste.

Weiter: [[DE-API|API-Referenz]]

# Überblick & Einstieg

**Bastion** ist ein selbstgehosteter Reverse Proxy mit Sicherheits-Kontrollzentrum. Ein einzelnes Go-Binary enthält Proxy, API und die eingebettete Web-Oberfläche; PostgreSQL speichert Konfiguration und Audit-Daten.

## Funktionsumfang

- Reverse Proxy mit TLS-Terminierung (SNI, mehrere Zertifikate)
- Let's Encrypt (HTTP-01) mit automatischer Erneuerung + manueller Zertifikats-Upload
- Mehrere Domains/Aliase pro Site
- Templates mit Vererbung und Overrides pro Site
- GeoIP-Filterung (DB-IP Lite) pro Site/Template + mandantenweite Regeln
- CrowdSec-Bouncer, Alerts und Good-Actor-Whitelist
- Globale und per-Site IP-Allow-/Deny-Listen (IPv4, IPv6, CIDR)
- Per-Site Basic Auth mit Benutzern und 3-Fehlversuche/5-Minuten-Lockout
- Optionales TOTP-MFA für die GUI (Authenticator-Apps)
- Rate Limiting (Token-Bucket, pro Site + Client-IP)
- HTTP-Response-Cache mit Backend-Header-Unterstützung
- HTTP/2 und HTTP/3 (QUIC)
- Multi-Tenancy + RBAC (superadmin, tenant_admin, editor, viewer)
- GUI auf Deutsch und Englisch (umschaltbar unter Settings → Sprache & Oberfläche)
- Zugriffslogs (JSONL + Combined Log) und Audit-Log
- Dashboard mit Systemmetriken, 24h-Traffic und Zertifikats-Ablauf
- Update-Check gegen GitHub-Releases

## Architektur

| Komponente | Technologie |
|---|---|
| Backend / API | Go 1.26, Gin, GORM |
| Datenbank | PostgreSQL |
| Frontend | React, Vite, Tailwind (im Binary eingebettet) |
| Proxy | `net/http` ReverseProxy, TLS-SNI, quic-go (HTTP/3) |
| Zertifikate | certmagic (Let's Encrypt) + manuelle Zertifikate |
| GeoIP | DB-IP Lite (MMDB) |
| Bouncer | CrowdSec LAPI |

## Pfade und Ports

| Pfad / Port | Zweck |
|---|---|
| `/opt/bastion/bin/bastion` | Binary |
| `/opt/bastion/src` | Quellcode (bei Installation aus dem Repo) |
| `/etc/bastion/config.yaml` | Konfiguration |
| `/etc/bastion/.secrets.env` | DB-Passwort und Secrets (vom Installer erzeugt) |
| `/var/lib/bastion/certs` | Zertifikate (inkl. ACME-Storage) |
| `/var/lib/bastion/geoip` | GeoIP-Datenbank |
| `/var/lib/bastion/logs` | Zugriffslogs |
| `/var/lib/bastion/cache.json` | Persistierte Cache-Konfiguration |
| `/var/lib/bastion/version_cache.json` | Update-Check-Cache (24 h) |
| `/var/lib/bastion/good-actors.yaml` | CrowdSec-Whitelist aus der GUI |
| `:80` | HTTP → HTTPS-Redirect |
| `:443` | Proxy + GUI (HTTPS, HTTP/3) |
| `:8080` | API (HTTP, intern) |
| `127.0.0.1:8081` | CrowdSec LAPI |

## Request-Ablauf (vereinfacht)

1. TLS-Terminierung über SNI-Zertifikat.
2. **Admin-Host?** → GUI/API, Sicherheitsfilter werden übersprungen (Aussperrschutz).
3. Globale IP-Deny-Liste → Block.
4. CrowdSec-Ban → Block.
5. Site anhand `Host` auflösen (`domain` + `extra_domains`).
6. Site-IP-Regeln und optional Basic Auth.
7. GeoIP-Filter (Site/Template + Regeln).
8. Rate Limit.
9. Response-Cache (HIT → direkt aus dem RAM).
10. Sonst Reverse Proxy zum Upstream.

> **Admin-Hosts:** Jede IP-Adresse als Host gilt automatisch als Admin-Host. Zusätzlich gelten `server.admin_hosts`; ist die Liste leer, werden `localhost` und ein interner Fallback verwendet. Admin-Hosts sind von IP-Listen, CrowdSec und GeoIP ausgenommen, damit du dich nicht selbst aussperrst.

## Weiterlesen

- [Installation & Upgrade](https://github.com/Inboxes-CIG/Bastion/wiki/DE-Installation)
- [Erste Schritte](https://github.com/Inboxes-CIG/Bastion/wiki/DE-Erste-Schritte)
- [Konfiguration & Betrieb](https://github.com/Inboxes-CIG/Bastion/wiki/DE-Konfiguration-und-Betrieb)

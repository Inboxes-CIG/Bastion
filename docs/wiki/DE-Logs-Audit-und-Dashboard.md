# Logs, Audit & Dashboard

## Zugriffslogs

Bastion schreibt zwei Formate:

| Datei | Format |
|---|---|
| `/var/lib/bastion/logs/access-YYYY-MM-DD.jsonl` | ein JSON-Objekt pro Request |
| `/var/lib/bastion/logs/access.log` | Combined Log Format (nginx-ähnlich) |

JSONL-Felder:

```json
{
  "time": "2026-11-24T14:00:00+01:00",
  "client_ip": "203.0.113.10",
  "host": "app.example.com",
  "domain": "app.example.com",
  "site_id": "…",
  "method": "GET",
  "path": "/api/users",
  "status": 200,
  "bytes": 1234,
  "duration_ms": 12,
  "blocked": false,
  "block_reason": "",
  "country": "DE",
  "user_agent": "…",
  "referer": "…",
  "proto": "HTTP/2.0",
  "admin": false
}
```

- Rotation: täglich
- Retention: `logging.retention_days` (Standard 30 Tage), ältere Dateien werden beim Start gelöscht
- Zusätzlich ein In-Memory-Ringpuffer (`logging.max_memory`, Standard 10000) für schnelle Abfragen
- Admin-Requests werden protokolliert, aber aus den Site-Statistiken ausgeschlossen

### Logs in der GUI

**Logs** mit Filtern:

- Site/Domain
- `blocked=true/false`
- Statuscode
- Suche über Pfad, Client-IP, Host
- Limit
- Tagesdatei auswählen
- Admin-Requests ausblenden

API: `GET /api/v1/logs?site=…&blocked=true&status=404&q=…&limit=200&file=access-2026-11-24.jsonl&hide_admin=true`

## Audit-Log

Das Audit-Log liegt in PostgreSQL (`audit_logs`) und wird in der GUI unter **Audit** angezeigt.

Erfasst werden:

- Alle schreibenden API-Requests (`POST`, `PUT`, `PATCH`, `DELETE`) mit Aktion `create`/`update`/`delete`
- Login-Erfolge (`login`) und Fehlversuche (`login_failed`)
- Automatische 404-Bans (`auto_ban`)

Felder: Zeit, Benutzer/E-Mail, Aktion, Ressource, Ressourcen-ID, Methode, Pfad, Status, IP, Details.

Die Spalte **Vorgang** zeigt eine lesbare Beschreibung, z. B.:

- `Tenant "Kunde A" angelegt`
- `Site "app.example.com" geändert`
- `Location "/api" angelegt`
- `Zertifikat "example.com" ausgestellt`
- `IP-Eintrag "203.0.113.10" angelegt`
- `Cache geleert`
- `MFA aktiviert`
- `auto_ban`-Einträge mit Details `blocked after repeated 404 responses`

Die Suche im Audit-Log durchsucht auch diese Details.

Nicht erfasst: lesende GET-Requests (außer Login).

## Dashboard

`GET /api/v1/stats` liefert:

| Bereich | Inhalt |
|---|---|
| Zähler | Sites, Zertifikate, Benutzer, Tenants, GeoIP-Regeln, Templates |
| `expiring_certs` | Zertifikate, die in 30 Tagen ablaufen |
| `metrics` | Gesamt-Requests, geblockt/erlaubt, Statusverteilung, Blockgründe, Top-Sites, blockierte Länder, Uptime, RPS |
| `traffic_24h` | dieselben Aggregate für die letzten 24 Stunden (aus den JSONL-Dateien) |
| `system` | CPU, RAM, Disk, Load, Uptime |

- `traffic_24h` wird 30 Sekunden gecacht.
- `metrics` basiert auf dem In-Memory-Ringpuffer seit dem Start (bzw. seit dem Laden der letzten Logs).
- `system` liest `/proc/stat`, `/proc/meminfo` und `statfs`.

## Nützliche Befehle

```sh
# Letzte Requests
tail -f /var/lib/bastion/logs/access-$(date +%F).jsonl | jq .

# Top-Statuscodes
jq -r '.status' /var/lib/bastion/logs/access-$(date +%F).jsonl | sort | uniq -c | sort -rn

# Geblockte Requests
jq -c 'select(.blocked==true)' /var/lib/bastion/logs/access-$(date +%F).jsonl | tail

# Audit (PostgreSQL)
sudo -u postgres psql -d waf -c "SELECT created_at, user_email, action, resource, status, ip FROM audit_logs ORDER BY created_at DESC LIMIT 20;"
```

Weiter: [Benutzer, Tenants & Rollen](https://github.com/Inboxes-CIG/Bastion/wiki/DE-Benutzer-Tenants-und-Rollen)

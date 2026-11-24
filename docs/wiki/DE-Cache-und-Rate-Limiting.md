# Cache & Rate Limiting

## Response-Cache

Bastion cached anonyme `GET`-Antworten **im RAM** des Prozesses.

### Cache-Key

```
Host + Request-URI (Pfad + Query) + Accept-Encoding
```

Komprimierte und unkomprimierte Varianten werden getrennt gespeichert.

### Wann gecacht wird

- Nur `GET`
- Kein `Authorization`-Header
- Kein `Cache-Control: no-cache` / `Pragma: no-cache` im Request
- Status: `2xx` (außer 206), `300–308` (außer 304), `404`, `410`
- Kein `Set-Cookie`
- Kein `Vary` außer `Accept-Encoding`

### Backend-Header

| Header | Verhalten |
|---|---|
| `Cache-Control: s-maxage=N` | TTL = N (Vorrang) |
| `Cache-Control: max-age=N` | TTL = N |
| `Expires` | TTL bis zum Zeitpunkt |
| `Age: N` | wird von der TTL abgezogen |
| `no-store`, `no-cache`, `private` | **nicht** cachen |
| `ETag` / `Last-Modified` | 304-Antworten aus dem Cache |
| `Vary: Accept-Encoding` | cachebar |
| andere `Vary`-Felder | **nicht** cachen |
| keine Frische-Header | konfigurierte TTL (Fallback) |

### Grenzen und Eviction

Pro Cache global:

- `max_entries`
- `max_object_kb` pro Antwort
- `max_total_mb`

Abgelaufene Einträge werden zuerst entfernt, dann die ältesten, bis es passt. Der Cache ist **nicht persistent** – ein Neustart leert ihn. Gespeichert wird nur die Konfiguration (`/var/lib/bastion/cache.json`), die `config.yaml`-Werte überschreibt.

### Beobachten

```sh
curl -sI https://app.example.com/ | grep -i -E 'x-cache|age'
```

- `X-Cache: MISS` beim ersten Request
- `X-Cache: HIT` bei folgenden
- `Age` = Sekunden seit dem Speichern

### Leeren

- Global: **Settings → Response-Cache → Cache leeren**
- Pro Site: **Site → Cache → Cache dieser Site leeren**
- Wird bei Location-Änderungen automatisch für die Site ausgeführt.
- Site-Löschung leert den Cache **nicht** automatisch.

### Aktivierung

- Global: `cache.enabled` in `config.yaml`
- Pro Site/Location: `cache_enabled` (Vererbung)
- `cache_ttl` ist nur Fallback; Backend-Header haben Vorrang

## Rate Limiting

Token-Bucket pro **Site + Client-IP**.

- `rate_limit_rps`: Nachfüllrate (Requests/Sekunde); `0` = deaktiviert
- `rate_limit_burst`: Bucket-Größe; `0` → gleich `rps`
- Antwort bei Überschreitung: `429` mit `Retry-After: 1`
- Vererbung: Location → Site → Template
- Die globale Allowlist überspringt das Rate Limit
- Admin-Hosts sind ausgenommen
- In-Memory; Neustart setzt alle Buckets zurück
- Idle Buckets werden nach 10 Minuten bereinigt

### Beispiel

| Ebene | RPS | Burst |
|---|---|---|
| Template | 20 | 40 |
| Site | 10 | 20 |
| Location `/api` | 5 | 10 |

Ein Client darf pro Sekunde 5 Requests (kurzzeitig 10) auf `/api` senden; andere Pfade nutzen 10/20.

### Empfehlungen

- Statische Assets: großzügig oder gar kein Limit
- Login/API: niedriger
- Zusammen mit Auto-Ban (404-Flut) und CrowdSec kombinieren
- Bei CDN/Proxy vor Bastion: `X-Forwarded-For` korrekt setzen (Bastion vertraut dem Header)

Weiter: [[DE-Logs-Audit-und-Dashboard|Logs, Audit & Dashboard]]

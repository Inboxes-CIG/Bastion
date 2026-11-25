# API-Referenz

Basis-URLs:

- Standard: `http://127.0.0.1:8080/api/v1` – die API bindet ab Werk **nur Loopback** (`server.api_addr` in `/etc/bastion/config.yaml`, leer = `127.0.0.1:8080`)
- Über Admin-Host: `https://<admin-host>/api/v1`

Die API spricht JSON. Antworten haben die Form:

```json
{ "data": … }
```

Fehler:

```json
{ "error": "message" }
```

## Authentifizierung

```http
Authorization: Bearer <access_token>
```

### Login

```http
POST /auth/login
Content-Type: application/json

{ "email": "admin@bastion.local", "password": "…" }
```

Nach **drei** Fehlversuchen innerhalb von 10 Minuten werden IP-Adresse **und** E-Mail für 10 Minuten gesperrt → `429` mit `Retry-After`. Ein erfolgreicher Login setzt beide Zähler zurück.

Antwort ohne MFA:

```json
{ "data": { "tokens": { "access_token": "…", "refresh_token": "…", "expires_in": 900 },
            "user": { "id": "…", "email": "…", "role": "superadmin", "tenant_id": null,
                      "must_change_password": false } } }
```

Antwort mit MFA:

```json
{ "data": { "mfa_required": true, "mfa_token": "…" } }
```

Dann:

```http
POST /auth/login/mfa
{ "mfa_token": "…", "code": "123456" }
```

### Refresh

```http
POST /auth/refresh
{ "refresh_token": "…" }
```

Refresh-Tokens **rotieren**: jedes eingesetzte Token wird serverseitig auf eine Sperrliste gesetzt und durch ein neues ersetzt. Wird ein bereits verwandtes Token erneut eingereicht, gilt das als Diebstahlverdacht – die gesamte Token-Familie des Benutzers wird widerrufen und der Nutzer muss sich neu anmelden. Abmeldung:

```http
POST /auth/logout
{ "refresh_token": "…" }
```

`/auth/logout` sperrt das mitgeschickte Refresh-Token sofort.

### Passwort ändern

```http
POST /auth/change-password
{ "current_password": "…", "new_password": "…" }
```

Erfolgreiche Änderungen widerrufen **alle** Refresh-Tokens des Kontos. Ist `must_change_password` gesetzt, blockiert die API alle Endpunkte außer `/auth/me`, `/auth/change-password` und `/auth/logout` mit `403` (`code: password_change_required`), bis das Passwort geändert wurde. Die GUI zeigt dazu einen Pflichtdialog.

### Eigenes Konto / MFA

| Methode | Pfad | Zweck |
|---|---|---|
| GET | `/auth/me` | Benutzer + `mfa_enabled` + `must_change_password` |
| POST | `/auth/mfa/setup` | Secret + QR + otpauth-URL |
| POST | `/auth/mfa/enable` | `{ "code": "…" }` |
| POST | `/auth/mfa/disable` | `{ "code": "…", "password": "…" }` – zusätzlich zum TOTP-Code ist das Konto-Passwort nötig |

### Health

```http
GET /healthz
```

## Endpunkte

Schreibende Endpunkte sind auf `superadmin` und `tenant_admin` beschränkt, sofern angegeben.

### Sites

| Methode | Pfad | Rolle |
|---|---|---|
| GET | `/sites` | alle |
| GET | `/sites/:id` | alle (inkl. `locations`) |
| POST | `/sites` | superadmin, tenant_admin |
| PUT | `/sites/:id` | superadmin, tenant_admin |
| DELETE | `/sites/:id` | superadmin, tenant_admin |

### Locations

| Methode | Pfad | Rolle |
|---|---|---|
| GET | `/sites/:id/locations` | alle |
| POST | `/sites/:id/locations` | superadmin, tenant_admin |
| PUT | `/sites/:id/locations/:lid` | superadmin, tenant_admin |
| DELETE | `/sites/:id/locations/:lid` | superadmin, tenant_admin |

Payload:

```json
{
  "path_prefix": "/api",
  "enabled": true,
  "upstream_url": "http://backend.example.com",
  "upstream_host": null,
  "upstream_insecure_skip_verify": false,
  "auth_enabled": null,
  "auth_users_mode": "inherit",
  "ip_rules_mode": "inherit",
  "cache_enabled": true,
  "cache_ttl": 300,
  "cache_max_object_kb": 512,
  "rate_limit_rps": 10,
  "rate_limit_burst": 20,
  "geoip_enabled": false,
  "geoip_mode": null,
  "geoip_countries": null,
  "http3_enabled": null
}
```

`null` = von Site/Template erben. Der Prefix wird normalisiert (`/api/` → `/api`).

> **Upstream-Schutz (SSRF):** `upstream_url` wird beim Speichern geprüft: nur `http`/`https`, keine Loopback-, Link-Local-, CGNAT- oder RFC1918-Ziele und keine Hostnamen, die auf private Adressen auflösen. Ausnahmen: `security.upstream.allow_private: true`, der globale Eintrag `security.upstream.allowed_hosts` oder die Tenant-Allowlist `allowed_upstream_hosts` (Hosts, `*.domain` oder CIDRs). Abweichungen werden mit `400` (`upstream: …`) abgelehnt; im Proxy-Pfad blockierte Upstreams liefern `502` mit `block_reason=ssrf`.

### Templates

| Methode | Pfad | Rolle |
|---|---|---|
| GET | `/templates` | alle |
| GET | `/templates/:id` | alle |
| POST | `/templates` | superadmin, tenant_admin |
| PUT | `/templates/:id` | superadmin, tenant_admin |
| DELETE | `/templates/:id` | superadmin, tenant_admin |

### Zertifikate

| Methode | Pfad | Rolle |
|---|---|---|
| GET | `/certificates` | alle |
| GET | `/certificates/:id` | alle |
| POST | `/certificates` | superadmin, tenant_admin |
| POST | `/certificates/issue` | superadmin, tenant_admin |
| DELETE | `/certificates/:id` | superadmin, tenant_admin |

`POST /certificates` = manueller Upload (`domain`, `cert_pem`, `key_pem`, `tenant_id`).
`POST /certificates/issue` = Let's Encrypt (`site_id` oder `domain`, `email`).

### Benutzer / Tenants / Rollen

| Methode | Pfad | Rolle |
|---|---|---|
| GET/POST/PUT/DELETE | `/users` | superadmin, tenant_admin (Schreiben) |
| GET | `/roles` | alle |
| GET | `/tenants` | alle |
| POST/PUT/DELETE | `/tenants` | superadmin |

- `POST /users` verlangt zwingend ein `password`; neue Benutzer erhalten `must_change_password = true` und müssen es beim ersten Login ändern.
- Sensible Änderungen per `PUT /users/:id` (Rolle, E-Mail, Tenant, Status, Passwort-Reset) erfordern `reauth_password` – das Passwort des **administrierenden** Kontos. Fehlt es oder ist es falsch: `403`.
- Tenants akzeptieren `allowed_upstream_hosts` (Liste aus Hosts, `*.domains` oder CIDRs) für private Upstreams.

### Site-Zugriff

| Methode | Pfad | Rolle |
|---|---|---|
| GET/POST/PUT/DELETE | `/sites/:id/users` | Schreiben: superadmin, tenant_admin |
| GET/POST/PUT/DELETE | `/sites/:id/ip-rules` | Schreiben: superadmin, tenant_admin |
| GET/POST/PUT/DELETE | `/sites/:id/locations/:lid/users` | Schreiben: superadmin, tenant_admin |
| GET/POST/PUT/DELETE | `/sites/:id/locations/:lid/ip-rules` | Schreiben: superadmin, tenant_admin |

### GeoIP

| Methode | Pfad | Rolle |
|---|---|---|
| GET | `/geoip/countries` | alle |
| GET/POST/DELETE | `/geoip/rules` | Schreiben: superadmin, tenant_admin |

### CrowdSec

| Methode | Pfad | Rolle |
|---|---|---|
| GET | `/crowdsec/status` | alle |
| GET | `/crowdsec/decisions` | alle |
| POST/DELETE | `/crowdsec/decisions` | superadmin, tenant_admin |
| GET | `/crowdsec/alerts` | alle |
| GET/POST/PUT/DELETE | `/crowdsec/good-actors` | Schreiben: superadmin, tenant_admin |

### IP-Listen (global)

| Methode | Pfad | Rolle |
|---|---|---|
| GET | `/iplist` | superadmin |
| POST/PUT/DELETE | `/iplist` | superadmin, tenant_admin |

### Cache

| Methode | Pfad | Rolle |
|---|---|---|
| GET | `/cache` | alle |
| PUT | `/cache/config` | superadmin, tenant_admin |
| POST | `/cache/clear` | superadmin, tenant_admin |

`POST /cache/clear` mit `{ "site_id": "…" }` leert eine Site, ohne Body alles.

### Logs / Audit / Stats / Version

| Methode | Pfad | Zweck |
|---|---|---|
| GET | `/logs` | Zugriffslogs (Filter: `site`, `blocked`, `status`, `q`, `limit`, `file`, `hide_admin`) |
| GET | `/logs/files` | verfügbare Tagesdateien (superadmin) |
| GET | `/audit` | Audit-Log |
| GET | `/stats` | Dashboard-Daten |
| GET | `/version` | installierte/neueste Version |
| GET | `/admin/info` | Admin-Hosts/Info |

`/logs`, `/stats` und `/audit` sind **tenant-scope**: Nicht-Superadmins sehen nur Einträge und Zahlen der Sites ihres Tenants; `/logs?file=…` (Tagesdateien) und System-Kennzahlen sind Superadmin-only.

## Sicherheits-Header & CORS

Alle Antworten (API und GUI) tragen `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY` und `Referrer-Policy: no-referrer`. Cross-Origin-Requests werden nur akzeptiert, wenn der `Origin` in `server.cors_origins` gelistet ist; Standard ist keine CORS-Freigabe – die GUI ist same-origin und braucht keine.

## Beispiele

```sh
TOKEN=$(curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@bastion.local","password":"…"}' \
  | jq -r '.data.tokens.access_token')

# Sites
curl -s http://127.0.0.1:8080/api/v1/sites -H "Authorization: Bearer $TOKEN" | jq .

# Location anlegen
curl -s -X POST http://127.0.0.1:8080/api/v1/sites/<site-id>/locations \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"path_prefix":"/api","upstream_url":"http://backend.example.com","cache_enabled":false}'

# Cache einer Site leeren
curl -s -X POST http://127.0.0.1:8080/api/v1/cache/clear \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"site_id":"<site-id>"}'
```

## Statuscodes

| Code | Bedeutung |
|---|---|
| 200/201/204 | Erfolg |
| 400 | Validierungsfehler (auch abgelehnte Upstream-Ziele: `upstream: …`) |
| 401 | nicht angemeldet / ungültiges oder widerrufenes Token |
| 403 | Rolle nicht erlaubt, Reauth fehlgeschlagen oder `password_change_required` |
| 404 | Ressource nicht gefunden – oder nicht im eigenen Tenant sichtbar (IDOR-Schutz: fremde Tenant-Ressourcen liefern stets 404, nie 403) |
| 429 | Rate Limit oder Login-Lockout (3 Fehlversuche / 10 min pro IP+E-Mail) |
| 500 | interner Fehler |
| 502 | unbekannte Site oder Upstream-Fehler (inkl. `block_reason=ssrf`) |

# API reference

Base URLs:

- Default: `http://127.0.0.1:8080/api/v1` – the API binds to **loopback only** out of the box (`server.api_addr` in `/etc/bastion/config.yaml`, empty = `127.0.0.1:8080`)
- Via admin host: `https://<admin-host>/api/v1`

The API speaks JSON. Responses use:

```json
{ "data": … }
```

Errors:

```json
{ "error": "message" }
```

## Authentication

```http
Authorization: Bearer <access_token>
```

### Login

```http
POST /auth/login
Content-Type: application/json

{ "email": "admin@bastion.local", "password": "…" }
```

After **three** failed attempts within 10 minutes, both the IP address **and** the email are locked for 10 minutes → `429` with `Retry-After`. A successful login resets both counters.

Without MFA:

```json
{ "data": { "tokens": { "access_token": "…", "refresh_token": "…", "expires_in": 900 },
            "user": { "id": "…", "email": "…", "role": "superadmin", "tenant_id": null,
                      "must_change_password": false } } }
```

With MFA:

```json
{ "data": { "mfa_required": true, "mfa_token": "…" } }
```

Then:

```http
POST /auth/login/mfa
{ "mfa_token": "…", "code": "123456" }
```

### Refresh

```http
POST /auth/refresh
{ "refresh_token": "…" }
```

Refresh tokens **rotate**: every token presented is added to a server-side blacklist and replaced by a new one. Re-submitting an already-used token is treated as theft – the user's entire token family is revoked and they must sign in again. Sign out:

```http
POST /auth/logout
{ "refresh_token": "…" }
```

`/auth/logout` immediately blacklists the refresh token provided.

### Change password

```http
POST /auth/change-password
{ "current_password": "…", "new_password": "…" }
```

A successful change revokes **all** refresh tokens of the account. If `must_change_password` is set, the API blocks every endpoint except `/auth/me`, `/auth/change-password` and `/auth/logout` with `403` (`code: password_change_required`) until the password is changed. The GUI shows a mandatory dialog.

### Own account / MFA

| Method | Path | Purpose |
|---|---|---|
| GET | `/auth/me` | User + `mfa_enabled` + `must_change_password` |
| POST | `/auth/mfa/setup` | Secret + QR + otpauth URL |
| POST | `/auth/mfa/enable` | `{ "code": "…" }` |
| POST | `/auth/mfa/disable` | `{ "code": "…", "password": "…" }` – requires the account password in addition to the TOTP code |

### Health

```http
GET /healthz
```

## Endpoints

Write endpoints are restricted to `superadmin` and `tenant_admin` where noted.

### Sites

| Method | Path | Role |
|---|---|---|
| GET | `/sites` | any |
| GET | `/sites/:id` | any (incl. `locations`) |
| POST | `/sites` | superadmin, tenant_admin |
| PUT | `/sites/:id` | superadmin, tenant_admin |
| DELETE | `/sites/:id` | superadmin, tenant_admin |

### Locations

| Method | Path | Role |
|---|---|---|
| GET | `/sites/:id/locations` | any |
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

`null` = inherit from site/template. The prefix is normalized (`/api/` → `/api`).

> **Upstream protection (SSRF):** `upstream_url` is validated on save **and on every request** (hostnames are re-resolved with a short DNS cache): only `http`/`https`; no loopback, link-local, CGNAT or RFC1918 targets and no hostnames that resolve to private addresses. Exceptions: the tenant allowlist `allowed_upstream_hosts` (GUI → Tenants; recommended), optionally the global `security.upstream.allowed_hosts`, or `security.upstream.allow_private: true`. Violations are rejected with `400` (`upstream: …`); upstreams blocked in the proxy path return `502` with `block_reason=ssrf`.

### Templates

| Method | Path | Role |
|---|---|---|
| GET | `/templates` | any |
| GET | `/templates/:id` | any |
| POST | `/templates` | superadmin, tenant_admin |
| PUT | `/templates/:id` | superadmin, tenant_admin |
| DELETE | `/templates/:id` | superadmin, tenant_admin |

### Certificates

| Method | Path | Role |
|---|---|---|
| GET | `/certificates` | any |
| GET | `/certificates/:id` | any |
| POST | `/certificates` | superadmin, tenant_admin |
| POST | `/certificates/issue` | superadmin, tenant_admin |
| DELETE | `/certificates/:id` | superadmin, tenant_admin |

`POST /certificates` = manual upload (`domain`, `cert_pem`, `key_pem`, `tenant_id`).
`POST /certificates/issue` = Let's Encrypt (`site_id` or `domain`, `email`). Free `domain`/`domains` without `site_id` are superadmin-only.

### Users / tenants / roles

| Method | Path | Role |
|---|---|---|
| GET/POST/PUT/DELETE | `/users` | superadmin, tenant_admin (writes) |
| GET | `/roles` | any |
| GET | `/tenants` | any |
| POST/PUT/DELETE | `/tenants` | superadmin |

- `POST /users` requires a `password`; new users get `must_change_password = true` and must change it at first login.
- Sensitive `PUT /users/:id` changes (role, email, tenant, status, password reset) require `reauth_password` – the password of the **acting** account. Missing or wrong: `403`.
- Tenants accept `allowed_upstream_hosts` (list of hosts, `*.domains` or CIDRs) to permit private upstreams.

### Site access

| Method | Path | Role |
|---|---|---|
| GET/POST/PUT/DELETE | `/sites/:id/users` | writes: superadmin, tenant_admin |
| GET/POST/PUT/DELETE | `/sites/:id/ip-rules` | writes: superadmin, tenant_admin |
| GET/POST/PUT/DELETE | `/sites/:id/locations/:lid/users` | writes: superadmin, tenant_admin |
| GET/POST/PUT/DELETE | `/sites/:id/locations/:lid/ip-rules` | writes: superadmin, tenant_admin |

### GeoIP

| Method | Path | Role |
|---|---|---|
| GET | `/geoip/countries` | any |
| GET/POST/DELETE | `/geoip/rules` | writes: superadmin, tenant_admin |

### CrowdSec

| Method | Path | Role |
|---|---|---|
| GET | `/crowdsec/status` | any |
| GET | `/crowdsec/decisions` | any |
| POST/DELETE | `/crowdsec/decisions` | superadmin |
| GET | `/crowdsec/alerts` | any |
| GET/POST/PUT/DELETE | `/crowdsec/good-actors` | read: any, writes: superadmin |

### Global IP lists

| Method | Path | Role |
|---|---|---|
| GET | `/iplist` | superadmin |
| POST/PUT/DELETE | `/iplist` | superadmin |

`POST`/`PUT` additionally accept `ttl_seconds` (int; 0 = unlimited) or `expires_at` (RFC3339). A `PUT` without either field keeps the existing expiry. Responses include `expires_at` and `source` (`manual`/`auto_ban`); expired entries are hidden by `GET` and deleted on the next reload.

### Cache

| Method | Path | Role |
|---|---|---|
| GET | `/cache` | any |
| PUT | `/cache/config` | superadmin |
| POST | `/cache/clear` | superadmin, tenant_admin |

`POST /cache/clear` with `{ "site_id": "…" }` clears one site; without a body it clears everything.

### Logs / audit / stats / version

| Method | Path | Purpose |
|---|---|---|
| GET | `/logs` | Access logs (`site`, `blocked`, `status`, `q`, `limit`, `file`, `hide_admin`) |
| GET | `/logs/files` | Available daily files (superadmin) |
| GET | `/audit` | Audit log (search includes `details`) |
| GET | `/stats` | Dashboard data |
| GET | `/stats/sites` | Per-site statistics (`range=24h\|7d\|30d\|all`) |
| GET | `/version` | Installed/latest version |
| GET | `/admin/info` | Admin hosts/info |

`/stats/sites` returns per-site (tenant-filtered for non-superadmins) requests, blocks, block rate, status distribution and block reasons for the selected range; `all` = the full log retention period.

`/logs`, `/stats` and `/audit` are **tenant-scoped**: non-superadmins only see entries and counters of their own tenant's sites; `/logs?file=…` (daily files) and system metrics are superadmin-only.

## Security headers & CORS

All responses (API and GUI) carry `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY` and `Referrer-Policy: no-referrer`. Cross-origin requests are accepted only if the `Origin` is listed in `server.cors_origins`; the default is no CORS allowance – the GUI is same-origin and needs none.

## Examples

```sh
TOKEN=$(curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@bastion.local","password":"…"}' \
  | jq -r '.data.tokens.access_token')

# Sites
curl -s http://127.0.0.1:8080/api/v1/sites -H "Authorization: Bearer $TOKEN" | jq .

# Create a location
curl -s -X POST http://127.0.0.1:8080/api/v1/sites/<site-id>/locations \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"path_prefix":"/api","upstream_url":"http://backend.example.com","cache_enabled":false}'

# Clear one site's cache
curl -s -X POST http://127.0.0.1:8080/api/v1/cache/clear \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"site_id":"<site-id>"}'
```

## Status codes

| Code | Meaning |
|---|---|
| 200/201/204 | Success |
| 400 | Validation error (including rejected upstream targets: `upstream: …`) |
| 401 | Not signed in / invalid or revoked token |
| 403 | Role not allowed, re-auth failed or `password_change_required` |
| 404 | Resource not found – or not visible in your tenant (IDOR protection: foreign tenant resources always return 404, never 403) |
| 429 | Rate limit or login lockout (3 failed attempts / 10 min per IP+email) |
| 500 | Internal error |
| 502 | Unknown site or upstream error (incl. `block_reason=ssrf`) |

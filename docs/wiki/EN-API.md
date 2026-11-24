# API reference

Base URLs:

- Internal: `http://<server>:8080/api/v1`
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

Without MFA:

```json
{ "data": { "tokens": { "access_token": "…", "refresh_token": "…", "expires_in": 900 },
            "user": { "id": "…", "email": "…", "role": "superadmin", "tenant_id": null } } }
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

### Own account / MFA

| Method | Path | Purpose |
|---|---|---|
| GET | `/auth/me` | User + `mfa_enabled` |
| POST | `/auth/mfa/setup` | Secret + QR + otpauth URL |
| POST | `/auth/mfa/enable` | `{ "code": "…" }` |
| POST | `/auth/mfa/disable` | `{ "code": "…" }` |

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
  "upstream_url": "http://127.0.0.1:8080",
  "upstream_host": null,
  "upstream_insecure_skip_verify": false,
  "auth_enabled": null,
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
`POST /certificates/issue` = Let's Encrypt (`site_id` or `domain`, `email`).

### Users / tenants / roles

| Method | Path | Role |
|---|---|---|
| GET/POST/PUT/DELETE | `/users` | superadmin, tenant_admin (writes) |
| GET | `/roles` | any |
| GET | `/tenants` | any |
| POST/PUT/DELETE | `/tenants` | superadmin |

### Site access

| Method | Path | Role |
|---|---|---|
| GET/POST/PUT/DELETE | `/sites/:id/users` | writes: superadmin, tenant_admin |
| GET/POST/PUT/DELETE | `/sites/:id/ip-rules` | writes: superadmin, tenant_admin |

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
| POST/DELETE | `/crowdsec/decisions` | superadmin, tenant_admin |
| GET | `/crowdsec/alerts` | any |
| GET/POST/PUT/DELETE | `/crowdsec/good-actors` | writes: superadmin, tenant_admin |

### Global IP lists

| Method | Path | Role |
|---|---|---|
| GET | `/iplist` | any |
| POST/PUT/DELETE | `/iplist` | superadmin, tenant_admin |

### Cache

| Method | Path | Role |
|---|---|---|
| GET | `/cache` | any |
| PUT | `/cache/config` | superadmin, tenant_admin |
| POST | `/cache/clear` | superadmin, tenant_admin |

`POST /cache/clear` with `{ "site_id": "…" }` clears one site; without a body it clears everything.

### Logs / audit / stats / version

| Method | Path | Purpose |
|---|---|---|
| GET | `/logs` | Access logs (`site`, `blocked`, `status`, `q`, `limit`, `file`, `hide_admin`) |
| GET | `/logs/files` | Available daily files |
| GET | `/audit` | Audit log (search includes `details`) |
| GET | `/stats` | Dashboard data |
| GET | `/version` | Installed/latest version |
| GET | `/admin/info` | Admin hosts/info |

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
  -d '{"path_prefix":"/api","upstream_url":"http://127.0.0.1:8080","cache_enabled":false}'

# Clear one site's cache
curl -s -X POST http://127.0.0.1:8080/api/v1/cache/clear \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"site_id":"<site-id>"}'
```

## Status codes

| Code | Meaning |
|---|---|
| 200/201/204 | Success |
| 400 | Validation error |
| 401 | Not signed in / invalid token |
| 403 | Role not allowed or site/IP blocked |
| 404 | Resource not found |
| 429 | Rate limit or auth lockout |
| 500 | Internal error |
| 502 | Unknown site or upstream error |

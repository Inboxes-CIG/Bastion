# Sites & templates

A **site** connects one or more domains to an upstream. **Templates** hold reusable defaults; each site can override or inherit individual values. **Locations** override the site configuration for a path prefix.

## Site fields

| Field | Meaning |
|---|---|
| `domain` | Primary domain (host matching) |
| `extra_domains` | Additional domains/aliases, same upstream |
| `upstream_url` | Target, e.g. `http://127.0.0.1:3000` |
| `upstream_host` | Optional `Host` header to the backend |
| `upstream_insecure_skip_verify` | Ignore the HTTPS upstream's TLS certificate (self-signed backends only) |
| `tenant_id` | Tenant |
| `template_id` | Optional template |
| `cert_id` | Associated certificate |
| `enabled` | Site active/inactive (inactive → `403`) |
| `auth_enabled` | Per-site Basic Auth |
| `cache_enabled` / `cache_ttl` / `cache_max_object_kb` | Response cache |
| `rate_limit_rps` / `rate_limit_burst` | Token bucket |
| `geoip_enabled` / `geoip_mode` / `geoip_countries` | Country filter |
| `http3_enabled` | Advertise HTTP/3 via `Alt-Svc` |
| `http2_enabled` | HTTP/2 (always available server-side) |
| `force_https`, `hsts_enabled`, `hsts_max_age` | See limitations |
| `websocket_enabled` | WebSocket upgrades (passed through automatically) |
| `compression_enabled` | See limitations |
| `max_body_size_mb` | See limitations |
| `custom_headers` | See limitations |

## Locations (path overrides)

A location applies to a path prefix and can override almost everything a site configures:

| Field | Meaning |
|---|---|
| `path_prefix` | Path, e.g. `/api` or `/static` |
| `enabled` | Location active/inactive |
| `upstream_url` / `upstream_host` | Dedicated backend for this path |
| `upstream_insecure_skip_verify` | Disable TLS verification for the HTTPS upstream |
| `auth_enabled` | Basic Auth for this path |
| `auth_users_mode` | `inherit` = site users, `own` = location-specific users |
| `ip_rules_mode` | `inherit` = site rules, `own` = location-specific allow/deny rules |
| `cache_enabled` / `cache_ttl` / `cache_max_object_kb` | Cache override |
| `rate_limit_rps` / `rate_limit_burst` | Rate-limit override |
| `geoip_enabled` / `geoip_mode` / `geoip_countries` | GeoIP override |
| `http3_enabled` | HTTP/3 override |

**Inheritance:** `Location → Site → Template → internal default`. Unset fields (`null` / “inherit”) fall back to the next level.

**Path matching:**

- The **longest** matching prefix wins.
- Prefixes match whole segments: `/api` matches `/api` and `/api/v1`, but **not** `/apix`.
- `/` is a catch-all for every path.
- There is **no** wildcard matching for paths.
- Disabled locations are skipped; the site configuration applies instead.

**Important:**

- **Own or inherited users:** `auth_users_mode=inherit` uses the site users, `own` uses a location-specific user list.
- **Own or inherited IP rules:** `ip_rules_mode=inherit` uses the site rules, `own` uses a location-specific allow/deny list. “Own” **replaces** the site rules; it is not additive.
- Own users/rules are managed under **Site → Access** with the **Access scope** selector (site or location).
- `auth_enabled` still controls whether a login is required for the location.
- If `own` is selected and the list is empty, there are no valid users/rules – auth denies everyone not allow-listed by the location.
- Creating/updating/deleting a location clears the site's cache.
- Locations are managed in the GUI under **Site → Locations**.

### Example

| Prefix | Upstream | Auth | Cache |
|---|---|---|---|
| `/` (site) | `http://127.0.0.1:3000` | off | off |
| `/api` | `http://127.0.0.1:8080` | off | off |
| `/static` | `http://127.0.0.1:9000` | off | on, 1 h |
| `/admin` | `http://127.0.0.1:3000` | **on** | off |

## Template inheritance

Each field is a pointer (`*bool`, `*int`, `*string`):

- **Value set** → site/location value applies.
- **`null` / “inherit”** → value from the next level.
- **Template empty** → internal default.

## Host matching

Bastion compares the `Host` header (without port), case-insensitively, with `domain` and all `extra_domains`. No match → `502 Bad Gateway` with `block_reason=unknown_site`.

## Upstream

- `upstream_url` must be a complete `http://` or `https://` target.
- Bastion sets `X-Forwarded-Proto: https`, `X-Forwarded-Host` and (if configured) the `Host` header.
- WebSocket upgrades are supported automatically.
- The upstream must be reachable from the Bastion host; `127.0.0.1` refers to the Bastion server.

### SSRF protection (v1.0.6)

Upstream targets are checked both on save and at request time to prevent server-side request forgery:

- Only `http`/`https` schemes; `file:`, `gopher:` and friends are rejected.
- Loopback, link-local (including `169.254.169.254` cloud metadata), CGNAT (`100.64/10`) and RFC1918 ranges are blocked by default.
- Hostnames are resolved on save; a name that resolves to a private address is rejected.
- **Exceptions:** `security.upstream.allow_private: true` (global), `security.upstream.allowed_hosts` (hosts/`*.domains`/CIDRs) or the tenant list `allowed_upstream_hosts`.
- On create/update: `400` with `upstream: …`; at runtime: `502` with `block_reason=ssrf`.

> **Example:** a backend in the `10.10.20.0/24` network only works if that CIDR is in `security.upstream.allowed_hosts` (or the tenant allowlist).

> **HTTPS upstream with a self-signed certificate:** set `upstream_insecure_skip_verify` (site/location/template) to `true`. This disables certificate verification towards the backend. Only use it when the upstream is trusted and the certificate cannot be fixed otherwise.

## HTTP/2 and HTTP/3

- **HTTP/2** is always active over TLS.
- **HTTP/3** is advertised when `http3_enabled` (location/site/template) or global `server.http3` is enabled:

```
Alt-Svc: h3=":443"; ma=86400
```

UDP/443 must be open in the firewall.

## Force HTTPS

If **Force HTTPS** is enabled for a site or template (default), HTTP requests are redirected to HTTPS with `301`. If it is explicitly disabled for a site, HTTP requests are proxied to the upstream (`X-Forwarded-Proto: http`).

Admin hosts and IP addresses are **always** redirected to HTTPS, regardless of the site setting.

## Delete, disable and cache

- **Delete site** removes it (soft delete). The cache is **not** cleared automatically.
- **Disable site** → all requests receive `403`.
- **Disable location** → fall back to the site configuration.
- Clear the site cache manually after site changes; location changes clear it automatically.

## Known limitations (v1.0.6)

These fields exist in the model and UI but are **not** applied in the request path yet:

| Option | Actual behavior |
|---|---|
| `hsts_enabled` / `hsts_max_age` | No `Strict-Transport-Security` header |
| `compression_enabled` | No automatic compression; only what the upstream sends |
| `max_body_size_mb` | No request body limit |
| `custom_headers` | Not applied |
| `http2_enabled` | HTTP/2 is always active; switch has no effect |
| `websocket_enabled` | Upgrades work regardless of the switch |

**Effective:** upstream, domains, locations, templates/inheritance, enabled, auth, cache, rate limit, GeoIP, HTTP/3 and Force HTTPS.

Next: [Certificates](https://github.com/Inboxes-CIG/Bastion/wiki/EN-Certificates)

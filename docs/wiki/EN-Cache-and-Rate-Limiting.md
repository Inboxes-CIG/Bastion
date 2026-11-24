# Cache & rate limiting

## Response cache

Bastion caches anonymous `GET` responses **in memory** (inside the process).

### Cache key

```
Host + request URI (path + query) + Accept-Encoding
```

Compressed and uncompressed variants are stored separately.

### When a response is cached

- Only `GET`
- No `Authorization` header
- No `Cache-Control: no-cache` / `Pragma: no-cache` in the request
- Status: `2xx` (except 206), `300–308` (except 304), `404`, `410`
- No `Set-Cookie`
- No `Vary` other than `Accept-Encoding`

### Backend headers

| Header | Behavior |
|---|---|
| `Cache-Control: s-maxage=N` | TTL = N (preferred) |
| `Cache-Control: max-age=N` | TTL = N |
| `Expires` | TTL until that time |
| `Age: N` | subtracted from the TTL |
| `no-store`, `no-cache`, `private` | **not** cached |
| `ETag` / `Last-Modified` | 304 responses from cache |
| `Vary: Accept-Encoding` | cacheable |
| other `Vary` fields | **not** cached |
| no freshness headers | configured TTL (fallback) |

### Limits and eviction

Global cache limits:

- `max_entries`
- `max_object_kb` per response
- `max_total_mb`

Expired entries are removed first, then the oldest, until the new entry fits. The cache is **not persistent** – a restart clears it. Only the configuration (`/var/lib/bastion/cache.json`) is persisted and overrides `config.yaml`.

### Observe

```sh
curl -sI https://app.example.com/ | grep -i -E 'x-cache|age'
```

- `X-Cache: MISS` on the first request
- `X-Cache: HIT` afterwards
- `Age` = seconds since it was stored

### Clear

- Global: **Settings → Response Cache → Clear cache**
- Per site: **Site → Cache → Clear this site's cache**
- Location changes clear the site cache automatically.
- Deleting a site does **not** clear the cache automatically.

### Enable

- Global: `cache.enabled` in `config.yaml`
- Per site/location: `cache_enabled` (inheritance)
- `cache_ttl` is only a fallback; backend headers win

## Rate limiting

Token bucket per **site + client IP**.

- `rate_limit_rps`: refill rate (requests/second); `0` disables
- `rate_limit_burst`: bucket size; `0` → same as `rps`
- Over limit: `429` with `Retry-After: 1`
- Inheritance: location → site → template
- The global allowlist skips rate limiting
- Admin hosts are exempt
- In-memory; a restart resets all buckets
- Idle buckets are cleaned up after 10 minutes

### Example

| Level | RPS | Burst |
|---|---|---|
| Template | 20 | 40 |
| Site | 10 | 20 |
| Location `/api` | 5 | 10 |

A client may send 5 requests/second (10 briefly) to `/api`; other paths use 10/20.

### Recommendations

- Static assets: generous or no limit
- Login/API: lower
- Combine with the 404 auto-ban and CrowdSec
- Behind a CDN/proxy: set `X-Forwarded-For` correctly (Bastion trusts the header)

Next: [Logs, audit & dashboard](https://github.com/Inboxes-CIG/Bastion/wiki/EN-Logs-Audit-and-Dashboard)

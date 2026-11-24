# Access & MFA

Bastion checks several protection layers in a fixed order.

## Check order

1. **Admin host?** → GUI/API, security filters skipped (lockout protection).
2. **Global IP deny list** → block.
3. **CrowdSec ban** → block.
4. **Resolve site** (domain/extra domains) → otherwise `502`.
5. **Site disabled** → `403`.
6. **Site IP rules** (deny → block, allow → auth bypass).
7. **Basic Auth** (site or location).
8. **GeoIP** (location/site/template + rules).
9. **Rate limit** (location/site/template).
10. **Cache** / reverse proxy to the upstream.

> **Admin hosts:** Any IP address used as a host plus all entries in `server.admin_hosts` are admin hosts. If the list is empty, `waf.inboxes.de` and `localhost` are used. Admin hosts are exempt from IP lists, CrowdSec, GeoIP and rate limiting.

## Global IP lists

**IP Lists** in the GUI, table `ip_list_entries`.

- `kind`: `allow` or `deny`
- `value`: IPv4, IPv6 or CIDR
- `note`: free text

Behavior:

- **Deny** blocks immediately (also during the TLS handshake).
- **Allow** skips CrowdSec, GeoIP and rate limiting — **not** the per-site login.
- The list reloads every 60 seconds; GUI changes take effect immediately.

## Site IP rules

Per site under **Site → Access**:

- **Deny** blocks the IP for this site.
- **Allow** skips the site login (Basic Auth), but **not** GeoIP or rate limiting.

The global whitelist does **not** bypass the site login.

## Basic Auth per site/location

- Users are managed per site (`SiteAuthUser`, bcrypt).
- `auth_enabled` can be set per site and per location.
- After **3 failed attempts**, the site+IP combination is locked for **5 minutes** → `429` with `Retry-After: 300`.
- A successful login resets the counter.
- Requests with an `Authorization` header bypass the response cache.

## Automatic 404 ban

Bastion counts **404 responses per client IP** inside a time window. When the threshold is exceeded, the IP is added automatically as a **deny** entry with note `404` to the global blocklist.

Defaults:

```yaml
security:
  auto_ban_404:
    enabled: true
    threshold: 50
    window_seconds: 600
```

- 50 × 404 within 10 minutes → ban.
- The ban is permanent until you delete the entry under **IP Lists**.
- **Not counted:** admin requests, globally allowlisted IPs, status ≠ 404.
- The ban is recorded in the **audit log** as `auto_ban`.
- The IP list reloads immediately; further requests are blocked.

**Tuning:** edit `/etc/bastion/config.yaml` and restart. Raise the threshold for busy APIs; lower it for static sites.

**Caution:** the ban is global across all sites. A single page with many missing assets could hit a legitimate IP, so do not set the threshold too low.

## TOTP MFA (optional)

Every GUI user can enable MFA under **Settings → Two-Factor Authentication (MFA)**.

- Standard TOTP (RFC 6238), 6 digits, 30 seconds
- QR code + manual key
- Secret stored encrypted with **AES-256-GCM** (key derived from the JWT secret)
- Two-step login: password → `mfa_token` (5 min) → code → tokens
- No recovery codes

**Reset MFA** (e.g. lost device):

```sql
UPDATE users SET mfa_enabled = false, mfa_secret = '' WHERE email = '<user-email>';
```

## Sessions

- Access token: 15 minutes
- Refresh token: 7 days
- MFA token: 5 minutes
- Logout removes the browser token; there is no server-side revocation list.

Next: [[EN-GeoIP-and-CrowdSec|GeoIP & CrowdSec]]

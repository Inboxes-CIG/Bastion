# First steps

This page takes you from a fresh install to the first protected site.

## 1. Prepare DNS

Create an A (IPv4) and optionally AAAA (IPv6) record for every site pointing to the server:

```
app.example.com.    A     203.0.113.10
```

Verify:

```sh
dig +short app.example.com
```

## 2. Open the GUI

- Via server IP: `https://203.0.113.10/`
- Or via a domain listed in `server.admin_hosts`

Accept the self-signed certificate warning once. Then sign in with `admin@bastion.local` and the password from the journal.

**Do this immediately:**

1. Change the password (Users → your user).
2. Enable MFA (Settings → Two-Factor Authentication), see [Access & MFA](https://github.com/Inboxes-CIG/Bastion/wiki/EN-Access-and-MFA).

## 3. Check the tenant

The tenant **Default** exists after the first start. Create more tenants for isolation if needed (superadmin only). Sites, certificates and users belong to a tenant.

## 4. Create the first site

**Sites → New site**

| Field | Example | Note |
|---|---|---|
| Domain | `app.example.com` | Primary domain |
| Extra domains | `www.app.example.com` | Aliases, same upstream |
| Upstream URL | `http://127.0.0.1:3000` | Backend target |
| Upstream host | `app.internal` | Optional `Host` header to the backend |
| Cache enabled | off | Start with off for dynamic apps |
| Basic Auth | off | Enable later if needed |
| GeoIP | off | Enable later if needed |

Save. Bastion resolves the site by `Host` immediately.

> The upstream must be reachable from the Bastion server. `127.0.0.1:<port>` is a good choice for local backends.

## 5. Set up a certificate

Two options:

- **Let's Encrypt** (Site → TLS & certificate → Issue): the domain must resolve to the server and port 80 must be reachable. Wildcards are **not** possible with HTTP-01.
- **Manual upload** (Certificates → Upload): paste certificate + key. The “Domain” field is the primary label; all SANs and wildcards in the PEM are served automatically.

Details: [Certificates](https://github.com/Inboxes-CIG/Bastion/wiki/EN-Certificates).

## 6. Test

```sh
# HTTPS + certificate
curl -I https://app.example.com/

# Cache status (if enabled)
curl -sI https://app.example.com/ | grep -i x-cache

# HTTP → HTTPS redirect
curl -I http://app.example.com/
```

Expected:

- `200`/`30x` from the backend
- a valid certificate for the domain
- `X-Cache: MISS` on the first and `X-Cache: HIT` on subsequent cacheable requests

## 7. Harden

Recommended order:

1. **Global IP allowlist** for office/VPN (skips CrowdSec/GeoIP/rate limit, **not** the site login).
2. **Basic Auth** per site for sensitive apps.
3. **GeoIP rules** (e.g. allow only DACH).
4. Keep **CrowdSec** enabled and watch alerts.
5. Configure **rate limiting** per site.
6. Enable **cache** only for cacheable responses.
7. Keep the **404 auto-ban** enabled (default) or tune its threshold.

## Checklist

- [ ] DNS points to the server
- [ ] Admin password changed
- [ ] MFA enabled
- [ ] Tenant/site created
- [ ] Certificate active and valid
- [ ] Upstream reachable
- [ ] Logs and dashboard show data
- [ ] Firewall: 80/443 open, 8080 internal

Next: [Sites & templates](https://github.com/Inboxes-CIG/Bastion/wiki/EN-Sites-and-Templates)

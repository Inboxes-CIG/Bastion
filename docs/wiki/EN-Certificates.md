# Certificates

Bastion obtains TLS certificates via **Let's Encrypt (HTTP-01)**, **manual upload**, or a self-signed bootstrap certificate for the admin UI.

## Let's Encrypt (HTTP-01)

Requirements:

- Domain resolves to the server
- Port 80 reachable from the internet
- Public domain (no `.local`, `.test`, IPs)
- ACME email address

In the GUI: **Certificates → Issue** or **Site → TLS & certificate → Issue**.

- One certificate with **all site domains** (primary + aliases) as SANs.
- Local/test domains are filtered out (`excluded` in the response).
- **Wildcards** (`*.example.com`) are **not** possible via HTTP-01; use manual upload instead.
- Renewal: on startup and every **12 hours**, certificates expiring within **30 days** are renewed.
- ACME storage: `/var/lib/bastion/certs/acme`

## Manual upload

**Certificates → Upload**

- Paste certificate (PEM, including chain) and private key (PEM).
- The **Domain** field is the primary label (CN/first domain).
- All **SANs** in the PEM are imported and served.
- **Wildcards** are supported: `*.example.com` matches `foo.example.com` (exactly one level).
- The apex (`example.com`) must also be a SAN if it should be served.
- Deleting a certificate clears the SNI cache; the next matching certificate or the admin bootstrap cert is used.

## Admin certificate

Issue a Let's Encrypt certificate for the GUI domain under **Settings → GUI certificate**. Admin domains come from `server.admin_hosts`; local IPs and `localhost` are served by a self-signed bootstrap certificate so the GUI is reachable right after installation.

## Verify

```sh
echo | openssl s_client -servername app.example.com -connect 127.0.0.1:443 2>/dev/null \
  | openssl x509 -noout -subject -issuer -dates

echo | openssl s_client -servername app.example.com -connect 127.0.0.1:443 2>/dev/null \
  | openssl x509 -noout -text | grep -A1 'Subject Alternative Name'
```

## SNI matching

1. Exact SNI match (`app.example.com`)
2. Wildcard fallback (`*.example.com` matches `foo.example.com`)
3. Otherwise: admin/bootstrap certificate

A domain listed only as an `extra_domain` but missing from the certificate will not be covered. Re-issue after domain changes.

## Troubleshooting

| Problem | Cause / fix |
|---|---|
| `precheck` fails | DNS does not point to the server or port 80 is blocked |
| `no domains provided` | No valid public domain left |
| Browser shows the wrong certificate | SNI domain is not in the certificate; check SANs |
| Wildcard does not work | HTTP-01 does not support wildcards → manual upload |
| GUI unreachable after install | Accept the self-signed cert or set `server.admin_hosts` |

Next: [[EN-Access-and-MFA|Access & MFA]]

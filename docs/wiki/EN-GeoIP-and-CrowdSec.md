# GeoIP & CrowdSec

## GeoIP

Bastion uses the **DB-IP Lite** database (MMDB) to map IPs to countries.

- Default path: `/var/lib/bastion/geoip/dbip-country-lite.mmdb`
- Update: `/usr/local/bin/bastion-geoip-update.sh`, monthly cron on the 1st at 03:00
- The service restarts after the download.
- If the database is missing, GeoIP filtering is **skipped** (fail open) and logged.

### Per-site/location filter

- `geoip_enabled`: filter active
- `geoip_mode`:
  - `block` → block selected countries
  - `allow` → allow only selected countries
- `geoip_countries`: ISO 3166-1 alpha-2 codes

Inheritance: location → site → template.

### Rules (tenant/site)

Additional rules under **GeoIP → Rules** (table `geo_ip_rules`):

| Field | Meaning |
|---|---|
| `tenant_id` | Tenant |
| `site_id` | Optional: this site only; `NULL` = whole tenant |
| `country_code` | ISO code or `*` |
| `action` | `block` or `allow` |
| `priority` | higher wins |

Evaluation: rules are ordered by `priority DESC`; the first match decides. Site-specific and tenant-wide rules are combined. Default is **allow**.

### Notes

- The global allowlist skips GeoIP.
- IPs without a country mapping are not blocked.
- The site/location country list is checked before the rules; if it does not block, the rules apply.

## CrowdSec

Bastion acts as a **bouncer** against the local CrowdSec LAPI.

- LAPI: `http://127.0.0.1:8081` (the installer moves CrowdSec from 8080 to 8081)
- API key: `/etc/bastion/config.yaml` → `crowdsec.api_key`
- Bouncer name: `bastion-bouncer`
- Refresh: every 30 seconds (configurable)
- Decisions of type `ban` are applied; `simulated` is ignored.
- Bans are checked both during the TLS handshake and per request.
- The global allowlist skips CrowdSec bans.
- Admin hosts are exempt.

### Manage bans

**CrowdSec → Decisions** in the GUI:

- Add ban: IP or CIDR, duration (e.g. `1h`, `24h`, `30m`), reason
- Delete ban
- Show recent alerts

Behind the scenes `cscli` is used. Manually:

```sh
cscli decisions list
cscli decisions add --ip 203.0.113.10 --duration 24h --reason "manual"
cscli decisions delete --ip 203.0.113.10
cscli alerts list --limit 20
```

### Good actors (whitelist)

**CrowdSec → Good Actors** maintains a whitelist that CrowdSec will not attack:

- Entries are written to `/var/lib/bastion/good-actors.yaml`.
- A systemd path unit copies the file to `/etc/crowdsec/parsers/s02-enrich/bastion-good-actors.yaml` and reloads CrowdSec.
- Supports IPs and CIDRs.
- This whitelist is **independent** of the Bastion IP allowlist: good actors prevent CrowdSec decisions; the Bastion allowlist additionally skips GeoIP/rate limiting (but not the site login).

### Check status

```sh
systemctl status crowdsec
cscli bouncers list
cscli decisions list
journalctl -u bastion | grep -i crowdsec
```

The GUI shows connection status, last sync, errors and decision count under **CrowdSec → Status**.

Next: [[EN-Cache-and-Rate-Limiting|Cache & rate limiting]]

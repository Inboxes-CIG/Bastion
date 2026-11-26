# Installation & upgrade

## Requirements

- Linux amd64 or arm64 (tested on Debian/Ubuntu)
- Root access
- Publicly reachable ports **80** and **443**
- DNS A/AAAA records for the domains you want to proxy
- PostgreSQL is installed by the installer if missing
- For Let's Encrypt: public domain, port 80 reachable (HTTP-01)

## Quick install

```sh
curl -fsSL https://raw.githubusercontent.com/Inboxes-CIG/Bastion/main/get.sh | sudo bash
```

`get.sh` detects the architecture, downloads the latest release tarball and runs `install.sh`.

### Installer options

```sh
curl -fsSL https://raw.githubusercontent.com/Inboxes-CIG/Bastion/main/get.sh | sudo bash -s -- \
  --hostname bastion.example.com \
  --admin-email admin@example.com
```

| Option | Meaning |
|---|---|
| `--hostname <fqdn>` | Admin hostname (GUI + self-signed certificate). Asked interactively if omitted; without a terminal `hostname -f` is used. |
| `--admin-email <email>` | Email of the first superadmin. Asked interactively if omitted; defaults to `admin@<hostname>` or `admin@bastion.local`. |
| `--admin-password <pass>` | Optional password. If omitted, a password is generated and printed to the journal. |

The options can also be set via `BASTION_HOSTNAME`, `BASTION_ADMIN_EMAIL` and `BASTION_ADMIN_PASSWORD`.

## Manual install

1. Download the release tarball for your architecture (`bastion_linux_amd64.tar.gz` or `bastion_linux_arm64.tar.gz`).
2. Extract and install:

```sh
tar -xzf bastion_linux_amd64.tar.gz -C /tmp/bastion-install
cd /tmp/bastion-install
sudo ./install.sh
```

## What the installer does

- Installs PostgreSQL (apt) and creates role/database `waf` (internal name, kept for compatibility)
- Creates the `bastion` system user and `/opt/bastion`, `/etc/bastion`, `/var/lib/bastion`
- Generates `/etc/bastion/config.yaml` and `/etc/bastion/.secrets.env` (first run only)
- Installs and enables `bastion.service`
- Moves the CrowdSec LAPI to `127.0.0.1:8081`, restarts CrowdSec and verifies that port **8080** is free for Bastion
- Creates the `bastion-bouncer`
- Installs `/usr/local/bin/bastion-geoip-update.sh` and a monthly cron job (`0 3 1 * *`)
- Asks for hostname and initial admin (or uses the options) and creates the superadmin on first start

## First login

The initial password is written **once** to the journal:

```sh
journalctl -u bastion | grep -A4 'INITIAL SUPERADMIN'
```

- Email: the address provided to the installer (default: `admin@<hostname>` or `admin@bastion.local`)
- GUI: `https://<hostname>/` or `https://<server-ip>/` (accept the self-signed certificate)
- The password is generated on first start and printed to the journal if none was supplied
- The first login shows a **mandatory dialog** that forces a password change (`must_change_password`)
- Private/internal upstreams: the upstream allowlist is empty by default – add entries per tenant under **Tenants → upstream allowlist** (e.g. `10.0.0.0/8`)

## Upgrade

The installer is idempotent: it replaces the binary and keeps configuration, data and the database.

```sh
curl -fsSL https://raw.githubusercontent.com/Inboxes-CIG/Bastion/main/get.sh | sudo bash
```

Manual upgrade:

```sh
sudo cp bastion /opt/bastion/bin/bastion
sudo chown bastion:bastion /opt/bastion/bin/bastion
sudo systemctl restart bastion
```

> The version is injected at release build time via `-ldflags`. A binary built without `-X .../version.Version=...` reports `dev`.

## Uninstall

```sh
sudo systemctl disable --now bastion
sudo rm -f /etc/systemd/system/bastion.service
sudo systemctl daemon-reload
sudo rm -rf /opt/bastion /etc/bastion /var/lib/bastion
sudo userdel bastion 2>/dev/null || true
```

The database remains. To remove everything:

```sh
sudo -u postgres dropdb waf
sudo -u postgres dropuser waf
```

Remove CrowdSec whitelist units and the GeoIP cron job as needed:

```sh
sudo systemctl disable --now crowdsec-bastion-whitelist.path
sudo rm -f /etc/systemd/system/crowdsec-bastion-whitelist.* /usr/local/bin/bastion-geoip-update.sh
sudo rm -f /etc/crowdsec/parsers/s02-enrich/bastion-good-actors.yaml
```

## Firewall

| Port | Access |
|---|---|
| 80/tcp | public (ACME + redirect) |
| 443/tcp, 443/udp | public (HTTPS + HTTP/3) |
| 8080/tcp | **internal only** (API, HTTP) |
| 8081/tcp | localhost only (CrowdSec LAPI) |

Example with ufw:

```sh
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 443/udp
sudo ufw deny 8080/tcp
```

Next: [First steps](https://github.com/Inboxes-CIG/Bastion/wiki/EN-First-Steps)

# Installation & Upgrade

## Voraussetzungen

- Linux amd64 oder arm64 (getestet auf Debian/Ubuntu)
- Root-Rechte
- Öffentlich erreichbare Ports **80** und **443**
- DNS-Einträge (A/AAAA) für die Domains, die du proxen willst
- PostgreSQL wird vom Installer installiert, falls nicht vorhanden
- Für Let's Encrypt: öffentliche Domain, Port 80 erreichbar (HTTP-01)

## Schnellinstallation

```sh
curl -fsSL https://raw.githubusercontent.com/Inboxes-CIG/Bastion/main/get.sh | sudo bash
```

`get.sh` erkennt die Architektur, lädt das neueste Release-Tarball und führt `install.sh` aus.

## Manuelle Installation

1. Release-Tarball für deine Architektur herunterladen (`bastion_linux_amd64.tar.gz` oder `bastion_linux_arm64.tar.gz`).
2. Entpacken und installieren:

```sh
tar -xzf bastion_linux_amd64.tar.gz -C /tmp/bastion-install
cd /tmp/bastion-install
sudo ./install.sh
```

## Was der Installer macht

- Installiert PostgreSQL (apt) und legt Rolle/Datenbank `waf` an (interner Name, bleibt aus Kompatibilitätsgründen bestehen)
- Legt Systembenutzer `bastion` und Verzeichnisse `/opt/bastion`, `/etc/bastion`, `/var/lib/bastion` an
- Erzeugt `/etc/bastion/config.yaml` und `/etc/bastion/.secrets.env` mit Zufalls-Secrets (nur beim ersten Lauf)
- Installiert und aktiviert `bastion.service`
- Installiert CrowdSec, verschiebt die LAPI auf `127.0.0.1:8081` und legt den Bouncer `bastion-bouncer` an
- Installiert `/usr/local/bin/bastion-geoip-update.sh` und einen monatlichen Cronjob (`0 3 1 * *`)
- Legt beim ersten Start den Superadmin `admin@bastion.local` an

## Erster Login

Das Initialpasswort wird **einmalig** ins Journal geschrieben:

```sh
journalctl -u bastion | grep -A4 'INITIAL SUPERADMIN'
```

- E-Mail: `admin@bastion.local`
- GUI: `https://<server-ip>/` (selbstsigniertes Zertifikat akzeptieren) oder über einen Eintrag in `server.admin_hosts`
- Passwort nach dem ersten Login ändern

## Upgrade

Der Installer ist idempotent: Er ersetzt das Binary und behält Konfiguration, Daten und Datenbank.

```sh
curl -fsSL https://raw.githubusercontent.com/Inboxes-CIG/Bastion/main/get.sh | sudo bash
```

Alternativ manuell:

```sh
sudo cp bastion /opt/bastion/bin/bastion
sudo chown bastion:bastion /opt/bastion/bin/bastion
sudo systemctl restart bastion
```

> Die Version wird beim Release-Build per `-ldflags` gesetzt. Ein selbst gebautes Binary ohne `-X .../version.Version=...` meldet `dev`.

## Deinstallation

```sh
sudo systemctl disable --now bastion
sudo rm -f /etc/systemd/system/bastion.service
sudo systemctl daemon-reload
sudo rm -rf /opt/bastion /etc/bastion /var/lib/bastion
sudo userdel bastion 2>/dev/null || true
```

Die Datenbank bleibt erhalten. Zum vollständigen Entfernen:

```sh
sudo -u postgres dropdb waf
sudo -u postgres dropuser waf
```

CrowdSec-Whitelist-Units und GeoIP-Cronjob bei Bedarf zusätzlich entfernen:

```sh
sudo systemctl disable --now crowdsec-bastion-whitelist.path
sudo rm -f /etc/systemd/system/crowdsec-bastion-whitelist.* /usr/local/bin/bastion-geoip-update.sh
sudo rm -f /etc/crowdsec/parsers/s02-enrich/bastion-good-actors.yaml
```

## Firewall

| Port | Zugriff |
|---|---|
| 80/tcp | öffentlich (ACME + Redirect) |
| 443/tcp, 443/udp | öffentlich (HTTPS + HTTP/3) |
| 8080/tcp | **nur intern** (API, HTTP) |
| 8081/tcp | nur localhost (CrowdSec LAPI) |

Beispiel mit ufw:

```sh
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 443/udp
sudo ufw deny 8080/tcp
```

Weiter: [Erste Schritte](https://github.com/Inboxes-CIG/Bastion/wiki/DE-Erste-Schritte)

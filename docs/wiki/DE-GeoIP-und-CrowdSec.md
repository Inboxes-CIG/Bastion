# GeoIP & CrowdSec

## GeoIP

Bastion nutzt die **DB-IP Lite**-Datenbank (MMDB), um Länder zuzuordnen.

- Standardpfad: `/var/lib/bastion/geoip/dbip-country-lite.mmdb`
- Aktualisierung: `/usr/local/bin/bastion-geoip-update.sh`, Cronjob monatlich am 1. um 03:00
- Nach dem Download wird der Dienst neu gestartet.
- Fehlt die Datenbank, wird der GeoIP-Filter **übersprungen** (fail open) und im Log vermerkt.

### Filter pro Site/Location

- `geoip_enabled`: Filter aktiv
- `geoip_mode`:
  - `block` → ausgewählte Länder sperren
  - `allow` → nur ausgewählte Länder erlauben
- `geoip_countries`: Liste von ISO-3166-1-alpha-2-Codes

Vererbung: Location → Site → Template.

### Regeln (Tenant/Site)

Zusätzlich gibt es Regeln unter **GeoIP → Regeln** (Tabelle `geo_ip_rules`):

| Feld | Bedeutung |
|---|---|
| `tenant_id` | Mandant |
| `site_id` | Optional: nur diese Site; `NULL` = ganze Tenant |
| `country_code` | ISO-Code oder `*` |
| `action` | `block` oder `allow` |
| `priority` | höher gewinnt |

Auswertung: Regeln werden nach `priority DESC` sortiert; die erste passende Regel entscheidet. Site-spezifische und tenant-weite Regeln werden zusammen betrachtet. Standard ist **allow**.

### Hinweise

- Die globale Allowlist überspringt GeoIP.
- IP-Adressen ohne Länderzuordnung werden nicht blockiert.
- Die Site/Location-Länderliste wird vor den Regeln geprüft; ist sie nicht blockierend, greifen die Regeln.

## CrowdSec

Bastion ist als **Bouncer** an die lokale CrowdSec-LAPI angebunden.

- LAPI: `http://127.0.0.1:8081` (der Installer verschiebt CrowdSec von 8080 auf 8081)
- API-Key: `/etc/bastion/config.yaml` → `crowdsec.api_key`
- Bouncer-Name: `bastion-bouncer`
- Refresh: alle 30 Sekunden (konfigurierbar)
- Entscheidungen vom Typ `ban` werden angewendet; `simulated` wird ignoriert.
- Bans werden sowohl beim TLS-Handshake als auch pro Request geprüft.
- Die globale Allowlist überspringt CrowdSec-Bans.
- Admin-Hosts sind ausgenommen.

### Bans verwalten

**CrowdSec → Decisions** in der GUI:

- Ban hinzufügen: IP oder CIDR, Dauer (z. B. `1h`, `24h`, `30m`), Grund
- Ban löschen
- Alerts der letzten Zeit anzeigen

Hinter den Kulissen wird `cscli` verwendet. Manuell:

```sh
cscli decisions list
cscli decisions add --ip 203.0.113.10 --duration 24h --reason "manual"
cscli decisions delete --ip 203.0.113.10
cscli alerts list --limit 20
```

### Good Actors (Whitelist)

**CrowdSec → Good Actors** pflegt eine Whitelist, die CrowdSec nicht mehr angreift:

- Einträge werden nach `/var/lib/bastion/good-actors.yaml` geschrieben.
- Ein systemd-Path-Unit kopiert die Datei nach `/etc/crowdsec/parsers/s02-enrich/bastion-good-actors.yaml` und lädt CrowdSec neu.
- Unterstützt IPs und CIDRs.
- Diese Whitelist ist **unabhängig** von der Bastion-IP-Allowlist: Good Actors verhindern CrowdSec-Entscheidungen, die Bastion-Allowlist überspringt zusätzlich GeoIP/Rate Limit (aber nicht den Site-Login).

### Status prüfen

```sh
systemctl status crowdsec
cscli bouncers list
cscli decisions list
journalctl -u bastion | grep -i crowdsec
```

In der GUI zeigt **CrowdSec → Status** Verbindung, letzten Sync, Fehler und Anzahl der Entscheidungen.

Weiter: [Cache & Rate Limiting](https://github.com/Inboxes-CIG/Bastion/wiki/DE-Cache-und-Rate-Limiting)

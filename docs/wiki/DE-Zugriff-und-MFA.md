# Zugriff & MFA

Bastion prüft mehrere Schutzschichten in fester Reihenfolge.

## Prüfreihenfolge

1. **Admin-Host?** → GUI/API, Sicherheitsfilter werden übersprungen (Aussperrschutz).
2. **Globale IP-Deny-Liste** → Block.
3. **CrowdSec-Ban** → Block.
4. **Site auflösen** (Domain/Extra-Domains) → sonst `502`.
5. **Site deaktiviert** → `403`.
6. **Site-IP-Regeln** (Deny → Block, Allow → Auth-Bypass).
7. **Basic Auth** (Site oder Location).
8. **GeoIP** (Location/Site/Template + Regeln).
9. **Rate Limit** (Location/Site/Template).
10. **Cache** / Reverse Proxy zum Upstream.

> **Admin-Hosts:** Jede IP-Adresse als Host sowie alle Einträge in `server.admin_hosts` gelten als Admin-Host. Ist die Liste leer, gelten `localhost` und ein interner Fallback; setze `server.admin_hosts` explizit. Admin-Hosts sind von IP-Listen, CrowdSec, GeoIP und Rate Limit ausgenommen.

## Globale IP-Listen

**IP Lists** in der GUI, Tabelle `ip_list_entries`.

- `kind`: `allow` oder `deny`
- `value`: IPv4, IPv6 oder CIDR
- `note`: freier Text

Verhalten:

- **Deny** blockt sofort (auch beim TLS-Handshake).
- **Allow** überspringt CrowdSec, GeoIP und Rate Limit – **nicht** den per-Site-Login.
- Die Liste wird alle 60 Sekunden neu geladen; Änderungen über die GUI sind sofort aktiv.

## Site-IP-Regeln

Pro Site unter **Site → Zugriff**:

- **Deny** blockt die IP für diese Site.
- **Allow** überspringt den Site-Login (Basic Auth), aber **nicht** GeoIP oder Rate Limit.

Für Locations gibt es `ip_rules_mode`:

- `inherit` → Site-Regeln gelten
- `own` → nur die eigenen Regeln der Location gelten (Site-Regeln werden **ersetzt**, nicht ergänzt)

Location-Regeln werden über **Site → Zugriff → Zugriffsebene** verwaltet. Ein Allow der Location überspringt den Login dieser Location; ein Deny blockt sie. Die globale Whitelist hebt den Site-Login **nicht** auf.

## Basic Auth pro Site/Location

- Benutzer werden pro Site gepflegt (`SiteAuthUser`, bcrypt).
- `auth_enabled` kann pro Site und pro Location gesetzt werden.
- Für Locations gibt es `auth_users_mode`:
  - `inherit` → Site-Benutzer
  - `own` → eigene Benutzerliste der Location
- Nach **3 Fehlversuchen** wird die Kombination Site+Location+IP für **5 Minuten** gesperrt → `429` mit `Retry-After: 300`.
- Ein erfolgreicher Login setzt den Zähler zurück.
- Requests mit `Authorization`-Header umgehen den Response-Cache.

**Verwaltung:** **Site → Zugriff → Zugriffsebene** (Site oder Location). Dort werden Benutzer und IP-Regeln der gewählten Ebene angezeigt und bearbeitet. Die Modi `inherit`/`own` werden im Tab **Locations** gesetzt.

## Automatischer 404-Ban

Bastion zählt **404-Antworten pro Client-IP** in einem Zeitfenster. Wird die Schwelle überschritten, landet die IP automatisch als **Deny**-Eintrag mit Notiz `404` in der globalen Blocklist.

Standardwerte:

```yaml
security:
  auto_ban_404:
    enabled: true
    threshold: 50
    window_seconds: 600
```

- 50 × 404 innerhalb von 10 Minuten → Ban.
- Der Ban ist dauerhaft, bis du den Eintrag unter **IP Lists** löschst.
- **Nicht gezählt:** Admin-Requests, globale Allowlist-IPs, Status ≠ 404.
- Der Ban wird im **Audit-Log** als `auto_ban` festgehalten.
- Nach dem Ban wird die IP-Liste sofort neu geladen; weitere Requests werden blockiert.

**Anpassen:** `/etc/bastion/config.yaml` bearbeiten und `systemctl restart bastion`. Für stark frequentierte APIs ggf. höher setzen; für statische Sites niedriger.

**Achtung:** Der Ban gilt global für alle Sites. Eine einzelne Seite mit vielen fehlenden Assets kann eine IP treffen – Schwelle deshalb nicht zu niedrig wählen.

## TOTP-MFA (optional)

Jeder GUI-Benutzer kann MFA unter **Settings → Zwei-Faktor-Authentifizierung (MFA)** aktivieren.

- Standard-TOTP (RFC 6238), 6 Stellen, 30 Sekunden
- QR-Code + manueller Schlüssel
- Secret wird mit **AES-256-GCM** verschlüsselt gespeichert (Key aus dem JWT-Secret abgeleitet)
- Login wird zweistufig: Passwort → `mfa_token` (5 Min.) → Code → Tokens
- Keine Recovery-Codes

**MFA zurücksetzen** (z. B. Gerät verloren):

```sql
UPDATE users SET mfa_enabled = false, mfa_secret = '' WHERE email = '<user-email>';
```

## Sessions

- Access-Token: 15 Minuten
- Refresh-Token: 7 Tage
- MFA-Token: 5 Minuten
- Logout entfernt das Token im Browser; es gibt keine serverseitige Token-Sperrliste.

Weiter: [GeoIP & CrowdSec](https://github.com/Inboxes-CIG/Bastion/wiki/DE-GeoIP-und-CrowdSec)

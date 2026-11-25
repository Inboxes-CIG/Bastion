# Benutzer, Tenants & Rollen

## Tenants

Ein Tenant ist eine Mandantengrenze für Sites, Zertifikate, Templates, GeoIP-Regeln und Benutzer.

- Beim ersten Start wird **Default** angelegt.
- Nur `superadmin` kann Tenants anlegen/ändern/löschen.
- Tenants haben `allowed_upstream_hosts`: Hosts, `*.domains` oder CIDRs, die als privates Upstream-Ziel erlaubt sind (SSRF-Allowlist).
- Der Superadmin hat `tenant_id = NULL` und sieht alles.
- Andere Benutzer sehen nur Daten ihres Tenants (Listen und Stats sind gefiltert; fremde Ressourcen liefern 404).

## Rollen

| Rolle | Scope | Rechte (Kurzfassung) |
|---|---|---|
| `superadmin` | system | Alles, inkl. Tenants |
| `tenant_admin` | tenant | Sites, Templates, Zertifikate, Benutzer, GeoIP, Site-Cache im eigenen Tenant; globale IP-Listen, CrowdSec-Schreibzugriffe, Cache-Konfiguration und Tenants bleiben Superadmin |
| `editor` | tenant | Lesen (schreibende API-Endpunkte sind auf superadmin/tenant_admin beschränkt) |
| `viewer` | tenant | Lesen |

Die Rollen sind in der Tabelle `roles` mit einer JSON-Permission-Liste gespeichert. Die API prüft für Schreibzugriffe derzeit die **Rolle** (`superadmin`/`tenant_admin`), nicht die einzelne Permission.

## Benutzer

Tabelle `users`:

| Feld | Bedeutung |
|---|---|
| `email` | Login, eindeutig |
| `password_hash` | bcrypt |
| `role_id` | Rolle |
| `tenant_id` | Mandant (NULL für Superadmin) |
| `active` | aktiv/inaktiv |
| `must_change_password` | erzwungene Passwortänderung beim nächsten Login |
| `mfa_enabled` / `mfa_secret` | TOTP |
| `last_login` | letzter erfolgreicher Login |

- Benutzer werden unter **Users** verwaltet.
- `tenant_admin` darf Benutzer im eigenen Tenant anlegen/ändern/löschen.
- Beim Löschen wird soft-deleted (`deleted_at`). Die E-Mail bleibt durch den Unique-Index blockiert; für eine erneute Verwendung derselben E-Mail muss der Datensatz hart gelöscht werden:

```sql
DELETE FROM users WHERE email = '<email>';
```

- Passwörter werden mit bcrypt gehasht. Beim **Anlegen** eines Benutzers ist das Passwort Pflicht; der Benutzer muss es beim ersten Login in einem Pflichtdialog ändern (`must_change_password`).
- Ein Reset erfolgt durch Setzen eines neuen Passworts in der GUI – das neue Passwort gilt dann wieder als „zu ändern".
- **Reauth:** Sensible Änderungen (Rolle, E-Mail, Tenant, Status, Passwort) erfordern zusätzlich das Passwort des administrierenden Kontos; ohne oder mit falschem Passwort antwortet die API mit `403`.

## Sessions und Tokens

- **Access-Token**: JWT, 15 Minuten
- **Refresh-Token**: JWT, 7 Tage, **rotierend** – jedes benutzte Token wird serverseitig gesperrt (Tabelle `refresh_tokens`); Wiederverwendung eines alten Tokens widerruft die ganze Token-Familie (Diebstahlerkennung)
- **MFA-Token**: JWT, 5 Minuten, nur für den zweiten Login-Schritt
- Signatur: HS256 mit `auth.jwt_secret`
- Logout und Passwort-Änderung widerrufen alle Refresh-Tokens des Kontos; Access-Tokens laufen bis zum Zeitablauf weiter.
- Login-Lockout: 3 Fehlversuche / 10 Minuten pro IP **und** E-Mail → `429`.
- Bei kompromittiertem `jwt_secret` alle Benutzer abmelden, indem das Secret in `/etc/bastion/config.yaml` geändert und der Dienst neu gestartet wird.

## MFA

Siehe [Zugriff & MFA](https://github.com/Inboxes-CIG/Bastion/wiki/DE-Zugriff-und-MFA).

## Erstinstallation

- Superadmin: `admin@bastion.local`
- Passwort: einmalig im Journal (`journalctl -u bastion | grep -A4 'INITIAL SUPERADMIN'`)
- Nach dem ersten Login erscheint der Pflichtdialog **Passwort ändern erforderlich**; danach MFA aktivieren.

Weiter: [Konfiguration & Betrieb](https://github.com/Inboxes-CIG/Bastion/wiki/DE-Konfiguration-und-Betrieb)

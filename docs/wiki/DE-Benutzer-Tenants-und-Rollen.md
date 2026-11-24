# Benutzer, Tenants & Rollen

## Tenants

Ein Tenant ist eine Mandantengrenze für Sites, Zertifikate, Templates, GeoIP-Regeln und Benutzer.

- Beim ersten Start wird **Default** angelegt.
- Nur `superadmin` kann Tenants anlegen/ändern/löschen.
- Der Superadmin hat `tenant_id = NULL` und sieht alles.
- Andere Benutzer sehen nur Daten ihres Tenants (Listen und Stats sind gefiltert).

## Rollen

| Rolle | Scope | Rechte (Kurzfassung) |
|---|---|---|
| `superadmin` | system | Alles, inkl. Tenants |
| `tenant_admin` | tenant | Sites, Templates, Zertifikate, Benutzer, GeoIP, IP-Listen, Cache, CrowdSec im eigenen Tenant |
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
| `mfa_enabled` / `mfa_secret` | TOTP |
| `last_login` | letzter erfolgreicher Login |

- Benutzer werden unter **Users** verwaltet.
- `tenant_admin` darf Benutzer im eigenen Tenant anlegen/ändern/löschen.
- Beim Löschen wird soft-deleted (`deleted_at`). Die E-Mail bleibt durch den Unique-Index blockiert; für eine erneute Verwendung derselben E-Mail muss der Datensatz hart gelöscht werden:

```sql
DELETE FROM users WHERE email = '<email>';
```

- Passwörter werden mit bcrypt gehasht; ein Reset erfolgt durch Setzen eines neuen Passworts in der GUI.

## Sessions und Tokens

- **Access-Token**: JWT, 15 Minuten
- **Refresh-Token**: JWT, 7 Tage
- **MFA-Token**: JWT, 5 Minuten, nur für den zweiten Login-Schritt
- Signatur: HS256 mit `auth.jwt_secret`
- Kein serverseitiges Session-Backend; Logout entfernt das Token im Browser.
- Bei kompromittiertem `jwt_secret` alle Benutzer abmelden, indem das Secret in `/etc/bastion/config.yaml` geändert und der Dienst neu gestartet wird.

## MFA

Siehe [[DE-Zugriff-und-MFA|Zugriff & MFA]].

## Erstinstallation

- Superadmin: `admin@bastion.local`
- Passwort: einmalig im Journal (`journalctl -u bastion | grep -A4 'INITIAL SUPERADMIN'`)
- Nach dem ersten Login Passwort ändern und MFA aktivieren.

Weiter: [[DE-Konfiguration-und-Betrieb|Konfiguration & Betrieb]]

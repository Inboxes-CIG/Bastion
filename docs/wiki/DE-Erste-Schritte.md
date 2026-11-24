# Erste Schritte

Diese Seite führt von der frischen Installation bis zur ersten geschützten Site.

## 1. DNS vorbereiten

Lege für jede Site einen A- (IPv4) und optional AAAA-Eintrag (IPv6) auf die Server-IP an:

```
app.example.com.    A     203.0.113.10
```

Warte, bis die Auflösung propagiert ist:

```sh
dig +short app.example.com
```

## 2. GUI öffnen

- Über die Server-IP: `https://203.0.113.10/`
- Oder über eine Domain, die in `server.admin_hosts` steht

Beim selbstsignierten Zertifikat muss die Browser-Warnung einmal akzeptiert werden. Danach mit `admin@bastion.local` und dem Passwort aus dem Journal anmelden.

**Sofort erledigen:**

1. Passwort ändern (Users → eigener Benutzer).
2. MFA aktivieren (Settings → Zwei-Faktor-Authentifizierung), siehe [Zugriff & MFA](https://github.com/Inboxes-CIG/Bastion/wiki/DE-Zugriff-und-MFA).

## 3. Tenant prüfen

Beim ersten Start existiert der Tenant **Default**. Für getrennte Umgebungen kannst du weitere Tenants anlegen (nur Superadmin). Sites, Zertifikate und Benutzer werden einem Tenant zugeordnet.

## 4. Erste Site anlegen

**Sites → Neue Site**

| Feld | Beispiel | Hinweis |
|---|---|---|
| Domain | `app.example.com` | Primäre Domain |
| Weitere Domains | `www.app.example.com` | Aliase, gleicher Upstream |
| Upstream-URL | `http://127.0.0.1:3000` | Ziel-Backend |
| Upstream-Host | `app.internal` | Optional: `Host`-Header zum Backend |
| Cache aktiv | aus | Für dynamische Apps zunächst aus |
| Basic Auth | aus | Später bei Bedarf |
| GeoIP | aus | Später bei Bedarf |

Speichern. Bastion löst die Site ab sofort über den `Host`-Header auf.

> Der Upstream muss vom Bastion-Server aus erreichbar sein. Für Backends auf demselben Host hat sich `127.0.0.1:<port>` bewährt.

## 5. Zertifikat einrichten

**Zwei Wege:**

- **Let's Encrypt** (Site → TLS & Zertifikat → Ausstellen): Domain muss öffentlich auf den Server zeigen und Port 80 erreichbar sein. Wildcards sind über HTTP-01 **nicht** möglich.
- **Manueller Upload** (Certificates → Upload): Zertifikat + Key einfügen. Das Feld „Domain" ist die primäre Bezeichnung; alle SANs und Wildcards aus dem PEM werden automatisch ausgeliefert.

Details: [Zertifikate](https://github.com/Inboxes-CIG/Bastion/wiki/DE-Zertifikate).

## 6. Testen

```sh
# HTTPS + Zertifikat
curl -I https://app.example.com/

# Cache-Status (falls Cache aktiv)
curl -sI https://app.example.com/ | grep -i x-cache

# HTTP → HTTPS Redirect
curl -I http://app.example.com/
```

Erwartung:

- `200`/`30x` vom Backend
- gültiges Zertifikat für die Domain
- `X-Cache: MISS` beim ersten, `X-Cache: HIT` bei folgenden cachebaren Requests

## 7. Absichern

Empfohlene Reihenfolge:

1. **Globale IP-Allowlist** für Büro/VPN (überspringt CrowdSec/GeoIP/Rate-Limit, **nicht** den Site-Login).
2. **Basic Auth** pro Site für sensible Anwendungen.
3. **GeoIP-Regeln** (z. B. nur DACH erlauben).
4. **CrowdSec** aktiv lassen und Alerts beobachten.
5. **Rate Limiting** pro Site konfigurieren.
6. **Cache** nur für cachebare Antworten aktivieren.

## Checkliste

- [ ] DNS zeigt auf den Server
- [ ] Admin-Passwort geändert
- [ ] MFA aktiviert
- [ ] Tenant/Site angelegt
- [ ] Zertifikat aktiv und gültig
- [ ] Upstream erreichbar
- [ ] Logs und Dashboard liefern Daten
- [ ] Firewall: 80/443 offen, 8080 intern

Weiter: [Sites & Templates](https://github.com/Inboxes-CIG/Bastion/wiki/DE-Sites-und-Templates)

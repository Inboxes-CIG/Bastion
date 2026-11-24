# Zertifikate

Bastion bezieht TLS-Zertifikate über **Let's Encrypt (HTTP-01)**, per **manueller Upload** oder als selbstsigniertes Bootstrap-Zertifikat für die Admin-Oberfläche.

## Let's Encrypt (HTTP-01)

Voraussetzungen:

- Domain zeigt per DNS auf den Server
- Port 80 ist von außen erreichbar
- Domain ist öffentlich (keine `.local`, `.test`, IPs)
- E-Mail-Adresse für ACME

Ablauf in der GUI: **Site → TLS & Zertifikat → Ausstellen**. Die Seite **Certificates** dient nur dem manuellen Upload; die Ausstellung erfolgt pro Site.

- Es wird **ein Zertifikat mit allen Domains der Site** (Primary + Aliase) als SANs ausgestellt.
- Lokale/Test-Domains werden vorher aussortiert (`excluded` in der Antwort).
- **Wildcards** (`*.example.com`) sind über HTTP-01 **nicht** möglich – dafür ist DNS-01 nötig. Nutze in diesem Fall den manuellen Upload.
- Erneuerung: Beim Start und danach alle **12 Stunden** prüft Bastion Zertifikate, die in **30 Tagen** ablaufen, und erneuert sie automatisch.
- ACME-Storage: `/var/lib/bastion/certs/acme`

## Manueller Upload

**Certificates → Upload**

- Zertifikat (PEM, inkl. Kette) und privater Schlüssel (PEM) einfügen.
- Das Feld **Domain** ist die primäre Bezeichnung (CN/erste Domain).
- Beim Upload werden **alle SANs** aus dem PEM übernommen und ausgeliefert.
- **Wildcards** werden unterstützt: `*.example.com` matcht `foo.example.com` (genau eine Ebene).
- Die Apex-Domain (`example.com`) muss zusätzlich als SAN im Zertifikat stehen, wenn sie bedient werden soll.
- Beim Löschen eines Zertifikats wird der SNI-Cache bereinigt; danach greift das nächste passende Zertifikat oder das Admin-Bootstrap-Zertifikat.

## Admin-Zertifikat

Für die GUI-Domain kannst du unter **Settings → GUI-Zertifikat** ein Let's-Encrypt-Zertifikat ausstellen. Die Admin-Domains kommen aus `server.admin_hosts`; zusätzlich werden lokale IPs und `localhost` mit einem selbstsignierten Bootstrap-Zertifikat bedient, damit die GUI nach der Installation erreichbar ist.

## Prüfen

```sh
# Welches Zertifikat liefert der Server für eine Domain?
echo | openssl s_client -servername app.example.com -connect 127.0.0.1:443 2>/dev/null \
  | openssl x509 -noout -subject -issuer -dates

# SANs anzeigen
echo | openssl s_client -servername app.example.com -connect 127.0.0.1:443 2>/dev/null \
  | openssl x509 -noout -text | grep -A1 'Subject Alternative Name'
```

## SNI-Matching

1. Exakter Treffer der SNI (`app.example.com`)
2. Wildcard-Fallback (`*.example.com` matcht `foo.example.com`)
3. Sonst: Admin-/Bootstrap-Zertifikat

Es gibt **kein** Zertifikat für eine Domain, die nur als `extra_domain` eingetragen, aber nicht im Zertifikat enthalten ist. Nach Domain-Änderungen daher neu ausstellen.

## Fehlerbehebung

| Problem | Ursache / Lösung |
|---|---|
| `precheck` schlägt fehl | DNS zeigt nicht auf den Server oder Port 80 blockiert |
| `no domains provided` | Keine gültige öffentliche Domain übrig |
| Browser zeigt falsches Zertifikat | SNI-Domain nicht im Zertifikat; SANs prüfen |
| Wildcard funktioniert nicht | HTTP-01 unterstützt keine Wildcards → manueller Upload |
| Admin-GUI nach Installation nicht erreichbar | Selbstsigniertes Zertifikat akzeptieren oder `server.admin_hosts` setzen |

Weiter: [Zugriff & MFA](https://github.com/Inboxes-CIG/Bastion/wiki/DE-Zugriff-und-MFA)

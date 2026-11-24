# Sites & Templates

Eine **Site** verbindet eine oder mehrere Domains mit einem Upstream. **Templates** enthalten wiederverwendbare Standardwerte; jede Site kann einzelne Werte überschreiben oder erben. **Locations** überschreiben die Site-Konfiguration zusätzlich für einen Pfad-Prefix.

## Site-Felder

| Feld | Bedeutung |
|---|---|
| `domain` | Primäre Domain (Host-Matching) |
| `extra_domains` | Weitere Domains/Aliase, gleicher Upstream |
| `upstream_url` | Ziel, z. B. `http://127.0.0.1:3000` |
| `upstream_host` | Optionaler `Host`-Header zum Backend |
| `upstream_insecure_skip_verify` | TLS-Zertifikat des HTTPS-Upstreams ignorieren (nur für selbstsignierte Backends) |
| `tenant_id` | Mandant |
| `template_id` | Optionales Template |
| `cert_id` | Zugeordnetes Zertifikat |
| `enabled` | Site aktiv/inaktiv (inaktiv → 403) |
| `auth_enabled` | Per-Site Basic Auth |
| `cache_enabled` / `cache_ttl` / `cache_max_object_kb` | Response-Cache |
| `rate_limit_rps` / `rate_limit_burst` | Token-Bucket |
| `geoip_enabled` / `geoip_mode` / `geoip_countries` | Länderfilter |
| `http3_enabled` | HTTP/3 per `Alt-Svc` bewerben |
| `http2_enabled` | HTTP/2 (Server-seitig immer verfügbar) |
| `force_https`, `hsts_enabled`, `hsts_max_age` | Siehe Einschränkungen |
| `websocket_enabled` | WebSocket-Upgrades (werden automatisch durchgereicht) |
| `compression_enabled` | Siehe Einschränkungen |
| `max_body_size_mb` | Siehe Einschränkungen |
| `custom_headers` | Siehe Einschränkungen |

## Locations (Pfad-Overrides)

Eine Location gilt für einen Pfad-Prefix und kann fast alles überschreiben, was auch eine Site konfiguriert:

| Feld | Bedeutung |
|---|---|
| `path_prefix` | Pfad, z. B. `/api` oder `/static` |
| `enabled` | Location aktiv/inaktiv |
| `upstream_url` / `upstream_host` | Eigenes Backend für diesen Pfad |
| `upstream_insecure_skip_verify` | TLS-Prüfung zum HTTPS-Upstream deaktivieren |
| `auth_enabled` | Basic Auth für diesen Pfad |
| `auth_users_mode` | `inherit` = Site-Benutzer, `own` = eigene Benutzer der Location |
| `ip_rules_mode` | `inherit` = Site-Regeln, `own` = eigene Allow/Deny-Regeln |
| `cache_enabled` / `cache_ttl` / `cache_max_object_kb` | Cache-Override |
| `rate_limit_rps` / `rate_limit_burst` | Rate-Limit-Override |
| `geoip_enabled` / `geoip_mode` / `geoip_countries` | GeoIP-Override |
| `http3_enabled` | HTTP/3-Override |

**Vererbung:** `Location → Site → Template → interner Standard`. Nicht gesetzte Felder (`null` / „inherit") werden von der nächsthöheren Ebene übernommen.

**Pfad-Matching:**

- Der **längste** passende Prefix gewinnt.
- Prefixes matchen ganze Segmente: `/api` matcht `/api` und `/api/v1`, aber **nicht** `/apix`.
- `/` ist ein Catch-all für alle Pfade.
- Es gibt **kein** Wildcard-Matching für Pfade.
- Deaktivierte Locations werden übersprungen; dann gilt die Site-Konfiguration.

**Wichtig:**

- **Eigene oder geerbte Benutzer:** `auth_users_mode=inherit` nutzt die Site-Benutzer, `own` eine eigene Benutzerliste der Location.
- **Eigene oder geerbte IP-Regeln:** `ip_rules_mode=inherit` nutzt die Site-Regeln, `own` eine eigene Allow/Deny-Liste. „Own" **ersetzt** die Site-Regeln, es ist keine Addition.
- Eigene Benutzer/Regeln werden unter **Site → Zugriff** über die Auswahl **Zugriffsebene** (Site oder Location) verwaltet.
- `auth_enabled` bleibt der Schalter, ob für die Location überhaupt ein Login nötig ist.
- Ist `own` gewählt und die Liste leer, gibt es keine gültigen Benutzer bzw. Regeln – Auth lehnt dann alle ab, die nicht über eine Allow-Regel der Location freigeschaltet sind.
- Beim Anlegen/Ändern/Löschen einer Location wird der Cache der Site geleert.
- Locations werden in der GUI unter **Site → Locations** verwaltet.

### Beispiel

| Prefix | Upstream | Auth | Cache |
|---|---|---|---|
| `/` (Site) | `http://127.0.0.1:3000` | aus | aus |
| `/api` | `http://127.0.0.1:8080` | aus | aus |
| `/static` | `http://127.0.0.1:9000` | aus | an, 1 h |
| `/admin` | `http://127.0.0.1:3000` | **an** | aus |

## Vererbung mit Templates

Jedes Feld ist ein Zeiger (`*bool`, `*int`, `*string`):

- **Wert gesetzt** → Site-/Location-Wert gilt.
- **`null` / „inherit"** → Wert aus der nächsthöheren Ebene.
- **Template leer** → interner Standard.

## Host-Matching

Bastion vergleicht den `Host`-Header (ohne Port) case-insensitiv mit `domain` und allen `extra_domains`. Kein Treffer → `502 Bad Gateway` mit `block_reason=unknown_site`.

## Upstream

- `upstream_url` muss ein vollständiges `http://`- oder `https://`-Ziel sein.
- Bastion setzt `X-Forwarded-Proto: https`, `X-Forwarded-Host` und (falls konfiguriert) den `Host`-Header.
- WebSocket-Upgrades werden automatisch unterstützt.
- Der Upstream muss vom Bastion-Host erreichbar sein; `127.0.0.1` bezieht sich auf den Bastion-Server.

> **HTTPS-Upstream mit selbstsigniertem Zertifikat:** Setze `upstream_insecure_skip_verify` (Site/Location/Template) auf `true`. Damit wird die Zertifikatsprüfung zum Backend deaktiviert. Nur verwenden, wenn der Upstream vertrauenswürdig und das Zertifikat nicht anders lösbar ist.

## HTTP/2 und HTTP/3

- **HTTP/2** ist über TLS immer aktiv.
- **HTTP/3** wird beworben, wenn `http3_enabled` (Location/Site/Template) oder global `server.http3` aktiv ist:

```
Alt-Svc: h3=":443"; ma=86400
```

UDP/443 muss in der Firewall offen sein.

## Löschen, Deaktivieren und Cache

- **Site löschen** entfernt die Site (Soft-Delete). Der Cache wird **nicht** automatisch geleert.
- **Site deaktivieren** → alle Requests erhalten `403`.
- **Location deaktivieren** → Fallback auf die Site-Konfiguration.
- Nach Änderungen an Site oder Locations den Cache im Site-Detail manuell leeren, falls nötig (Location-Änderungen tun das automatisch).

## Aktueller Stand: nicht erzwungene Optionen (v1.0.2)

Diese Felder sind im Modell und in der GUI vorhanden, werden aber derzeit **nicht** im Request-Pfad angewendet:

| Option | Tatsächliches Verhalten |
|---|---|
| `force_https` | HTTP wird **global** für alle Hosts auf HTTPS umgeleitet – der Schalter ändert nichts |
| `hsts_enabled` / `hsts_max_age` | Kein `Strict-Transport-Security`-Header |
| `compression_enabled` | Keine automatische Kompression; nur was der Upstream liefert |
| `max_body_size_mb` | Keine Begrenzung der Request-Größe |
| `custom_headers` | Werden nicht gesetzt |
| `http2_enabled` | HTTP/2 ist immer aktiv; Schalter ohne Wirkung |
| `websocket_enabled` | Upgrades funktionieren unabhängig vom Schalter |

**Wirksam sind:** Upstream, Domains, Locations, Templates/Vererbung, Enabled, Auth, Cache, Rate Limit, GeoIP und HTTP/3.

Weiter: [Zertifikate](https://github.com/Inboxes-CIG/Bastion/wiki/DE-Zertifikate)

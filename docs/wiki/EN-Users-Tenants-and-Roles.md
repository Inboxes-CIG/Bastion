# Users, tenants & roles

## Tenants

A tenant is a boundary for sites, certificates, templates, GeoIP rules and users.

- **Default** is created on first start.
- Only `superadmin` can create/update/delete tenants.
- Tenants have `allowed_upstream_hosts`: hosts, `*.domains` or CIDRs permitted as private upstream targets (SSRF allowlist; **primary place to manage**, empty by default).
- The superadmin has `tenant_id = NULL` and sees everything.
- Other users only see data in their tenant (lists and stats are filtered; foreign resources return 404).

## Roles

| Role | Scope | Permissions (summary) |
|---|---|---|
| `superadmin` | system | Everything, incl. tenants |
| `tenant_admin` | tenant | Sites, templates, certificates, users, GeoIP, site cache within the tenant; global IP lists, CrowdSec writes, cache config and tenants remain superadmin-only |
| `editor` | tenant | Read (write endpoints are restricted to superadmin/tenant_admin) |
| `viewer` | tenant | Read |

Roles are stored in the `roles` table with a JSON permission list. The API currently checks the **role** (`superadmin`/`tenant_admin`) for writes, not individual permissions.

**GUI for tenant roles (v1.0.8+):** The role dropdown hides system roles (`superadmin`) from non-superadmins; tenant selection (sites/templates/certificates/users) is locked to the own tenant; sidebar entries (IP lists, CrowdSec, Tenants), the `(admin)/(unknown)` log filters, the "Hide Bastion" toggle and the system resources widget are shown to `superadmin` only.

## Users

Table `users`:

| Field | Meaning |
|---|---|
| `email` | Login, unique |
| `password_hash` | bcrypt |
| `role_id` | Role |
| `tenant_id` | Tenant (NULL for superadmin) |
| `active` | active/inactive |
| `must_change_password` | forced password change on next login |
| `mfa_enabled` / `mfa_secret` | TOTP |
| `last_login` | last successful login |

- Users are managed under **Users**.
- `tenant_admin` can create/update/delete users in their tenant.
- Deleting a user is a soft delete (`deleted_at`). The email stays blocked by the unique index; to reuse it, hard-delete the row:

```sql
DELETE FROM users WHERE email = '<email>';
```

- Passwords are bcrypt-hashed. Creating a user **requires** a password; the user must change it in a mandatory dialog at first login (`must_change_password`).
- Reset by setting a new password in the GUI – the new password is then again flagged as "to be changed".
- **Re-auth:** sensitive changes (role, email, tenant, status, password) additionally require the password of the acting account; without it, or with a wrong one, the API answers `403`.

## Sessions and tokens

- **Access token**: JWT, 15 minutes
- **Refresh token**: JWT, 7 days, **rotating** – every token used is blacklisted server-side (table `refresh_tokens`); reusing an old token revokes the whole token family (theft detection)
- **MFA token**: JWT, 5 minutes, only for the second login step
- Signature: HS256 with `auth.jwt_secret`
- Logout and password changes revoke all refresh tokens of the account; access tokens remain valid until they expire.
- Login lockout: 3 failed attempts / 10 minutes per IP **and** email → `429`.
- If `jwt_secret` is compromised, change it in `/etc/bastion/config.yaml` and restart to invalidate all tokens.

## MFA

See [Access & MFA](https://github.com/Inboxes-CIG/Bastion/wiki/EN-Access-and-MFA).

## First install

- Superadmin: `admin@bastion.local`
- Password: printed once in the journal (`journalctl -u bastion | grep -A4 'INITIAL SUPERADMIN'`)
- After the first login a mandatory dialog forces a password change; then enable MFA.

Next: [Configuration & operations](https://github.com/Inboxes-CIG/Bastion/wiki/EN-Configuration-and-Operations)

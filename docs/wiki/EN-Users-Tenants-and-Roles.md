# Users, tenants & roles

## Tenants

A tenant is a boundary for sites, certificates, templates, GeoIP rules and users.

- **Default** is created on first start.
- Only `superadmin` can create/update/delete tenants.
- The superadmin has `tenant_id = NULL` and sees everything.
- Other users only see data in their tenant (lists and stats are filtered).

## Roles

| Role | Scope | Permissions (summary) |
|---|---|---|
| `superadmin` | system | Everything, incl. tenants |
| `tenant_admin` | tenant | Sites, templates, certificates, users, GeoIP, IP lists, cache, CrowdSec within the tenant |
| `editor` | tenant | Read (write endpoints are restricted to superadmin/tenant_admin) |
| `viewer` | tenant | Read |

Roles are stored in the `roles` table with a JSON permission list. The API currently checks the **role** (`superadmin`/`tenant_admin`) for writes, not individual permissions.

## Users

Table `users`:

| Field | Meaning |
|---|---|
| `email` | Login, unique |
| `password_hash` | bcrypt |
| `role_id` | Role |
| `tenant_id` | Tenant (NULL for superadmin) |
| `active` | active/inactive |
| `mfa_enabled` / `mfa_secret` | TOTP |
| `last_login` | last successful login |

- Users are managed under **Users**.
- `tenant_admin` can create/update/delete users in their tenant.
- Deleting a user is a soft delete (`deleted_at`). The email stays blocked by the unique index; to reuse it, hard-delete the row:

```sql
DELETE FROM users WHERE email = '<email>';
```

- Passwords are bcrypt-hashed; reset by setting a new password in the GUI.

## Sessions and tokens

- **Access token**: JWT, 15 minutes
- **Refresh token**: JWT, 7 days
- **MFA token**: JWT, 5 minutes, only for the second login step
- Signature: HS256 with `auth.jwt_secret`
- No server-side session backend; logout removes the browser token.
- If `jwt_secret` is compromised, change it in `/etc/bastion/config.yaml` and restart to invalidate all tokens.

## MFA

See [Access & MFA](https://github.com/Inboxes-CIG/Bastion/wiki/EN-Access-and-MFA).

## First install

- Superadmin: `admin@bastion.local`
- Password: printed once in the journal (`journalctl -u bastion | grep -A4 'INITIAL SUPERADMIN'`)
- Change the password and enable MFA after the first login.

Next: [Configuration & operations](https://github.com/Inboxes-CIG/Bastion/wiki/EN-Configuration-and-Operations)

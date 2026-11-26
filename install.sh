#!/bin/bash
# Bastion Installer
set -e

SRC_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="${BINARY:-$SRC_DIR/bastion}"
[ -f "$BINARY" ] || BINARY="/opt/bastion/bin/bastion"
CONFIG_DIR="/etc/bastion"
DATA_DIR="/var/lib/bastion"
APP_USER="bastion"

usage() {
  cat <<EOF
Bastion installer

Options:
  --hostname <fqdn>          Admin hostname (also used for the self-signed cert)
  --admin-email <email>      Initial superadmin email
  --admin-password <pass>    Initial superadmin password (empty = generate)
  -h, --help                 Show this help

Environment variables:
  BASTION_HOSTNAME, BASTION_ADMIN_EMAIL, BASTION_ADMIN_PASSWORD

If an option is omitted and a terminal is available, the installer asks.
Without a terminal, the system hostname and admin@<hostname> are used and the
password is generated and logged by Bastion on first start.
EOF
}

HOSTNAME_OPT=""
ADMIN_EMAIL_OPT=""
ADMIN_PASSWORD_OPT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --hostname) HOSTNAME_OPT="${2:-}"; shift 2 ;;
    --admin-email) ADMIN_EMAIL_OPT="${2:-}"; shift 2 ;;
    --admin-password) ADMIN_PASSWORD_OPT="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1"; usage; exit 1 ;;
  esac
done
HOSTNAME_OPT="${HOSTNAME_OPT:-${BASTION_HOSTNAME:-}}"
ADMIN_EMAIL_OPT="${ADMIN_EMAIL_OPT:-${BASTION_ADMIN_EMAIL:-}}"
ADMIN_PASSWORD_OPT="${ADMIN_PASSWORD_OPT:-${BASTION_ADMIN_PASSWORD:-}}"

normalize_host() {
  printf '%s' "$1" | sed -E 's#^[a-zA-Z][a-zA-Z0-9+.-]*://##; s#/.*$##; s/[[:space:]]//g; s/"//g' | sed "s/'//g"
}

ask() {
  local label="$1" def="${2:-}" value="" prompt
  prompt="$label${def:+ [$def]}: "
  if can_prompt; then
    if [ -t 0 ]; then
      read -r -t 120 -p "$prompt" value || true
    else
      read -r -t 120 -p "$prompt" value < /dev/tty || true
    fi
  fi
  printf '%s' "${value:-$def}"
}

# can_prompt reports whether the installer may read from a terminal. Reading
# /dev/tty while being in a background process group stops the shell with
# SIGTTIN (e.g. nested sudo with use_pty in a "curl | sudo bash" pipeline).
# In that case we fall back to defaults/flags instead of hanging.
can_prompt() {
  if [ -t 0 ]; then
    return 0
  fi
  [ -t 2 ] || return 1
  [ -r /dev/tty ] || return 1
  command -v ps >/dev/null 2>&1 || return 1
  local tpgid pgid
  tpgid=$(ps -o tpgid= -p $$ 2>/dev/null | tr -d '[:space:]')
  pgid=$(ps -o pgid= -p $$ 2>/dev/null | tr -d '[:space:]')
  if [ -n "$tpgid" ] && [ "$tpgid" != "-1" ] && [ -n "$pgid" ]; then
    [ "$tpgid" = "$pgid" ] || return 1
  fi
  return 0
}

ask_secret() {
  local label="$1" value=""
  if can_prompt; then
    if [ -t 0 ]; then
      read -r -s -t 120 -p "$label: " value || true
    else
      read -r -s -t 120 -p "$label: " value < /dev/tty || true
    fi
    echo >&2
  fi
  printf '%s' "$value"
}

port_in_use() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltn 2>/dev/null | awk '{print $4}' | grep -qE "[:.]${port}$"
  elif command -v netstat >/dev/null 2>&1; then
    netstat -ltn 2>/dev/null | awk '{print $4}' | grep -qE "[:.]${port}$"
  else
    return 1
  fi
}

echo "=== Bastion Installer ==="
if ! can_prompt; then
  echo "Info: Kein interaktives Terminal verfügbar – es werden Standardwerte bzw. --hostname/--admin-* verwendet."
fi

if command -v apt-get >/dev/null 2>&1; then
  apt-get update -qq
  apt-get install -y -qq postgresql postgresql-contrib curl iproute2 openssl
fi

if ! id -u "$APP_USER" >/dev/null 2>&1; then
  useradd -r -s /usr/sbin/nologin -d /opt/bastion -m "$APP_USER"
fi

mkdir -p /opt/bastion/bin "$CONFIG_DIR" "$DATA_DIR"/{certs,geoip,data,logs}
chown -R "$APP_USER:$APP_USER" /opt/bastion "$DATA_DIR"
chmod 700 "$CONFIG_DIR" "$DATA_DIR/certs"
chmod 750 "$CONFIG_DIR" "$DATA_DIR" "$DATA_DIR/geoip" "$DATA_DIR/data"
chown root:"$APP_USER" "$CONFIG_DIR"

if [ -f "$BINARY" ]; then
  if [ "$(readlink -f "$BINARY" 2>/dev/null)" = "$(readlink -f /opt/bastion/bin/bastion 2>/dev/null)" ]; then
    echo "Binary already in place"
  else
    cp "$BINARY" /opt/bastion/bin/bastion
    chmod 755 /opt/bastion/bin/bastion
    chown "$APP_USER:$APP_USER" /opt/bastion/bin/bastion
    echo "Binary installed"
  fi
else
  echo "ERROR: binary not found at $BINARY"
  exit 1
fi

# --- Hostname (admin host / self-signed cert) ---
HOSTNAME_VALUE="$(normalize_host "$HOSTNAME_OPT")"
if [ -z "$HOSTNAME_VALUE" ]; then
  DEFAULT_HOST="$(hostname -f 2>/dev/null || hostname 2>/dev/null || echo localhost)"
  if [ -t 0 ] || [ -r /dev/tty ]; then
    HOSTNAME_VALUE="$(ask "Hostname/FQDN für die Admin-Oberfläche" "$DEFAULT_HOST")"
  else
    HOSTNAME_VALUE="$DEFAULT_HOST"
  fi
fi
HOSTNAME_VALUE="$(normalize_host "$HOSTNAME_VALUE")"
[ -n "$HOSTNAME_VALUE" ] || HOSTNAME_VALUE="localhost"
echo "Admin hostname: $HOSTNAME_VALUE"

# --- CrowdSec LAPI must not occupy Bastion's API port 8080 ---
CROWDSEC_KEY=""
if ! command -v cscli >/dev/null 2>&1; then
  echo "Installing CrowdSec..."
  curl -s https://install.crowdsec.net | sh || true
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq crowdsec || true
fi
if command -v cscli >/dev/null 2>&1; then
  echo "Configuring CrowdSec LAPI on 127.0.0.1:8081 (Bastion uses 8080)..."
  systemctl stop crowdsec 2>/dev/null || true
  for f in /etc/crowdsec/config.yaml /etc/crowdsec/config.yaml.local; do
    if [ -f "$f" ]; then
      sed -i -E 's|^([[:space:]]*listen_uri:)[[:space:]]*.*|\1 127.0.0.1:8081|' "$f"
    fi
  done
  if [ -f /etc/crowdsec/local_api_credentials.yaml ]; then
    sed -i -E 's|^([[:space:]]*url:)[[:space:]]*.*|\1 http://127.0.0.1:8081|' /etc/crowdsec/local_api_credentials.yaml
  fi
  systemctl enable crowdsec 2>/dev/null || true
  systemctl restart crowdsec 2>/dev/null || true
  for _ in $(seq 1 20); do
    port_in_use 8081 && break
    sleep 1
  done
  if ! port_in_use 8081; then
    echo "WARN: CrowdSec LAPI is not listening on 8081 yet."
  fi
  for _ in 1 2 3 4 5; do
    cscli bouncers list -o json >/dev/null 2>&1 && break
    sleep 2
  done
  if cscli bouncers list -o json 2>/dev/null | grep -q bastion-bouncer; then
    echo "Bouncer bastion-bouncer exists"
  else
    CROWDSEC_KEY=$(cscli bouncers add bastion-bouncer -o raw 2>/dev/null || true)
    [ -n "$CROWDSEC_KEY" ] && echo "Created CrowdSec bouncer"
  fi
  if [ ! -f "$CONFIG_DIR/config.yaml" ] && [ -z "$CROWDSEC_KEY" ]; then
    CROWDSEC_KEY=$(cscli bouncers add "bastion-bouncer-$(date +%s)" -o raw 2>/dev/null || true)
    [ -n "$CROWDSEC_KEY" ] && echo "Created replacement CrowdSec bouncer"
  fi
fi

# --- Config ---
if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
  DB_PASS=$(openssl rand -hex 24)
  JWT_SECRET=$(openssl rand -hex 32)
  SESSION_SECRET=$(openssl rand -hex 32)
  cat > "$CONFIG_DIR/config.yaml" <<EOF
server:
  http_addr: ":80"
  https_addr: ":443"
  api_addr: "127.0.0.1:8080"
  admin_hosts:
    - "${HOSTNAME_VALUE}"
database:
  url: "postgres://waf:${DB_PASS}@localhost:5432/waf?sslmode=disable"
auth:
  jwt_secret: "${JWT_SECRET}"
  session_secret: "${SESSION_SECRET}"
certs:
  storage_dir: "${DATA_DIR}/certs"
geoip:
  db_path: "${DATA_DIR}/geoip/dbip-country-lite.mmdb"
crowdsec:
  enabled: true
  lapi_url: "http://127.0.0.1:8081"
  api_key: "${CROWDSEC_KEY}"
  refresh_seconds: 30
logging:
  dir: "${DATA_DIR}/logs"
  max_memory: 10000
  retention_days: 30
cache:
  enabled: true
  ttl_seconds: 60
  max_entries: 500
  max_object_kb: 256
  max_total_mb: 64
security:
  auto_ban_404:
    enabled: true
    threshold: 50
    window_seconds: 600
EOF
  chmod 640 "$CONFIG_DIR/config.yaml"
  chown root:"$APP_USER" "$CONFIG_DIR/config.yaml"
  cat > "$CONFIG_DIR/.secrets.env" <<EOF
DB_PASS=$DB_PASS
JWT_SECRET=$JWT_SECRET
SESSION_SECRET=$SESSION_SECRET
EOF
  chmod 600 "$CONFIG_DIR/.secrets.env"
else
  echo "Existing configuration found at $CONFIG_DIR/config.yaml (keeping it)."
  if [ -n "$HOSTNAME_OPT" ]; then
    echo "Note: admin_hosts is not changed automatically on existing installs."
  fi
fi

# --- Database ---
systemctl enable --now postgresql
source "$CONFIG_DIR/.secrets.env"
sudo -u postgres psql -v ON_ERROR_STOP=1 <<SQL
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'waf') THEN
    CREATE ROLE waf LOGIN PASSWORD '$DB_PASS';
  END IF;
END
\$\$;
SQL
if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='waf'" | grep -q 1; then
  sudo -u postgres createdb -O waf waf
fi

# --- Initial admin (fresh database only) ---
FRESH_ADMIN=0
USERS_TABLE=$(sudo -u postgres psql -d waf -tAc "SELECT to_regclass('public.users')" 2>/dev/null | tr -d '[:space:]' || true)
if [ -z "$USERS_TABLE" ]; then
  FRESH_ADMIN=1
else
  USER_COUNT=$(sudo -u postgres psql -d waf -tAc "SELECT count(*) FROM users WHERE deleted_at IS NULL" 2>/dev/null | tr -d '[:space:]' || true)
  [ "$USER_COUNT" = "0" ] && FRESH_ADMIN=1
fi

ADMIN_EMAIL_VALUE=""
if [ "$FRESH_ADMIN" -eq 1 ]; then
  DEFAULT_ADMIN="admin@bastion.local"
  if printf '%s' "$HOSTNAME_VALUE" | grep -qE '^[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'; then
    DEFAULT_ADMIN="admin@$HOSTNAME_VALUE"
  fi
  ADMIN_EMAIL_VALUE="$ADMIN_EMAIL_OPT"
  if [ -z "$ADMIN_EMAIL_VALUE" ]; then
    ADMIN_EMAIL_VALUE="$(ask "Admin-E-Mail" "$DEFAULT_ADMIN")"
  fi
  [ -n "$ADMIN_EMAIL_VALUE" ] || ADMIN_EMAIL_VALUE="$DEFAULT_ADMIN"
  ADMIN_PASSWORD_VALUE="$ADMIN_PASSWORD_OPT"
  if [ -z "$ADMIN_PASSWORD_VALUE" ]; then
    ADMIN_PASSWORD_VALUE="$(ask_secret "Admin-Passwort (leer = automatisch generieren)")"
  fi

  OLD_UMASK=$(umask)
  umask 077
  cat > "$CONFIG_DIR/initial-admin" <<EOF
email=$ADMIN_EMAIL_VALUE
password=$ADMIN_PASSWORD_VALUE
EOF
  chown "$APP_USER:$APP_USER" "$CONFIG_DIR/initial-admin"
  chmod 600 "$CONFIG_DIR/initial-admin"
  umask "$OLD_UMASK"
  echo "Initial admin: $ADMIN_EMAIL_VALUE"
else
  echo "Existing users found; keeping the current admin account."
fi

# --- GeoIP updater ---
cat > /usr/local/bin/bastion-geoip-update.sh <<'GEO'
#!/bin/bash
set -e
DIR=/var/lib/bastion/geoip
mkdir -p "$DIR"; cd "$DIR"
YM=$(date +%Y-%m)
curl -sSL -o "$DIR/dbip-country-lite.mmdb.gz" "https://download.db-ip.com/free/dbip-country-lite-${YM}.mmdb.gz"
if [ -f "$DIR/dbip-country-lite.mmdb.gz" ]; then
  gunzip -kf "$DIR/dbip-country-lite.mmdb.gz"
  f=$(ls "$DIR"/dbip-country-lite-*.mmdb 2>/dev/null | head -1)
  [ -n "$f" ] && mv -f "$f" "$DIR/dbip-country-lite.mmdb"
  chown bastion:bastion "$DIR/dbip-country-lite.mmdb"
  systemctl restart bastion || true
fi
GEO
chmod +x /usr/local/bin/bastion-geoip-update.sh

if [ ! -f "$DATA_DIR/geoip/dbip-country-lite.mmdb" ]; then
  /usr/local/bin/bastion-geoip-update.sh 2>/dev/null || true
fi

# --- systemd ---
if [ -f "$SRC_DIR/systemd/bastion.service" ]; then
  cp "$SRC_DIR/systemd/bastion.service" /etc/systemd/system/bastion.service
else
  cat > /etc/systemd/system/bastion.service <<'UNIT'
[Unit]
Description=Bastion Service
After=network.target postgresql.service crowdsec.service
Wants=postgresql.service crowdsec.service
[Service]
Type=simple
User=bastion
Group=bastion
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
Environment=GIN_MODE=release
ExecStart=/opt/bastion/bin/bastion
Restart=on-failure
RestartSec=5
AmbientCapabilities=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/bastion /etc/bastion
[Install]
WantedBy=multi-user.target
UNIT
fi
systemctl daemon-reload

if port_in_use 8080; then
  echo "ERROR: Port 8080 is already in use; Bastion needs it for its API."
  if command -v ss >/dev/null 2>&1; then
    ss -ltnp 2>/dev/null | grep ':8080 ' || true
  fi
  echo "CrowdSec LAPI should use 127.0.0.1:8081. Stop the process on 8080 or change server.api_addr."
  exit 1
fi

systemctl enable --now bastion

(crontab -l 2>/dev/null; echo "0 3 1 * * /usr/local/bin/bastion-geoip-update.sh >> /var/log/bastion-geoip-update.log 2>&1") | sort -u | crontab -

echo "=== Bastion installed ==="
echo "Admin hostname: $HOSTNAME_VALUE"
if [ -n "$ADMIN_EMAIL_VALUE" ]; then
  echo "Admin user: $ADMIN_EMAIL_VALUE"
fi
echo "Admin password: see journalctl -u bastion (or the password you entered)"
echo "GUI: https://$HOSTNAME_VALUE/ (or https://<server-ip>/)"
echo "Ports: Bastion API :8080, CrowdSec LAPI 127.0.0.1:8081"

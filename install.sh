#!/bin/bash
# Bastion Installer
set -e

SRC_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="${BINARY:-$SRC_DIR/bastion}"
[ -f "$BINARY" ] || BINARY="/opt/bastion/bin/bastion"
CONFIG_DIR="/etc/bastion"
DATA_DIR="/var/lib/bastion"
APP_USER="bastion"

echo "=== Bastion Installer ==="

if command -v apt-get >/dev/null 2>&1; then
  apt-get update -qq
  apt-get install -y -qq postgresql postgresql-contrib curl
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
  cp "$BINARY" /opt/bastion/bin/bastion
  chmod 755 /opt/bastion/bin/bastion
  chown "$APP_USER:$APP_USER" /opt/bastion/bin/bastion
  echo "Binary installed"
else
  echo "ERROR: binary not found at $BINARY"
  exit 1
fi

CROWDSEC_KEY=""
if ! command -v cscli >/dev/null 2>&1; then
  echo "Installing CrowdSec..."
  curl -s https://install.crowdsec.net | sh || true
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq crowdsec || true
fi
if command -v cscli >/dev/null 2>&1; then
  sed -i 's|listen_uri: 127.0.0.1:8080|listen_uri: 127.0.0.1:8081|' /etc/crowdsec/config.yaml 2>/dev/null || true
  sed -i 's|url: http://127.0.0.1:8080|url: http://127.0.0.1:8081|' /etc/crowdsec/local_api_credentials.yaml 2>/dev/null || true
  systemctl enable --now crowdsec 2>/dev/null || true
  sleep 3
  if cscli bouncers list -o json 2>/dev/null | grep -q bastion-bouncer; then
    echo "Bouncer bastion-bouncer exists"
  else
    CROWDSEC_KEY=$(cscli bouncers add bastion-bouncer -o raw 2>/dev/null || true)
    echo "Created CrowdSec bouncer"
  fi
fi

if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
  DB_PASS=$(openssl rand -hex 24)
  JWT_SECRET=$(openssl rand -hex 32)
  SESSION_SECRET=$(openssl rand -hex 32)
  cat > "$CONFIG_DIR/config.yaml" <<EOF
server:
  http_addr: ":80"
  https_addr: ":443"
  api_addr: ":8080"
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
EOF
  chmod 640 "$CONFIG_DIR/config.yaml"
  chown root:"$APP_USER" "$CONFIG_DIR/config.yaml"
  cat > "$CONFIG_DIR/.secrets.env" <<EOF
DB_PASS=$DB_PASS
JWT_SECRET=$JWT_SECRET
SESSION_SECRET=$SESSION_SECRET
EOF
  chmod 600 "$CONFIG_DIR/.secrets.env"
fi

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

if [ ! -f "$DATA_DIR/geoip/dbip-country-lite.mmdb" ]; then
  /usr/local/bin/bastion-geoip-update.sh 2>/dev/null || true
fi

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
systemctl enable --now bastion

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
  chown waf:waf "$DIR/dbip-country-lite.mmdb"
  systemctl restart bastion || true
fi
GEO
chmod +x /usr/local/bin/bastion-geoip-update.sh
(crontab -l 2>/dev/null; echo "0 3 1 * * /usr/local/bin/bastion-geoip-update.sh >> /var/log/bastion-geoip-update.log 2>&1") | sort -u | crontab -

echo "=== Bastion installed ==="
echo "GUI: https://<server-ip>/ (accept the self-signed cert)"
echo "Admin credentials: journalctl -u bastion"

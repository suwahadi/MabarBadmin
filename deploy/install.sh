#!/usr/bin/env bash
# Installs or updates MabarBadmin on a Debian/Ubuntu server with systemd, building the binary on
# the server itself. Go and Bun are downloaded into a temporary folder and removed afterwards, so
# no toolchain stays on the server. Running the script again updates to the latest commit.
#
# Usage (as a user with sudo):
#   curl -fsSLo install.sh https://raw.githubusercontent.com/suwahadi/MabarBadmin/main/deploy/install.sh
#   sudo bash install.sh --base-url https://<public-url>
#
# Options:
#   --base-url URL   public address of the site (required on first install)
#   --port N         port the app listens on (default 8080)
#   --listen ADDR    listen address (default 0.0.0.0:<port>, use 127.0.0.1:<port> behind Caddy)
#   --ref REF        git branch, tag, or commit to build (default main)
#   --repo URL       git repository (default https://github.com/suwahadi/MabarBadmin.git)
set -euo pipefail

REPO="https://github.com/suwahadi/MabarBadmin.git"
REF="main"
PORT="8080"
LISTEN=""
BASE_URL=""
BUN_VERSION="bun-v1.4.4"

while [ $# -gt 0 ]; do
  case "$1" in
    --base-url) BASE_URL="${2%/}"; shift 2 ;;
    --port) PORT="$2"; shift 2 ;;
    --listen) LISTEN="$2"; shift 2 ;;
    --ref) REF="$2"; shift 2 ;;
    --repo) REPO="$2"; shift 2 ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done
LISTEN="${LISTEN:-0.0.0.0:$PORT}"

say() { printf '\n\033[1;32m==> %s\033[0m\n' "$*"; }
die() { printf '\n\033[1;31mERROR: %s\033[0m\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "Run with sudo: sudo bash install.sh ..."
[ -d /run/systemd/system ] || die "systemd is not running on this machine; this script needs systemd."
case "$(uname -m)" in
  x86_64) GOARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64 ;;
  *) die "Unsupported architecture $(uname -m)" ;;
esac

ENV_FILE=/etc/mabar/env
if [ ! -f "$ENV_FILE" ] && [ -z "$BASE_URL" ]; then
  die "First install needs --base-url, e.g. --base-url https://your-instabox-url"
fi
case "${BASE_URL:-https://x}" in http://*|https://*) ;; *) die "--base-url must start with http:// or https://" ;; esac

say "Installing system packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq postgresql git curl ca-certificates unzip rsync openssl tar >/dev/null
systemctl enable --now postgresql >/dev/null

say "Preparing user and folders"
id mabar >/dev/null 2>&1 || useradd --system --home /var/lib/mabar --shell /usr/sbin/nologin mabar
mkdir -p /opt/mabar/releases /var/lib/mabar/uploads /var/backups/mabar /etc/mabar
chown -R mabar:mabar /var/lib/mabar /var/backups/mabar
chmod 750 /var/lib/mabar /var/backups/mabar

if [ ! -f "$ENV_FILE" ]; then
  say "Creating database and $ENV_FILE"
  DB_PASS="$(openssl rand -hex 24)"
  if sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='mabar'" | grep -q 1; then
    sudo -u postgres psql -qc "ALTER USER mabar WITH PASSWORD '$DB_PASS';"
  else
    sudo -u postgres psql -qc "CREATE USER mabar WITH PASSWORD '$DB_PASS';"
  fi
  sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='mabar'" | grep -q 1 \
    || sudo -u postgres createdb -O mabar mabar
  umask 077
  cat > "$ENV_FILE" <<EOF
DATABASE_URL=postgres://mabar:${DB_PASS}@127.0.0.1:5432/mabar?sslmode=disable
LISTEN_ADDR=${LISTEN}
UPLOAD_DIR=/var/lib/mabar/uploads
BASE_URL=${BASE_URL}
SESSION_SECRET=$(openssl rand -hex 32)
EOF
  umask 022
else
  say "Keeping existing $ENV_FILE"
  if [ -n "$BASE_URL" ]; then sed -i "s|^BASE_URL=.*|BASE_URL=${BASE_URL}|" "$ENV_FILE"; fi
  sed -i "s|^LISTEN_ADDR=.*|LISTEN_ADDR=${LISTEN}|" "$ENV_FILE"
fi
chown root:root "$ENV_FILE"
chmod 600 "$ENV_FILE"

WORK="$(mktemp -d /tmp/mabar-build.XXXXXX)"
trap 'rm -rf "$WORK"' EXIT

say "Fetching source ($REF)"
git clone --quiet "$REPO" "$WORK/src"
git -C "$WORK/src" checkout --quiet "$REF"
VERSION="$(git -C "$WORK/src" describe --tags --always)"
GO_VERSION="$(sed -n 's/^toolchain //p' "$WORK/src/go.mod")"
GO_VERSION="${GO_VERSION:-go$(sed -n 's/^go //p' "$WORK/src/go.mod")}"

say "Downloading $GO_VERSION and Bun (temporary)"
curl -fsSL "https://go.dev/dl/${GO_VERSION}.linux-${GOARCH}.tar.gz" | tar -xz -C "$WORK"
export BUN_INSTALL="$WORK/bun"
curl -fsSL https://bun.sh/install | bash -s "$BUN_VERSION" >/dev/null
export PATH="$WORK/go/bin:$BUN_INSTALL/bin:$PATH"
export GOPATH="$WORK/gopath" GOCACHE="$WORK/gocache" GOTOOLCHAIN=local

say "Building CSS and JS"
(cd "$WORK/src/frontend" && bun install --frozen-lockfile >/dev/null && bun run build >/dev/null)

say "Building mabar $VERSION"
(cd "$WORK/src" && CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -tags nodynamic -trimpath \
  -ldflags "-s -w -X main.version=$VERSION" -o "$WORK/mabar" ./cmd/mabar)

say "Installing release $VERSION"
RELEASE="/opt/mabar/releases/$VERSION"
mkdir -p "$RELEASE"
install -m 755 "$WORK/mabar" "$RELEASE/mabar"
ln -sfn "$RELEASE" /opt/mabar/current
install -m 644 "$WORK/src/deploy/mabar.service" /etc/systemd/system/mabar.service
install -m 644 "$WORK/src/deploy/mabar-backup.service" /etc/systemd/system/mabar-backup.service
install -m 644 "$WORK/src/deploy/mabar-backup.timer" /etc/systemd/system/mabar-backup.timer
install -m 755 "$WORK/src/deploy/backup.sh" /opt/mabar/backup.sh
systemctl daemon-reload

mabar_cli() {
  systemd-run --quiet --wait --collect --pipe --uid=mabar -p EnvironmentFile="$ENV_FILE" /opt/mabar/current/mabar "$@"
}

say "Running migrations and default seed"
mabar_cli migrate up
mabar_cli seed

ADMINS="$(sudo -u postgres psql -d mabar -tAc "SELECT count(*) FROM users WHERE role='admin'" 2>/dev/null || echo 0)"
if [ "${ADMINS:-0}" = "0" ]; then
  say "Creating the admin account"
  if [ -r /dev/tty ]; then
    systemd-run --quiet --wait --collect --pty --uid=mabar -p EnvironmentFile="$ENV_FILE" /opt/mabar/current/mabar create-admin </dev/tty
  else
    echo "No terminal available. Create the admin later with:"
    echo "  sudo systemd-run --pty --wait --collect --uid=mabar -p EnvironmentFile=$ENV_FILE /opt/mabar/current/mabar create-admin"
  fi
fi

say "Starting services"
systemctl enable mabar >/dev/null 2>&1
systemctl restart mabar
systemctl enable --now mabar-backup.timer >/dev/null 2>&1

for _ in $(seq 1 20); do
  if curl -fsS "http://127.0.0.1:${PORT}/healthz" >/dev/null 2>&1; then break; fi
  sleep 1
done
if curl -fsS "http://127.0.0.1:${PORT}/healthz"; then
  BASE="$(sed -n 's/^BASE_URL=//p' "$ENV_FILE")"
  say "MabarBadmin $VERSION is running on port ${PORT}"
  echo "Open ${BASE}/login and sign in with the admin account."
  echo "Logs: journalctl -u mabar -f"
else
  journalctl -u mabar -n 40 --no-pager || true
  die "The app did not answer on port ${PORT}. See the log above."
fi

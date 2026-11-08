#!/bin/sh
# Daily backup: database dump plus payment-proof uploads. Run from a systemd timer as the mabar user.
set -eu
dest="${BACKUP_DIR:-/var/backups/mabar}"
stamp="$(date +%Y%m%d-%H%M)"
mkdir -p "$dest/db" "$dest/uploads"
. /etc/mabar/env
pg_dump -Fc "$DATABASE_URL" > "$dest/db/mabar-$stamp.dump"
rsync -a "$UPLOAD_DIR/" "$dest/uploads/"
find "$dest/db" -name 'mabar-*.dump' -mtime +14 -delete

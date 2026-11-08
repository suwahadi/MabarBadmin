#!/bin/sh
# Daily backup: database dump plus payment-proof uploads. Run by mabar-backup.service, which
# passes DATABASE_URL and UPLOAD_DIR from /etc/mabar/env (that file is root-only, mode 600).
set -eu
: "${DATABASE_URL:?DATABASE_URL is not set}"
: "${UPLOAD_DIR:?UPLOAD_DIR is not set}"
dest="${BACKUP_DIR:-/var/backups/mabar}"
stamp="$(date +%Y%m%d-%H%M)"
mkdir -p "$dest/db" "$dest/uploads"
pg_dump -Fc "$DATABASE_URL" > "$dest/db/mabar-$stamp.dump"
rsync -a "$UPLOAD_DIR/" "$dest/uploads/"
find "$dest/db" -name 'mabar-*.dump' -mtime +14 -delete

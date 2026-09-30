#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
[ -f .env ] || { echo 'Missing .env'; exit 1; }
# .env must use shell-compatible KEY=value assignments; keep credentials quoted.
set -a
. ./.env
set +a
: "${RESTIC_REPOSITORY:?Configure an off-server restic repository}"
: "${RESTIC_PASSWORD:?Configure RESTIC_PASSWORD}"
command -v restic >/dev/null
mkdir -p backups/metrics
umask 077
lock=backups/.backup-lock
mkdir "$lock" 2>/dev/null || { echo 'Backup already running'; exit 1; }
restart_needed=0
cleanup() {
 if [ "$restart_needed" = 1 ]; then docker compose start storage api worker; fi
 rmdir "$lock"
}
trap cleanup EXIT INT TERM
# Brief maintenance window: quiesce writers and storage for a consistent DB/object snapshot.
restart_needed=1
docker compose stop api worker storage
docker compose exec -T db pg_dump -U strefa -Fc strefa > backups/database.dump.tmp
mv backups/database.dump.tmp backups/database.dump
docker compose run --rm --no-deps -T --entrypoint sh storage -c 'tar czf - /data' > backups/storage.tar.gz.tmp
mv backups/storage.tar.gz.tmp backups/storage.tar.gz
docker compose start storage api worker
restart_needed=0
restic backup backups/database.dump backups/storage.tar.gz --tag strefa
restic forget --tag strefa --keep-within 14d --prune
restic check
printf 'strefa_backup_last_success_seconds %s\n' "$(date +%s)" > backups/metrics/backup.prom.tmp
mv backups/metrics/backup.prom.tmp backups/metrics/backup.prom

#!/bin/sh
# Restore ONLY into an empty, separately named Compose project.
set -eu
cd "$(dirname "$0")/.."
: "${RESTORE_PROJECT:?Set a NEW compose project name}"
: "${RESTORE_DATABASE:?Absolute path to database.dump}"
: "${RESTORE_STORAGE:?Absolute path to storage.tar.gz}"
[ "$RESTORE_PROJECT" != strefa-treningow ] || { echo 'Refusing to overwrite primary project'; exit 1; }
# Never reuse any existing project, including a previous failed restore.
existing=$(docker compose -p "$RESTORE_PROJECT" -f compose.yaml -f infra/compose.restore.yaml ps -aq)
[ -z "$existing" ] || { echo 'Target project already exists; use a new project name'; exit 1; }
# Start only a new isolated db/storage pair (no host ports).
docker compose -p "$RESTORE_PROJECT" -f compose.yaml -f infra/compose.restore.yaml up -d --wait db storage
# Abort if target already contains application tables.
count=$(docker compose -p "$RESTORE_PROJECT" -f compose.yaml -f infra/compose.restore.yaml exec -T db psql -U strefa -d strefa -Atc "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='users'")
[ "$count" = 0 ] || { echo 'Target is not empty; refusing restore'; exit 1; }
# PostGIS image seeds tiger/topology schemas; a pristine template0 avoids collisions.
docker compose -p "$RESTORE_PROJECT" -f compose.yaml -f infra/compose.restore.yaml exec -T db dropdb -U strefa strefa
docker compose -p "$RESTORE_PROJECT" -f compose.yaml -f infra/compose.restore.yaml exec -T db createdb -U strefa -T template0 strefa
docker compose -p "$RESTORE_PROJECT" -f compose.yaml -f infra/compose.restore.yaml exec -T db pg_restore -U strefa -d strefa --no-owner --exit-on-error < "$RESTORE_DATABASE"
docker compose -p "$RESTORE_PROJECT" -f compose.yaml -f infra/compose.restore.yaml stop storage
docker compose -p "$RESTORE_PROJECT" -f compose.yaml -f infra/compose.restore.yaml run --rm --no-deps -T --entrypoint sh storage -c 'tar xzf - -C /' < "$RESTORE_STORAGE"
docker compose -p "$RESTORE_PROJECT" -f compose.yaml -f infra/compose.restore.yaml up -d storage
echo "Restored into isolated project $RESTORE_PROJECT. Verify SQL counts and image checksums before switching traffic."

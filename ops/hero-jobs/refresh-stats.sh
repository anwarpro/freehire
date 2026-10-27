#!/bin/sh
set -eu
cd /opt/hero-jobs
[ ! -e state/deploying ] || exit 0
mkdir -p state
exec 9>state/stats.lock
flock -n 9 || exit 0
docker compose run --rm -T --no-deps --entrypoint /app/rollup-stats tools
docker compose run --rm -T --no-deps --entrypoint /app/reindex-companies tools

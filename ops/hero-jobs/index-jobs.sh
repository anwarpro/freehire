#!/bin/sh
set -eu
cd /opt/hero-jobs
[ ! -e state/deploying ] || exit 0
mkdir -p state
exec 9>state/index.lock
flock -n 9 || exit 0
docker compose run --rm -T --entrypoint /app/search-drain tools
docker compose run --rm -T --entrypoint /app/rollup-facets tools

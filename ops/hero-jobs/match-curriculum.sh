#!/bin/sh
set -eu
umask 077
cd /opt/hero-jobs
[ ! -e state/deploying ] || exit 0
mkdir -p state logs
exec 9>state/curriculum.lock
flock -n 9 || exit 0
docker compose run --rm -T --no-deps --entrypoint /app/curriculum-match tools

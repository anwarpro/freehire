#!/bin/sh
# Share collection's lock: discovery and stale-job probes use the same host budget.
set -eu
umask 077
cd /opt/hero-jobs
[ ! -e state/deploying ] || exit 0
mkdir -p state logs
exec 9>state/collect.lock
flock -n 9 || exit 0
docker compose run --rm -T --no-deps -e GOMEMLIMIT=900MiB --entrypoint /app/liveness tools -source=linkedin

#!/bin/sh
# Run one provider at a time, oldest attempt first, so a large catalog stays bounded.
set -eu
umask 077
cd /opt/hero-jobs
[ ! -e state/deploying ] || exit 0
mkdir -p state/providers logs
exec 9>state/collect.lock
flock -n 9 || exit 0
selection=$(python3 ./next-collection.py)
provider=$(printf '%s\n' "$selection" | sed -n '1p')
board=$(printf '%s\n' "$selection" | sed -n '2p')
region=$(printf '%s\n' "$selection" | sed -n '3p')
[ -n "$provider" ] || exit 0
set -- "$provider"
if [ -n "$board" ]; then set -- "$@" "--board=$board" "--region=$region"; fi
printf '%s starting %s\n' "$(date -u +%FT%TZ)" "$provider"
log="logs/collect-$provider.log"
if [ -n "$board" ]; then
    key=$(printf '%s\n%s' "$board" "$region" | sha256sum | cut -c1-16)
    log="logs/collect-$provider-$key.log"
fi
status=0
docker compose run --rm -T --name hero-jobs-collector -e GOMEMLIMIT=900MiB -e INGEST_RUN_TIMEOUT_SECONDS=1200 -e INGEST_SWEEP_RESERVE_SECONDS=120 --entrypoint /app/ingest tools "$@" > "$log" 2>&1 || status=$?
printf '%s provider=%s exit=%s log=%s\n' "$(date -u +%FT%TZ)" "$provider" "$status" "$log" >> logs/collector-results.log
docker compose run --rm -T --entrypoint /app/recount-companies app
exit "$status"

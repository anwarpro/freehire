#!/bin/sh
# The existing notify worker owns matching, daily timing, retries and deduplication.
set -eu
umask 077
runtime=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
cd "$runtime"
# Deployment installs the schedule without activating delivery. An operator enables
# it only after configuring email and verifying controlled opt-in recipients.
[ -f state/job-alerts.enabled ] || exit 0
mkdir -p state logs
exec 8>state/deploy.lock
flock -sn 8 || exit 0
[ ! -e state/deploying ] || exit 0
exec 9>state/notify.lock
flock -n 9 || exit 0
docker compose run --rm -T --no-deps --entrypoint /bin/sh tools -c '
  if [ -z "${AWS_REGION:-}" ] || [ -z "${NOTIFY_EMAIL_FROM:-}" ] || [ -z "${MEILI_MASTER_KEY:-}" ] || [ -z "${JWT_SECRET:-}" ]; then
    echo "notify: configure email region/sender, search and session credentials before enabling job alerts" >&2
    exit 1
  fi
  exec /app/notify
'

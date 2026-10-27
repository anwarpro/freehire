#!/bin/sh
# Run one provider at a time, oldest attempt first, so a large catalog stays bounded.
set -eu
cd /opt/hero-jobs
[ ! -e state/deploying ] || exit 0
mkdir -p state/providers logs
exec 9>state/collect.lock
flock -n 9 || exit 0
provider=$(python3 - <<'PYTHON'
from pathlib import Path
import re
import subprocess
result = subprocess.run(
    ['docker', 'compose', 'exec', '-T', 'db', 'psql', '-U', 'hire', '-d', 'hire',
     '-Atc', "SELECT DISTINCT provider FROM boards WHERE status IN ('active','pending')"],
    check=True, capture_output=True, text=True,
)
policy = Path('enabled-providers.txt')
enabled = set(policy.read_text().split()) if policy.exists() else None
candidates = []
for name in result.stdout.splitlines():
    if enabled is not None and name not in enabled:
        continue
    if not re.fullmatch(r'[a-zA-Z0-9_-]+', name):
        continue
    path = Path('state/providers') / name
    candidates.append((path.stat().st_mtime if path.exists() else 0, name))
print(min(candidates)[1] if candidates else '')
PYTHON
)
[ -n "$provider" ] || exit 0
touch "state/providers/$provider"
printf '%s starting %s\n' "$(date -u +%FT%TZ)" "$provider"
status=0
docker compose run --rm -T --name hero-jobs-collector -e GOMEMLIMIT=900MiB -e INGEST_RUN_TIMEOUT_SECONDS=1200 -e INGEST_SWEEP_RESERVE_SECONDS=120 --entrypoint /app/ingest tools "$provider" > "logs/collect-$provider.log" 2>&1 || status=$?
printf '%s provider=%s exit=%s\n' "$(date -u +%FT%TZ)" "$provider" "$status" >> logs/collector-results.log
docker compose run --rm -T --entrypoint /app/recount-companies app
exit "$status"

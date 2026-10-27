#!/bin/sh
# Try one board-keyed provider per run using upstream discovery support.
set -eu
cd /opt/hero-jobs
[ ! -e state/deploying ] || exit 0
mkdir -p state/discovery logs
exec 9>state/discovery.lock
flock -n 9 || exit 0
[ -s source-inventory.json ] || exit 0
provider=$(python3 - <<'PY'
import json
from pathlib import Path
items=json.loads(Path('source-inventory.json').read_text())
candidates=[]
supported=set(Path('discovery-providers.txt').read_text().split())
for item in items:
    if not item['needs_company_board'] or not item['credentials_ready']: continue
    name=item['provider']
    if name not in supported: continue
    if not all(c.isalnum() or c in '_-' for c in name): continue
    p=Path('state/discovery') / name
    candidates.append((p.stat().st_mtime if p.exists() else 0,name))
print(min(candidates)[1] if candidates else '')
PY
)
[ -n "$provider" ] || exit 0
touch "state/discovery/$provider"
trap 'docker stop -t 30 hero-jobs-discovery >/dev/null 2>&1 || true' EXIT INT TERM
status=0
timeout --signal=TERM --kill-after=60 1800 docker compose run --rm -T --name hero-jobs-discovery --entrypoint /app/harvest-boards tools --apply -workers 2 -pace 1 "$provider" > "logs/discovery-$provider.log" 2>&1 || status=$?
printf '%s provider=%s exit=%s\n' "$(date -u +%FT%TZ)" "$provider" "$status" >> logs/discovery-results.log
exit "$status"

#!/bin/sh
# Discover public tech-company board candidates, then validate them in bounded batches.
set -eu
cd /opt/hero-jobs
[ ! -e state/deploying ] || exit 0
mkdir -p state/tech-seeds logs
exec 9>state/tech-expansion.lock
flock -n 9 || exit 0
PYTHONPATH=/opt/freehire/scripts python3 - <<'PYTHON'
import json
from pathlib import Path
import harvest_boards
candidates = harvest_boards.harvest_aggregators()
if not candidates:
    raise SystemExit('No candidates discovered; retaining previous catalog and seeds')
grouped = {}
for (provider, board), company in candidates.items():
    grouped.setdefault(provider, []).append({'board': board, 'company': company})
for provider, rows in grouped.items():
    path = Path('state/tech-seeds') / f'{provider}.json'
    path.write_text(json.dumps(rows))
    print(f'{provider}: {len(rows)} candidate boards')
PYTHON
status=0
for seed in state/tech-seeds/*.json; do
    provider=$(basename "$seed" .json)
    grep -qxF "$provider" enabled-providers.txt || continue
    result=0
    ./import-sources.sh "$provider" "$seed" > "logs/import-$provider.log" 2>&1 || result=$?
    printf '%s provider=%s exit=%s\n' "$(date -u +%FT%TZ)" "$provider" "$result" >> logs/tech-expansion-results.log
    [ "$result" -eq 0 ] || status=1
done
exit "$status"

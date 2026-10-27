#!/bin/sh
# Probe candidate boards in bounded batches and persist only live validated boards.
set -eu
[ "$#" -eq 2 ] || { echo 'Usage: import-sources.sh PROVIDER SEED_JSON_FILE' >&2; exit 2; }
provider=$1
seed=$(realpath "$2")
cd /opt/hero-jobs
mkdir -p state/imports
batch_dir=$(mktemp -d /opt/hero-jobs/state/imports/batch.XXXXXX)
trap 'rm -rf "$batch_dir"' EXIT HUP INT TERM
python3 - "$seed" "$batch_dir" <<'PYTHON'
import json
import sys
from pathlib import Path
items = json.loads(Path(sys.argv[1]).read_text())
if not isinstance(items, list) or not items:
    raise SystemExit('Seed JSON must contain a non-empty array of board IDs or board/company objects')
for item in items:
    if not isinstance(item, (str, dict)) or (isinstance(item, dict) and not item.get('board')):
        raise SystemExit('Each seed must be a board ID string or an object with a board field')
for start in range(0, len(items), 50):
    path = Path(sys.argv[2]) / f'{start // 50:06d}.json'
    path.write_text(json.dumps(items[start:start + 50]))
    path.chmod(0o644)
print(f'Validating {len(items)} candidate boards in batches of at most 50')
PYTHON
for batch in "$batch_dir"/*.json; do
    docker compose run --rm -T --no-deps -v "$batch:/seeds.json:ro" \
        --entrypoint /app/harvest-boards tools \
        --apply -workers 2 -pace 1 "$provider" /seeds.json
done

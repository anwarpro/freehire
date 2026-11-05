#!/usr/bin/env python3
"""Choose a due company source before the general provider rotation.

Output three lines (provider, board, region). A blank board means a provider run.
Source attempts are recorded locally even when a worker fails before board health
can be persisted, preventing a broken source from starving the rotation.
"""
from pathlib import Path
import hashlib
import json
import re
import subprocess
import time


def choose(rows, enabled, state, now):
    careers = []
    providers = set()
    for row in rows:
        name = row['provider']
        if not re.fullmatch(r'[a-zA-Z0-9_-]+', name):
            continue
        if enabled is None or name in enabled:
            providers.add(name)
        if row['surface'] != 'company-career':
            continue
        board, region = row['board'], row['region']
        if any(c in board + region for c in '\r\n\x00'):
            continue
        key = hashlib.sha256(f'{name}\0{board}\0{region}'.encode()).hexdigest()
        stamp = state / 'career-pages' / key
        attempted = stamp.stat().st_mtime if stamp.exists() else 0
        if now - attempted >= 3600:
            careers.append((attempted, name, board, region, stamp))
    if careers:
        _, provider, board, region, stamp = min(careers)
        stamp.parent.mkdir(parents=True, exist_ok=True)
        stamp.touch()
        return provider, board, region
    candidates = []
    for provider in providers:
        stamp = state / 'providers' / provider
        candidates.append((stamp.stat().st_mtime if stamp.exists() else 0, provider))
    if not candidates:
        return '', '', ''
    provider = min(candidates)[1]
    stamp = state / 'providers' / provider
    stamp.parent.mkdir(parents=True, exist_ok=True)
    stamp.touch()
    return provider, '', ''


def main():
    result = subprocess.run(
        ['docker', 'compose', 'exec', '-T', 'db', 'psql', '-U', 'hire', '-d', 'hire', '-Atc',
         "SELECT COALESCE(json_agg(t), '[]'::json) FROM (SELECT DISTINCT provider, ''::text AS board, ''::text AS region, 'curator'::text AS surface FROM boards WHERE status IN ('active','pending') UNION ALL SELECT provider, board, region, surface FROM boards WHERE status IN ('active','pending') AND surface='company-career') t"],
        check=True, capture_output=True, text=True,
    )
    policy = Path('enabled-providers.txt')
    enabled = set(policy.read_text().split()) if policy.exists() else None
    for value in choose(json.loads(result.stdout), enabled, Path('state'), time.time()):
        print(value)


if __name__ == '__main__':
    main()

#!/bin/sh
# Run as root on the Droplet after checking out the desired fork revision.
set -eu
umask 077
repo=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
config="$repo/ops/hero-jobs"
runtime=/opt/hero-jobs
[ "$repo" = /opt/freehire ] || { echo 'Expected source checkout at /opt/freehire' >&2; exit 1; }
mkdir -p "$runtime/logs" "$runtime/state" "$runtime/backups"
exec 8>"$runtime/state/deploy.lock"
flock -n 8 || { echo 'A deployment is already running' >&2; exit 1; }
touch "$runtime/state/deploying"
trap 'rm -f "$runtime/state/deploying"' EXIT HUP INT TERM
for file in next-collection.py match-curriculum.sh check-linkedin.sh compose.yaml compose.override.yaml Caddyfile nginx.conf Dockerfile.tools build-tools.sh backup.sh refresh-jobs.sh index-jobs.sh refresh-stats.sh discover-boards.sh discovery-providers.txt enabled-providers.txt hero-jobs.cron logrotate.conf greenhouse-seeds.json add-source.sh import-sources.sh expand-tech-catalog.sh; do
    install -m 644 "$config/$file" "$runtime/$file"
done
for file in "$runtime"/*.sh; do chmod 700 "$file"; done
if [ ! -f "$runtime/.env" ]; then
    printf 'DB_PASSWORD=%s\nJWT_SECRET=%s\nMEILI_MASTER_KEY=%s\n' \
        "$(openssl rand -hex 24)" "$(openssl rand -hex 32)" "$(openssl rand -hex 32)" > "$runtime/.env"
fi
chmod 600 "$runtime/.env"
cd "$runtime"
docker compose config --quiet
docker compose up -d db redis meilisearch
docker compose build app
docker compose build web
./build-tools.sh
./backup.sh
docker compose run --rm -T --no-deps --entrypoint /app/migrate app
# Stop collection only for the service replacement, retaining its cursor and all data.
docker stop -t 150 hero-jobs-collector >/dev/null 2>&1 || true
docker compose up -d --no-build app web proxy
docker compose run --rm -T --no-deps --entrypoint /app/hero-bootstrap tools > source-inventory.json
docker compose run --rm -T --no-deps --entrypoint /app/hero-bootstrap tools --apply
# Source onboarding is independent of service health: a third-party outage must
# remain visible without failing an otherwise healthy application deployment.
if [ ! -f state/career-shortlist-initialized ]; then
    if docker compose run --rm -T --no-deps --entrypoint /app/company-careers tools --apply > logs/career-shortlist-bootstrap.log 2>&1; then
        touch state/career-shortlist-initialized
    else
        echo 'Some career sources were not enabled; inspect logs/career-shortlist-bootstrap.log and the moderator source manager.' >&2
    fi
fi
install -m 644 hero-jobs.cron /etc/cron.d/hero-jobs
install -m 644 logrotate.conf /etc/logrotate.d/hero-jobs
systemctl enable --now cron
revision=$(git -C "$repo" rev-parse HEAD)
printf '%s\n' "$revision" > deployed-revision
attempt=0
until curl -fsS --max-time 10 https://jobs.programming-hero.org/health; do
    attempt=$((attempt + 1))
    [ "$attempt" -lt 12 ] || { echo 'Health verification failed' >&2; exit 1; }
    sleep 5
done
printf '\nDeployed %s\n' "$revision"

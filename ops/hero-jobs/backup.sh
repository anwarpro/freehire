#!/bin/sh
set -eu
cd /opt/hero-jobs
umask 077
mkdir -p backups
stamp=$(date -u +%Y%m%dT%H%M%SZ)
docker compose exec -T db pg_dump -U hire -d hire -Fc > "backups/jobs-$stamp.dump.partial"
mv "backups/jobs-$stamp.dump.partial" "backups/jobs-$stamp.dump"
find backups -name 'jobs-*.dump' -mtime +7 -delete

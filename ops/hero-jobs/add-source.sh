#!/bin/sh
# Usage: add-source.sh PROVIDER BOARD_ID COMPANY_NAME
set -eu
[ "$#" -eq 3 ] || { echo 'Usage: add-source.sh PROVIDER BOARD_ID COMPANY_NAME' >&2; exit 2; }
cd /opt/hero-jobs
docker compose run --rm -T --no-deps --entrypoint /app/add-board tools \
    --provider="$1" --board="$2" --company="$3" --apply
printf 'Source registered. The collector rotation will include it automatically.\n'

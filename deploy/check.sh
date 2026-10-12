#!/usr/bin/env bash
# Dedicated disposable Compose project. Never point this check at real club data.
set -euo pipefail
cd "$(dirname "$0")"
project="${1:-judeoslite-check}"
if [[ "$project" != judeoslite-check* && "$project" != judeoslite-ci* ]]; then
  echo 'Check project name must begin with judeoslite-check or judeoslite-ci.' >&2
  exit 1
fi
export POSTGRES_DB=check POSTGRES_USER=check POSTGRES_PASSWORD=check-only APP_PORT=18080
export APP_IMAGE_TAG="${APP_IMAGE_TAG:-ci}"
compose=(docker compose --project-name "$project" --env-file /dev/null -f compose.yml)
cleanup() { "${compose[@]}" down --volumes --remove-orphans >/dev/null; }
trap cleanup EXIT
"${compose[@]}" up -d --no-build --wait --wait-timeout 120
curl --fail --silent http://127.0.0.1:18080/api/healthz
curl --fail --silent http://127.0.0.1:18080/api/readyz
curl --fail --silent http://127.0.0.1:18080/ | python3 -c 'import sys; assert "PRIDE" in sys.stdin.read()'
"${compose[@]}" stop db
status="$(curl --silent --output /dev/null --write-out '%{http_code}' http://127.0.0.1:18080/api/readyz)"
test "$status" = 503
curl --fail --silent http://127.0.0.1:18080/api/healthz
"${compose[@]}" start db
python3 - <<'PY'
import time, urllib.request, urllib.error
for _ in range(30):
    try:
        with urllib.request.urlopen('http://127.0.0.1:18080/api/readyz', timeout=3) as r:
            if r.status == 200: break
    except (OSError, urllib.error.URLError): pass
    time.sleep(1)
else: raise SystemExit('Readiness did not recover')
PY
"${compose[@]}" run --rm migrate
"${compose[@]}" exec -T db psql -U check -d check -Atc 'SELECT count(*) FROM public.schema_migrations' | python3 -c 'import sys; assert sys.stdin.read().strip()=="1"'
# A full restart must preserve the checksum history and data volume.
"${compose[@]}" down
"${compose[@]}" up -d --no-build --wait --wait-timeout 120
curl --fail --silent http://127.0.0.1:18080/api/readyz
"${compose[@]}" exec -T db psql -U check -d check -Atc 'SELECT count(*) FROM public.schema_migrations' | python3 -c 'import sys; assert sys.stdin.read().strip()=="1"'
# Production template has no db ports; only app is bound to localhost.
"${compose[@]}" config --format json | python3 -c 'import json,sys; c=json.load(sys.stdin); assert not c["services"]["db"].get("ports"); assert c["services"]["app"]["ports"][0]["host_ip"]=="127.0.0.1"'
echo 'Compose clean startup, rerun, restart and port checks passed.'

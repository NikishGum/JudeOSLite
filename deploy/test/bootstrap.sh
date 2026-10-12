#!/usr/bin/env bash
# Manual root operation on VPS from an owner-reviewed checkout. No remote fetch.
set -euo pipefail
test "$(id -u)" = 0 || { echo 'Run as root.' >&2; exit 1; }
test "$#" = 0 || { echo 'No arguments; paths are fixed.' >&2; exit 1; }
source_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
command -v python3 >/dev/null
command -v docker >/dev/null
command -v systemd-run >/dev/null
command -v ufw >/dev/null
getent passwd judeos-deploy >/dev/null
# Validate root-owned ancestry before writing; refuse symlink installations.
python3 - "$source_dir" <<'PY'
import os, stat, sys
from pathlib import Path
for target in ('/srv/judeos-test', '/srv/judeos-test-config', '/usr/local/lib/judeos-deploy'):
    p=Path(target)
    for node in (p, *p.parents):
        if node.exists() or node.is_symlink():
            s=node.lstat()
            if stat.S_ISLNK(s.st_mode) or s.st_uid != 0 or s.st_mode & 0o022:
                raise SystemExit('Unsafe target ancestry: '+str(node))
PY
docker compose version
docker info --format '{{.ServerVersion}}'
install -d -o root -g root -m 0700 /srv/judeos-test /srv/judeos-test-config
install -d -o root -g root -m 0755 /usr/local/lib/judeos-deploy
install -d -o root -g root -m 0700 /srv/judeos-test/backups /srv/judeos-test/state /srv/judeos-test-config/registry
# Upload directory reserved for the future file API; application does not use it yet.
install -d -o 65532 -g 65532 -m 0700 /srv/judeos-test/uploads
install -o root -g root -m 0600 "$source_dir/compose.yml" /srv/judeos-test/compose.yml
install -o root -g root -m 0600 "$source_dir/Caddyfile" /srv/judeos-test/Caddyfile
install -o root -g root -m 0755 "$source_dir/admin.py" /usr/local/lib/judeos-deploy/admin.py
install -o root -g root -m 0644 "$source_dir/operations.py" /usr/local/lib/judeos-deploy/operations.py
install -o root -g root -m 0755 "$source_dir/controller.py" /usr/local/lib/judeos-deploy/controller.v1-19.py
python3 -I /usr/local/lib/judeos-deploy/admin.py init-secrets
echo 'Staged. Existing controller, authorized_keys, sudoers and firewall are unchanged.'
echo 'Next: root registry-login; review/activate controller; install restricted public key; firewall; preflight.'

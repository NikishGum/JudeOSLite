#!/usr/bin/python3 -I
"""Root-only encrypted backups, using the V1-19 deployment lock and archives."""
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent))
import operations as ops

RESTIC = '/usr/bin/restic'
TAG = 'judeos-v1'
HOST = 'judeos-test'


def configuration():
    path = ops.CONFIG / 'offsite.json'
    ops.trusted(path)
    if path.stat().st_mode & 0o077:
        raise RuntimeError('offsite.json must be root-only 0600.')
    config = json.loads(path.read_text())
    if set(config) != {'repository', 'password_file', 'environment', 'retention', 'local_keep'}:
        raise RuntimeError('Invalid offsite configuration fields.')
    # The operator must select an actual offsite repository; no local fallback.
    if not isinstance(config['repository'], str) or not config['repository'].startswith(('s3:https://', 'sftp:', 'rest:https://', 'azure:', 'gs:', 'b2:')):
        raise RuntimeError('An encrypted offsite repository is required.')
    password = Path(config['password_file'])
    if not password.is_absolute() or '..' in password.parts or not password.is_relative_to(ops.CONFIG):
        raise RuntimeError('Password file must be in the root-only config directory.')
    ops.trusted(password)
    if not password.is_file() or password.stat().st_mode & 0o077:
        raise RuntimeError('Repository password file must be root-only 0600.')
    allowed = {'AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY', 'AWS_DEFAULT_REGION',
               'AWS_SESSION_TOKEN', 'AZURE_ACCOUNT_NAME', 'AZURE_ACCOUNT_KEY',
               'GOOGLE_PROJECT_ID', 'GOOGLE_APPLICATION_CREDENTIALS',
               'B2_ACCOUNT_ID', 'B2_ACCOUNT_KEY'}
    if not isinstance(config['environment'], dict) or not set(config['environment']) <= allowed or any(not isinstance(v, str) for v in config['environment'].values()):
        raise RuntimeError('Unsupported credential environment.')
    if set(config['retention']) != {'daily', 'weekly', 'monthly'} or any(type(v) is not int or not 1 <= v <= 365 for v in config['retention'].values()):
        raise RuntimeError('Explicit positive daily/weekly/monthly retention required.')
    if type(config['local_keep']) is not int or not 2 <= config['local_keep'] <= 365:
        raise RuntimeError('Explicit local retention (2..365 archives) required.')
    return config


def restic(config, *arguments):
    environment = dict(ops.ENV, RESTIC_REPOSITORY=config['repository'],
                       RESTIC_PASSWORD_FILE=config['password_file'],
                       RESTIC_CACHE_DIR=str(ops.CONFIG / 'restic-cache'), **config['environment'])
    result = subprocess.run([RESTIC, *arguments], env=environment, capture_output=True)
    # Backend errors can include access keys/URLs; never relay stdout/stderr.
    if result.returncode: raise RuntimeError('Offsite command failed; no retention cleanup performed unless upload and check succeeded.')
    return result.stdout


def prune_local(config):
    # Called only after successful remote upload/check/retention. Retain archives
    # referenced by current/previous/operation even beyond the local count.
    protected = {(ops.load(name) or {}).get('backup') for name in ('current', 'previous', 'operation')}
    archives = []
    for path in (ops.BASE / 'backups').iterdir():
        if re.fullmatch(r'[0-9]{8}T[0-9]{6}-[0-9a-f]{8}', path.name):
            ops.trusted_tree(path)
            if (path / 'metadata.json').is_file(): archives.append(path)
    for path in sorted(archives, reverse=True)[config['local_keep']:]:
        if path.name in protected: continue
        # Refuse unexpected contents, incomplete archives and subdirectories.
        expected = {'database.dump', 'uploads.tar', 'metadata.json'}
        if {p.name for p in path.iterdir()} != expected or any(not p.is_file() for p in path.iterdir()):
            raise RuntimeError('Unexpected local archive contents; cleanup refused.')
        for item in path.iterdir(): item.unlink()
        path.rmdir()


def backup_offsite(config):
    ops.save('offsite', {'phase': 'backup', 'started_at': datetime.now(timezone.utc).isoformat()})
    name = ops.maintenance_backup()
    output = restic(config, 'backup', '--json', '--host', HOST, '--tag', TAG, str(ops.BASE / 'backups' / name))
    summary = next((json.loads(line) for line in output.splitlines() if json.loads(line).get('message_type') == 'summary'), {})
    snapshot = summary.get('snapshot_id', '')
    if not re.fullmatch(r'[0-9a-f]{64}', snapshot): raise RuntimeError('Offsite upload did not return a full snapshot ID.')
    ops.save('offsite', {'phase': 'uploaded', 'backup': name, 'snapshot': snapshot})
    restic(config, 'check')
    retention = config['retention']
    restic(config, 'forget', '--host', HOST, '--tag', TAG, '--group-by', 'host,tags',
           '--keep-daily', str(retention['daily']), '--keep-weekly', str(retention['weekly']),
           '--keep-monthly', str(retention['monthly']), '--prune')
    prune_local(config)
    ops.save('offsite', {'phase': 'ready', 'backup': name, 'snapshot': snapshot,
                        'completed_at': datetime.now(timezone.utc).isoformat()})
    print('PASS: encrypted offsite upload and repository check. Snapshot:', snapshot)


def restore_offsite(config, snapshot):
    if not re.fullmatch(r'[0-9a-f]{64}', snapshot): raise RuntimeError('Full immutable snapshot ID required.')
    snapshots = json.loads(restic(config, 'snapshots', '--json', snapshot))
    if len(snapshots) != 1 or snapshots[0]['id'] != snapshot or snapshots[0].get('hostname') != HOST or TAG not in snapshots[0].get('tags', []):
        raise RuntimeError('Snapshot does not belong to this installation.')
    with tempfile.TemporaryDirectory(prefix='offsite-restore-', dir=ops.BASE / 'backups') as temporary:
        target = Path(temporary)
        restic(config, 'restore', snapshot, '--target', str(target))
        manifests = list(target.rglob('metadata.json'))
        if len(manifests) != 1: raise RuntimeError('Expected one complete archive in snapshot.')
        archive = manifests[0].parent
        db = ops.restore_check(archive.name, source_root=archive.parent)
        ops.save('offsite-restore', {'snapshot': snapshot, 'database': db,
                 'verified_at': datetime.now(timezone.utc).isoformat()})
        print('PASS: archive downloaded from offsite and restored; live database unchanged.')
        return db


def main():
    if os.geteuid() != 0: raise RuntimeError('Run as root.')
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    sub.add_parser('init')
    sub.add_parser('backup')
    sub.add_parser('check')
    sub.add_parser('restore-check').add_argument('snapshot')
    args = parser.parse_args()
    ops.trusted_tree(ops.LIB)
    config = configuration()
    with ops.lock():
        if args.command == 'init':
            restic(config, 'init'); print('Encrypted repository initialized; timer is not enabled automatically.')
        elif args.command == 'backup': backup_offsite(config)
        elif args.command == 'check': restic(config, 'check'); print('Repository indexes checked; use restore-check to verify actual recovery.')
        elif args.command == 'restore-check': restore_offsite(config, args.snapshot)


if __name__ == '__main__':
    try: main()
    except RuntimeError as error:
        print(str(error), file=sys.stderr); sys.exit(1)
    except Exception:
        print('Offsite operation failed; inspect safe state and configuration locally.', file=sys.stderr); sys.exit(1)

#!/usr/bin/python3 -I
"""Root-only manual operations. Never granted through sudoers to deploy user."""
import argparse
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import operations as ops


def main():
    if os.geteuid() != 0:
        raise RuntimeError('Run as root.')
    p = argparse.ArgumentParser(description=__doc__)
    sub = p.add_subparsers(dest='command', required=True)
    for name in ('init-secrets', 'registry-login', 'preflight', 'backup', 'backup-local', 'status', 'repair-ssh', 'restart-check'):
        sub.add_parser(name)
    a = sub.add_parser('activate-controller')
    a.add_argument('--expected-old-sha256', required=True)
    a = sub.add_parser('install-key')
    a.add_argument('public_key_file')
    a = sub.add_parser('deploy')
    a.add_argument('sha'); a.add_argument('image'); a.add_argument('generation', type=int)
    a = sub.add_parser('restore-check')
    a.add_argument('backup_name')
    sub.add_parser('drop-restore-check').add_argument('database')
    a = sub.add_parser('rollback')
    a.add_argument('--confirm-compatible', action='store_true', required=True)
    a = sub.add_parser('create-owner')
    a.add_argument('--login', required=True); a.add_argument('--name', required=True)
    args = p.parse_args()
    ops.trusted_tree(ops.LIB)
    if args.command == 'init-secrets': ops.init_secrets()
    elif args.command == 'registry-login': ops.registry_login()
    elif args.command == 'activate-controller': ops.activate_controller(args.expected_old_sha256)
    elif args.command == 'install-key': ops.install_key(Path(args.public_key_file))
    elif args.command == 'preflight': ops.preflight()
    elif args.command == 'repair-ssh': ops.repair_ssh()
    elif args.command == 'status': ops.status()
    else:
        with ops.lock():
            if args.command == 'deploy': ops.deploy(ops.validate_request({'sha': args.sha, 'image': args.image, 'generation': args.generation}))
            elif args.command == 'backup': print('Backup:', ops.maintenance_backup())
            elif args.command == 'backup-local': ops.local_backup()
            elif args.command == 'restart-check': ops.restart_check()
            elif args.command == 'restore-check': ops.restore_check(args.backup_name)
            elif args.command == 'drop-restore-check': ops.drop_restore_check(args.database)
            elif args.command == 'rollback': ops.rollback()
            elif args.command == 'create-owner': ops.create_owner(args.login, args.name)


if __name__ == '__main__':
    try:
        main()
    except RuntimeError as error:
        # Only our fixed, secret-free diagnostic messages use RuntimeError.
        print(str(error), file=sys.stderr)
        sys.exit(1)
    except Exception:
        # PostgreSQL/Docker output may include secrets. Inspect root-only unit
        # status, not environment dumps or unredacted application logs.
        print('Operation failed; inspect status and run preflight as root.', file=sys.stderr)
        sys.exit(1)

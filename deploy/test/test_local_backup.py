import configparser
import contextlib
import io
import json
import os
from pathlib import Path
import tempfile
import tarfile
import unittest
from unittest.mock import patch

import admin
import operations as ops


def archive(base, index, *, legacy=False):
    path = base / 'backups' / f'20261013T0000{index:02d}-aaaaaaaa'
    path.mkdir()
    (path / 'database.dump').write_bytes(b'PGDMP synthetic dump')
    with tarfile.open(path / 'uploads.tar', 'w') as output:
        directory = tarfile.TarInfo('uploads'); directory.type = tarfile.DIRTYPE
        output.addfile(directory)
    metadata = {'current': None, 'history': '1|checksum\n',
                'database_sha256': ops.checksum(path / 'database.dump'),
                'uploads_sha256': ops.checksum(path / 'uploads.tar')}
    if not legacy: metadata.update(data={}, uploads={})
    (path / 'metadata.json').write_text(json.dumps(metadata))
    os.utime(path / 'metadata.json', ns=(1_500_000_000_000_000_000 + index,)*2)
    return path


class LocalBackupTests(unittest.TestCase):
    def test_seven_latest_plus_each_release_reference_including_legacy_archives(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary); (base / 'backups').mkdir()
            copies = [archive(base, n, legacy=n%2 == 0) for n in range(13)]
            states = {name: {'backup': copies[index].name} for name, index in [('current', 0), ('previous', 2), ('operation', 4)]}
            with patch.object(ops, 'BASE', base), patch.object(ops, 'trusted'), patch.object(ops, 'load', side_effect=states.get):
                result = ops.prune_local_backups()
            expected = {p.name for p in copies[6:]} | {copies[i].name for i in (0,2,4)}
            self.assertEqual({p.name for p in (base / 'backups').iterdir()}, expected)
            self.assertEqual(set(result['archives']), expected)
            self.assertEqual(result['completed'], 10); self.assertEqual(result['removed'], 3)
            self.assertEqual(result['preserved_unknown'], 0)

    def test_same_second_uses_manifest_completion_time_not_random_suffix(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary); (base / 'backups').mkdir()
            old = archive(base, 0); recent = archive(base, 1)
            recent = recent.rename(base / 'backups' / '20261013T000000-00000000')
            with patch.object(ops, 'BASE', base), patch.object(ops, 'trusted'), patch.object(ops, 'load', return_value=None):
                ops.prune_local_backups(keep=1)
            self.assertTrue(recent.exists()); self.assertFalse(old.exists())

    def test_unknown_partial_corrupt_extra_files_and_links_never_deleted(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary); backups = base / 'backups'; backups.mkdir()
            known = [archive(base, n) for n in range(9)]
            unknown = []
            incomplete = backups / '20261012T000000-aaaaaaaa'; incomplete.mkdir()
            (incomplete / 'database.dump.partial').write_text('keep'); unknown.append(incomplete)
            folder = backups / 'operator-notes'; folder.mkdir(); (folder / 'keep').write_text('keep'); unknown.append(folder)
            plain = backups / 'unexpected-file'; plain.write_text('keep'); unknown.append(plain)
            for index, kind in enumerate(('extra', 'json', 'checksum', 'future-schema', 'symlink-file', 'directory-file', 'hardlink-file', 'invalid-dump-format', 'invalid-tar-format'), 10):
                path = archive(base, index)
                if kind == 'extra': (path / 'operator-note').write_text('keep')
                elif kind == 'json': (path / 'metadata.json').write_text('not JSON')
                elif kind == 'checksum': (path / 'database.dump').write_text('corrupt')
                elif kind == 'future-schema':
                    metadata = json.loads((path / 'metadata.json').read_text()); metadata['new-field'] = True
                    (path / 'metadata.json').write_text(json.dumps(metadata))
                elif kind in ('invalid-dump-format', 'invalid-tar-format'):
                    filename, field = ('database.dump', 'database_sha256') if kind == 'invalid-dump-format' else ('uploads.tar', 'uploads_sha256')
                    (path / filename).write_bytes(b'Unfamiliar format')
                    metadata = json.loads((path / 'metadata.json').read_text()); metadata[field] = ops.checksum(path / filename)
                    (path / 'metadata.json').write_text(json.dumps(metadata))
                else:
                    (path / 'database.dump').unlink()
                    if kind == 'symlink-file': (path / 'database.dump').symlink_to(plain)
                    elif kind == 'hardlink-file': os.link(plain, path / 'database.dump')
                    else: (path / 'database.dump').mkdir()
                unknown.append(path)
            linked = backups / '20261012T000001-bbbbbbbb'; linked.symlink_to(folder, target_is_directory=True); unknown.append(linked)
            invalid_date = archive(base, 20).rename(backups / '20269999T000000-aaaaaaaa'); unknown.append(invalid_date)
            with patch.object(ops, 'BASE', base), patch.object(ops, 'trusted'), patch.object(ops, 'load', return_value=None):
                result = ops.prune_local_backups()
            self.assertEqual(result['preserved_unknown'], len(unknown))
            self.assertEqual({p.name for p in backups.iterdir()}, {p.name for p in known[-7:]+unknown})
            self.assertTrue(linked.is_symlink()); self.assertEqual(plain.read_text(), 'keep')

    def test_invalid_reference_aborts_cleanup_before_any_unlink(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary); (base / 'backups').mkdir()
            copies = [archive(base, n) for n in range(9)]
            with patch.object(ops, 'BASE', base), patch.object(ops, 'trusted'), patch.object(ops, 'load', return_value={'backup': '../unknown'}):
                with self.assertRaisesRegex(RuntimeError, 'cleanup refused'): ops.prune_local_backups()
            self.assertTrue(all(p.is_dir() for p in copies))

    def test_failed_backup_or_readiness_never_runs_cleanup_or_reports_success(self):
        states = []
        with patch.object(ops, 'save', side_effect=lambda _, data: states.append(dict(data))), \
             patch.object(ops, 'maintenance_backup', side_effect=RuntimeError('backup/readiness failed')), \
             patch.object(ops, 'prune_local_backups') as prune, contextlib.redirect_stdout(io.StringIO()) as output:
            with self.assertRaises(RuntimeError): ops.local_backup()
        prune.assert_not_called(); self.assertEqual(states[-1]['phase'], 'failed')
        self.assertNotIn('PASS', output.getvalue())

    def test_cleanup_failure_retains_backup_id_and_non_success_status(self):
        states = []
        with patch.object(ops, 'save', side_effect=lambda _, data: states.append(dict(data))), \
             patch.object(ops, 'maintenance_backup', return_value='20261013T000000-aaaaaaaa'), \
             patch.object(ops, 'prune_local_backups', side_effect=RuntimeError('cleanup failed')):
            with self.assertRaises(RuntimeError): ops.local_backup()
        self.assertEqual(states[-1]['phase'], 'failed')
        self.assertEqual(states[-1]['backup'], '20261013T000000-aaaaaaaa')

    def test_root_admin_regular_backup_uses_same_deploy_lock(self):
        locked = False
        @contextlib.contextmanager
        def lock():
            nonlocal locked
            locked = True
            try: yield
            finally: locked = False
        def backup(): self.assertTrue(locked)
        with patch.object(admin.os, 'geteuid', return_value=0), patch.object(admin.sys, 'argv', ['admin.py', 'backup-local']), \
             patch.object(ops, 'trusted_tree'), patch.object(ops, 'lock', side_effect=lock), patch.object(ops, 'local_backup', side_effect=backup) as run:
            admin.main()
        run.assert_called_once(); self.assertFalse(locked)

    def test_agreed_service_and_exact_timer_without_offsite_or_jitter(self):
        root = Path(__file__).resolve().parent
        service = configparser.ConfigParser(); service.read(root / 'judeos-backup.service')
        self.assertEqual(service['Service']['ExecStart'], '/usr/bin/python3 -I /usr/local/lib/judeos-deploy/admin.py backup-local')
        self.assertEqual(service['Service']['User'], 'root')
        timer = configparser.ConfigParser(); timer.read(root / 'judeos-backup.timer')
        self.assertEqual(timer['Timer']['OnCalendar'], '*-*-* 03:00:00 Europe/Minsk')
        self.assertEqual(timer['Timer']['Persistent'], 'true')
        self.assertEqual(timer['Timer']['AccuracySec'], '1s')
        self.assertEqual(timer['Timer'].get('RandomizedDelaySec', '0'), '0')


if __name__ == '__main__': unittest.main()

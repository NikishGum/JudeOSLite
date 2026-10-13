import contextlib
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import operations as ops


class RecoveryTests(unittest.TestCase):
    def test_cleanup_never_accepts_live_or_arbitrary_database(self):
        for name in ('judeoslite', 'restorecheck_bad', 'restorecheck_aaaaaaaaaaaa; DROP DATABASE judeoslite'):
            with patch.object(ops, 'compose') as compose, self.assertRaises(RuntimeError):
                ops.drop_restore_check(name)
            compose.assert_not_called()

    def test_manifest_hashes_all_rows_without_retaining_or_logging_contacts(self):
        def compose(image, *args, **kwargs):
            if 'pg_tables' in args[-1]: return b'pride|participants\npublic|schema_migrations\n'
            kwargs['output'].write(b'{"id":"1","contact":"private"}\n{"id":"2","contact":"other"}\n')
        with patch.object(ops, 'compose', side_effect=compose), contextlib.redirect_stdout(io.StringIO()) as output:
            data = ops.data_manifest()
        self.assertEqual(set(data), {'pride|participants', 'public|schema_migrations'})
        self.assertEqual(data['pride|participants']['rows'], 2)
        self.assertEqual(len(data['pride|participants']['sha256']), 64)
        self.assertNotIn('private', json.dumps(data) + output.getvalue())

    def test_backup_refuses_changes_in_business_rows_before_completed_metadata(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary); (base / 'backups').mkdir(); (base / 'uploads').mkdir()
            def compose(image, *args, **kwargs):
                if 'pg_dump' in args: kwargs['output'].write(b'dump')
                return b''
            def run(command): Path(command[-2]).write_bytes(b'tar')
            with patch.object(ops, 'BASE', base), patch.object(ops, 'trusted'), patch.object(ops, 'compose', side_effect=compose), \
                 patch.object(ops, 'run', side_effect=run), patch.object(ops, 'data_manifest', side_effect=[{'rows': 1}, {'rows': 2}]):
                with self.assertRaisesRegex(RuntimeError, 'Data changed'): ops.backup()
            self.assertFalse(list((base / 'backups').rglob('metadata.json')))

    def test_failed_backup_resumes_verified_app_but_never_starts_stopped_release(self):
        states = {'current': {'sha': 'a'*40, 'history': 'same'}, 'operation': {'phase': 'ready'}}
        with patch.object(ops, 'load', side_effect=states.get), patch.object(ops, 'history', return_value='same'), \
             patch.object(ops, 'compose', return_value=b'app\ndb\ncaddy\n') as compose, \
             patch.object(ops, 'backup', side_effect=RuntimeError('backup failed')), patch.object(ops, 'verify_https') as https:
            with self.assertRaisesRegex(RuntimeError, 'backup failed'): ops.maintenance_backup()
            self.assertIn(unittest.mock.call(None, 'stop', 'app'), compose.call_args_list)
            self.assertIn(unittest.mock.call(None, 'up', '-d', '--no-deps', '--wait', '--wait-timeout', '180', 'app'), compose.call_args_list)
            https.assert_called_once_with('a'*40)
            compose.reset_mock(); compose.return_value = b'db\ncaddy\n'
            with self.assertRaisesRegex(RuntimeError, 'Application is stopped'): ops.maintenance_backup()
            self.assertFalse(any('up' in c.args for c in compose.call_args_list))
            states['operation']['phase'] = 'migrate'; compose.reset_mock()
            with self.assertRaisesRegex(RuntimeError, 'Verified release required'): ops.maintenance_backup()
            compose.assert_not_called()

    def test_archive_refuses_path_traversal_and_links(self):
        for name, kind in [('../escape', tarfile.REGTYPE), ('/escape', tarfile.REGTYPE),
                           ('uploads/link', tarfile.SYMTYPE), ('uploads/link', tarfile.LNKTYPE)]:
            with self.subTest(name=name, kind=kind), tempfile.TemporaryDirectory() as temporary:
                base = Path(temporary)
                with tarfile.open(base / 'uploads.tar', 'w') as archive:
                    member = tarfile.TarInfo(name); member.type = kind; member.linkname = '/etc/passwd'
                    archive.addfile(member, io.BytesIO())
                with self.assertRaises(RuntimeError): ops.verify_uploads(base, {})

    def test_restore_refuses_wrong_table_digest_even_when_counts_match(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary); archive = base / 'backups' / '20261012T000000-aaaaaaaa'; archive.mkdir(parents=True)
            (archive / 'database.dump').write_bytes(b'dump'); (archive / 'uploads.tar').write_bytes(b'tar')
            (archive / 'metadata.json').write_text(json.dumps({'history': 'same', 'data': {'pride|attendance_records': {'rows': 2, 'sha256': 'old'}},
                'uploads': {}, 'database_sha256': ops.checksum(archive / 'database.dump'), 'uploads_sha256': ops.checksum(archive / 'uploads.tar')}))
            with patch.object(ops, 'BASE', base), patch.object(ops, 'trusted_tree'), patch.object(ops, 'compose') as compose, \
                 patch.object(ops, 'history', return_value='same'), patch.object(ops, 'data_manifest', return_value={'pride|attendance_records': {'rows': 2, 'sha256': 'changed'}}):
                with self.assertRaisesRegex(RuntimeError, 'table contents differ'): ops.restore_check(archive.name)
                for call in compose.call_args_list:
                    args = call.args
                    if 'createdb' in args: self.assertRegex(args[-1], r'^restorecheck_[0-9a-f]{12}$')
                    if '-d' in args: self.assertRegex(args[args.index('-d')+1], r'^restorecheck_[0-9a-f]{12}$')


if __name__ == "__main__": unittest.main()

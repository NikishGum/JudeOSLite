import importlib.util
import io
import contextlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import threading
import unittest
from unittest.mock import patch

import operations as ops
import controller

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('select_deploy', ROOT / '.github/scripts/select_deploy.py')
selection = importlib.util.module_from_spec(spec)
spec.loader.exec_module(selection)
SHA = 'a' * 40
IMAGE = ops.IMAGE + '@sha256:' + 'b' * 64
REQUEST = {'sha': SHA, 'image': IMAGE, 'generation': 12}


class PolicyTests(unittest.TestCase):
    def test_controller_starts_detached_supervised_worker_without_shell(self):
        with patch.object(controller.os, 'geteuid', return_value=0), patch.object(controller.sys, 'argv', ['controller.py']), \
             patch.object(controller.sys, 'stdin') as stdin, patch.object(controller, 'trusted_tree'), \
             patch.object(controller.subprocess, 'run') as run, contextlib.redirect_stdout(io.StringIO()):
            stdin.buffer = io.BytesIO(json.dumps(REQUEST).encode())
            run.return_value.returncode = 0
            controller.main()
            command = run.call_args.args[0]
            self.assertEqual(command[0], '/usr/bin/systemd-run')
            self.assertIn('--property=TimeoutStartSec=0', command)
            self.assertNotIn('--pipe', command)
            self.assertEqual(command[-4:], ['deploy', SHA, IMAGE, '12'])
            self.assertNotIn('shell', run.call_args.kwargs)
    def test_only_successful_current_main_push_can_deploy(self):
        run = {'repository': {'id': 1}, 'head_repository': {'id': 1},
               'workflow_id': 2, 'event': 'push', 'head_branch': 'main',
               'status': 'completed', 'conclusion': 'success',
               'head_sha': SHA, 'run_number': 12}
        self.assertEqual(selection.validate(run, 1, 2, SHA), {'sha': SHA, 'generation': '12'})
        for name, value in [('repository', {'id': 3}), ('head_repository', {'id': 3}),
                            ('workflow_id', 3), ('event', 'pull_request'), ('head_branch', 'branch'),
                            ('status', 'in_progress'), ('conclusion', 'failure'), ('head_sha', 'c' * 40)]:
            with self.subTest(name=name), self.assertRaises(ValueError):
                selection.validate(dict(run, **{name: value}), 1, 2, SHA)

    def test_controller_rejects_arbitrary_image_path_command_and_payload(self):
        self.assertEqual(ops.validate_request(REQUEST), REQUEST)
        for request in [dict(REQUEST, command='sh'), dict(REQUEST, path='/root'),
                        dict(REQUEST, sha='a;id'), dict(REQUEST, image='alpine:latest'),
                        dict(REQUEST, image=IMAGE + ';id'), dict(REQUEST, generation=True),
                        dict(REQUEST, generation=-1), dict(REQUEST, generation='12'), []]:
            with self.subTest(request=request), self.assertRaises(ValueError):
                ops.validate_request(request)

    def test_watermark_rejects_stale_or_conflicting_retry_after_failure(self):
        ops.admission(REQUEST, REQUEST)
        ops.admission(dict(REQUEST, generation=13), REQUEST)
        for request in [dict(REQUEST, generation=11), dict(REQUEST, image=ops.IMAGE + '@sha256:' + 'c' * 64)]:
            with self.assertRaises(RuntimeError): ops.admission(request, REQUEST)

    def test_trust_refuses_symlinks_and_user_writable_tree(self):
        for uid, mode in [(1000, 0o100600), (0, 0o100666), (0, 0o120777)]:
            with patch.object(Path, 'lstat') as mock:
                mock.return_value.st_uid = uid
                mock.return_value.st_mode = mode
                with self.assertRaises(RuntimeError): ops.trusted(Path('/srv/test'))


class DeploymentTests(unittest.TestCase):
    def setUp(self):
        self.calls = []
        self.states = {'current': dict(REQUEST, generation=11, history='old')}
        self.patches = [patch.object(ops, 'preflight'),
                        patch.object(ops, 'load', side_effect=lambda name: self.states.get(name)),
                        patch.object(ops, 'save', side_effect=lambda name, value: self.states.__setitem__(name, value)),
                        patch.object(ops, 'compose', side_effect=self.compose),
                        patch.object(ops, 'backup', side_effect=lambda image: self.calls.append(('backup',)) or 'backup'),
                        patch.object(ops, 'history', return_value='new'),
                        patch.object(ops, 'run', return_value=(SHA + '\n').encode()),
                        patch.object(ops, 'verify_https', side_effect=lambda sha: self.calls.append(('https', sha)))]
        for p in self.patches: p.start(); self.addCleanup(p.stop)

    def compose(self, image=None, *args, **kwargs):
        self.calls.append(args)
        return b''

    def test_backup_then_one_migration_then_readiness_then_commit_state(self):
        ops.deploy(REQUEST)
        self.assertLess(self.calls.index(('stop', 'app')), self.calls.index(('backup',)))
        self.assertLess(self.calls.index(('backup',)), self.calls.index(('run', '--rm', '--no-deps', 'migrate')))
        self.assertLess(self.calls.index(('run', '--rm', '--no-deps', 'migrate')), self.calls.index(('https', SHA)))
        self.assertEqual(self.states['current']['sha'], SHA)
        self.assertEqual(self.states['previous']['generation'], 11)
        self.assertEqual(self.states['accepted'], REQUEST)
        self.assertFalse(any('down' in c or '--volumes' in c or 'seed' in c for c in self.calls))

    def test_migration_failure_retains_watermark_and_previous_release_no_restore(self):
        def fail(image=None, *args, **kwargs):
            self.calls.append(args)
            if args == ('run', '--rm', '--no-deps', 'migrate'): raise RuntimeError('fail')
            return b''
        with patch.object(ops, 'compose', side_effect=fail), self.assertRaises(RuntimeError): ops.deploy(REQUEST)
        self.assertEqual(self.states['accepted'], REQUEST)
        self.assertEqual(self.states['current']['generation'], 11)
        self.assertEqual(self.states['operation']['phase'], 'migrate')
        self.assertFalse(any('restore' in str(c) or 'up' in c and 'app' in c for c in self.calls))

    def test_revision_mismatch_blocks_migrations(self):
        with patch.object(ops, 'run', return_value=b'bad'), self.assertRaises(RuntimeError): ops.deploy(REQUEST)
        self.assertNotIn(('run', '--rm', '--no-deps', 'migrate'), self.calls)

    def test_changed_history_blocks_application_rollback(self):
        self.states['previous'] = dict(REQUEST, history='old')
        with self.assertRaises(RuntimeError): ops.rollback()
        self.assertEqual(self.calls, [])


class FileTests(unittest.TestCase):
    def test_installed_public_key_is_readable_by_sshd_and_not_user_writable(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary); home = base / 'home'; key = base / 'public'
            home.mkdir(); (home / '.ssh').mkdir()
            (home / '.ssh' / 'authorized_keys').write_text('ssh-ed25519 AAAA old-unrestricted-duplicate\n')
            key.write_text('ssh-ed25519 AAAA actions\n')
            with patch.object(ops, 'KEY_HOME', home), patch.object(ops, 'trusted'), patch.object(ops.os, 'chown'):
                ops.install_key(key)
            installed = home / '.ssh' / 'authorized_keys'
            self.assertEqual(home.stat().st_mode & 0o777, 0o755)
            self.assertEqual((home / '.ssh').stat().st_mode & 0o777, 0o755)
            self.assertEqual(installed.stat().st_mode & 0o777, 0o644)
            self.assertEqual(len(installed.read_text().splitlines()), 1)
            self.assertTrue(installed.read_text().startswith('restrict,command="/usr/bin/sudo -n /usr/local/lib/judeos-deploy/controller.py" '))
    def test_real_flock_serializes_two_workers(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary); (base / 'state').mkdir()
            attempted = threading.Event(); acquired = threading.Event()
            with patch.object(ops, 'BASE', base), patch.object(ops, 'trusted'):
                def worker():
                    attempted.set()
                    with ops.lock(): acquired.set()
                with ops.lock():
                    thread = threading.Thread(target=worker); thread.start()
                    self.assertTrue(attempted.wait(1))
                    self.assertFalse(acquired.wait(0.05))
                self.assertTrue(acquired.wait(1)); thread.join(1)

    def test_restore_rejects_live_name_and_checks_dump_before_database_creation(self):
        with self.assertRaises(ValueError): ops.restore_check('judeoslite')
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary); directory = base / 'backups' / '20261012T000000-aaaaaaaa'
            directory.mkdir(parents=True)
            (directory / 'metadata.json').write_text(json.dumps({'database_sha256': 'wrong'}))
            (directory / 'database.dump').write_bytes(b'archive')
            with patch.object(ops, 'BASE', base), patch.object(ops, 'trusted_tree'), patch.object(ops, 'compose') as compose:
                with self.assertRaises(RuntimeError): ops.restore_check(directory.name)
                compose.assert_not_called()

    def test_secret_init_does_not_read_or_replace_legacy_files(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary); legacy = base / 'db-password'; legacy.write_text('legacy')
            with patch.object(ops, 'CONFIG', base), patch.object(ops, 'trusted'):
                ops.init_secrets()
                before = (base / 'db-password-v1-19').read_bytes()
                ops.init_secrets()
                self.assertEqual((base / 'db-password-v1-19').read_bytes(), before)
                self.assertEqual(legacy.read_text(), 'legacy')
                self.assertEqual((base / 'pgpass-v1-19').stat().st_mode & 0o777, 0o444)

    def test_compose_has_only_proxy_ports_no_build_or_socket_or_inline_password(self):
        result = subprocess.run(['docker', 'compose', '--env-file', '/dev/null', '-f', str(ROOT / 'deploy/test/compose.yml'), 'config', '--format', 'json'],
                                env=dict(os.environ, APP_IMAGE=IMAGE), capture_output=True, check=True)
        config = json.loads(result.stdout)
        for name in ('app', 'migrate', 'db'):
            self.assertFalse(config['services'][name].get('ports'))
            self.assertNotIn('build', config['services'][name])
            self.assertNotIn('POSTGRES_PASSWORD', config['services'][name]['environment'])
        ports = config['services']['caddy']['ports']
        self.assertEqual({str(p['published']) for p in ports}, {'80', '443'})
        self.assertNotIn('docker.sock', result.stdout.decode())


if __name__ == '__main__': unittest.main()

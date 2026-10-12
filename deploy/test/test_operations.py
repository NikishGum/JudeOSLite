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
import urllib.error
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
SSH_CONFIG = ('pubkeyauthentication yes\nauthorizedkeyscommand none\n'
              'authorizedkeysfile /var/lib/judeos-deploy/.ssh/authorized_keys\n')


class GitHubAPITests(unittest.TestCase):
    def setUp(self):
        environment = patch.dict(os.environ, {'GH_TOKEN': 'test-only-token'})
        environment.start(); self.addCleanup(environment.stop)
        sleep = patch.object(selection.time, 'sleep')
        self.sleep = sleep.start(); self.addCleanup(sleep.stop)
        stderr = contextlib.redirect_stderr(io.StringIO())
        self.stderr = stderr.__enter__(); self.addCleanup(stderr.__exit__, None, None, None)

    def http_error(self, code):
        return urllib.error.HTTPError('https://api.github.com/test', code, 'test-only-token', {}, io.BytesIO())

    def test_transient_http_failure_recovers_without_logging_credentials(self):
        for code in (500, 502, 503, 504):
            with self.subTest(code=code), patch.object(selection.urllib.request, 'urlopen',
                    side_effect=[self.http_error(code), io.BytesIO(b'{"id": 12}')]) as fetch:
                self.assertEqual(selection.get('/actions/runs/12'), {'id': 12})
                self.assertEqual(fetch.call_count, 2)
                self.assertEqual(fetch.call_args.kwargs['timeout'], 30)
        self.assertNotIn('test-only-token', self.stderr.getvalue())

    def test_connection_and_timeout_failures_recover(self):
        with patch.object(selection.urllib.request, 'urlopen', side_effect=[
                urllib.error.URLError('test-only-token'), TimeoutError(), ConnectionResetError(),
                io.BytesIO(b'{"id": 12}')]) as fetch:
            self.assertEqual(selection.get('/actions/runs/12'), {'id': 12})
            self.assertEqual(fetch.call_count, 4)
            self.assertEqual([call.args[0] for call in self.sleep.call_args_list], [2, 4, 8])
        self.assertNotIn('test-only-token', self.stderr.getvalue())

    def test_persistent_api_failure_stops_after_bounded_retries(self):
        with patch.object(selection.urllib.request, 'urlopen',
                side_effect=[self.http_error(502) for _ in range(4)]) as fetch:
            with self.assertRaisesRegex(RuntimeError, '4 attempts.*HTTP 502'):
                selection.get('/actions/runs/12')
            self.assertEqual(fetch.call_count, 4)
            self.assertEqual([call.args[0] for call in self.sleep.call_args_list], [2, 4, 8])

    def test_permanent_http_errors_are_not_retried(self):
        for code in (401, 403, 404, 422, 429):
            with self.subTest(code=code), patch.object(selection.urllib.request, 'urlopen',
                    side_effect=self.http_error(code)) as fetch:
                with self.assertRaises(urllib.error.HTTPError): selection.get('/actions/runs/12')
                self.assertEqual(fetch.call_count, 1)
        self.sleep.assert_not_called()

    def test_invalid_response_is_not_retried(self):
        with patch.object(selection.urllib.request, 'urlopen', return_value=io.BytesIO(b'not JSON')) as fetch:
            with self.assertRaises(json.JSONDecodeError): selection.get('/actions/runs/12')
            self.assertEqual(fetch.call_count, 1)
        self.sleep.assert_not_called()

    def test_main_advance_during_api_retry_still_blocks_deployment_outputs(self):
        run = {'repository': {'id': 1}, 'head_repository': {'id': 1},
               'workflow_id': 2, 'event': 'push', 'head_branch': 'main',
               'status': 'completed', 'conclusion': 'success', 'head_sha': SHA, 'run_number': 12}
        responses = [self.http_error(502)] + [io.BytesIO(json.dumps(value).encode())
                    for value in (run, {'id': 1}, {'id': 2}, {'commit': {'sha': 'c' * 40}})]
        with tempfile.TemporaryDirectory() as temporary:
            event = Path(temporary) / 'event.json'; event.write_text('{"workflow_run":{"id":12}}')
            output = Path(temporary) / 'output'; output.write_text('')
            with patch.dict(os.environ, {'GITHUB_REPOSITORY': selection.REPO,
                    'GITHUB_EVENT_NAME': 'workflow_run', 'GITHUB_EVENT_PATH': str(event),
                    'GITHUB_OUTPUT': str(output)}), patch.object(selection.urllib.request, 'urlopen', side_effect=responses):
                with self.assertRaisesRegex(ValueError, 'current main'): selection.main()
            self.assertEqual(output.read_text(), '')


class PolicyTests(unittest.TestCase):
    def test_effective_sshd_config_detects_override_and_wrong_keys_source(self):
        for forced in ('none', ops.SSH_CONTROLLER_COMMAND):
            ops.validate_sshd_config((SSH_CONFIG + 'forcecommand ' + forced + '\n').encode(), ops.KEY_HOME)
        for raw in [SSH_CONFIG + 'forcecommand ' + str(ops.LEGACY_ENTRYPOINT) + '\n',
                    (SSH_CONFIG + 'forcecommand none\n').replace('/var/lib/judeos-deploy/.ssh/authorized_keys', '/etc/ssh/other-keys'),
                    (SSH_CONFIG + 'forcecommand none\n').replace('pubkeyauthentication yes', 'pubkeyauthentication no'),
                    (SSH_CONFIG + 'forcecommand none\n').replace('authorizedkeyscommand none', 'authorizedkeyscommand /external')]:
            with self.subTest(raw=raw), self.assertRaises(RuntimeError):
                ops.validate_sshd_config(raw.encode(), ops.KEY_HOME)
        ops.validate_sshd_config((SSH_CONFIG.replace('/var/lib/judeos-deploy/.ssh/authorized_keys', '%h/.ssh/authorized_keys .ssh/authorized_keys2') + 'forcecommand none\n').encode(), ops.KEY_HOME)

    def test_existing_pinned_wrapper_is_preserved_with_direct_actions_key(self):
        canonical = 'restrict,command="/usr/bin/sudo -n /usr/local/lib/judeos-deploy/controller.py" ssh-ed25519 AAAA\n'
        with tempfile.TemporaryDirectory() as temporary:
            wrapper = Path(temporary) / 'ssh-entry.py'; wrapper.write_text('owner-controlled fixture')
            with patch.object(ops, 'LEGACY_ENTRYPOINT', wrapper), patch.object(ops, 'LEGACY_ENTRYPOINT_SHA256', ops.checksum(wrapper)), patch.object(ops, 'trusted'):
                for command in (str(wrapper), '/usr/bin/python3 ' + str(wrapper), '/usr/bin/python3 -I ' + str(wrapper)):
                    with self.subTest(command=command):
                        ops.validate_authorized_keys(f'command="{command}",restrict ssh-ed25519 BBBB\n' + canonical)
                with self.assertRaisesRegex(RuntimeError, 'Actions key missing'):
                    ops.validate_authorized_keys(f'restrict,command="{wrapper}" ssh-ed25519 BBBB\n')
                wrapper.write_text('changed file')
                with self.assertRaisesRegex(RuntimeError, 'hash changed'):
                    ops.validate_authorized_keys(f'restrict,command="{wrapper}" ssh-ed25519 BBBB\n' + canonical)

    def test_wrapper_arguments_shell_and_ownership_are_not_bypasses(self):
        canonical = 'restrict,command="/usr/bin/sudo -n /usr/local/lib/judeos-deploy/controller.py" ssh-ed25519 AAAA\n'
        with tempfile.TemporaryDirectory() as temporary:
            wrapper = Path(temporary) / 'ssh-entry.py'; wrapper.write_text('fixture')
            with patch.object(ops, 'LEGACY_ENTRYPOINT', wrapper), patch.object(ops, 'LEGACY_ENTRYPOINT_SHA256', ops.checksum(wrapper)), patch.object(ops, 'trusted') as trusted:
                for command in (str(wrapper) + ' extra', str(wrapper) + '; id', '/bin/sh ' + str(wrapper), 'echo /usr/local/lib/judeos-deploy/controller.py'):
                    with self.subTest(command=command), self.assertRaises(RuntimeError):
                        ops.validate_authorized_keys(f'restrict,command="{command}" ssh-ed25519 BBBB\n' + canonical)
                trusted.side_effect = RuntimeError('Unsafe ownership')
                with self.assertRaisesRegex(RuntimeError, 'Unsafe ownership'):
                    ops.validate_authorized_keys(f'restrict,command="{wrapper}" ssh-ed25519 BBBB\n' + canonical)

    def test_key_option_order_quotes_and_comments(self):
        command = '/usr/bin/sudo -n /usr/local/lib/judeos-deploy/controller.py'
        for field in [f'restrict,command="{command}"', f'command="{command}",restrict',
                      f'from="127.0.0.1,::1",command="{command}",restrict']:
            with self.subTest(field=field):
                ops.validate_authorized_keys('# ignored\n  ' + field + ' ssh-ed25519 AAAA comment\n')
        key_type, options = ops.authorized_options('command="echo \\"comma,inside\\"",restrict ssh-ed25519 AAAA')
        self.assertEqual(key_type, 'ssh-ed25519')
        self.assertEqual(options['command'], 'echo "comma,inside"')

    def test_unreviewed_legacy_handler_identifies_line_without_echoing_command(self):
        contents = 'restrict,command="/usr/local/lib/judeos-deploy/legacy.py secret-value" ssh-ed25519 AAAA\n'
        with self.assertRaises(RuntimeError) as error: ops.validate_authorized_keys(contents)
        self.assertIn('line 1', str(error.exception))
        self.assertNotIn('secret-value', str(error.exception))

    def test_restrict_cannot_be_faked_in_command_or_overridden(self):
        command = '/usr/bin/sudo -n /usr/local/lib/judeos-deploy/controller.py'
        for field in [f'command="restrict,{command}"', f'no-restrict,command="{command}"',
                      f'restrict,port-forwarding,command="{command}"',
                      f'restrict,command="{command}",command="other"']:
            with self.subTest(field=field), self.assertRaises(RuntimeError):
                ops.validate_authorized_keys(field + ' ssh-ed25519 AAAA restrict')

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
    def test_manual_ssh_repair_is_scoped_and_rolls_back_before_reload_on_invalid_policy(self):
        original = ('Match User judeos-deploy\n    AuthenticationMethods publickey\n'
                    '    ForceCommand /usr/local/lib/judeos-deploy/ssh-entry.py\n'
                    'Match User root\n    ForceCommand /root/admin-only\nMatch all\n')
        for failure in (None, 'syntax', 'root-policy', 'reload'):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as temporary:
                path = Path(temporary) / '60-judeos-deploy.conf'; path.write_text(original)
                calls = []; root_calls = 0; reload_calls = 0
                def run(command, **kwargs):
                    nonlocal root_calls, reload_calls
                    calls.append(command)
                    if '-T' in command:
                        if 'user=root,' in command[-1]:
                            root_calls += 1
                            return b'root-original' if root_calls == 1 or failure != 'root-policy' else b'changed'
                        return (SSH_CONFIG + 'forcecommand ' + ops.SSH_CONTROLLER_COMMAND + '\n').encode()
                    if command == ['/usr/sbin/sshd', '-t'] and failure == 'syntax':
                        raise RuntimeError('Invalid config')
                    if command == ['/usr/bin/systemctl', 'reload', 'ssh']:
                        reload_calls += 1
                        if failure == 'reload' and reload_calls == 1: raise RuntimeError('Reload failed')
                    return b''
                with patch.object(ops, 'SSH_MATCH_FILE', path), patch.object(ops, 'trusted'), patch.object(ops, 'run', side_effect=run):
                    if failure:
                        with self.assertRaises(RuntimeError): ops.repair_ssh()
                        self.assertEqual(path.read_text(), original)
                        self.assertEqual(reload_calls, 2 if failure == 'reload' else 0)
                    else:
                        ops.repair_ssh()
                        self.assertIn('Match User root\n    ForceCommand /root/admin-only', path.read_text())
                        self.assertIn('AuthenticationMethods publickey', path.read_text())
                        self.assertIn('ForceCommand ' + ops.SSH_CONTROLLER_COMMAND, path.read_text())
                        self.assertEqual(reload_calls, 1)
                backups = list(path.parent.glob('*.before-json-*'))
                self.assertEqual(len(backups), 1)
                self.assertEqual(backups[0].read_text(), original)
                self.assertEqual(backups[0].stat().st_mode & 0o777, 0o600)

    def test_manual_ssh_repair_refuses_unexpected_match_scope_or_command(self):
        for source in ('Match User root\n ForceCommand /usr/local/lib/judeos-deploy/ssh-entry.py\n',
                       'Match User judeos-deploy\n ForceCommand /different\n'):
            with tempfile.TemporaryDirectory() as temporary:
                path = Path(temporary) / '60-judeos-deploy.conf'; path.write_text(source)
                with patch.object(ops, 'SSH_MATCH_FILE', path), patch.object(ops, 'trusted'), patch.object(ops, 'run') as run:
                    with self.assertRaises(RuntimeError): ops.repair_ssh()
                    run.assert_not_called()
                self.assertEqual(path.read_text(), source)
                self.assertEqual(list(path.parent.glob('*.before-json-*')), [])

    def test_already_correct_disk_rule_is_reloaded_without_rewriting(self):
        source = 'Match User judeos-deploy\n ForceCommand ' + ops.SSH_CONTROLLER_COMMAND + '\nMatch all\n'
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / '60-judeos-deploy.conf'; path.write_text(source)
            with patch.object(ops, 'SSH_MATCH_FILE', path), patch.object(ops, 'trusted'), patch.object(ops, 'run') as run:
                run.return_value = (SSH_CONFIG + 'forcecommand ' + ops.SSH_CONTROLLER_COMMAND + '\n').encode()
                ops.repair_ssh()
                self.assertIn(unittest.mock.call(['/usr/sbin/sshd', '-t']), run.call_args_list)
                self.assertIn(unittest.mock.call(['/usr/bin/systemctl', 'reload', 'ssh']), run.call_args_list)
            self.assertEqual(path.read_text(), source)
            self.assertEqual(list(path.parent.glob('*.before-json-*')), [])

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

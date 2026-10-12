#!/usr/bin/env python3
"""Disposable Cloud/CI check; never reads/writes the fixed VPS directories."""
import contextlib
import io
import http.cookiejar
import json
import os
from pathlib import Path
import re
import pty
import secrets
import select
import shutil
import ssl
import subprocess
import tarfile
import tempfile
import time
import urllib.request
import uuid
from unittest.mock import patch

import operations as ops


def main():
    root = Path(__file__).resolve().parents[2]
    docker = shutil.which('docker')
    image = 'judeoslite:ci'
    sha = subprocess.check_output([docker, 'image', 'inspect', image, '--format', '{{ index .Config.Labels "org.opencontainers.image.revision" }}'], text=True).strip()
    project = 'judeoslite-deploy-check-' + uuid.uuid4().hex[:8]
    env = dict(os.environ, APP_IMAGE=image)
    config = json.loads(subprocess.check_output([docker, 'compose', '--env-file', '/dev/null', '-f', str(root / 'deploy/test/compose.yml'), 'config', '--format', 'json'], env=env))
    with tempfile.TemporaryDirectory(prefix='judeos-deploy-check-') as temporary:
        base = Path(temporary)
        for directory in ('state', 'backups', 'uploads', 'config'): (base / directory).mkdir(mode=0o700)
        # Keep the nonroot application able to traverse its mounted directory.
        (base / 'uploads').chmod(0o777)
        (base / 'Caddyfile').write_text('https://localhost {\n tls internal\n reverse_proxy app:8080\n}\n')
        config['name'] = project
        config['services']['app']['volumes'][0]['source'] = str(base / 'uploads')
        config['services']['app']['environment']['APP_ORIGIN'] = 'https://localhost:18443'
        config['services']['caddy']['volumes'][0]['source'] = str(base / 'Caddyfile')
        for port in config['services']['caddy']['ports']:
            port['published'] = '18443' if port['target'] == 443 else '18081'
            port['host_ip'] = '127.0.0.1'
        for name, secret in config['secrets'].items():
            secret['file'] = str(base / 'config' / ('db-password-v1-19' if name == 'db_password' else 'pgpass-v1-19'))
        for name, volume in config['volumes'].items(): volume['name'] = project + '-' + name
        for name, network in config['networks'].items(): network['name'] = project + '-' + name
        config_path = base / 'compose.json'
        config_path.write_text(json.dumps(config))
        command = [docker, 'compose', '--project-name', project, '--env-file', '/dev/null', '-f', str(config_path)]

        def compose(unused_image=None, *args, input=None, output=None):
            if args and args[0] == 'pull':
                # Application under test was built locally, never published.
                args = ('pull', *(name for name in args[1:] if name in ('db', 'caddy')))
                if len(args) == 1: return b''
            stream = input if hasattr(input, 'read') else None
            result = subprocess.run(command + list(args), env=env, input=None if stream else input, stdin=stream,
                                    stdout=output if output else subprocess.PIPE, stderr=subprocess.PIPE)
            if result.returncode:
                # The isolated configuration contains only generated test secrets.
                raise RuntimeError('Disposable compose failed: ' + ' '.join(args) + '\n' + result.stderr.decode())
            return result.stdout

        def run(args, **kwargs):
            if args[0] == '/usr/bin/docker': args = [docker] + args[1:]
            result = subprocess.run(args, env=env, input=kwargs.get('input'), capture_output=True, check=True)
            return result.stdout

        def https(expected_sha):
            last_error = None
            for _ in range(45):
                try:
                    compose(None, 'cp', 'caddy:/data/caddy/pki/authorities/local/root.crt', str(base / 'root.crt'))
                    context = ssl.create_default_context(cafile=str(base / 'root.crt'))
                    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPSHandler(context=context))
                    for path, expected in [('/api/readyz', {'status': 'ok'}), ('/release.json', {'sha': expected_sha})]:
                        with opener.open('https://localhost:18443' + path, timeout=3) as response:
                            assert response.status == 200 and json.load(response) == expected
                    return
                except (RuntimeError, OSError, AssertionError) as error:
                    last_error = error
                    time.sleep(1)
            raise RuntimeError('Disposable Caddy TLS/readiness failed: ' + str(last_error))

        def bootstrap_and_login():
            password = secrets.token_urlsafe(24)
            master, slave = pty.openpty()
            process = subprocess.Popen(command + ['exec', 'app', '/app/create-owner', '--login', 'deploy-check', '--name', 'Disposable check'],
                                       env=env, stdin=slave, stdout=slave, stderr=slave)
            os.close(slave)
            buffer = b''; first = second = False
            deadline = time.monotonic() + 30
            try:
                while process.poll() is None and time.monotonic() < deadline:
                    if select.select([master], [], [], 0.2)[0]:
                        try: buffer += os.read(master, 4096)
                        except OSError: break
                    if b'Password (12' in buffer and not first:
                        os.write(master, password.encode() + b'\n'); first = True
                    if b'Repeat password:' in buffer and not second:
                        os.write(master, password.encode() + b'\n'); second = True
                if process.poll() is None: process.kill()
                assert process.wait() == 0 and first and second, 'Hidden owner bootstrap failed.'
            finally:
                os.close(master)
                if process.poll() is None: process.kill(); process.wait()
            cookies = http.cookiejar.CookieJar()
            opener = urllib.request.build_opener(urllib.request.ProxyHandler({}),
                urllib.request.HTTPSHandler(context=ssl.create_default_context(cafile=str(base / 'root.crt'))),
                urllib.request.HTTPCookieProcessor(cookies))
            request = urllib.request.Request('https://localhost:18443/api/auth/login',
                data=json.dumps({'login': 'deploy-check', 'password': password}).encode(),
                headers={'Content-Type': 'application/json', 'Origin': 'https://localhost:18443'})
            with opener.open(request, timeout=5) as response:
                assert response.status == 200
                headers = response.headers.get_all('Set-Cookie')
                assert len(headers) == 2 and all('Secure' in h and 'HttpOnly' in h and 'SameSite=Strict' in h for h in headers)
            with opener.open('https://localhost:18443/api/auth/me', timeout=5) as response:
                assert response.status == 200

        # Production ownership, sudo, DNS, UFW and public ACME cannot be proved
        # by a disposable container test; production validation remains intact.
        with contextlib.ExitStack() as stack:
            for name, value in [('BASE', base), ('CONFIG', base / 'config')]: stack.enter_context(patch.object(ops, name, value))
            for name in ('trusted', 'trusted_tree', 'preflight'): stack.enter_context(patch.object(ops, name))
            for name, value in [('compose', compose), ('run', run), ('verify_https', https)]: stack.enter_context(patch.object(ops, name, side_effect=value))
            try:
                ops.init_secrets()
                request = {'sha': sha, 'image': image, 'generation': 1}
                with ops.lock(): ops.deploy(request)
                bootstrap_and_login()
                compose(None, 'exec', '-T', 'db', 'psql', '-U', 'judeoslite', '-d', 'judeoslite', '-v', 'ON_ERROR_STOP=1', '-c', "CREATE TABLE pride.deploy_probe (id text PRIMARY KEY); INSERT INTO pride.deploy_probe VALUES ('retained');")
                (base / 'uploads' / 'probe.txt').write_text('retained-file')
                with ops.lock(): backup_name = ops.backup()
                with ops.lock(): ops.deploy(dict(request, generation=2))
                assert compose(None, 'exec', '-T', 'db', 'psql', '-U', 'judeoslite', '-d', 'judeoslite', '-At', '-c', 'SELECT id FROM pride.deploy_probe').strip() == b'retained'
                assert (base / 'uploads' / 'probe.txt').read_text() == 'retained-file'
                output = io.StringIO()
                with ops.lock(), contextlib.redirect_stdout(output): ops.restore_check(backup_name)
                database = re.search(r'restorecheck_[0-9a-f]{12}', output.getvalue()).group()
                assert compose(None, 'exec', '-T', 'db', 'psql', '-U', 'judeoslite', '-d', database, '-At', '-c', 'SELECT id FROM pride.deploy_probe').strip() == b'retained'
                with tarfile.open(base / 'backups' / backup_name / 'uploads.tar') as archive:
                    assert archive.extractfile('uploads/probe.txt').read() == b'retained-file'
                with ops.lock(): ops.rollback()
                assert ops.load('accepted')['generation'] == 2
                assert ops.load('current')['generation'] == 1
                try: ops.admission(request, ops.load('accepted'))
                except RuntimeError: pass
                else: raise AssertionError('Stale deploy was admitted after manual rollback.')
                print('PASS: isolated TLS/readiness/SHA, hidden owner bootstrap, HTTPS login/secure cookies, migrations, repeated deploy, DB/files preservation, backup/restore rows, rollback and watermark.')
            finally:
                # Destructive cleanup is limited to random disposable check volumes.
                assert project.startswith('judeoslite-deploy-check-')
                assert all(v['name'].startswith(project + '-') for v in config['volumes'].values())
                subprocess.run(command + ['down', '--volumes', '--remove-orphans'], env=env, stdout=subprocess.DEVNULL, check=True)


if __name__ == '__main__': main()

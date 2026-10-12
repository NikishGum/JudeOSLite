"""Trusted server implementation. All paths/configuration are root-controlled."""
from contextlib import contextmanager
from datetime import datetime, timezone
import fcntl
import getpass
import hashlib
import json
import os
from pathlib import Path
import pwd
import re
import secrets
import shlex
import socket
import stat
import subprocess
import time
import urllib.request

BASE = Path('/srv/judeos-test')
CONFIG = Path('/srv/judeos-test-config')
LIB = Path('/usr/local/lib/judeos-deploy')
KEY_HOME = Path('/var/lib/judeos-deploy')
LEGACY_ENTRYPOINT = LIB / 'ssh-entry.py'
# Preserve the exact owner-managed restricted entrypoint reported by the owner.
# This does not claim that it understands the new JSON deployment protocol.
LEGACY_ENTRYPOINT_SHA256 = '85a9dd8c570d315e563b0b83e71caf698916388efa0320193e17e58946f21af3'
SSH_MATCH_FILE = Path('/etc/ssh/sshd_config.d/60-judeos-deploy.conf')
SSH_CONTROLLER_COMMAND = '/usr/bin/sudo -n /usr/local/lib/judeos-deploy/controller.py'
DOMAIN = 'test.judopride.tech'
IMAGE = 'ghcr.io/nikishgum/judeoslite-test'
ENV = {'PATH': '/usr/sbin:/usr/bin:/sbin:/bin', 'HOME': '/root',
       'DOCKER_CONFIG': str(CONFIG / 'registry')}


def trusted(path):
    for node in (path, *path.parents):
        info = node.lstat()
        if stat.S_ISLNK(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022:
            raise RuntimeError('Unsafe ownership, permissions or symlink.')


def trusted_tree(path):
    trusted(path)
    for node in path.rglob('*'):
        trusted(node)


def validate_request(request):
    if not isinstance(request, dict) or set(request) != {'sha', 'image', 'generation'}:
        raise ValueError('Invalid request fields.')
    if not isinstance(request['sha'], str) or not re.fullmatch(r'[0-9a-f]{40}', request['sha']):
        raise ValueError('Full SHA required.')
    if not isinstance(request['image'], str) or not re.fullmatch(re.escape(IMAGE) + r'@sha256:[0-9a-f]{64}', request['image']):
        raise ValueError('Only the fixed image by digest is accepted.')
    if type(request['generation']) is not int or not 0 < request['generation'] < 2**53:
        raise ValueError('Invalid CI generation.')
    return request


def admission(request, accepted):
    if accepted and (request['generation'] < accepted['generation'] or
                     (request['generation'] == accepted['generation'] and request != accepted)):
        raise RuntimeError('Stale/conflicting deployment refused.')


def run(command, *, input=None, output=None):
    # Never relay Docker/psql output to user logs: config/SQL can contain secrets.
    stream = input if hasattr(input, 'read') else None
    result = subprocess.run(command, env=ENV, input=None if stream else input, stdin=stream,
                            stdout=output if output is not None else subprocess.PIPE,
                            stderr=subprocess.PIPE)
    if result.returncode:
        raise RuntimeError('Server command failed.')
    return result.stdout


def write(path, content, mode=0o600):
    trusted(path.parent)
    if path.exists() or path.is_symlink(): trusted(path)
    temp = path.with_name(path.name + '.tmp')
    fd = os.open(temp, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, mode)
    os.fchmod(fd, mode)
    try:
        with os.fdopen(fd, 'w') as stream:
            stream.write(content); stream.flush(); os.fsync(stream.fileno())
        os.replace(temp, path)
        directory = os.open(path.parent, os.O_DIRECTORY)
        try: os.fsync(directory)
        finally: os.close(directory)
    finally:
        temp.unlink(missing_ok=True)


def save(name, data):
    write(BASE / 'state' / (name + '.json'), json.dumps(data) + '\n')


def load(name):
    path = BASE / 'state' / (name + '.json')
    if not path.exists(): return None
    trusted(path)
    return json.loads(path.read_text())


@contextmanager
def lock():
    trusted(BASE); trusted(BASE / 'state')
    fd = os.open(BASE / 'state' / 'deploy.lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'w') as stream:
        trusted(BASE / 'state' / 'deploy.lock')
        fcntl.flock(stream, fcntl.LOCK_EX)
        yield


def compose(image=None, *args, input=None, output=None):
    if image is None:
        current = load('current') or load('accepted')
        if not current: raise RuntimeError('No release yet.')
        image = current['image']
    trusted(BASE / 'compose.yml'); trusted(BASE / 'Caddyfile')
    # APP_IMAGE is nonsecret. No .env files from old installations are sourced.
    command = ['/usr/bin/docker', 'compose', '--project-name', 'judeos-test',
               '--env-file', '/dev/null', '-f', str(BASE / 'compose.yml')]
    environment = dict(ENV, APP_IMAGE=image)
    stream = input if hasattr(input, 'read') else None
    result = subprocess.run(command + list(args), env=environment, input=None if stream else input, stdin=stream,
                            stdout=output if output is not None else subprocess.PIPE,
                            stderr=subprocess.PIPE)
    if result.returncode: raise RuntimeError('Compose operation failed.')
    return result.stdout


def init_secrets():
    trusted(CONFIG)
    password = CONFIG / 'db-password-v1-19'
    pgpass = CONFIG / 'pgpass-v1-19'
    if password.exists() and pgpass.exists():
        trusted(password); trusted(pgpass)
        print('Existing V1-19 secret files retained.'); return
    if password.exists() or pgpass.exists():
        raise RuntimeError('Incomplete secrets; owner must diagnose without exposing values.')
    value = secrets.token_urlsafe(48)
    write(password, value + '\n')
    # The containing directory is 0700. The nonroot app reads only its bind
    # mounted pgpass, not this directory or the password used by PostgreSQL init.
    write(pgpass, 'db:5432:judeoslite:judeoslite:' + value + '\n', 0o444)
    print('New DB password generated locally; old secret files untouched.')


def registry_login():
    trusted(CONFIG / 'registry')
    user = input('GitHub username owning read:packages token: ').strip()
    if not re.fullmatch(r'[A-Za-z0-9-]{1,39}', user): raise ValueError('Invalid username.')
    token = getpass.getpass('GHCR read:packages token (hidden): ')
    if not token or '\n' in token: raise ValueError('Invalid token.')
    run(['/usr/bin/docker', 'login', 'ghcr.io', '-u', user, '--password-stdin'], input=token.encode())
    os.chmod(CONFIG / 'registry' / 'config.json', 0o600)
    print('GHCR login stored in root-only registry directory.')


def activate_controller(expected):
    if not re.fullmatch(r'[0-9a-f]{64}', expected): raise ValueError('Expected hash required.')
    current = LIB / 'controller.py'
    trusted(current); trusted(LIB / 'controller.v1-19.py')
    if hashlib.sha256(current.read_bytes()).hexdigest() != expected:
        raise RuntimeError('Existing controller changed; re-audit it.')
    archive = LIB / ('controller.before-v1-19.' + expected + '.py')
    if not archive.exists():
        write(archive, current.read_text(), 0o700)
    write(current, (LIB / 'controller.v1-19.py').read_text(), 0o755)
    print('Restricted controller activated; old controller archived root-only. Sudoers unchanged.')


def install_key(public_file):
    raw = public_file.read_text().strip()
    if not re.fullmatch(r'ssh-ed25519 [A-Za-z0-9+/]+={0,2}(?: [^\r\n]+)?', raw):
        raise ValueError('Use one Ed25519 public key.')
    home = KEY_HOME
    # The deploy account must not be able to replace the restricted key file by
    # renaming its parent. Account needs no writable home for forced commands.
    for path in (home, home / '.ssh'):
        if path.is_symlink(): raise RuntimeError('Symlink home refused.')
        path.mkdir(exist_ok=True)
        # sshd reads authorized_keys under the target account's UID; public
        # keys must remain readable, but neither directory can be user-writable.
        os.chown(path, 0, 0); os.chmod(path, 0o755)
        trusted(path)
    keys = home / '.ssh' / 'authorized_keys'
    if keys.is_symlink(): raise RuntimeError('Symlink key file refused.')
    old = keys.read_text() if keys.exists() else ''
    key = ' '.join(raw.split()[:2])
    # Replace this public key's existing entry rather than retain an unrestricted
    # duplicate; leave other owner keys unchanged for separate review.
    lines = [line for line in old.splitlines() if raw.split()[1] not in line.split()]
    lines.append('restrict,command="/usr/bin/sudo -n /usr/local/lib/judeos-deploy/controller.py" ' + key + ' github-actions-v1-19')
    if keys.exists(): os.chown(keys, 0, 0); os.chmod(keys, 0o644)
    write(keys, '\n'.join(lines) + '\n', 0o644)
    print('Restricted Actions public key installed; other owner keys retained for review.')


def authorized_options(line):
    # Whitespace/commas inside a quoted forced command are not separators.
    match = re.match(r'((?:[^"\s]|"(?:\\.|[^"\\])*")+)\s+(\S+)\s+\S+', line.strip())
    if not match: raise ValueError('Malformed authorized_keys entry.')
    field, key_type = match.groups()
    parts = re.findall(r'(?:[^",]|"(?:\\.|[^"\\])*")+', field)
    if ','.join(parts) != field: raise ValueError('Malformed key options.')
    options = {}
    for part in parts:
        name, equal, value = part.partition('=')
        name = name.lower()
        if not re.fullmatch(r'[a-z0-9-]+', name) or name in options:
            raise ValueError('Malformed or duplicate key option.')
        if equal:
            value = json.loads(value) if value.startswith('"') else value
            if not isinstance(value, str): raise ValueError('Invalid key option value.')
        else: value = None
        options[name] = value
    return key_type, options


def validate_authorized_keys(contents):
    canonical = SSH_CONTROLLER_COMMAND
    actions_key = False
    for number, line in enumerate(contents.splitlines(), 1):
        if not line.strip() or line.lstrip().startswith('#'): continue
        try: key_type, options = authorized_options(line)
        except ValueError:
            raise RuntimeError(f'authorized_keys line {number}: malformed/duplicate options; review locally.') from None
        if 'restrict' not in options or options['restrict'] is not None:
            raise RuntimeError(f'authorized_keys line {number}: restrict flag missing.')
        if set(options) & {'pty', 'port-forwarding', 'agent-forwarding', 'x11-forwarding', 'user-rc'}:
            raise RuntimeError(f'authorized_keys line {number}: an option re-enables a restricted capability.')
        command = options.get('command')
        if command != canonical:
            try: argv = tuple(shlex.split(command or ''))
            except ValueError: argv = ()
            allowed = {(str(LEGACY_ENTRYPOINT),), ('/usr/bin/python3', str(LEGACY_ENTRYPOINT)),
                       ('/usr/bin/python3', '-I', str(LEGACY_ENTRYPOINT))}
            if argv not in allowed:
                raise RuntimeError(f'authorized_keys line {number}: forced command uses an unreviewed handler; audit or retire this legacy key explicitly.')
            trusted(LEGACY_ENTRYPOINT)
            if checksum(LEGACY_ENTRYPOINT) != LEGACY_ENTRYPOINT_SHA256:
                raise RuntimeError(f'authorized_keys line {number}: legacy handler hash changed; review locally.')
        if key_type == 'ssh-ed25519' and command == canonical: actions_key = True
    if not actions_key: raise RuntimeError('Restricted Actions key missing.')


def sshd_config(user):
    # No connection is opened. Match evaluation uses a documentation-only IP;
    # actual Address/Host-specific overrides still require an owner connection test.
    context = f'user={user},host=github-actions,addr=192.0.2.10,laddr=187.7.69.230,lport=22'
    return run(['/usr/sbin/sshd', '-T', '-C', context])


def validate_sshd_config(raw, home):
    fields = dict(line.split(None, 1) for line in raw.decode().splitlines() if ' ' in line)
    if fields.get('forcecommand') not in ('none', SSH_CONTROLLER_COMMAND):
        raise RuntimeError('sshd ForceCommand overrides the Actions key; root must repair the judeos-deploy Match rule.')
    if fields.get('pubkeyauthentication') != 'yes':
        raise RuntimeError('sshd public-key authentication is disabled.')
    if fields.get('authorizedkeyscommand') != 'none':
        raise RuntimeError('External AuthorizedKeysCommand requires separate owner review.')
    paths = []
    for value in fields.get('authorizedkeysfile', '').split():
        expanded = Path(value.replace('%h', str(home)).replace('%u', 'judeos-deploy'))
        paths.append(expanded if expanded.is_absolute() else home / expanded)
    if home / '.ssh' / 'authorized_keys' not in paths:
        raise RuntimeError('sshd reads a different authorized_keys file from the installed Actions key.')


def repair_ssh():
    # Explicit root-only admin action, never part of bootstrap or SSH payloads.
    path = SSH_MATCH_FILE
    trusted(path)
    original = path.read_text()
    lines = original.splitlines(keepends=True)
    scope = []; matches = []
    for index, line in enumerate(lines):
        fields = line.strip().split()
        if not fields or fields[0].startswith('#'): continue
        if fields[0].lower() == 'match':
            scope = [fields[1].lower(), *fields[2:]] if len(fields) > 1 else []
        elif fields[0].lower() == 'forcecommand' and scope == ['user', 'judeos-deploy']:
            matches.append((index, ' '.join(fields[1:])))
    if len(matches) != 1 or matches[0][1] not in (str(LEGACY_ENTRYPOINT), SSH_CONTROLLER_COMMAND):
        raise RuntimeError('Expected exactly one known ForceCommand in Match User judeos-deploy; review the file locally.')
    if matches[0][1] == SSH_CONTROLLER_COMMAND:
        run(['/usr/sbin/sshd', '-t'])
        validate_sshd_config(sshd_config('judeos-deploy'), KEY_HOME)
        # -T reads files on disk, not the running daemon's cached configuration.
        run(['/usr/bin/systemctl', 'reload', 'ssh'])
        print('SSH Match rule already correct; validated and reloaded.'); return
    root_before = sshd_config('root')
    backup = path.with_name(path.name + '.before-json-' + secrets.token_hex(4))
    write(backup, original, 0o600)
    index = matches[0][0]
    indent = lines[index][:len(lines[index]) - len(lines[index].lstrip())]
    lines[index] = indent + 'ForceCommand ' + SSH_CONTROLLER_COMMAND + '\n'
    mode = stat.S_IMODE(path.stat().st_mode)
    changed = reload_attempted = False
    try:
        write(path, ''.join(lines), mode); changed = True
        run(['/usr/sbin/sshd', '-t'])
        if sshd_config('root') != root_before:
            raise RuntimeError('Root SSH policy changed.')
        validate_sshd_config(sshd_config('judeos-deploy'), KEY_HOME)
        reload_attempted = True
        run(['/usr/bin/systemctl', 'reload', 'ssh'])
    except Exception:
        if changed:
            write(path, original, mode)
            if reload_attempted:
                try: run(['/usr/bin/systemctl', 'reload', 'ssh'])
                except RuntimeError: pass
        raise RuntimeError('SSH repair failed; original file retained/restored. Inspect the backup and ssh service locally.') from None
    print('Restricted SSH controller configured; root policy unchanged. Backup:', backup)


def preflight():
    trusted_tree(LIB); trusted(CONFIG); trusted(BASE); trusted(BASE / 'state')
    for directory in (BASE, CONFIG, BASE / 'state', BASE / 'backups', CONFIG / 'registry'):
        trusted(directory)
        if directory.stat().st_mode & 0o777 != 0o700:
            raise RuntimeError('Private server directories must have mode 0700.')
    for name in ('compose.yml', 'Caddyfile'): trusted(BASE / name)
    for name in ('db-password-v1-19', 'pgpass-v1-19'): trusted(CONFIG / name)
    for name, mode in [('db-password-v1-19', 0o600), ('pgpass-v1-19', 0o444)]:
        if (CONFIG / name).stat().st_mode & 0o777 != mode:
            raise RuntimeError('DB secret permissions differ from bootstrap.')
    registry = CONFIG / 'registry' / 'config.json'
    if registry.exists():
        trusted(registry)
        if registry.stat().st_mode & 0o777 != 0o600: raise RuntimeError('Registry credentials must have mode 0600.')
    if not (CONFIG / 'db-password-v1-19').read_text().strip(): raise RuntimeError('Empty DB secret.')
    expected = 'db:5432:judeoslite:judeoslite:' + (CONFIG / 'db-password-v1-19').read_text().strip()
    if (CONFIG / 'pgpass-v1-19').read_text().strip() != expected: raise RuntimeError('DB secret files differ.')
    if (LIB / 'controller.py').read_bytes() != (LIB / 'controller.v1-19.py').read_bytes():
        raise RuntimeError('Staged controller has not been activated.')
    user = pwd.getpwnam('judeos-deploy')
    if os.getgrouplist(user.pw_name, user.pw_gid) != [user.pw_gid]:
        raise RuntimeError('Deploy user has unexpected supplemental groups.')
    keys = Path(user.pw_dir) / '.ssh' / 'authorized_keys'
    trusted(keys)
    validate_authorized_keys(keys.read_text())
    validate_sshd_config(sshd_config(user.pw_name), Path(user.pw_dir))
    listing = run(['/usr/bin/sudo', '-l', '-U', 'judeos-deploy']).decode()
    grants = [line.strip() for line in listing.splitlines() if line.strip().startswith('(')]
    if grants != ['(root) NOPASSWD: /usr/local/lib/judeos-deploy/controller.py']:
        raise RuntimeError('Review sudo grants; only the controller may be allowed.')
    run(['/usr/bin/docker', 'info'])
    version = run(['/usr/bin/docker', 'compose', 'version', '--short']).decode().strip().lstrip('v')
    if int(version.split('.')[0]) < 2: raise RuntimeError('Compose v2+ required; no reinstall attempted.')
    placeholder = IMAGE + '@sha256:' + '0' * 64
    config = json.loads(compose(placeholder, 'config', '--format', 'json'))
    for name in ('db', 'app', 'migrate'):
        if config['services'][name].get('ports'): raise RuntimeError('Unexpected public port.')
    uploads = BASE / 'uploads'
    if uploads.is_symlink() or uploads.stat().st_uid != 65532 or uploads.stat().st_mode & 0o077:
        raise RuntimeError('Unsafe uploads directory.')
    if '187.7.69.230' not in {entry[4][0] for entry in socket.getaddrinfo(DOMAIN, 443)}:
        raise RuntimeError('DNS A record unexpected.')
    firewall = run(['/usr/sbin/ufw', 'status']).decode()
    if not firewall.startswith('Status: active'): raise RuntimeError('Firewall not active.')
    if Path('/var/run/reboot-required').exists(): raise RuntimeError('Reboot and rerun preflight first.')
    # A stopped unless-stopped proxy can still conflict when managed by Hostinger.
    containers = [json.loads(line) for line in run(['/usr/bin/docker', 'ps', '-a', '--format', 'json']).decode().splitlines()]
    if any('traefik' in c.get('Names', '').lower() for c in containers):
        raise RuntimeError('Resolve Hostinger Traefik management/remove it explicitly before Caddy.')
    ports = run(['/usr/bin/ss', '-H', '-ltn']).decode()
    caddy = compose(placeholder, 'ps', '--status', 'running', '--quiet', 'caddy').strip()
    if not caddy and any(re.search(r':(?:80|443)\s', line) for line in ports.splitlines()):
        raise RuntimeError('HTTP/HTTPS ports already occupied.')
    print('Local VPS preflight passed. GHCR pull/TLS/HTTPS/PWA still require real deployment.')


def history(image=None, database='judeoslite'):
    # Before the first migration public.schema_migrations is absent.
    present = compose(image, 'exec', '-T', 'db', 'psql', '-U', 'judeoslite', '-d', database, '-At', '-c', "SELECT to_regclass('public.schema_migrations') IS NOT NULL").strip()
    if present == b'f': return ''
    return compose(image, 'exec', '-T', 'db', 'psql', '-U', 'judeoslite', '-d', database, '-At', '-c', 'SELECT version, checksum FROM public.schema_migrations ORDER BY version').decode()


def checksum(path):
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''): digest.update(chunk)
    return digest.hexdigest()


def backup(image=None):
    trusted(BASE / 'backups')
    name = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S') + '-' + secrets.token_hex(4)
    directory = BASE / 'backups' / name
    directory.mkdir(mode=0o700)
    partial = directory / 'database.dump.partial'
    with partial.open('xb') as output:
        os.chmod(partial, 0o600)
        compose(image, 'exec', '-T', 'db', 'pg_dump', '-U', 'judeoslite', '-d', 'judeoslite', '-Fc', output=output)
        output.flush(); os.fsync(output.fileno())
    with partial.open('rb') as source:
        compose(image, 'exec', '-T', 'db', 'pg_restore', '--list', input=source)
    partial.rename(directory / 'database.dump')
    run(['/usr/bin/tar', '-C', str(BASE), '-cf', str(directory / 'uploads.tar'), 'uploads'])
    os.chmod(directory / 'uploads.tar', 0o600)
    write(directory / 'metadata.json', json.dumps({'current': load('current'), 'history': history(image),
         'database_sha256': checksum(directory / 'database.dump'),
         'uploads_sha256': checksum(directory / 'uploads.tar')}) + '\n')
    return name


def verify_https(sha):
    # Do not use the server's HTTP_PROXY; verify certificate and exact JSON SHA.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    for _ in range(60):
        try:
            with opener.open('https://' + DOMAIN + '/api/readyz', timeout=5) as r:
                if r.status != 200 or json.load(r) != {'status': 'ok'}: raise ValueError('Not ready.')
            with opener.open('https://' + DOMAIN + '/release.json', timeout=5) as r:
                if json.load(r) != {'sha': sha}: raise ValueError('Wrong release.')
            return
        except (OSError, ValueError): time.sleep(2)
    raise RuntimeError('HTTPS readiness/release check failed; no automatic DB restore.')


def deploy(request):
    image = request['image']
    admission(request, load('accepted'))
    preflight()
    # Reserve before pull/migration, even on failure. An older run cannot replace
    # an interrupted/newer release. Retry only the same generation+SHA+digest.
    save('accepted', request)
    save('operation', {'phase': 'pull', **request})
    compose(image, 'config', '--quiet')
    compose(image, 'pull', 'db', 'caddy', 'app', 'migrate')
    # Immutable digest is necessary but also verify it was built for this SHA.
    label = run(['/usr/bin/docker', 'image', 'inspect', image, '--format', '{{ index .Config.Labels "org.opencontainers.image.revision" }}']).decode().strip()
    if label != request['sha']: raise RuntimeError('Image revision does not match request.')
    compose(image, 'run', '--rm', '--no-deps', 'caddy', 'caddy', 'validate', '--config', '/etc/caddy/Caddyfile')
    compose(image, 'up', '-d', '--wait', '--wait-timeout', '180', 'db')
    previous = load('current')
    # Maintenance prevents writes/uploads changing while taking the pair of
    # backups and keeps older binaries away from a possibly incompatible schema.
    compose(image, 'stop', 'app')
    save('operation', {'phase': 'backup', **request})
    before = backup(image)
    save('operation', {'phase': 'migrate', 'backup': before, **request})
    compose(image, 'run', '--rm', '--no-deps', 'migrate')
    save('operation', {'phase': 'start', 'backup': before, **request})
    compose(image, 'up', '-d', '--no-deps', '--wait', '--wait-timeout', '180', 'app')
    compose(image, 'up', '-d', '--no-deps', 'caddy')
    verify_https(request['sha'])
    if previous: save('previous', previous)
    save('current', dict(request, history=history(image), backup=before))
    save('operation', {'phase': 'ready', **request})
    print('Deployment ready; SHA, schema history and backup recorded.')


def restore_check(name):
    if not re.fullmatch(r'[0-9]{8}T[0-9]{6}-[0-9a-f]{8}', name): raise ValueError('Invalid backup name.')
    directory = BASE / 'backups' / name
    trusted_tree(directory)
    metadata = json.loads((directory / 'metadata.json').read_text())
    if checksum(directory / 'database.dump') != metadata['database_sha256']: raise RuntimeError('Backup checksum mismatch.')
    if checksum(directory / 'uploads.tar') != metadata['uploads_sha256']: raise RuntimeError('Uploads checksum mismatch.')
    db = 'restorecheck_' + secrets.token_hex(6)
    compose(None, 'exec', '-T', 'db', 'createdb', '-U', 'judeoslite', db)
    # On failure keep the isolated database for root investigation. Never touch
    # live judeoslite or automatically down/delete a data volume.
    with (directory / 'database.dump').open('rb') as source:
        compose(None, 'exec', '-T', 'db', 'pg_restore', '-U', 'judeoslite', '-d', db, '--exit-on-error', '--single-transaction', input=source)
    if history(database=db) != metadata['history']: raise RuntimeError('Restored migration history differs.')
    run(['/usr/bin/tar', '-tf', str(directory / 'uploads.tar')])
    print('Archive restored into isolated database:', db)
    print('Migration history verified. Manually verify business rows and uploaded files before dropping it.')


def rollback():
    previous = load('previous')
    current = load('current')
    if not previous or not current: raise RuntimeError('No verified previous release.')
    if history() != previous['history']: raise RuntimeError('Migration history changed; rollback refused.')
    compose(previous['image'], 'pull', 'app')
    compose(previous['image'], 'up', '-d', '--no-deps', '--wait', '--wait-timeout', '180', 'app')
    verify_https(previous['sha'])
    save('current', previous); save('previous', current)
    # Keep accepted watermark so delayed CI cannot undo an intentional rollback.
    save('operation', {'phase': 'manual-rollback', 'sha': previous['sha']})
    print('Application rollback ready; DB and admission watermark retained.')


def status():
    for name in ('accepted', 'current', 'previous', 'operation'):
        print(name + ':', json.dumps(load(name)))


def create_owner(login, name):
    current = load('current')
    if not current: raise RuntimeError('Complete deployment first.')
    trusted(BASE / 'compose.yml')
    # Root's terminal is passed through for create-owner's hidden prompt.
    # Password never becomes an argument, env var, file or SSH payload.
    command = ['/usr/bin/docker', 'compose', '--project-name', 'judeos-test',
               '--env-file', '/dev/null', '-f', str(BASE / 'compose.yml'),
               'exec', 'app', '/app/create-owner', '--login', login, '--name', name]
    subprocess.run(command, env=dict(ENV, APP_IMAGE=current['image']), check=True)

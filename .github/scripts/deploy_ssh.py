"""Restricted SSH request, no file transfer or shell script sent to the VPS."""
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile


def main():
    expected = {'DEPLOY_HOST': '187.7.69.230', 'DEPLOY_PORT': '22',
                'DEPLOY_USER': 'judeos-deploy', 'TEST_DOMAIN': 'test.judopride.tech',
                'DEPLOY_PATH': '/srv/judeos-test'}
    if any(os.environ.get(name) != value for name, value in expected.items()):
        raise ValueError('Environment Variables must match the reviewed server configuration.')
    sha, image, generation = os.environ['RELEASE_SHA'], os.environ['RELEASE_IMAGE'], os.environ['RELEASE_GENERATION']
    if not re.fullmatch(r'[0-9a-f]{40}', sha) or not re.fullmatch(r'ghcr\.io/nikishgum/judeoslite-test@sha256:[0-9a-f]{64}', image) or not re.fullmatch(r'[1-9][0-9]*', generation):
        raise ValueError('Invalid release metadata.')
    with tempfile.TemporaryDirectory() as temporary:
        key, hosts = Path(temporary) / 'key', Path(temporary) / 'known_hosts'
        key.write_text(os.environ['DEPLOY_SSH_KEY'].rstrip() + '\n')
        hosts.write_text(os.environ['DEPLOY_KNOWN_HOSTS'].rstrip() + '\n')
        key.chmod(0o600); hosts.chmod(0o600)
        subprocess.run(['ssh-keygen', '-F', expected['DEPLOY_HOST'], '-f', str(hosts)],
                       check=True, stdout=subprocess.DEVNULL)
        command = ['ssh', '-T', '-p', expected['DEPLOY_PORT'], '-i', str(key),
                   '-o', 'BatchMode=yes', '-o', 'IdentitiesOnly=yes',
                   '-o', 'StrictHostKeyChecking=yes', '-o', 'GlobalKnownHostsFile=/dev/null',
                   '-o', 'UserKnownHostsFile=' + str(hosts), '-o', 'ConnectTimeout=15',
                   '-o', 'ServerAliveInterval=30', '-o', 'ServerAliveCountMax=6',
                   expected['DEPLOY_USER'] + '@' + expected['DEPLOY_HOST']]
        subprocess.run(command, input=json.dumps({'sha': sha, 'image': image, 'generation': int(generation)}).encode(), check=True)


if __name__ == '__main__': main()

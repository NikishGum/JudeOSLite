#!/usr/bin/python3 -I
"""Only entry point allowed to judeos-deploy. No shell, paths or uploads."""
import json
import os
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from operations import ENV, LIB, trusted_tree, validate_request


def main():
    if os.geteuid() != 0 or len(sys.argv) != 1:
        raise RuntimeError('Use the forced SSH command without arguments.')
    trusted_tree(LIB)
    # Limit input before JSON parsing. No diagnostic/restore operation over SSH.
    raw = sys.stdin.buffer.read(2049)
    if len(raw) > 2048:
        raise ValueError('Request too large.')
    request = validate_request(json.loads(raw))
    unit = f"judeos-test-deploy-{request['generation']}"
    # systemd owns the worker: losing SSH or cancelling Actions does not kill
    # the migration. Use explicit infinity: systemd-run passes transient
    # TimeoutStartSec=0 as a zero deadline, unlike the unit-file parser.
    command = [
        '/usr/bin/systemd-run', '--quiet', '--wait', '--collect',
        '--unit=' + unit, '--property=Type=oneshot',
        '--property=TimeoutStartSec=infinity',
        '/usr/bin/python3', '-I', str(LIB / 'admin.py'), 'deploy',
        request['sha'], request['image'], str(request['generation']),
    ]
    result = subprocess.run(command, env=ENV, capture_output=True)
    if result.returncode:
        raise RuntimeError('Deployment failed or is already running; root can inspect the unit and status.')
    print('Deployment completed; HTTPS readiness and release SHA verified.')


if __name__ == '__main__':
    try:
        main()
    except Exception:
        # Neither raw payload nor subprocess output can leak into the SSH log.
        print('Controller refused/failed request. Use root diagnostics.', file=sys.stderr)
        sys.exit(1)

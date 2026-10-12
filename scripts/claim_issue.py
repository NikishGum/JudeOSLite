"""Request a serialized claim, then confirm its result before starting work."""

import argparse
import json
import subprocess
import time
import uuid

REPOSITORY = 'NikishGum/JudeOSLite'


def gh_json(path, data=None):
    command = ['gh', 'api', path]
    if data is not None:
        command.extend(['--method', 'POST', '--input', '-'])
    result = subprocess.run(command, input=None if data is None else json.dumps(data), text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError(result.stderr.strip() or 'GitHub request failed.')
    return json.loads(result.stdout) if result.stdout.strip() else None


def wait_for_claim(repository, number, actor, request_id, timeout):
    title = f'Claim #{number} · {actor} · {request_id}'
    endpoint = f'repos/{repository}/actions/workflows/issue-claim.yml/runs?event=workflow_dispatch&per_page=100'
    deadline = time.monotonic() + timeout
    run = None
    while time.monotonic() < deadline:
        if run is None:
            runs = gh_json(endpoint)['workflow_runs']
            run = next((item for item in runs if item['display_title'] == title and item['actor']['login'] == actor), None)
        else:
            run = gh_json(f"repos/{repository}/actions/runs/{run['id']}")
        if run is not None and run['status'] == 'completed':
            if run['conclusion'] != 'success':
                raise RuntimeError(f"Claim did not succeed ({run['conclusion']}). Do not start. See {run['html_url']}")
            return run
        time.sleep(3)
    suffix = f" See {run['html_url']}" if run is not None else f' Request ID: {request_id}'
    raise RuntimeError('Claim result is unknown; do not start or repeat blindly.' + suffix)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('issue_number', type=int)
    parser.add_argument('--dry-run', action='store_true', help='Check via the same queue without taking the issue.')
    parser.add_argument('--timeout', type=int, default=600)
    args = parser.parse_args()
    if args.issue_number < 1 or args.timeout < 1:
        parser.error('Issue number and timeout must be positive.')
    actor = gh_json('user')['login']
    issue_path = f'repos/{REPOSITORY}/issues/{args.issue_number}'
    issue = gh_json(issue_path)
    if issue['state'] != 'open' or issue['assignees']:
        owners = ', '.join(user['login'] for user in issue['assignees']) or 'none'
        raise RuntimeError(f"Issue is {issue['state']}, assignees: {owners}. Do not claim or start another chat on it.")
    request_id = uuid.uuid4().hex
    print(f'Requesting #{args.issue_number} for {actor}; request {request_id}. Waiting for the queue.', flush=True)
    gh_json(f'repos/{REPOSITORY}/actions/workflows/issue-claim.yml/dispatches', {
        'ref': 'main',
        'inputs': {'issue_number': str(args.issue_number), 'request_id': request_id, 'dry_run': str(args.dry_run).lower()},
    })
    run = wait_for_claim(REPOSITORY, args.issue_number, actor, request_id, args.timeout)
    if args.dry_run:
        print(json.dumps({'acquired': False, 'eligible': True, 'dry_run': True, 'run_url': run['html_url']}))
        return
    confirmed = gh_json(issue_path)
    statuses = {label['name'] for label in confirmed['labels']} & {'Todo', 'In progress', 'Done'}
    if confirmed['state'] != 'open' or [user['login'] for user in confirmed['assignees']] != [actor] or statuses != {'In progress'}:
        raise RuntimeError('Live assignment changed or is invalid. Do not start or unassign anyone automatically.')
    print(json.dumps({'acquired': True, 'issue': args.issue_number, 'actor': actor, 'request_id': request_id, 'run_url': run['html_url']}))


if __name__ == '__main__':
    try:
        main()
    except RuntimeError as error:
        raise SystemExit(str(error))

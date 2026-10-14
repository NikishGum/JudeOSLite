"""Run only inside the per-issue GitHub Actions concurrency group."""

import json
import os
from pathlib import Path
import re
import urllib.request


class ClaimRejected(RuntimeError):
    pass


def load_plan_registry(root):
    """Keep completed V1 and the redesign separate while sharing one queue."""
    plans = [json.loads((root / 'docs' / name).read_text()) for name in (
        'github-issues-v1.json', 'github-issues-redesign.json',
    )]
    repository = plans[0]['repository']
    issues, ids, numbers = [], set(), set()
    for plan in plans:
        if plan['repository'] != repository:
            raise ClaimRejected('Plan registries refer to different repositories.')
        for item in plan['issues']:
            if item['id'] in ids or item['number'] in numbers:
                raise ClaimRejected('Duplicate issue ID or number in plan registries.')
            ids.add(item['id'])
            numbers.add(item['number'])
            issues.append(item)
    if any(dep not in ids for item in issues for dep in item['dependencies']):
        raise ClaimRejected('Unknown prerequisite in plan registries.')
    return {'repository': repository, 'issues': issues}


class GitHub:
    def __init__(self, repository, token, api_url):
        self.base = api_url.rstrip('/') + '/repos/' + repository
        self.api_url = api_url.rstrip('/')
        self.headers = {
            'Authorization': 'Bearer ' + token,
            'Accept': 'application/vnd.github+json',
            'X-GitHub-Api-Version': '2022-11-28',
            'Content-Type': 'application/json',
        }

    def request(self, method, path, data=None):
        url = self.api_url + path if path.startswith('/users/') else self.base + path
        body = None if data is None else json.dumps(data).encode()
        request = urllib.request.Request(url, data=body, headers=self.headers, method=method)
        with urllib.request.urlopen(request, timeout=20) as response:
            return json.load(response)

    def dependencies(self, number):
        result = []
        page = 1
        while True:
            batch = self.request('GET', f'/issues/{number}/dependencies/blocked_by?per_page=100&page={page}')
            result.extend(batch)
            if len(batch) < 100:
                return result
            page += 1


def claim(api, number, actor, registry, dry_run=False):
    record = next((item for item in registry['issues'] if item['number'] == number), None)
    if record is None:
        raise ClaimRejected('Issue is outside the registered plans.')
    issue = api.request('GET', f'/issues/{number}')
    if 'pull_request' in issue or not issue['title'].startswith(f"[{record['id']}] "):
        raise ClaimRejected('Issue does not match the plan registry.')
    if issue['state'] != 'open':
        raise ClaimRejected('Issue is closed.')
    # Never treat assignment to the same account as a new claim: another chat
    # belonging to that person may already be working on it.
    if issue['assignees']:
        owners = ', '.join(user['login'] for user in issue['assignees'])
        raise ClaimRejected('Issue already assigned to ' + owners + '.')
    if api.request('GET', '/users/' + actor).get('type') != 'User':
        raise ClaimRejected('The workflow must be dispatched by a human account.')

    by_id = {item['id']: item for item in registry['issues']}
    expected = {by_id[dep]['number'] for dep in record['dependencies']}
    dependencies = api.dependencies(number)
    if {item['number'] for item in dependencies} != expected:
        raise ClaimRejected('Live dependencies differ from the plan registry; reconcile them first.')
    blocked = [str(item['number']) for item in dependencies if item['state'] != 'closed']
    if blocked:
        raise ClaimRejected('Open prerequisites: #' + ', #'.join(blocked) + '.')
    if dry_run:
        return {'acquired': False, 'eligible': True, 'dry_run': True, 'issue': number, 'actor': actor}

    # Re-read after dependency checks, immediately before the only mutation.
    # Manual changes made outside the queue are never intentionally overwritten.
    current = api.request('GET', f'/issues/{number}')
    if current['state'] != 'open' or current['assignees']:
        raise ClaimRejected('Issue changed while checking eligibility; claim refused.')
    statuses = {'Todo', 'In progress', 'Done'}
    labels = [label['name'] for label in current['labels'] if label['name'] not in statuses]
    labels.append('In progress')
    api.request('PATCH', f'/issues/{number}', {'assignees': [actor], 'labels': labels})
    confirmed = api.request('GET', f'/issues/{number}')
    if (
        confirmed['state'] != 'open'
        or [user['login'] for user in confirmed['assignees']] != [actor]
        or {label['name'] for label in confirmed['labels']} & statuses != {'In progress'}
    ):
        raise ClaimRejected('Assignment was not confirmed. Do not start or remove anyone automatically.')
    return {'acquired': True, 'issue': number, 'actor': actor}


def main():
    raw_number = os.environ['ISSUE_NUMBER']
    request_id = os.environ['CLAIM_REQUEST_ID']
    if not re.fullmatch(r'[1-9][0-9]*', raw_number):
        raise ClaimRejected('Invalid issue number.')
    if not re.fullmatch(r'[0-9a-f]{32}', request_id):
        raise ClaimRejected('Invalid request ID.')
    if os.environ['GITHUB_REF'] != 'refs/heads/main' or os.environ['GITHUB_RUN_ATTEMPT'] != '1':
        raise ClaimRejected('Use a new dispatch on main, not another branch or a rerun.')
    registry = load_plan_registry(Path(__file__).resolve().parents[2])
    if os.environ['GITHUB_REPOSITORY'] != registry['repository']:
        raise ClaimRejected('Unexpected repository.')
    api = GitHub(registry['repository'], os.environ['GH_TOKEN'], os.environ['GITHUB_API_URL'])
    result = claim(api, int(raw_number), os.environ['CLAIM_ACTOR'], registry, os.environ.get('CLAIM_DRY_RUN') == 'true')
    result['request_id'] = request_id
    print(json.dumps(result))
    summary = os.environ.get('GITHUB_STEP_SUMMARY')
    if summary:
        with open(summary, 'a') as output:
            output.write('```json\n' + json.dumps(result, indent=2) + '\n```\n')


if __name__ == '__main__':
    try:
        main()
    except ClaimRejected as error:
        raise SystemExit('Claim refused: ' + str(error))

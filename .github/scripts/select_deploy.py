"""Select only a successful push-CI of the current main in this repository."""
import json
import os
import re
import sys
import time
import urllib.error
import urllib.request

REPO = 'NikishGum/JudeOSLite'
RETRY_DELAYS = (2, 4, 8)
RETRY_HTTP_CODES = {500, 502, 503, 504}


def validate(run, repository_id, workflow_id, main_sha):
    if (run['repository']['id'] != repository_id or
        run['head_repository']['id'] != repository_id or
        run['workflow_id'] != workflow_id or run['event'] != 'push' or
        run['head_branch'] != 'main' or run['status'] != 'completed' or
        run['conclusion'] != 'success' or run['head_sha'] != main_sha):
        raise ValueError('CI must be a successful push of current main in the original repository.')
    if not re.fullmatch(r'[0-9a-f]{40}', main_sha) or type(run['run_number']) is not int or run['run_number'] < 1:
        raise ValueError('Invalid SHA or CI run number.')
    return {'sha': main_sha, 'generation': str(run['run_number'])}


def get(path):
    request = urllib.request.Request('https://api.github.com/repos/' + REPO + path,
        headers={'Authorization': 'Bearer ' + os.environ['GH_TOKEN'],
                 'Accept': 'application/vnd.github+json', 'X-GitHub-Api-Version': '2022-11-28'})
    for attempt in range(len(RETRY_DELAYS) + 1):
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                return json.load(response)
        except urllib.error.HTTPError as error:
            if error.code not in RETRY_HTTP_CODES:
                raise
            failure = 'HTTP ' + str(error.code)
            error.close()
        except (urllib.error.URLError, TimeoutError, ConnectionError):
            failure = 'connection error'
        if attempt == len(RETRY_DELAYS):
            raise RuntimeError(f'GitHub API failed after {attempt + 1} attempts ({failure}); deployment selection stopped.') from None
        delay = RETRY_DELAYS[attempt]
        print(f'GitHub API {failure}; retry {attempt + 1}/{len(RETRY_DELAYS)} in {delay}s.', file=sys.stderr)
        time.sleep(delay)


def main():
    if os.environ['GITHUB_REPOSITORY'] != REPO: raise ValueError('Original repository required.')
    with open(os.environ['GITHUB_EVENT_PATH']) as stream: event = json.load(stream)
    if os.environ['GITHUB_EVENT_NAME'] == 'workflow_run':
        run_id = str(event['workflow_run']['id'])
    elif os.environ['GITHUB_EVENT_NAME'] == 'workflow_dispatch' and os.environ['GITHUB_REF'] == 'refs/heads/main':
        run_id = event['inputs']['ci_run_id']
    else: raise ValueError('Invalid trigger/ref.')
    if not re.fullmatch(r'[1-9][0-9]*', run_id): raise ValueError('Numeric CI run ID required.')
    run = get('/actions/runs/' + run_id)
    selected = validate(run, get('')['id'], get('/actions/workflows/build.yml')['id'], get('/branches/main')['commit']['sha'])
    with open(os.environ['GITHUB_OUTPUT'], 'a') as output:
        for key, value in selected.items(): output.write(key + '=' + value + '\n')


if __name__ == '__main__': main()

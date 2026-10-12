import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('claim_client', Path(__file__).resolve().parents[2] / 'scripts/claim_issue.py')
client = importlib.util.module_from_spec(spec)
spec.loader.exec_module(client)


def run(request_id, **changes):
    result = {'id': 123, 'display_title': f'Claim #150 · developer · {request_id}', 'actor': {'login': 'developer'},
              'status': 'completed', 'conclusion': 'success', 'html_url': 'https://github.com/example/actions/runs/123'}
    result.update(changes)
    return result


class ClaimClientTests(unittest.TestCase):
    def test_another_chat_of_same_user_does_not_confirm_this_request(self):
        wrong = run('other-request')
        expected = run('this-request')
        with patch.object(client, 'gh_json', return_value={'workflow_runs': [wrong, expected]}):
            result = client.wait_for_claim('example/repo', 150, 'developer', 'this-request', 10)
        self.assertEqual(result['display_title'], expected['display_title'])

    def test_failure_or_cancellation_never_grants_the_claim(self):
        for conclusion in ['failure', 'cancelled', 'timed_out']:
            with self.subTest(conclusion=conclusion):
                with patch.object(client, 'gh_json', return_value={'workflow_runs': [run('request', conclusion=conclusion)]}):
                    with self.assertRaisesRegex(RuntimeError, 'Do not start'):
                        client.wait_for_claim('example/repo', 150, 'developer', 'request', 10)

    def test_unknown_result_requires_inspecting_existing_request(self):
        with patch.object(client.time, 'monotonic', side_effect=[0, 11]):
            with self.assertRaisesRegex(RuntimeError, 'unknown.*do not start or repeat blindly'):
                client.wait_for_claim('example/repo', 150, 'developer', 'request', 10)

    def test_dry_run_reports_no_acquisition(self):
        with patch.object(client, 'gh_json', side_effect=[{'login': 'developer'}, {'state': 'open', 'assignees': []}, None]):
            with patch.object(client, 'wait_for_claim', return_value=run('request')):
                with patch('sys.argv', ['claim_issue.py', '150', '--dry-run']), patch('builtins.print') as output:
                    client.main()
        self.assertIn('"acquired": false', output.call_args.args[0])


if __name__ == '__main__':
    unittest.main()

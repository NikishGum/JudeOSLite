import copy
import json
from pathlib import Path
import tempfile
import unittest

from claim_issue import claim, load_plan_registry, ClaimRejected


REGISTRY = {'issues': [
    {'id': 'V1-01', 'number': 149, 'dependencies': []},
    {'id': 'V1-02', 'number': 150, 'dependencies': ['V1-01']},
]}


class FakeGitHub:
    def __init__(self):
        self.issue = {'title': '[V1-02] Login', 'state': 'open', 'assignees': [], 'labels': [{'name': 'Todo'}, {'name': 'backend'}]}
        self.blocked_by = [{'number': 149, 'state': 'closed'}]
        self.writes = []
        self.user_type = 'User'
        self.on_dependencies = None
        self.ignore_assignment = False

    def request(self, method, path, data=None):
        if path.startswith('/users/'):
            return {'type': self.user_type}
        if method == 'PATCH':
            self.writes.append(copy.deepcopy(data))
            if not self.ignore_assignment:
                self.issue['assignees'] = [{'login': actor} for actor in data['assignees']]
            self.issue['labels'] = [{'name': name} for name in data['labels']]
        return copy.deepcopy(self.issue)

    def dependencies(self, number):
        if self.on_dependencies:
            self.on_dependencies()
        return copy.deepcopy(self.blocked_by)


class ClaimTests(unittest.TestCase):
    def registry(self, redesign=None):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        (root / 'docs').mkdir()
        v1 = {'repository': 'owner/repo', **REGISTRY}
        rd = redesign or {'repository': 'owner/repo', 'issues': [
            {'id': 'RD-01', 'number': 190, 'dependencies': ['V1-02']},
        ]}
        for name, content in [('github-issues-v1.json', v1), ('github-issues-redesign.json', rd)]:
            (root / 'docs' / name).write_text(json.dumps(content))
        return root

    def test_redesign_cross_plan_dependency_uses_same_claim_guard(self):
        root = self.registry()
        registry = load_plan_registry(root)
        api = FakeGitHub()
        api.issue['title'] = '[RD-01] Shell'
        api.blocked_by = [{'number': 150, 'state': 'open'}]
        with self.assertRaisesRegex(ClaimRejected, 'Open prerequisites'):
            claim(api, 190, 'developer', registry)
        self.assertEqual(api.writes, [])
        api.blocked_by[0]['state'] = 'closed'
        self.assertTrue(claim(api, 190, 'developer', registry, dry_run=True)['eligible'])
        self.assertEqual(api.writes, [])
        self.assertTrue(claim(api, 190, 'developer', registry)['acquired'])
        with self.assertRaisesRegex(ClaimRejected, 'already assigned'):
            claim(api, 190, 'colleague', registry)
        self.assertEqual(len(api.writes), 1)

    def test_invalid_plan_merge_fails_closed(self):
        for rd in [
            {'repository': 'different/repo', 'issues': []},
            {'repository': 'owner/repo', 'issues': [{'id': 'V1-01', 'number': 190, 'dependencies': []}]},
            {'repository': 'owner/repo', 'issues': [{'id': 'RD-01', 'number': 149, 'dependencies': []}]},
            {'repository': 'owner/repo', 'issues': [{'id': 'RD-01', 'number': 190, 'dependencies': ['MISSING']}]},
        ]:
            with self.subTest(rd=rd), self.assertRaises(ClaimRejected):
                load_plan_registry(self.registry(rd))

    def test_repository_registries_load_together(self):
        registry = load_plan_registry(Path(__file__).resolve().parents[2])
        self.assertEqual(registry['repository'], 'NikishGum/JudeOSLite')
        self.assertTrue(any(item['id'] == 'RD-01' for item in registry['issues']))

    def test_two_people_and_two_chats_of_one_person_have_one_winner(self):
        for second_actor in ['colleague', 'developer']:
            with self.subTest(second_actor=second_actor):
                api = FakeGitHub()
                self.assertTrue(claim(api, 150, 'developer', REGISTRY)['acquired'])
                with self.assertRaisesRegex(ClaimRejected, 'already assigned'):
                    claim(api, 150, second_actor, REGISTRY)
                self.assertEqual(len(api.writes), 1)
                self.assertEqual(api.issue['assignees'], [{'login': 'developer'}])
                self.assertEqual(api.writes[0]['labels'], ['backend', 'In progress'])

    def test_already_assigned_issue_is_never_reassigned(self):
        api = FakeGitHub()
        api.issue['assignees'] = [{'login': 'colleague'}]
        with self.assertRaises(ClaimRejected):
            claim(api, 150, 'developer', REGISTRY)
        self.assertEqual(api.writes, [])

    def test_blocked_or_mismatched_dependencies_prevent_assignment(self):
        for dependencies in [[{'number': 149, 'state': 'open'}], [], [{'number': 151, 'state': 'closed'}]]:
            with self.subTest(dependencies=dependencies):
                api = FakeGitHub()
                api.blocked_by = dependencies
                with self.assertRaises(ClaimRejected):
                    claim(api, 150, 'developer', REGISTRY)
                self.assertEqual(api.writes, [])

    def test_manual_assignment_during_checks_is_preserved(self):
        api = FakeGitHub()
        api.on_dependencies = lambda: api.issue.update(assignees=[{'login': 'colleague'}])
        with self.assertRaisesRegex(ClaimRejected, 'changed'):
            claim(api, 150, 'developer', REGISTRY)
        self.assertEqual(api.writes, [])

    def test_unconfirmed_assignment_never_returns_acquired(self):
        api = FakeGitHub()
        api.ignore_assignment = True
        with self.assertRaisesRegex(ClaimRejected, 'not confirmed'):
            claim(api, 150, 'developer', REGISTRY)

    def test_dry_run_cannot_take_the_issue(self):
        api = FakeGitHub()
        result = claim(api, 150, 'developer', REGISTRY, dry_run=True)
        self.assertFalse(result['acquired'])
        self.assertTrue(result['eligible'])
        self.assertEqual(api.writes, [])

    def test_closed_non_v1_pull_request_and_bot_are_refused(self):
        for change in [{'state': 'closed'}, {'title': '[OTHER] Task'}, {'pull_request': {}}, {'user_type': 'Bot'}]:
            with self.subTest(change=change):
                api = FakeGitHub()
                if 'user_type' in change:
                    api.user_type = change['user_type']
                else:
                    api.issue.update(change)
                with self.assertRaises(ClaimRejected):
                    claim(api, 150, 'developer', REGISTRY)
                self.assertEqual(api.writes, [])
        with self.assertRaises(ClaimRejected):
            claim(FakeGitHub(), 999, 'developer', REGISTRY)


if __name__ == '__main__':
    unittest.main()

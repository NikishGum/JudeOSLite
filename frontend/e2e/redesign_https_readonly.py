"""Opt-in HTTPS smoke of existing QA188. Only login/logout may use POST.

No attendance, account, role or fixture mutation; passwords and cookies stay in
memory. This verifies the deployed UI/API and does not count as employee or
physical PWA acceptance. Supply QA188_PASSWORD or use the hidden TTY prompt.
"""
import argparse
from datetime import datetime
import getpass
import hashlib
import http.cookiejar
import json
import os
from pathlib import Path
import urllib.error
import urllib.parse
import urllib.request
from zoneinfo import ZoneInfo

from playwright.sync_api import expect, sync_playwright
from runtime_ui import observe_api, wait_for_api_idle

ORIGIN = 'https://test.judopride.tech'
CLUB = '867a31f5-7d4d-4f35-9916-65119041beb4'
ACCESS = '1e8d6955-7af7-4cbb-a761-a647bdba1ab1'
GROUP = '697f2685-64d1-4993-bcc7-c3c238a059a7'
CONTROL = '7846284c-7823-47a6-adf9-4c65b53aae38'
DATE = '2026-10-15'


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True).encode()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--expected-sha', required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    args.out.mkdir(parents=True, exist_ok=True)
    result = {'at': datetime.now(ZoneInfo('Europe/Minsk')).isoformat(),
              'origin': ORIGIN, 'releaseSha': args.expected_sha,
              'result': 'INCOMPLETE', 'checks': [], 'requests': [],
              'browserRequests': [], 'layouts': [], 'jsErrors': [],
              'tlsVerification': True, 'secretsPersisted': False,
              'runnerSha256': hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
              'limits': ['headless Linux Chromium, no physical PWA installation',
                         'no employee or visual acceptance',
                         'no new revoke/expiry/mutation/update injection on HTTPS']}
    clients = []
    password = os.environ.get('QA188_PASSWORD') or getpass.getpass('QA188 password: ')

    class Client:
        def __init__(self, role, login):
            self.role = role
            self.jar = http.cookiejar.CookieJar()
            self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.jar))
            self.request('POST', '/api/auth/login', {'login': login, 'password': password})
            clients.append(self)
            self.me = self.request('GET', '/api/auth/me')

        def request(self, method, path, data=None, expected=200):
            assert method == 'GET' or (method == 'POST' and path in ('/api/auth/login', '/api/auth/logout'))
            headers = {'Content-Type': 'application/json', 'Origin': ORIGIN}
            if method == 'POST' and hasattr(self, 'me'):
                headers['X-CSRF-Token'] = self.me['csrfToken']
            req = urllib.request.Request(ORIGIN + path, method=method, headers=headers,
                                         data=None if data is None else json.dumps(data).encode())
            try:
                response = self.opener.open(req, timeout=25)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                body = response.read()
                result['requests'].append({'role': self.role, 'method': method,
                    'path': path, 'status': response.status, 'cache': response.headers.get('Cache-Control')})
                assert response.status == expected, (self.role, method, path, response.status, expected)
                assert response.headers.get('Cache-Control') == 'no-store', path
                return json.loads(body) if body else None

    def release():
        with urllib.request.urlopen(ORIGIN + '/release.json', timeout=25) as response:
            assert json.load(response) == {'sha': args.expected_sha}, 'Unexpected HTTPS release SHA'

    try:
        release()
        # Check the runtime before spending any login attempts.
        with sync_playwright() as runtime:
            proxy = {'server': os.environ['HTTPS_PROXY']} if os.environ.get('HTTPS_PROXY') else None
            browser = runtime.chromium.launch(headless=True, proxy=proxy)
            result['browser'] = browser.version
            owner = Client('owner', 'owner')
            admin = Client('administrator', 'qa188-admin-20261015')
            trainer = Client('trainer', 'qa188-trainer-20261015')
            assert owner.me['user']['isOwner']
            assert {item['id'] for item in admin.me['clubs']} == {CLUB, ACCESS}
            assert {item['id'] for item in trainer.me['clubs']} == {CLUB}
            assert next(c for c in owner.me['clubs'] if c['id'] == CLUB)['name'].startswith('QA188')
            base = '/api/clubs/' + CLUB
            journal = admin.request('GET', base + '/journals/' + CONTROL)
            result['controlBeforeSha256'] = digest(journal)
            records = journal['records']
            assert len(records) == 5 and sum(r['state'] == 'present' for r in records) == 2
            assert sum(r['state'] == 'absent' for r in records) == 3
            expected = {'closed': 1, 'present': 2, 'absent': 3, 'unknown': 2,
                        'illness': 0, 'excused': 1, 'percentage': 40}
            query = {'from': DATE, 'until': DATE, 'clubId': CLUB, 'groupId': GROUP}
            report = owner.request('GET', '/api/reports/attendance?' + urllib.parse.urlencode(query))
            assert all(report['total'][key] == value for key, value in expected.items())
            result['controlTotals'] = expected
            for client in (admin, trainer):
                client.request('GET', '/api/reports/filters', expected=403)
                client.request('GET', '/api/reports/attendance?' + urllib.parse.urlencode(query), expected=403)
            trainer.request('GET', '/api/audit/filters', expected=403)
            trainer.request('GET', base + '/journals/' + CONTROL, expected=404)
            trainer.request('GET', base + '/trainer-sessions/' + CONTROL + '/journal')
            trainer.request('GET', '/api/clubs/' + ACCESS + '/trainer-sessions?from=' + DATE + '&until=' + DATE, expected=404)
            result['checks'].append('QA188 control/API 2/5=40%; owner/admin/trainer and foreign-club denials')

            audit_query = {'clubId': CLUB, 'from': DATE, 'until': DATE, 'action': 'corrected'}
            history = admin.request('GET', '/api/audit/events?' + urllib.parse.urlencode(audit_query))
            assert history['total'] == 1 and len(history['items']) == 1
            event = history['items'][0]
            assert event['entityId'] in {r['id'] for r in records}
            search = '#' + event['entityId'][:8] + ' corrected'
            narrowed = {**audit_query, 'q': search, 'actorId': event['actorId']}
            found = admin.request('GET', '/api/audit/events?' + urllib.parse.urlencode(narrowed))
            assert found['total'] == 1 and found['items'][0]['id'] == event['id']
            owner_found = owner.request('GET', '/api/audit/events?' + urllib.parse.urlencode(narrowed))
            assert owner_found['total'] == 1 and owner_found['items'][0]['id'] == event['id']

            def page_for(client, path, width=1280):
                context = browser.new_context(viewport={'width': width, 'height': 900},
                                              reduced_motion='reduce')
                context.add_cookies([{'name': c.name, 'value': c.value, 'domain': c.domain,
                    'path': c.path, 'secure': c.secure, 'httpOnly': c.has_nonstandard_attr('HttpOnly'),
                    'sameSite': 'Strict'} for c in client.jar])
                page = context.new_page()
                observe_api(page)
                page.on('pageerror', lambda error: result['jsErrors'].append(str(error)))
                def response_seen(response):
                    url = urllib.parse.urlsplit(response.url)
                    if url.path.startswith('/api/'):
                        result['browserRequests'].append({'role': client.role,
                            'method': response.request.method, 'path': url.path,
                            'status': response.status, 'cache': response.headers.get('cache-control')})
                page.on('response', response_seen)
                page.goto(ORIGIN + path)
                wait_for_api_idle(page)
                expect(page.get_by_role('button', name='Меню профиля', exact=True)).to_be_visible()
                return page

            def layout(page, name):
                wait_for_api_idle(page)
                measure = page.evaluate('({viewport:innerWidth,pageWidth:document.documentElement.scrollWidth})')
                assert measure['pageWidth'] <= measure['viewport'], (name, measure)
                result['layouts'].append({'screen': name, **measure})

            params = {'reportFrom': DATE, 'reportUntil': DATE, 'reportClub': CLUB, 'reportGroup': GROUP}
            own = page_for(owner, '/statistics?' + urllib.parse.urlencode(params))
            panel = own.locator('.owner-dashboard')
            expect(panel.locator('[data-metric="percentage"]')).to_have_text('40,0%')
            for key in ('closed', 'present', 'absent'):
                expect(panel.locator('[data-metric="' + key + '"]')).to_have_text(str(expected[key]))
            own.get_by_role('button', name='Период', exact=True).click()
            own.get_by_label('Период с', exact=True).fill(DATE)
            own.get_by_label('Период по', exact=True).fill(DATE)
            own.get_by_role('button', name='Применить', exact=True).click()
            own.get_by_role('button', name='Месяц', exact=True).click()
            wait_for_api_idle(own)
            own.get_by_role('button', name='Период', exact=True).click()
            expect(own.get_by_label('Период с', exact=True)).to_have_value(DATE)
            expect(own.get_by_label('Период по', exact=True)).to_have_value(DATE)
            own.get_by_role('button', name='Применить', exact=True).click()
            expect(panel.locator('[data-metric="percentage"]')).to_have_text('40,0%')
            own.get_by_role('button', name='Сравнить эти филиалы', exact=True).click()
            expect(panel.get_by_role('heading', name='Филиалы с посещаемостью ниже 80%', exact=True)).to_be_visible()
            own.get_by_role('button', name='К обзору сети', exact=True).click()
            layout(own, 'statistics1280')
            panel.locator('.summary-panel').screenshot(path=str(args.out / 'https-summary-1280.png'))
            own.set_viewport_size({'width': 390, 'height': 844})
            layout(own, 'statistics390')
            panel.locator('.summary-panel').screenshot(path=str(args.out / 'https-summary-390.png'))
            expect(own.get_by_role('tab', name='Финансы', exact=True)).to_be_visible()
            result['checks'].append('HTTPS statistics UI 40%; manual apply/range retention, problem detail, finance tab;1280/390')

            own.wait_for_function('navigator.serviceWorker.controller !== null')
            cache = own.evaluate('''async()=>{const result=[];for(const name of await caches.keys()){
              const store=await caches.open(name);for(const key of await store.keys())result.push({name,path:new URL(key.url).pathname});}return result;}''')
            assert cache and all(c['name'].startswith('pride-shell-') and c['path'].startswith('/assets/') for c in cache)
            result['cache'] = cache
            result['checks'].append('real HTTPS worker Cache Storage contains only versioned static assets')

            hist = page_for(admin, '/history')
            audit = hist.locator('.audit-history')
            audit.get_by_label('Филиал', exact=True).select_option(CLUB)
            audit.get_by_label('Период', exact=True).select_option('custom')
            audit.get_by_label('С даты', exact=True).fill(DATE)
            audit.get_by_label('По дату', exact=True).fill(DATE)
            audit.get_by_label('Действие', exact=True).select_option('corrected')
            audit.get_by_label('Сотрудник', exact=True).select_option(event['actorId'])
            audit.get_by_label('Поиск по истории', exact=True).fill(search)
            audit.get_by_label('Поиск по истории', exact=True).press('Enter')
            expect(audit.locator('.audit-event')).to_have_count(1)
            expect(audit.get_by_role('status')).to_have_text('Найдено событий: 1')
            audit.get_by_role('button', name='Подробнее о событии #' + event['id'], exact=True).click()
            expect(audit.locator('.audit-detail-row')).to_contain_text(event['entityId'])
            layout(hist, 'history1280')
            # Toolbar contains synthetic scope/ID only, no staff names or profile.
            audit.locator('.audit-toolbar').screenshot(path=str(args.out / 'https-history-toolbar-1280.png'))
            hist.set_viewport_size({'width': 390, 'height': 844})
            layout(hist, 'history390')
            audit.locator('.audit-toolbar').screenshot(path=str(args.out / 'https-history-toolbar-390.png'))
            hist.reload()
            expect(hist.locator('.audit-history')).to_be_visible()
            result['checks'].append('HTTPS history API/UI exact corrected event, #ID+action AND search, actor UUID, total/details/reload;1280/390')

            train = page_for(trainer, '/clubs/' + CLUB + '/sessions/' + CONTROL + '?journal=trainer', 390)
            expect(train.locator('.session-detail')).to_contain_text('QA188')
            expect(train.locator('.session-journal')).to_contain_text('Завершено')
            layout(train, 'trainer390')
            train.reload()
            expect(train.locator('.session-journal')).to_contain_text('Завершено')
            train.goto(ORIGIN + '/statistics')
            wait_for_api_idle(train)
            expect(train.locator('.owner-dashboard')).to_have_count(0)
            hist.goto(ORIGIN + '/statistics')
            wait_for_api_idle(hist)
            expect(hist.locator('.owner-dashboard')).to_have_count(0)
            result['checks'].append('trainer canonical control URL/reload and closed journal; admin/trainer statistics UI denial')
            after = admin.request('GET', base + '/journals/' + CONTROL)
            result['controlAfterSha256'] = digest(after)
            assert result['controlAfterSha256'] == result['controlBeforeSha256'], 'Control journal changed'
            assert all(r['method'] == 'GET' for r in result['browserRequests']), 'Unexpected browser mutation'
            assert all(r['cache'] == 'no-store' for r in result['browserRequests'])
            assert not result['jsErrors'], result['jsErrors']
            release()
            browser.close()
            result['result'] = 'PASS'
    finally:
        cleanup = []
        for client in clients:
            try:
                client.request('POST', '/api/auth/logout', {})
                client.request('GET', '/api/auth/me', expected=401)
                cleanup.append({'role': client.role, 'session': 'logged out; me401'})
            except Exception as error:
                cleanup.append({'role': client.role, 'session': 'cleanup failed', 'errorType': type(error).__name__})
                result['result'] = 'INCOMPLETE'
        result['cleanup'] = cleanup
        result['finishedAt'] = datetime.now(ZoneInfo('Europe/Minsk')).isoformat()
        (args.out / 'https-readonly.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
        print(json.dumps({'result': result['result'], 'checks': result['checks'],
                          'requestCount': len(result['requests']), 'cleanup': cleanup}, ensure_ascii=False), flush=True)
    assert result['result'] == 'PASS'


if __name__ == '__main__':
    main()

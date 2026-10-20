"""Opt-in QA188 HTTPS regression; credentials stay in memory, never in evidence.

Uses the existing synthetic club/group/control journal. Creates only a dedicated
trainer/session; finally cancels that session and disables/revokes that trainer.
Does not run in CI or change the original 40% control journal.
"""
import argparse
from datetime import datetime, timedelta
import getpass
import hashlib
import http.cookiejar
import json
import os
from pathlib import Path
import secrets
import urllib.error
import urllib.parse
import urllib.request
import uuid
from zoneinfo import ZoneInfo

from playwright.sync_api import expect, sync_playwright
from runtime_ui import observe_api, signout, wait_for_api_idle


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--origin', required=True)
    parser.add_argument('--club', required=True, type=uuid.UUID)
    parser.add_argument('--group', required=True, type=uuid.UUID)
    parser.add_argument('--control-session', required=True, type=uuid.UUID)
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--allow-fixture-writes', action='store_true', required=True)
    args = parser.parse_args()
    origin = args.origin.rstrip('/')
    assert origin == 'https://test.judopride.tech', 'This runner is for the agreed test origin only'
    args.out.mkdir(parents=True, exist_ok=True)
    source_hash = hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
    password = os.environ.get('QA188_PASSWORD') or getpass.getpass('QA188 password: ')
    events, checks, errors = [], [], []
    ui_layout = {}
    club, group, control_id = map(str, (args.club, args.group, args.control_session))
    base = '/api/clubs/'+club

    class Client:
        def __init__(self, role, login, secret):
            self.role = role
            self.jar = http.cookiejar.CookieJar()
            self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.jar))
            self.request('POST', '/api/auth/login', {'login': login, 'password': secret})
            self.me = self.request('GET', '/api/auth/me')

        def request(self, method, path, data=None, expected=200):
            headers = {'Content-Type': 'application/json', 'Origin': origin}
            if method != 'GET' and hasattr(self, 'me'):
                headers['X-CSRF-Token'] = self.me['csrfToken']
            request = urllib.request.Request(origin+path, method=method, headers=headers,
                data=None if data is None else json.dumps(data).encode())
            try:
                response = self.opener.open(request, timeout=25)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                body = json.load(response)
                events.append({'role': self.role, 'method': method, 'path': path,
                    'status': response.status, 'cache': response.headers.get('Cache-Control'),
                    'errorCode':body.get('error',{}).get('code') if isinstance(body,dict) else None})
                assert response.status == expected, (self.role, method, path, response.status, expected)
                assert response.headers.get('Cache-Control') == 'no-store', path
                return body

    owner = Client('owner', 'owner', password)
    admin = Client('administrator', 'qa188-admin-20261015', password)
    trainer = Client('trainer', 'qa188-trainer-20261015', password)
    assert owner.me['user']['isOwner']
    assert {c['id'] for c in admin.me['clubs']} == {club, '1e8d6955-7af7-4cbb-a761-a647bdba1ab1'}
    assert {c['id'] for c in trainer.me['clubs']} == {club}
    assert next(c for c in owner.me['clubs'] if c['id'] == club)['name'].startswith('QA188'), 'Only synthetic QA188 club is permitted'
    journal = admin.request('GET', base+'/journals/'+control_id)
    records = journal['records']
    assert len(records) == 5 and sum(r['state'] == 'present' for r in records) == 2
    assert sum(r['state'] == 'absent' for r in records) == 3
    assert sum(r['absenceReason'] == 'unknown' for r in records) == 2
    assert sum(r['absenceReason'] == 'excused' for r in records) == 1
    query = urllib.parse.urlencode({'from': '2026-10-15', 'until': '2026-10-15', 'clubId': club, 'groupId': group})
    report = owner.request('GET', '/api/reports/attendance?'+query)
    total = report['total']
    assert total['present'] == 2 and total['absent'] == 3 and total['percentage'] == 40, 'Control ledger changed'
    for client in (admin, trainer):
        client.request('GET', '/api/reports/filters', expected=403)
    trainer.request('GET', base+'/journals/'+control_id, expected=404)
    trainer.request('GET', base+'/participants', expected=404)
    checks.append('existing QA188 control journal/API ledger 2/5=40%; owner/admin/trainer isolation')

    probe = session = None
    successful = False
    cleanup = {'session':'not created', 'account':'not created'}
    probe_password = secrets.token_urlsafe(24)
    try:
        probe = admin.request('POST', base+'/trainers', {'login':'qa188-lifecycle-'+uuid.uuid4().hex[:8],
            'displayName':'QA188 lifecycle 20.10.2026', 'password':probe_password}, expected=201)
        today = datetime.now(ZoneInfo('Europe/Minsk')).date()
        slots = admin.request('GET', base+'/sessions?'+urllib.parse.urlencode({
            'from':str(today),'until':str(today+timedelta(days=1)),'groupId':group}))
        used = {datetime.fromisoformat(s['startsAt']).astimezone(ZoneInfo('Europe/Minsk')).strftime('%H:%M') for s in slots}
        # Cancelled sessions keep their unique group/time key. Choose a fresh
        # slot, preserving all previous probe records rather than retrying writes.
        starts_time = next(f'22:{minute:02d}' for minute in range(30) if f'22:{minute:02d}' not in used)
        session = admin.request('POST', base+'/sessions', {'groupId':group, 'trainerId':probe['id'],
            'date':str(today), 'startsTime':starts_time, 'durationMinutes':30}, expected=201)
        probe_client = Client('probe', probe['login'], probe_password)
        path = base+'/trainer-sessions/'+session['id']+'/journal'
        opened = probe_client.request('POST', path+'/open', {'version':session['version']})
        assert len(opened['records']) == 5 and all(r['state'] is None for r in opened['records'])

        with sync_playwright() as p:
            proxy = {'server':os.environ['HTTPS_PROXY']} if os.environ.get('HTTPS_PROXY') else None
            browser = p.chromium.launch(headless=True, proxy=proxy)
            def page(client, width=1280):
                context = browser.new_context(viewport={'width':width,'height':900}, reduced_motion='reduce')
                # Reuse the legitimately authenticated API session in this same
                # runner. No duplicate owner/probe login or persisted cookie file.
                context.add_cookies([{'name':c.name, 'value':c.value,
                    'domain':c.domain, 'path':c.path, 'secure':c.secure,
                    'httpOnly':c.has_nonstandard_attr('HttpOnly'), 'sameSite':'Strict'} for c in client.jar])
                value = context.new_page()
                observe_api(value)
                value.on('pageerror', lambda error: errors.append(str(error)))
                value.goto(origin)
                wait_for_api_idle(value)
                expect(value.get_by_role('button', name='Меню профиля', exact=True)).to_be_visible()
                return value

            probe_page = page(probe_client, 390)
            probe_page.goto(origin+'/clubs/'+club+'/journals')
            probe_page.get_by_role('button', name='Посмотреть журнал', exact=True).click()
            expect(probe_page.locator('.attendance-card')).to_be_visible()
            assert probe_page.evaluate('document.documentElement.scrollWidth <= innerWidth'), 'Mobile page overflow'
            wait_for_api_idle(probe_page)
            card = probe_page.locator('.attendance-card h4').inner_text()
            probe_page.context.set_offline(True)
            try:
                probe_page.wait_for_function('navigator.onLine === false')
                probe_page.get_by_role('button', name='Присутствует', exact=True).click()
                expect(probe_page.locator('.session-journal [role="alert"]')).to_contain_text('Нет сети')
                expect(probe_page.locator('.attendance-card h4')).to_have_text(card)
            finally:
                probe_page.context.set_offline(False)
            assert all(r['state'] is None for r in admin.request('GET', base+'/journals/'+session['id'])['records'])
            checks.append('HTTPS browser offline rejects mark before transport; card and server records unchanged')
            owner.request('POST', base+'/trainers/'+probe['id']+'/revoke', {'version':probe['version']})
            probe_page.get_by_role('button', name='Присутствует', exact=True).click()
            expect(probe_page.locator('.session-journal')).to_contain_text('Доступ к журналу отозван')
            expect(probe_page.locator('.attendance-card')).to_have_count(0)
            probe_client.request('GET', path, expected=404)
            after = admin.request('GET', base+'/journals/'+session['id'])
            assert all(r['state'] is None for r in after['records']), 'Revoked save wrote a mark'
            checks.append('live HTTPS role withdrawal removes visible roster; rejected save leaves five unmarked records')

            def current_probe():
                return next(item for item in owner.request('GET', '/api/trainer-accounts') if item['id'] == probe['id'])
            current = current_probe()
            owner.request('POST', '/api/staff/'+probe['id']+'/trainer-assignments', {'clubId':club, 'version':current['version']})
            current = current_probe()
            owner.request('PUT', '/api/staff/'+probe['id']+'/account', {'login':probe['login'],
                'displayName':probe['displayName'], 'disabled':True, 'version':current['version']})
            probe_client.request('GET', '/api/auth/me', expected=401)
            wait_for_api_idle(probe_page)
            probe_page.reload()
            expect(probe_page.get_by_label('Логин', exact=True)).to_be_visible()
            expect(probe_page.locator('.attendance-card')).to_have_count(0)
            current = current_probe()
            owner.request('PUT', '/api/staff/'+probe['id']+'/account', {'login':probe['login'],
                'displayName':probe['displayName'], 'disabled':False, 'version':current['version']})
            probe_client.request('GET', '/api/auth/me', expected=401)
            fresh = Client('probe-fresh', probe['login'], probe_password)
            current = current_probe()
            owner.request('POST', '/api/staff/'+probe['id']+'/password', {'password':secrets.token_urlsafe(24), 'version':current['version']})
            fresh.request('GET', '/api/auth/me', expected=401)
            checks.append('disable revokes live session; enable does not revive it; password change revokes a fresh session')
            # The owner browser below shares this legitimate API session. Finish
            # privileged cleanup before its deliberate logout invalidates it.
            current = current_probe()
            current = owner.request('PUT', '/api/staff/'+probe['id']+'/account', {'login':probe['login'],
                'displayName':probe['displayName'], 'disabled':True, 'version':current['version']})
            owner.request('POST', base+'/trainers/'+probe['id']+'/revoke', {'version':current['version']})
            cleanup['account'] = 'disabled; trainer assignment revoked'

            own_page = page(owner)
            own_page.goto(origin+'/clubs/'+club+'/statistics')
            own_page.get_by_role('button', name='Свой период', exact=True).click()
            own_page.get_by_label('Период с', exact=True).fill('2026-10-15')
            own_page.get_by_label('Период по', exact=True).fill('2026-10-15')
            own_page.get_by_label('Филиал отчёта', exact=True).select_option(club)
            own_page.get_by_label('Группа отчёта', exact=True).select_option(group)
            own_page.get_by_role('button', name='Обновить статистику', exact=True).click()
            expect(own_page.locator('[data-metric="percentage"]')).to_have_text('40,0%')
            wait_for_api_idle(own_page)
            ui_layout = own_page.evaluate('''()=>{const p=document.querySelector('.owner-dashboard');
              return {viewport:innerWidth,pageWidth:document.documentElement.scrollWidth,
                panelWidth:p.clientWidth,panelScrollWidth:p.scrollWidth,scrollX};}''')
            assert ui_layout['pageWidth'] <= ui_layout['viewport'] and ui_layout['panelWidth'] == ui_layout['panelScrollWidth'], ui_layout
            # A tall element capture can resize the viewport and clip an animated
            # Sidebar layout. Capture the actual four metric cells without that
            # temporary resize, and keep page/panel measurements as evidence.
            own_page.locator('.report-totals').screenshot(path=str(args.out/'https-control-report.png'))
            own_page.wait_for_function('navigator.serviceWorker.controller !== null')
            cache = own_page.evaluate('''async()=>{const result=[];for(const name of await caches.keys()){
              const c=await caches.open(name);for(const key of await c.keys()){
                const r=await c.match(key);result.push({name,path:new URL(key.url).pathname,type:r.headers.get('Content-Type')});}}
              return result;}''')
            assert cache and all(v['name'].startswith('pride-shell-') and v['path'].startswith('/assets/') and 'json' not in (v['type'] or '') for v in cache)
            checks.append('HTTPS owner UI confirms 40%; real worker cache contains only static assets')
            # Hold a genuine response, then change account before its body reaches React.
            own_page.evaluate('''()=>{const original=window.fetch;window.releaseQa188=null;
              window.fetch=async(...args)=>{const r=await original(...args);
                if(new URL(typeof args[0]==='string'?args[0]:args[0].url,location.href).pathname==='/api/reports/attendance')
                  await new Promise(resolve=>window.releaseQa188=resolve);return r;};}''')
            own_page.get_by_role('button', name='Обновить статистику', exact=True).click()
            own_page.wait_for_function('window.releaseQa188 !== null')
            signout(own_page)
            expect(own_page.get_by_label('Логин', exact=True)).to_be_visible()
            own_page.get_by_label('Логин', exact=True).fill('qa188-admin-20261015')
            own_page.get_by_label('Пароль', exact=True).fill(password)
            own_page.get_by_role('button', name='Войти', exact=True).click()
            expect(own_page.get_by_role('button', name='Меню профиля', exact=True)).to_be_visible()
            own_page.evaluate('window.releaseQa188()')
            wait_for_api_idle(own_page)
            expect(own_page.locator('.owner-dashboard')).to_have_count(0)
            own_page.reload()
            expect(own_page.locator('.owner-dashboard')).to_have_count(0)
            checks.append('late owner report cannot return after logout/admin login or reload')
            assert not errors, errors
            browser.close()
            successful = True
    finally:
        if probe and cleanup['account'] == 'not created':
            current = next(item for item in owner.request('GET', '/api/trainer-accounts') if item['id'] == probe['id'])
            if not current['disabled']:
                owner.request('PUT', '/api/staff/'+probe['id']+'/account', {'login':probe['login'],
                    'displayName':probe['displayName'], 'disabled':True, 'version':current['version']})
                current = next(item for item in owner.request('GET', '/api/trainer-accounts') if item['id'] == probe['id'])
            if club in current['trainerClubIds']:
                owner.request('POST', base+'/trainers/'+probe['id']+'/revoke', {'version':current['version']})
            cleanup['account'] = 'disabled; trainer assignment revoked'
        if session:
            latest = admin.request('GET', base+'/journals/'+session['id'])
            unmarked = all(r['state'] is None for r in latest['records'])
            admin.request('POST', base+'/sessions/'+session['id']+'/cancel', {'version':latest['session']['version'],
                'confirmOpen':True, 'reason':'QA188 lifecycle 20.10.2026: technical verification complete'})
            cleanup['session'] = 'cancelled; all five records remain unmarked' if unmarked else 'cancelled; unexpected marks preserved for investigation'
            assert unmarked, 'Probe must not record attendance'
        result = {'at':datetime.now(ZoneInfo('Europe/Minsk')).isoformat(), 'origin':origin,
            'result':'PASS' if successful else 'INCOMPLETE', 'runnerSha256':source_hash,
            'checks':checks, 'requests':events, 'probeUserId':probe['id'] if probe else None,
            'probeSessionId':session['id'] if session else None, 'cleanup':cleanup,
            'uiLayout':ui_layout,
            'tlsVerification':True, 'secretsPersisted':False,
            'limits':['session natural expiry tested only on disposable DB','different-release SW activation tested only on two local builds',
                'native PWA installation and physical device checks not performed by this runner']}
        (args.out/'https-lifecycle.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
        print(json.dumps({'result':result['result'],'checks':checks,'requestCount':len(events)},ensure_ascii=False),flush=True)


if __name__ == '__main__':
    main()

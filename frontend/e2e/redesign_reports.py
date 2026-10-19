from runtime_ui import observe_api, wait_for_api_idle, signout
"""RD-04: real browser -> Go -> disposable PostgreSQL. No external club data."""
import json
import os
from pathlib import Path
import pty
import select
import shutil
import subprocess
import sys
import tempfile
import time
import uuid
from datetime import datetime, timedelta
from urllib.request import urlopen
from zoneinfo import ZoneInfo
from playwright.sync_api import sync_playwright, expect

ROOT = 'http://127.0.0.1:18086'
REPO = Path(__file__).resolve().parents[2]
ENGINE = os.environ.get('PWA_BROWSER', 'chromium')
IMAGE = os.environ.get('PWA_TEST_IMAGE', 'judeoslite:ci')
assert ENGINE in ('chromium', 'webkit') and IMAGE.startswith('judeoslite:')
PREFIX = 'rd186-' + uuid.uuid4().hex[:8]
DB, APP, NET = PREFIX+'-db', PREFIX+'-app', PREFIX+'-net'
PASSWORD = 'synthetic-browser-password'
STATIC_TEMP = tempfile.TemporaryDirectory(prefix='rd186-static-')
static = Path(STATIC_TEMP.name)/'static'
shutil.copytree(REPO/'frontend/dist', static)
for file in (static, *static.rglob('*')):
    file.chmod(0o755 if file.is_dir() else 0o644)

def cmd(*args):
    return subprocess.check_output(args, text=True).strip()

def sql(statement):
    return cmd('docker', 'exec', DB, 'psql', '-U', 'test', '-d', 'rd186', '-Atc', statement)

def owner(env):
    master, slave = pty.openpty()
    process = subprocess.Popen(['docker', 'run', '--rm', '-it', '--network', NET, *env, IMAGE,
        '/app/create-owner', '--login', 'owner', '--name', 'Synthetic Owner'], stdin=slave, stdout=slave, stderr=slave)
    os.close(slave)
    output = b''
    for prompt in (b'Password (', b'Repeat password:'):
        deadline = time.monotonic()+30
        while prompt not in output:
            assert time.monotonic() < deadline, 'Owner bootstrap timeout'
            if select.select([master], [], [], 1)[0]:
                output += os.read(master, 4096)
        os.write(master, (PASSWORD+'\n').encode()); output = b''
    assert process.wait(timeout=30) == 0
    os.close(master)

try:
    cmd('docker', 'network', 'create', NET)
    cmd('docker', 'run', '-d', '--name', DB, '--network', NET, '-e', 'POSTGRES_USER=test', '-e', 'POSTGRES_PASSWORD=test-only', '-e', 'POSTGRES_DB=rd186', 'postgres:18.6-bookworm')
    for _ in range(30):
        if subprocess.run(['docker', 'exec', DB, 'pg_isready', '-h', '127.0.0.1', '-U', 'test', '-d', 'rd186'], stdout=subprocess.DEVNULL).returncode == 0:
            break
        time.sleep(1)
    else:
        raise AssertionError('PostgreSQL not ready')
    env = ['-e', f'DATABASE_URL=postgres://test:test-only@{DB}:5432/rd186?sslmode=disable']
    cmd('docker', 'run', '--rm', '--network', NET, *env, IMAGE, '/app/migrate')
    owner(env)
    cmd('docker', 'create', '--name', APP, '--network', NET, *env, '-e', f'APP_ORIGIN={ROOT}', '-e', 'AUTH_INSECURE_LOCAL=true', '-p', '127.0.0.1:18086:8080', IMAGE)
    cmd('docker', 'cp', str(static)+'/.', APP+':/app/static')
    cmd('docker', 'start', APP)
    for _ in range(30):
        try:
            with urlopen(ROOT+'/api/readyz', timeout=2) as response:
                if response.status == 200:
                    break
        except OSError:
            pass
        time.sleep(1)
    else:
        raise AssertionError('App not ready')
    with sync_playwright() as p:
        chromium_path = os.environ.get('CHROMIUM_PATH')
        browser = getattr(p, ENGINE).launch(headless=True, executable_path=chromium_path if ENGINE == 'chromium' else None)
        errors = []
        def page(login):
            context = browser.new_context(viewport={'width': 1280, 'height': 900}, timezone_id='Pacific/Honolulu', service_workers='block', reduced_motion='reduce')
            value = context.new_page()
            observe_api(value)
            value.on('pageerror', lambda error: errors.append(str(error)))
            value.goto(ROOT)
            # Settle the initial session read before filling controlled inputs.
            value.wait_for_load_state('networkidle')
            wait_for_api_idle(value)
            value.get_by_label('Логин', exact=True).fill(login)
            value.get_by_label('Пароль', exact=True).fill(PASSWORD)
            expect(value.get_by_label('Логин', exact=True)).to_have_value(login)
            value.get_by_role('button', name='Войти', exact=True).click()
            expect(value.get_by_role('button', name='Меню профиля', exact=True)).to_be_visible()
            wait_for_api_idle(value)
            return value
        def api(value, method, path, data=None, status=None):
            result = value.evaluate('''async ([method,path,data])=>{
              const me=await (await fetch('/api/auth/me')).json();
              const r=await fetch(path,{method,headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrfToken},body:method==='GET'?undefined:JSON.stringify(data)});
              return {status:r.status,body:await r.json(),cache:r.headers.get('Cache-Control')};
            }''', [method, path, data])
            assert result['status'] == status if status else result['status'] in (200, 201), result
            assert result['cache'] == 'no-store', result
            return result['body']
        owner = page('owner')
        suffix = uuid.uuid4().hex[:8]
        clubs, groups, trainers, tpages = [], [], [], []
        for name, count in [('A', 4), ('B', 3)]:
            c = api(owner, 'POST', '/api/clubs', {'name': 'Report '+name+' '+suffix, 'timezone': 'Europe/Minsk'}, 201)
            base = '/api/clubs/'+c['id']
            g = api(owner, 'POST', base+'/groups', {'name': 'Same group '+suffix}, 201)
            trainer = api(owner, 'POST', base+'/trainers', {'login': 'rt-'+name.lower()+'-'+suffix, 'displayName': 'Synthetic Trainer '+name, 'password': PASSWORD}, 201)
            for i in range(count):
                api(owner, 'POST', base+'/participants', {'identity': {'fullName': f'{i} Synthetic {suffix}', 'birthDate': '1990-01-01'}, 'joinedOn': '1999-01-01', 'contactKind': 'phone', 'contactValue': '+375291234567', 'initialGroup': {'groupId': g['id'], 'from': '1999-01-01'}}, 201)
            tp = page(trainer['login'])
            clubs.append(c); groups.append(g); trainers.append(trainer); tpages.append(tp)
        a, b = clubs; ga, gb = groups; ta, tb = tpages
        admin = api(owner, 'POST', '/api/staff', {'login': 'ra-'+suffix, 'displayName': 'Synthetic Administrator', 'password': PASSWORD, 'clubIds': [a['id'], b['id']]}, 201)
        empty = api(owner, 'POST', '/api/clubs/'+a['id']+'/groups', {'name': 'Empty '+suffix}, 201)
        def new_session(index, group, time, date='2000-02-01', source='2000-02-01'):
            # AT TIME ZONE applies historical Europe/Minsk offset (UTC+02 in 2000).
            return sql(f"INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES('{clubs[index]['id']}','{group['id']}','{trainers[index]['id']}','{source}','Europe/Minsk',('{date} {time}'::timestamp AT TIME ZONE 'Europe/Minsk'),('{date} {time}'::timestamp AT TIME ZONE 'Europe/Minsk')+interval '1 hour') RETURNING id").splitlines()[0]
        def journal(index, sid, marks, close=True):
            path = '/api/clubs/'+clubs[index]['id']+'/trainer-sessions/'+sid+'/journal'
            j = api(tpages[index], 'POST', path+'/open', {'version': 1})
            for i, state, reason in marks:
                j = api(tpages[index], 'PUT', path+'/records/'+j['records'][i]['id'], {'version': j['records'][i]['version'], 'sessionVersion': j['session']['version'], 'state': state, 'absenceReason': reason})
            return api(tpages[index], 'POST', path+'/close', {'version': j['session']['version']}) if close else j
        sa = new_session(0, ga, '00:30', source='2000-01-31')
        ja = journal(0, sa, [(0, 'present', None), (1, 'present', None), (2, 'absent', 'illness')])
        sb = new_session(1, gb, '18:00')
        journal(1, sb, [(0, 'present', None), (1, 'absent', 'illness'), (2, 'absent', 'excused')])
        journal(0, new_session(0, empty, '19:00'), [])
        journal(0, new_session(0, ga, '18:00'), [(0, 'present', None)], close=False)
        new_session(1, gb, '19:00')
        sc = new_session(0, ga, '20:00')
        jc = journal(0, sc, [(0, 'absent', 'illness')], close=False)
        api(owner, 'POST', '/api/clubs/'+a['id']+'/sessions/'+sc+'/cancel', {'version': jc['session']['version'], 'reason': 'synthetic cancel', 'confirmOpen': True})
        journal(0, new_session(0, empty, '00:00', date='2000-02-02', source='2000-02-02'), [])
        mid = ja['records'][0]['membershipId']
        sql(f"UPDATE pride.group_memberships SET valid_until='2000-02-02' WHERE club_membership_id='{mid}'; INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from) VALUES('{a['id']}','{empty['id']}','{mid}','2000-02-02'); UPDATE pride.club_memberships SET archived_at=now() WHERE id='{mid}'; UPDATE pride.training_groups SET archived_at=now() WHERE id='{ga['id']}'")
        report_path = '/api/reports/attendance?from=2000-02-01&until=2000-02-01'
        r = api(owner, 'GET', report_path)
        assert [r['total'][k] for k in ('closed', 'planned', 'open', 'present', 'absent', 'unknown', 'illness', 'excused')] == [3, 1, 1, 3, 4, 1, 2, 1], r
        assert len(r['clubs']) == 2 and len(r['groups']) == 3
        owner.goto(ROOT+'/statistics')
        screen = owner.locator('.owner-dashboard')
        def metric(name, value):
            expect(screen.locator('[data-metric="'+name+'"]')).to_have_text(value)
        def period(start='2000-02-01', end='2000-02-01'):
            screen.get_by_label('Период с', exact=True).fill(start)
            screen.get_by_label('Период по', exact=True).fill(end)
        def shots(value, name):
            for width in (1280, 390, 320):
                value.set_viewport_size({'width': width, 'height': 900})
                value.wait_for_load_state('networkidle')
                value.evaluate('window.scrollTo(0,0)')
                assert value.evaluate('document.documentElement.scrollWidth <= innerWidth'), (name, width)
                for control in value.locator('main button:visible,main input:visible,main select:visible,main summary:visible,.report-session-link:visible').all():
                    rect = control.bounding_box()
                    assert rect['height'] >= 44 and rect['width'] >= 44, (name, width, rect, control.text_content())
                    if width <= 390 and control.evaluate('el=>["BUTTON","INPUT","SELECT"].includes(el.tagName)'):
                        assert float(control.evaluate('el=>parseFloat(getComputedStyle(el).fontSize)')) >= 16, control.text_content()
                value.screenshot(path=f'/tmp/rd186-{ENGINE}-{name}-{width}.png', full_page=True)
            value.set_viewport_size({'width': 1280, 'height': 900})
        period()
        for key, value in [('closed','3'),('present','3'),('absent','4'),('planned','1'),('open','1'),('percentage','42,9%')]: metric(key, value)
        expect(screen.locator('[data-metric="reasons"]')).to_contain_text('неизвестна — 1; болезнь — 2; уважительная — 1')
        shots(owner, 'report')
        # Explicit quick calendar across a Sunday/year boundary, independent of browser Honolulu.
        owner.clock.set_fixed_time(datetime(2027, 1, 3, 0, 30, tzinfo=ZoneInfo('Europe/Minsk')))
        for label, start in [('Сегодня','2027-01-03'),('Неделя','2026-12-28'),('Месяц','2027-01-01')]:
            screen.get_by_role('button', name=label, exact=True).click()
            expect(screen.get_by_label('Период с', exact=True)).to_have_value(start)
            expect(screen.get_by_label('Период по', exact=True)).to_have_value('2027-01-03')
        screen.get_by_role('button', name='Свой период', exact=True).click(); metric('percentage', '42,9%')
        expect(screen.get_by_label('Период с', exact=True)).to_have_value('2000-02-01')
        # Same group names are distinguished by club and UUID. Drilldown does not change workspace.
        screen.locator(f'tr[data-group-id="{gb["id"]}"] button').click()
        expect(screen.get_by_label('Филиал отчёта', exact=True)).to_have_value(b['id'])
        expect(screen.get_by_label('Группа отчёта', exact=True)).to_have_value(gb['id']); metric('percentage','33,3%')
        expect(owner.get_by_label('Филиал', exact=True)).to_have_value('')
        screen.get_by_role('link', name='Занятия выбранного контекста').click()
        expect(owner.get_by_label('Филиал', exact=True)).to_have_value(b['id'])
        expect(owner.get_by_label('Группа календаря', exact=True)).to_have_value(gb['id'])
        expect(owner.get_by_label('С даты', exact=True)).to_have_value('2000-02-01')
        expect(owner.get_by_label('По дату включительно', exact=True)).to_have_value('2000-02-01')
        owner.goto(ROOT+'/statistics'); period()
        screen.get_by_label('Филиал отчёта', exact=True).select_option(a['id'])
        screen.get_by_label('Группа отчёта', exact=True).select_option(empty['id']); metric('closed','1'); metric('percentage','—')
        screen.get_by_label('Филиал отчёта', exact=True).select_option(''); metric('percentage','42,9%')
        # A real UI correction from a second page, then attendance-changed re-reads SQL totals.
        edit = page('owner'); edit.goto(ROOT+f'/clubs/{a["id"]}/sessions?session={sa}&from=2000-02-01&through=2000-02-01&view=period')
        row = edit.locator(f'[data-record-id="{ja["records"][2]["id"]}"]')
        row.get_by_role('button', name='Исправить отметку', exact=True).click()
        edit.get_by_label('Новая отметка', exact=True).select_option('present')
        edit.get_by_label('Причина исправления', exact=True).fill('PRIVATE correction marker')
        correction_url = ROOT+f"/api/clubs/{a['id']}/journals/{sa}/records/{ja['records'][2]['id']}"
        with edit.expect_response(lambda response: response.url == correction_url and response.request.method == 'PUT') as corrected:
            edit.get_by_role('button', name='Сохранить исправление', exact=True).click()
        assert corrected.value.status == 200
        # The open select already contains the word 'Присутствует'. Await the
        # real commit and closed editor before refreshing the observing report.
        expect(row.locator('.correction-form')).to_have_count(0)
        expect(row.locator('.training-status')).to_have_text('Присутствует')
        assert sql(f"SELECT state FROM pride.attendance_records WHERE id='{ja['records'][2]['id']}'") == 'present'
        # The report deliberately refreshes only while visible. The correction
        # page is a separate browser context; make the observing page visible
        # before asking it to re-read the committed totals.
        owner.bring_to_front()
        owner.wait_for_function("document.visibilityState === 'visible'")
        owner.evaluate("window.dispatchEvent(new Event('attendance-changed'))")
        metric('present','4'); metric('absent','3'); metric('percentage','57,1%')
        assert sql(f"SELECT count(*) FROM pride.audit_events WHERE entity_id='{ja['records'][2]['id']}' AND action='corrected'") == '1'
        # Error/retry has no fabricated zero. Hold a genuine response across a newer filter.
        owner.route('**/api/reports/attendance?*', lambda route: route.abort())
        screen.get_by_role('button', name='Обновить статистику', exact=True).click()
        expect(screen.get_by_role('alert')).to_contain_text('Нет соединения')
        expect(screen.locator('[data-metric="closed"]')).to_have_count(0)
        owner.unroute('**/api/reports/attendance?*')
        screen.get_by_role('button', name='Повторить загрузку статистики', exact=True).click(); metric('percentage','57,1%')
        def hold(value, target):
            value.evaluate('''target=>{const original=window.fetch;window.held=[];window.restore=()=>window.fetch=original;
              window.fetch=async(...args)=>{const response=await original(...args);
                if(String(args[0]).includes(target))await new Promise(resolve=>window.held.push(resolve));return response;};}''', target)
        def release(value):
            value.evaluate('window.restore();window.held.forEach(resolve=>resolve())')
        hold(owner, 'clubId='+a['id'])
        screen.get_by_label('Филиал отчёта', exact=True).select_option(a['id'])
        owner.wait_for_function('window.held.length > 0')
        screen.get_by_label('Филиал отчёта', exact=True).select_option(b['id']); metric('percentage','33,3%')
        release(owner); metric('percentage','33,3%')
        screen.get_by_label('Филиал отчёта', exact=True).select_option(''); metric('percentage','57,1%')
        period('2000-02-02','2000-02-01'); expect(screen.get_by_role('alert')).to_contain_text('начало периода')
        expect(screen.locator('[data-metric="closed"]')).to_have_count(0)
        period('2000-02-02','2000-02-02'); metric('percentage','—'); metric('closed','1')
        # Archive retains historical sums and never offers an active-club action.
        sql(f"UPDATE pride.clubs SET archived_at=now() WHERE id='{b['id']}'")
        period(); screen.get_by_label('Филиал отчёта', exact=True).select_option(b['id']); metric('percentage','33,3%')
        expect(screen.get_by_role('link', name='Занятия выбранного контекста')).to_have_count(0)
        expect(screen.locator('.report-context')).to_contain_text('Филиал в архиве')
        screen.get_by_label('Филиал отчёта', exact=True).select_option(''); metric('percentage','57,1%')
        owner.set_viewport_size({'width':390,'height':900})
        screen.locator(f'.report-mobile-list [data-group-id="{ga["id"]}"] button').click()
        expect(screen.get_by_label('Группа отчёта', exact=True)).to_have_value(ga['id'])
        screen.get_by_label('Филиал отчёта', exact=True).select_option(''); metric('percentage','57,1%')
        owner.set_viewport_size({'width':1280,'height':900})
        # Cocoa's default keyboard navigation uses Option+Tab for buttons.
        # Keep the same forward/reverse focus assertions on each platform.
        tab = 'Alt+Tab' if ENGINE == 'webkit' and sys.platform == 'darwin' else 'Tab'
        back_tab = 'Alt+Shift+Tab' if ENGINE == 'webkit' and sys.platform == 'darwin' else 'Shift+Tab'
        screen.get_by_role('button', name='Сегодня', exact=True).focus(); owner.keyboard.press(tab)
        expect(screen.get_by_role('button', name='Неделя', exact=True)).to_be_focused()
        owner.keyboard.press(back_tab); expect(screen.get_by_role('button', name='Сегодня', exact=True)).to_be_focused()
        # Stable audit pagination, public envelope only. Inserts are synthetic private fixtures.
        actor = api(owner, 'GET', '/api/auth/me')['user']['id']
        def events(club, count, action='created', stamp='2000-02-01 12:00Z'):
            club_sql = f"'{club}'" if club else 'NULL'
            sql(f"INSERT INTO pride.audit_events(actor_id,club_id,entity_type,entity_id,action,created_at,after_state,change_reason) SELECT '{actor}',{club_sql},'training_group',gen_random_uuid(),'{action}','{stamp}','{{\"contact\":\"PRIVATE payload marker\"}}','PRIVATE reason marker' FROM generate_series(1,{count})")
        events(a['id'],55); events(b['id'],2); events(None,1)
        owner.goto(ROOT+'/history'); audit=owner.locator('.audit-history')
        audit.get_by_label('С даты', exact=True).fill('2000-02-01'); audit.get_by_label('По дату', exact=True).fill('2000-02-01')
        audit.get_by_role('button', name='Показать историю', exact=True).click()
        expect(audit.locator('.audit-event')).to_have_count(50)
        first=audit.locator('.audit-event').all_text_contents()
        events(a['id'],1,stamp='2000-02-01 13:00Z')  # Newer insert stays before the current keyset cursor.
        audit.get_by_role('button', name='Следующая страница', exact=True).click()
        expect(audit.locator('.audit-event')).to_have_count(8)
        assert not set(first).intersection(audit.locator('.audit-event').all_text_contents())
        audit.get_by_role('button', name='Предыдущая страница', exact=True).click(); expect(audit.locator('.audit-event')).to_have_count(50)
        audit.get_by_label('Филиал истории', exact=True).select_option('network')
        audit.get_by_role('button', name='Показать историю', exact=True).click(); expect(audit.locator('.audit-event')).to_have_count(1)
        assert 'PRIVATE' not in audit.inner_text()
        audit.get_by_label('Действие', exact=True).select_option('corrected')
        audit.get_by_label('С даты', exact=True).fill(''); audit.get_by_label('По дату', exact=True).fill('')
        audit.get_by_label('Филиал истории', exact=True).select_option(a['id'])
        audit.get_by_role('button', name='Показать историю', exact=True).click(); expect(audit.locator('.audit-event')).to_have_count(1)
        assert 'PRIVATE' not in audit.inner_text()
        shots(owner, 'history')
        payload=api(owner,'GET','/api/audit/events?action=corrected')
        assert 'PRIVATE' not in json.dumps(payload) and len(payload['items'])==1
        audit.get_by_label('Действие', exact=True).select_option('created')
        audit.get_by_label('С даты', exact=True).fill('1990-01-01'); audit.get_by_label('По дату', exact=True).fill('1990-01-01')
        audit.get_by_role('button', name='Показать историю', exact=True).click(); expect(audit.get_by_role('status')).to_contain_text('изменений нет')
        owner.route('**/api/audit/events?*',lambda route:route.abort())
        audit.get_by_role('button', name='Обновить историю', exact=True).click(); expect(audit.get_by_role('alert')).to_be_visible()
        expect(audit.locator('.audit-event')).to_have_count(0)
        owner.unroute('**/api/audit/events?*'); audit.get_by_role('button', name='Повторить загрузку истории', exact=True).click()
        expect(audit.get_by_role('status')).to_contain_text('изменений нет')
        audit.get_by_label('С даты', exact=True).fill('2000-02-01'); audit.get_by_label('По дату', exact=True).fill('2000-02-01')
        hold(owner, 'clubId='+a['id'])
        audit.get_by_role('button', name='Показать историю', exact=True).click(); owner.wait_for_function('window.held.length > 0')
        audit.get_by_label('Филиал истории', exact=True).select_option(b['id'])
        audit.get_by_role('button', name='Показать историю', exact=True).click(); expect(audit.locator('.audit-event')).to_have_count(2)
        release(owner); expect(audit.locator('.audit-event')).to_have_count(2)
        # Admin scope/revoke removes both visible events and in-flight results; trainer cannot read.
        admin_page=page(admin['login'])
        sql(f"DELETE FROM pride.staff_club_roles WHERE staff_user_id='{admin['id']}' AND club_id='{b['id']}'")
        admin_page.goto(ROOT+'/history'); admin_audit=admin_page.locator('.audit-history')
        expect(admin_audit.locator('.audit-event').first).to_be_visible()
        assert b['name'] not in admin_audit.get_by_label('Филиал истории').inner_text()
        api(admin_page,'GET',report_path,status=403); api(ta,'GET','/api/audit/filters',status=403)
        hold(admin_page, '/api/audit/events?')
        admin_audit.get_by_role('button', name='Обновить историю', exact=True).click(); admin_page.wait_for_function('window.held.length > 0')
        sql(f"DELETE FROM pride.staff_club_roles WHERE staff_user_id='{admin['id']}'")
        admin_page.evaluate("window.dispatchEvent(new Event('focus'))")
        expect(admin_page.get_by_role('heading', name='История изменений', exact=True)).to_have_count(0)
        release(admin_page); expect(admin_page.locator('.audit-event')).to_have_count(0)
        # Owner demotion while a real report response is held cannot repopulate the DOM.
        owner.goto(ROOT+'/statistics'); period(); metric('percentage','57,1%')
        hold(owner,'/api/reports/attendance?')
        screen.get_by_role('button', name='Обновить статистику', exact=True).click(); owner.wait_for_function('window.held.length > 0')
        sql(f"UPDATE pride.staff_users SET is_owner=false WHERE id='{actor}'")
        owner.evaluate("window.dispatchEvent(new Event('focus'))")
        expect(screen).to_have_count(0); release(owner); expect(screen).to_have_count(0)
        signout(owner); expect(owner.get_by_label('Логин', exact=True)).to_be_visible()
        assert not errors, errors
        print(json.dumps({'engine':ENGINE,'version':browser.version,'result':'PASS','ledger':'3/7=42.9%; corrected 4/7=57.1%; empty closed; planned/open/cancelled; archived/transfer; saved session timezone','checks':'quick Sunday/year/Minsk vs browser date; custom restore; same names/ID drilldown/session period; error/retry/stale responses; audit 50+8 paging/scope/public envelope; live admin+owner revoke; keyboard; 320/390/1280'},ensure_ascii=False))
        browser.close()
finally:
    STATIC_TEMP.cleanup()
    subprocess.run(['docker','rm','-f',APP,DB],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    subprocess.run(['docker','network','rm',NET],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)

from runtime_ui import observe_api, wait_for_api_idle, signout
"""RD-05: real browser -> Go -> disposable PostgreSQL. No external club data."""
import json
import os
from pathlib import Path
import pty
import select
import shutil
import subprocess
import tempfile
import time
import uuid
from datetime import datetime, timedelta
from urllib.request import urlopen
from zoneinfo import ZoneInfo
from playwright.sync_api import sync_playwright, expect

ROOT = 'http://127.0.0.1:18087'
REPO = Path(__file__).resolve().parents[2]
ENGINE = os.environ.get('PWA_BROWSER', 'chromium')
IMAGE = os.environ.get('PWA_TEST_IMAGE', 'judeoslite:ci')
assert ENGINE in ('chromium', 'webkit') and IMAGE.startswith('judeoslite:')
PREFIX = 'rd187-' + uuid.uuid4().hex[:8]
DB, APP, NET = PREFIX+'-db', PREFIX+'-app', PREFIX+'-net'
PASSWORD = 'synthetic-browser-password'
STATIC_TEMP = tempfile.TemporaryDirectory(prefix='rd187-static-')
static = Path(STATIC_TEMP.name)/'static'
shutil.copytree(REPO/'frontend/dist', static)
for file in (static, *static.rglob('*')):
    file.chmod(0o755 if file.is_dir() else 0o644)

def cmd(*args):
    return subprocess.check_output(args, text=True).strip()

def sql(statement):
    return cmd('docker', 'exec', DB, 'psql', '-U', 'test', '-d', 'rd187', '-Atc', statement)

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
    cmd('docker', 'run', '-d', '--name', DB, '--network', NET, '-e', 'POSTGRES_USER=test', '-e', 'POSTGRES_PASSWORD=test-only', '-e', 'POSTGRES_DB=rd187', 'postgres:18.6-bookworm')
    for _ in range(30):
        if subprocess.run(['docker', 'exec', DB, 'pg_isready', '-h', '127.0.0.1', '-U', 'test', '-d', 'rd187'], stdout=subprocess.DEVNULL).returncode == 0:
            break
        time.sleep(1)
    else:
        raise AssertionError('PostgreSQL not ready')
    env = ['-e', f'DATABASE_URL=postgres://test:test-only@{DB}:5432/rd187?sslmode=disable']
    cmd('docker', 'run', '--rm', '--network', NET, *env, IMAGE, '/app/migrate')
    owner(env)
    cmd('docker', 'create', '--name', APP, '--network', NET, *env, '-e', f'APP_ORIGIN={ROOT}', '-e', 'AUTH_INSECURE_LOCAL=true', '-p', '127.0.0.1:18087:8080', IMAGE)
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
            recent = []
            value.on('requestfailed', lambda request: recent.append({'url':request.url,'failure':request.failure}))
            def page_error(error):
                errors.append(str(error))
                print(json.dumps({'engine':ENGINE,'check':'unexpected-page-error','url':value.url,'message':str(error),'stack':error.stack,'failedRequests':recent[-12:]},ensure_ascii=False),flush=True)
            value.on('pageerror', page_error)
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
        owner_page = page('owner')
        suffix = uuid.uuid4().hex[:8]
        clubs, groups, trainers, tpages, members = [], [], [], [], []
        for name, zone, count in [('A', 'Pacific/Kiritimati', 3), ('B', 'Pacific/Honolulu', 2)]:
            c = api(owner_page, 'POST', '/api/clubs', {'name': 'Dashboard '+name+' '+suffix, 'timezone': zone}, 201)
            base = '/api/clubs/'+c['id']
            g = api(owner_page, 'POST', base+'/groups', {'name': 'Same group '+suffix}, 201)
            trainer = api(owner_page, 'POST', base+'/trainers', {'login': 'dt-'+name.lower()+'-'+suffix, 'displayName': 'Synthetic Trainer '+name, 'password': PASSWORD}, 201)
            cards = []
            for i in range(count):
                cards.append(api(owner_page, 'POST', base+'/participants', {'identity': {'fullName': f'{i} Synthetic {name} {suffix}', 'birthDate': '1990-01-01'}, 'joinedOn': '1999-01-01', 'contactKind': 'phone', 'contactValue': '+375291234567', 'initialGroup': {'groupId': g['id'], 'from': '1999-01-01'}}, 201))
            clubs.append(c); groups.append(g); trainers.append(trainer); members.append(cards); tpages.append(page(trainer['login']))
        a, b = clubs; ga, gb = groups; ta, tb = tpages
        dates = [datetime.now(ZoneInfo(c['timezone'])).date().isoformat() for c in clubs]
        assert dates[0] != dates[1], '24-hour timezone difference must have separate local dates'
        yesterday = (datetime.fromisoformat(dates[0])-timedelta(days=1)).date().isoformat()
        admin = api(owner_page, 'POST', '/api/staff', {'login': 'da-'+suffix, 'displayName': 'Synthetic Administrator', 'password': PASSWORD, 'clubIds': [a['id']]}, 201)
        admin_page = page(admin['login'])
        def new_session(index, group, hour, date=None):
            date = date or dates[index]
            c = clubs[index]
            return sql(f"INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES('{c['id']}','{group['id']}','{trainers[index]['id']}','{date}','{c['timezone']}',('{date} {hour}'::timestamp AT TIME ZONE '{c['timezone']}'),('{date} {hour}'::timestamp AT TIME ZONE '{c['timezone']}')+interval '1 hour') RETURNING id").splitlines()[0]
        def journal(index, sid, marks=(), close=False):
            path = '/api/clubs/'+clubs[index]['id']+'/trainer-sessions/'+sid+'/journal'
            j = api(tpages[index], 'POST', path+'/open', {'version': 1})
            for i, state, reason in marks:
                j = api(tpages[index], 'PUT', path+'/records/'+j['records'][i]['id'], {'version': j['records'][i]['version'], 'sessionVersion': j['session']['version'], 'state': state, 'absenceReason': reason})
            return api(tpages[index], 'POST', path+'/close', {'version': j['session']['version']}) if close else j
        old = new_session(0, ga, '08:00', yesterday)
        jo = journal(0, old, [(0, 'present', None)])
        planned = new_session(0, ga, '18:00')
        sa = new_session(0, ga, '09:00'); ja = journal(0, sa, [(0, 'present', None)], True)
        cancelled = new_session(0, ga, '20:00')
        api(owner_page, 'POST', '/api/clubs/'+a['id']+'/sessions/'+cancelled+'/cancel', {'version': 1, 'reason': 'Synthetic cancellation'})
        bo = new_session(1, gb, '18:00'); journal(1, bo)
        sb = new_session(1, gb, '09:00'); journal(1, sb, [(0, 'present', None), (1, 'absent', 'illness')], True)
        start, until = min(dates), max(dates)
        def home(value, club=''):
            # A full navigation must not tear down journal/options fetches that
            # the previous action just scheduled. WebKit can report their
            # document teardown as access-control pageerrors even when caught.
            # Drain the real requests; keep the strict pageerror assertion.
            value.wait_for_load_state('networkidle')
            value.goto(ROOT+('/clubs/'+club+'/home' if club else '/'))
            expect(value.locator('.dashboard-agenda').first).to_be_visible()
        def report_dates(value=owner_page):
            screen = value.locator('.dashboard-report')
            screen.get_by_label('Период главной', exact=True).select_option('custom')
            screen.get_by_label('С даты', exact=True).fill(start)
            screen.get_by_label('По дату', exact=True).fill(until)
            screen.locator('.dashboard-date-editor > summary').click()
            return screen
        def metric(key, value):
            expect(owner_page.locator('.dashboard-report [data-metric="'+key+'"]').first).to_have_text(value)
        def counter(value, key, number):
            expect(value.locator('[data-counter="'+key+'"]').first).to_have_text(str(number))
        home(owner_page); screen = report_dates(); metric('closed', '2'); metric('present', '2'); metric('absent', '3'); metric('percentage', '40,0%')
        panel = api(owner_page, 'GET', '/api/dashboard')
        assert {c['id']: c['today'] for c in panel['clubs']} == {a['id']: dates[0], b['id']: dates[1]}
        assert panel['counters'] == {'participants': 5, 'groups': 2, 'today': 5, 'open': 2}, panel
        assert sql("SELECT count(*) FROM pride.club_memberships WHERE archived_at IS NULL") == '5'
        assert sql("SELECT count(*) FROM pride.training_sessions WHERE status='open'") == '2'
        assert panel['next']['id'] == old and panel['next']['roster'] == 3 and panel['next']['marked'] == 1
        assert sum(s['status'] == 'cancelled' for s in panel['today']) == 1
        home(admin_page, a['id']); counter(admin_page, 'participants', 3); counter(admin_page, 'groups', 1); counter(admin_page, 'today', 3); counter(admin_page, 'open', 1)
        assert api(admin_page, 'GET', '/api/dashboard')['mode'] == 'administrator'
        api(admin_page, 'GET', '/api/dashboard?clubId='+b['id'], status=404)
        expect(admin_page.locator('.dashboard-next > a')).to_have_attribute('href', __import__('re').compile('/sessions/'+old))
        home(ta, a['id']); counter(ta, 'groups', 1); counter(ta, 'open', 1)
        expect(ta.locator('[data-counter="participants"]')).to_have_count(0); expect(ta.locator('.dashboard-changes')).to_have_count(0); expect(ta.locator('.dashboard-report')).to_have_count(0)
        assert api(ta, 'GET', '/api/dashboard')['counters']['participants'] is None
        api(ta, 'GET', '/api/audit/events', status=403); api(ta, 'GET', '/api/reports/attendance?from='+start+'&until='+until, status=403)
        # Capture all requested widths together before mutations; ordinary mobile
        # heights prove that the real main action fits above the bottom bar.
        shots = REPO/'.impeccable/review/rd05'; shots.mkdir(parents=True, exist_ok=True)
        visual_failures = []
        for role, value in [('owner', owner_page), ('administrator', admin_page), ('trainer', ta)]:
            for width in [1440, 1280, 768, 390, 320]:
                height = 900 if width >= 768 else 844 if width == 390 else 780
                value.set_viewport_size({'width': width, 'height': height})
                value.evaluate('window.scrollTo(0,0)')
                expect(value.locator('.dashboard-agenda').first).to_be_visible()
                value.screenshot(path=str(shots/f'{ENGINE}-{role}-{width}.png'), full_page=True)
                if not value.evaluate('document.documentElement.scrollWidth <= innerWidth'):
                    visual_failures.append((role, width, 'overflow'))
                small = value.locator('.dashboard').evaluate('''root=>[...root.querySelectorAll('a,button,input,select')].filter(e=>e.getClientRects().length&&!e.disabled).filter(e=>e.getBoundingClientRect().height<43.5).map(e=>e.textContent||e.getAttribute('aria-label'))''')
                if small:
                    visual_failures.append((role, width, small))
                if role != 'owner' and width <= 390:
                    box = value.locator('.dashboard-next > .dashboard-primary').bounding_box()
                    navigation = value.locator('.bottom-navigation').bounding_box()
                    if not box or not navigation or box['y']+box['height'] > navigation['y']-4:
                        visual_failures.append((role, width, box, navigation, 'main action covered by navigation'))
                if width <= 768:
                    assert value.locator('.dashboard').evaluate('root=>Boolean(root.querySelector(".dashboard-side").compareDocumentPosition(root.querySelector(".dashboard-groups")) & Node.DOCUMENT_POSITION_FOLLOWING)'), (role, width, 'keyboard order differs from visual order')
            value.set_viewport_size({'width': 1280, 'height': 900})
        assert not visual_failures, visual_failures
        # Group/branch/session IDs survive real navigation, reload and history.
        admin_page.locator('.dashboard-group-list [data-group-id="'+ga['id']+'"]').click()
        expect(admin_page).to_have_url(__import__('re').compile('detail='+ga['id']))
        expect(admin_page.get_by_role('heading', name=ga['name'], exact=True)).to_be_visible()
        admin_page.go_back(); expect(admin_page.locator('.dashboard-next')).to_be_visible()
        main_action = admin_page.locator('.dashboard-next > a')
        main_action.focus(); expect(main_action).to_be_focused()
        assert main_action.evaluate('e=>parseFloat(getComputedStyle(e).outlineWidth) >= 2')
        main_action.press('Enter')
        expect(admin_page).to_have_url(__import__('re').compile('/sessions/'+old))
        expect(admin_page.locator('.admin-journal')).to_be_visible()
        expect(admin_page.locator('.admin-records > li')).to_have_count(3)
        admin_page.reload(); expect(admin_page.locator('.admin-records > li')).to_have_count(3)
        home(admin_page, a['id'])
        admin_page.get_by_role('link', name='Добавить участника · '+a['name'], exact=True).click()
        expect(admin_page.get_by_role('dialog', name='Новый участник', exact=True)).to_be_visible()
        assert 'detail=new' in admin_page.url
        form = admin_page.get_by_role('dialog', name='Новый участник', exact=True)
        form.get_by_label('ФИО участника', exact=True).fill('Added Synthetic '+suffix)
        form.get_by_label('Дата рождения', exact=True).fill('1990-01-01'); form.get_by_label('Дата вступления', exact=True).fill(dates[0])
        form.get_by_label('Номер телефона', exact=True).fill('+375291234567')
        form.get_by_label('Начальная группа (необязательно)', exact=True).select_option(ga['id'])
        form.get_by_label('Дата включения в группу', exact=True).fill(dates[0]); form.get_by_role('button', name='Сохранить участника', exact=True).click()
        expect(form).to_have_count(0)
        expect(admin_page.locator('.participant-rows > li')).to_have_count(4)
        expect(admin_page.locator('.membership-history .registry-list > li')).to_have_count(1)
        home(admin_page, a['id']); counter(admin_page, 'participants', 4)
        assert sql(f"SELECT count(*) FROM pride.club_memberships WHERE club_id='{a['id']}' AND archived_at IS NULL") == '4'
        assert api(admin_page, 'GET', '/api/dashboard?clubId='+a['id'])['next']['roster'] == 3
        # A genuine correction refreshes the existing SQL report, without a second formula.
        record = next(r for r in ja['records'] if r['state'] == 'absent')
        api(admin_page, 'PUT', '/api/clubs/'+a['id']+'/journals/'+sa+'/records/'+record['id'], {'version': record['version'], 'state': 'present', 'absenceReason': None, 'changeReason': 'Synthetic dashboard correction'})
        owner_page.evaluate("window.dispatchEvent(new Event('attendance-changed'))")
        metric('present', '3'); metric('absent', '2'); metric('percentage', '60,0%')
        assert sql("SELECT count(*) FILTER(WHERE r.state='present')||'/'||count(*) FROM pride.attendance_records r JOIN pride.training_sessions s ON s.id=r.training_session_id WHERE s.status='closed'") == '3/5'
        # Open the exact trainer journal and make a real sequential mark/close.
        ta.locator('.dashboard-next > a').click(); expect(ta).to_have_url(__import__('re').compile('/sessions/'+old))
        ta.get_by_role('button', name='Присутствует', exact=True).click()
        expect(ta.get_by_role('status').filter(has_text='Сохраняем')).to_have_count(0)
        ta.get_by_role('button', name='Завершить журнал', exact=True).click()
        ta.get_by_role('dialog').get_by_role('button', name='Подтвердить завершение', exact=True).click()
        expect(ta.get_by_role('dialog')).to_have_count(0)
        home(ta, a['id']); counter(ta, 'open', 0)
        expect(ta.locator('.dashboard-next > a')).to_have_attribute('href', __import__('re').compile('/sessions/'+planned))
        assert sql(f"SELECT status FROM pride.training_sessions WHERE id='{old}'") == 'closed'
        owner_page.evaluate("window.dispatchEvent(new Event('attendance-changed'))")
        # Ledger now includes the previously open three-person historical session.
        assert sql("SELECT count(*) FILTER(WHERE r.state='present')||'/'||count(*) FROM pride.attendance_records r JOIN pride.training_sessions s ON s.id=r.training_session_id WHERE s.status='closed'") == '5/8'
        metric('percentage', '62,5%')
        # Archive through UI; current counts change but the frozen ledger does not.
        admin_page.goto(ROOT+'/clubs/'+a['id']+'/participants?detail='+members[0][0]['id'])
        admin_page.get_by_role('button', name='В архив', exact=True).click()
        # A click dispatches the mutation; navigation must wait for its commit
        # and the UI acknowledgement, otherwise a fast reload can abort it.
        archive_url = ROOT+'/api/clubs/'+a['id']+'/participants/'+members[0][0]['id']+'/archive'
        with admin_page.expect_response(lambda response: response.url == archive_url and response.request.method == 'POST') as archive_response:
            admin_page.get_by_role('button', name='Подтвердить архивирование', exact=True).click()
        assert archive_response.value.status == 200
        expect(admin_page.get_by_role('status').filter(has_text='Карточка в архиве. История сохранена.')).to_be_visible()
        expect(admin_page.locator('.participant-rows > li')).to_have_count(3)
        expect(admin_page.locator('.membership-history .registry-list > li')).to_have_count(1)
        assert sql(f"SELECT count(*) FROM pride.club_memberships WHERE club_id='{a['id']}' AND archived_at IS NULL") == '3'
        home(admin_page, a['id']); counter(admin_page, 'participants', 3)
        owner_page.evaluate("window.dispatchEvent(new Event('attendance-changed'))"); metric('percentage', '62,5%')
        # Transport failures keep mounted data and expose an error for each block.
        agenda_before = owner_page.locator('.dashboard-agenda').first.element_handle()
        owner_page.route('**/api/dashboard?*', lambda route: route.abort())
        owner_page.get_by_role('button', name='Обновить главную', exact=True).click()
        expect(owner_page.get_by_role('button', name='Повторить загрузку дня', exact=True)).to_be_visible(); metric('percentage', '62,5%')
        assert agenda_before.evaluate('node => node.isConnected'), 'Network error unmounted the agenda'
        owner_page.unroute('**/api/dashboard?*'); owner_page.get_by_role('button', name='Повторить загрузку дня', exact=True).click(); expect(owner_page.locator('.dashboard-agenda').first).to_be_visible()
        totals_before = screen.locator('.report-totals').element_handle()
        owner_page.route('**/api/reports/attendance?*', lambda route: route.abort())
        owner_page.get_by_role('button', name='Обновить главную', exact=True).click()
        expect(screen.get_by_role('button', name='Повторить загрузку обзора', exact=True)).to_be_visible()
        metric('percentage', '62,5%')
        assert totals_before.evaluate('node => node.isConnected'), 'Network error unmounted the overview'
        expect(owner_page.locator('.dashboard-agenda').first).to_be_visible()
        owner_page.unroute('**/api/reports/attendance?*'); screen.get_by_role('button', name='Повторить загрузку обзора', exact=True).click(); metric('percentage', '62,5%')
        changes = owner_page.locator('.dashboard-changes')
        changes_before = changes.locator('ol').element_handle()
        owner_page.route('**/api/audit/events?*', lambda route: route.abort()); owner_page.get_by_role('button', name='Обновить главную', exact=True).click()
        expect(changes.get_by_role('button', name='Повторить загрузку изменений', exact=True)).to_be_visible()
        assert changes_before.evaluate('node => node.isConnected'), 'Network error unmounted recent changes'
        metric('percentage', '62,5%')
        owner_page.unroute('**/api/audit/events?*'); changes.get_by_role('button', name='Повторить загрузку изменений', exact=True).click(); expect(changes.locator('li').first).to_be_visible()
        assert '+375291234567' not in changes.inner_text() and 'Synthetic dashboard correction' not in changes.inner_text()
        # A closed empty group uses the report's null denominator, not a fallback.
        empty = api(owner_page, 'POST', '/api/clubs/'+a['id']+'/groups', {'name': 'Empty '+suffix}, 201)
        se = new_session(0, empty, '11:00'); journal(0, se, close=True)
        owner_page.evaluate("window.dispatchEvent(new Event('attendance-changed'))")
        screen.get_by_label('Группа отчёта', exact=True).select_option(empty['id']); metric('closed', '1'); metric('percentage', '—')
        screen.get_by_role('button', name='Вернуться к контексту главной', exact=True).click(); metric('percentage', '62,5%')
        # Multi-role administrator keeps the management home; actual assignment
        # alone enables the trainer action, with live role checks at the API.
        sql(f"INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES('{admin['id']}','{a['id']}','trainer')")
        planned_row = {'version': int(sql(f"SELECT version FROM pride.training_sessions WHERE id='{planned}'"))}
        api(owner_page, 'POST', '/api/clubs/'+a['id']+'/sessions/'+planned+'/replace-trainer', {'version': planned_row['version'], 'trainerId': admin['id']})
        home(admin_page, a['id']); expect(admin_page.locator('[data-dashboard-mode="administrator"]')).to_be_visible()
        admin_page.locator('.dashboard-next > a').click()
        admin_page.get_by_role('button', name='Подтвердить открытие журнала', exact=True).click()
        expect(admin_page.get_by_role('button', name='Присутствует', exact=True)).to_be_visible()
        home(admin_page, a['id']); expect(admin_page.locator('.dashboard-next > a')).to_have_text('Продолжить отметки')
        def hold(value, target):
            value.evaluate('''target=>{const original=window.fetch;window.held=[];window.restore=()=>window.fetch=original;
              window.fetch=async(...args)=>{const response=await original(...args);
                if(String(args[0]).includes(target))await new Promise(resolve=>window.held.push(resolve));return response;};}''', target)
        def release(value):
            value.evaluate('window.restore();window.held.forEach(resolve=>resolve())')
        hold(admin_page, '/api/dashboard?'); admin_page.get_by_role('button', name='Обновить главную', exact=True).click(); admin_page.wait_for_function('window.held.length > 0')
        sql(f"DELETE FROM pride.staff_club_roles WHERE staff_user_id='{admin['id']}' AND role='administrator'")
        admin_page.evaluate("window.dispatchEvent(new Event('focus'))")
        expect(admin_page.locator('[data-dashboard-mode="trainer"]')).to_be_visible(); release(admin_page)
        expect(admin_page.locator('[data-counter="participants"]')).to_have_count(0); expect(admin_page.locator('.dashboard-changes')).to_have_count(0)
        # Fully revoked trainer, foreign branch and late response after logout.
        home(ta, a['id']); hold(ta, '/api/dashboard?'); ta.get_by_role('button', name='Обновить главную', exact=True).click(); ta.wait_for_function('window.held.length > 0')
        sql(f"DELETE FROM pride.staff_club_roles WHERE staff_user_id='{trainers[0]['id']}'")
        ta.evaluate("window.dispatchEvent(new Event('focus'))"); expect(ta.get_by_role('heading', name='Раздел или филиал недоступен', exact=True)).to_be_visible(); release(ta); expect(ta.locator('.dashboard')).to_have_count(0)
        ta.goto(ROOT); expect(ta.get_by_role('heading', name='Нет доступных назначений', exact=True)).to_be_visible()
        home(owner_page); report_dates(); metric('percentage', '62,5%')
        hold(owner_page, '/api/reports/attendance?'); owner_page.get_by_role('button', name='Обновить главную', exact=True).click(); owner_page.wait_for_function('window.held.length > 0')
        signout(owner_page); expect(owner_page.get_by_label('Логин', exact=True)).to_be_visible(); release(owner_page)
        expect(owner_page.locator('.dashboard')).to_have_count(0); assert not errors, errors
        print(json.dumps({'engine': ENGINE, 'version': browser.version, 'result': 'PASS', 'ledger': '2/5=40%; correction3/5=60%; close5/8=62.5%; empty closed null', 'checks': 'two 24-hour-separated local dates; role scopes/counters/frozen roster; exact group/journal/create links; real mark/close/correction/archive; independent failures+retry; multi-role actual trainer; revoke/late replies/logout; 1440/1280/768/390/320; touch targets/main action'}, ensure_ascii=False))
        browser.close()
finally:
    STATIC_TEMP.cleanup()
    subprocess.run(['docker', 'rm', '-f', APP, DB], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    subprocess.run(['docker', 'network', 'rm', NET], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

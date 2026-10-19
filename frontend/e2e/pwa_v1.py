from runtime_ui import settle_shell, assert_page_targets
from runtime_ui import signout
"""Disposable UI -> Go -> PostgreSQL PWA/browser QA; docs/testing-v1.md."""
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
import urllib.request
from datetime import datetime, timedelta
from zoneinfo import ZoneInfo
from playwright.sync_api import sync_playwright, expect
from redesign_accessibility import contrast_audit
from redesign_shell import check_shell
from redesign_registry import check_registry
from shadcn_runtime import check_runtime

REPO = Path(__file__).resolve().parents[2]
ROOT = 'http://127.0.0.1:18081'
PASSWORD = 'synthetic-browser-password'
PREFIX = 'pwa161-' + uuid.uuid4().hex[:8]
DB, APP, NETWORK = PREFIX+'-db', PREFIX+'-app', PREFIX+'-net'
IMAGE = os.environ.get('PWA_TEST_IMAGE', 'judeoslite:ci')
assert IMAGE.startswith('judeoslite:')
ENGINE = os.environ.get('PWA_BROWSER', 'chromium')
assert ENGINE in ('chromium', 'webkit')

def command(*args, **kwargs):
    return subprocess.check_output(args, text=True, **kwargs).strip()

def sql(statement):
    return command('docker', 'exec', DB, 'psql', '-U', 'test', '-d', 'browser161', '-Atc', statement)

def readable(directory):
    directory.chmod(0o755)
    for path in directory.rglob('*'):
        path.chmod(0o755 if path.is_dir() else 0o644)

def bootstrap(env):
    master, slave = pty.openpty()
    process = subprocess.Popen(['docker', 'run', '--rm', '-it', '--network', NETWORK, *env, IMAGE,
                                '/app/create-owner', '--login', 'owner', '--name', 'Synthetic Owner'],
                               stdin=slave, stdout=slave, stderr=slave)
    os.close(slave)
    output = b''
    for prompt in (b'Password (', b'Repeat password:'):
        deadline = time.monotonic()+30
        while prompt not in output:
            assert time.monotonic() < deadline, output
            if select.select([master], [], [], 1)[0]:
                output += os.read(master, 4096)
        os.write(master, (PASSWORD+'\n').encode())
        output = b''
    assert process.wait(timeout=30) == 0
    os.close(master)

with tempfile.TemporaryDirectory(prefix='pwa161-') as temp:
    temp = Path(temp)
    static = temp/'static'
    shutil.copytree(REPO/'frontend/dist', static)
    # A genuine second build changes public HTML, and therefore shell/cache/SW version.
    source = temp/'frontend'
    shutil.copytree(REPO/'frontend', source, ignore=shutil.ignore_patterns('node_modules', 'dist'))
    (source/'node_modules').symlink_to(REPO/'frontend/node_modules', target_is_directory=True)
    (temp/'design-system').symlink_to(REPO/'design-system', target_is_directory=True)
    index = source/'index.html'
    index.write_text(index.read_text().replace('PRIDE — рабочее пространство</title>', 'PRIDE — рабочее пространство · B</title>'))
    command('npm', 'run', 'build', cwd=source)
    env = ['-e', f'DATABASE_URL=postgres://test:test-only@{DB}:5432/browser161?sslmode=disable']
    try:
        command('docker', 'network', 'create', NETWORK)
        command('docker', 'run', '-d', '--name', DB, '--network', NETWORK,
                '-e', 'POSTGRES_USER=test', '-e', 'POSTGRES_PASSWORD=test-only', '-e', 'POSTGRES_DB=browser161', 'postgres:18.6-bookworm')
        for _ in range(30):
            # The image's temporary init server accepts Unix sockets before the
            # final server opens TCP. Migrate connects over TCP from another
            # container, so socket readiness can race with initialization.
            if subprocess.run(['docker', 'exec', DB, 'pg_isready', '-h', '127.0.0.1', '-U', 'test', '-d', 'browser161'], stdout=subprocess.DEVNULL).returncode == 0:
                break
            time.sleep(1)
        else:
            raise AssertionError('PostgreSQL not ready')
        command('docker', 'run', '--rm', '--network', NETWORK, *env, IMAGE, '/app/migrate')
        bootstrap(env)
        # TemporaryDirectory is private by default; production image runs as UID 65532.
        temp.chmod(0o755)
        readable(static)
        command('docker', 'create', '--name', APP, '--network', NETWORK, *env,
                '-e', f'APP_ORIGIN={ROOT}', '-e', 'AUTH_INSECURE_LOCAL=true',
                '-p', '127.0.0.1:18081:8080', IMAGE)
        command('docker', 'cp', f'{static}/.', f'{APP}:/app/static')
        command('docker', 'start', APP)
        for _ in range(30):
            try:
                with urllib.request.urlopen(ROOT+'/api/readyz', timeout=2) as response:
                    if response.status == 200:
                        break
            except OSError:
                pass
            time.sleep(1)
        else:
            raise AssertionError(command('docker', 'logs', APP))
        with sync_playwright() as p:
            path = (os.environ.get('CHROMIUM_PATH') or ('/usr/bin/chromium' if Path('/usr/bin/chromium').exists() else None)) if ENGINE == 'chromium' else None
            browser_type = getattr(p, ENGINE)
            browser = browser_type.launch(headless=True, executable_path=path)
            errors = []
            contrast_floor = {4.5: float('inf'), 3: float('inf')}
            def page(context):
                value = context.new_page()
                value.pwa_api_events = []
                def page_error(error):
                    errors.append(str(error))
                    print(json.dumps({'engine': ENGINE, 'check': 'unexpected-page-error',
                        'url': value.url, 'message': str(error), 'stack': error.stack,
                        'api': value.pwa_api_events[-12:]}, ensure_ascii=False), flush=True)
                value.on('pageerror', page_error)
                value.on('request', lambda request: value.pwa_api_events.append({'event': 'request', 'time': time.monotonic(), 'method': request.method, 'path': request.url.removeprefix(ROOT)}) if request.url.startswith(ROOT+'/api/') else None)
                value.on('response', lambda response: value.pwa_api_events.append({'event': 'response', 'time': time.monotonic(), 'status': response.status, 'path': response.url.removeprefix(ROOT)}) if response.url.startswith(ROOT+'/api/') else None)
                value.on('requestfailed', lambda request: value.pwa_api_events.append({'event': 'failed', 'time': time.monotonic(), 'failure': request.failure, 'path': request.url.removeprefix(ROOT)}) if request.url.startswith(ROOT+'/api/') else None)
                return value
            def signin(value, login='owner', navigate=True):
                if navigate:
                    value.goto(ROOT)
                # Settle the initial auth read before typing into controlled fields.
                value.wait_for_load_state('networkidle')
                value.get_by_label('Логин', exact=True).fill(login)
                value.get_by_label('Пароль', exact=True).fill(PASSWORD)
                expect(value.get_by_label('Логин', exact=True)).to_have_value(login)
                value.get_by_role('button', name='Войти', exact=True).click()
                expect(value.get_by_role('button', name='Меню профиля', exact=True)).to_be_visible()
                value.wait_for_load_state('networkidle')
            def api(value, method, path, data=None):
                result = value.evaluate('''async ([method,path,data]) => {
                  const me = await (await fetch('/api/auth/me', {cache:'no-store'})).json();
                  const r = await fetch(path, {method,cache:'no-store',headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrfToken},body:method==='GET'?undefined:JSON.stringify(data)});
                  return {status:r.status,body:await r.json(),cache:r.headers.get('cache-control')};
                }''', [method, path, data])
                assert result['status'] in (200, 201) and result['cache'] == 'no-store', result
                return result['body']
            def wait_for_route(value, intercepted):
                # DOM busy state can precede delivery of the network callback in WebKit.
                deadline = time.monotonic()+5
                while not intercepted:
                    assert time.monotonic() < deadline, 'Expected an intercepted request'
                    value.wait_for_timeout(10)
            def hold_response(value, target, method):
                # Hold the actual server response before the API client can process it.
                # Unlike routing, this also works with WebKit's controlled pages.
                value.evaluate('''([target,method])=>{
                  const original=window.fetch; window.pwaTestHeld=[];
                  window.pwaTestRestore=()=>{window.fetch=original};
                  window.fetch=async (...args)=>{
                    const response=await original(...args);
                    const url=new URL(typeof args[0]==='string'?args[0]:args[0].url,location.href);
                    if(url.pathname===target && (args[1]?.method||'GET')===method)
                      await new Promise(resolve=>window.pwaTestHeld.push(resolve));
                    return response;
                  };
                }''', [target, method])
            def wait_for_response(value):
                value.wait_for_function('window.pwaTestHeld.length === 1')
            def release_response(value):
                value.evaluate('window.pwaTestRestore();window.pwaTestHeld.forEach(resolve=>resolve());delete window.pwaTestHeld;delete window.pwaTestRestore')
            def fault_fetch(value, target, after_commit=False):
                # WebKit can bypass Playwright routes on SW-controlled pages.
                # Intercept delivery at fetch, retaining the real server call
                # in the after-commit case and proving the fault was exercised.
                value.evaluate('''([target,afterCommit])=>{
                  const original=window.fetch; window.pwaFaultCalls=0;
                  window.pwaFaultAfterCommit=afterCommit; window.pwaFaultStatus=null;
                  window.pwaFaultRestore=()=>{window.fetch=original};
                  window.fetch=async(...args)=>{
                    const url=new URL(typeof args[0]==='string'?args[0]:args[0].url,location.href);
                    const match=url.pathname===target && ['POST','PUT'].includes(args[1]?.method);
                    if(match) {
                      window.pwaFaultCalls++;
                      if(!afterCommit)throw new TypeError('Synthetic pre-send network failure');
                    }
                    const response=await original(...args);
                    if(match) {
                      window.pwaFaultStatus=response.status;
                      if(!response.ok)throw new Error('Expected successful committed response');
                      await response.clone().arrayBuffer();
                      throw new TypeError('Synthetic committed response loss');
                    }
                    return response;
                  };
                }''', [target, after_commit])
            def restore_fault(value):
                assert value.evaluate('window.pwaFaultCalls') == 1, 'Network fault was not exercised exactly once'
                after_commit, status = value.evaluate('[window.pwaFaultAfterCommit,window.pwaFaultStatus]')
                assert status == (200 if after_commit else None), ('Unexpected response before synthetic loss', status)
                value.evaluate('window.pwaFaultRestore();delete window.pwaFaultRestore')
            def open_section(value, name):
                value.bring_to_front()
                expect(value.get_by_role('button', name='Меню профиля', exact=True)).to_be_visible()
                sidebar = value.get_by_role('navigation', name='Разделы филиала')
                if value.viewport_size['width'] >= 768:
                    expect(sidebar).to_be_visible()
                    sidebar.get_by_role('link', name=name, exact=True).click()
                else:
                    bottom = value.get_by_role('navigation', name='Основная навигация')
                    link = bottom.get_by_role('link', name=name, exact=True)
                    if link.count():
                        link.click()
                    else:
                        bottom.get_by_role('button', name='Ещё', exact=True).click()
                        value.get_by_role('dialog').get_by_role('link', name=name, exact=True).click()
                expect(value.locator('main h1')).to_have_text(name)
            def dimensions(value, role):
                for width in (320, 390, 768, 900, 1280, 1440):
                    value.set_viewport_size({'width': width, 'height': 900})
                    settle_shell(value, width)
                    if not value.evaluate('document.documentElement.scrollWidth <= innerWidth'):
                        value.screenshot(path=f'/tmp/rd08-overflow-{ENGINE}-{role}-{width}.png')
                        overflow=value.evaluate('''()=>[...document.querySelectorAll('body *')].filter(e=>{const r=e.getBoundingClientRect();return r.width&&r.right>innerWidth+1&&!e.closest('[data-slot="sidebar"]')}).slice(0,12).map(e=>({tag:e.tagName,cls:e.className,text:e.textContent.slice(0,60),rect:e.getBoundingClientRect().toJSON()}))''')
                        raise AssertionError((role,width,overflow))
                    contrast = contrast_audit(value)
                    assert not contrast['failures'] and not contrast['unmeasured'], (role, width, contrast)
                    for pair in contrast['pairs']:
                        minimum = pair['minimum']
                        contrast_floor[minimum] = min(contrast_floor[minimum], pair['ratio'])
                    assert_page_targets(value, width)
                    value.screenshot(path=f'/tmp/pwa161-{ENGINE}-{role}-{width}.png')
            def cache_check(value, expected_names=None):
                caches = value.evaluate('''async () => {
                  const result = [];
                  for (const name of await caches.keys()) {
                    const cache = await caches.open(name);
                    for (const request of await cache.keys()) {
                      const response = await cache.match(request);
                      result.push({name,path:new URL(request.url).pathname,type:response.headers.get('content-type'),text:request.url.endsWith('.html')?await response.text():''});
                    }
                  }
                  return result;
                }''')
                assert caches and all(v['name'].startswith('pride-shell-') and v['path'].startswith('/assets/') for v in caches), caches
                names = {v['name'] for v in caches}
                if expected_names is None:
                    assert len(names) == 1, caches
                else:
                    assert names == expected_names, caches
                assert not any(v['type'] and 'json' in v['type'] for v in caches), caches
                assert all('Synthetic' not in v['text'] and 'csrfToken' not in v['text'] for v in caches)
                return caches[0]['name']
            owner_context = browser_type.launch_persistent_context(str(temp/'profile'), headless=True, executable_path=path, viewport={'width': 320, 'height': 900})
            owner = page(owner_context)
            owner.goto(ROOT)
            expect(owner.get_by_role('heading', name='Вход в аккаунт')).to_be_visible()
            dimensions(owner, 'login')
            owner.locator('.install-help summary').click()
            expect(owner.locator('.install-help')).to_contain_text('iPhone / Safari')
            expect(owner.locator('.install-help')).to_contain_text('Android / Chrome')
            owner.locator('.install-help summary').click()
            signin(owner)
            owner.wait_for_function('navigator.serviceWorker.controller !== null')
            parsed = owner.evaluate("fetch('/manifest.webmanifest').then(r=>r.json())")
            assert parsed['display'] == 'standalone' and parsed['start_url'] == '/' and parsed['scope'] == '/'
            for icon in parsed['icons']:
                size = int(icon['sizes'].split('x')[0])
                assert owner.evaluate('''async ([src,size])=>{const img=new Image();img.src=src;await img.decode();return img.naturalWidth===size&&img.naturalHeight===size;}''', [icon['src'], size])
            if ENGINE == 'chromium':
                cdp = owner_context.new_cdp_session(owner)
                manifest = cdp.send('Page.getAppManifest')
                assert not manifest.get('errors'), manifest
                installability = cdp.send('Page.getInstallabilityErrors')['installabilityErrors']
                assert not installability, installability
            # Browser signals/standalone UI, distinct from real-device installation.
            owner.evaluate("const event=new Event('beforeinstallprompt',{cancelable:true});event.prompt=async()=>{};event.userChoice=Promise.resolve({outcome:'dismissed'});window.dispatchEvent(event)")
            owner.locator('.install-help summary').click()
            owner.get_by_role('button', name='Установить приложение', exact=True).click()
            expect(owner.get_by_role('button', name='Установить приложение', exact=True)).to_have_count(0)
            owner.locator('.install-help summary').click()
            standalone_context = browser.new_context()
            standalone_context.add_init_script("Object.defineProperty(navigator,'standalone',{value:true})")
            standalone_page = page(standalone_context)
            standalone_page.goto(ROOT+'/clubs/00000000-0000-0000-0000-000000000000')
            expect(standalone_page.get_by_label('Логин', exact=True)).to_be_visible()
            expect(standalone_page.locator('.install-help')).to_have_count(0)
            standalone_context.close()
            club = api(owner, 'POST', '/api/clubs', {'name': 'Synthetic PWA club', 'timezone': 'Europe/Minsk'})
            base = '/api/clubs/'+club['id']
            api(owner, 'POST', '/api/staff', {'login': 'admin', 'displayName': 'Synthetic Admin', 'password': PASSWORD, 'clubIds': [club['id']]})
            admin_context = browser.new_context(viewport={'width': 320, 'height': 900})
            admin = page(admin_context); signin(admin, 'admin')
            api_events = []
            admin.on('response', lambda response: api_events.append({'status': response.status, 'path': response.url.removeprefix(ROOT)}) if response.url.startswith(ROOT+'/api/') else None)
            admin.on('requestfailed', lambda request: api_events.append({'failure': request.failure, 'path': request.url.removeprefix(ROOT)}) if request.url.startswith(ROOT+'/api/') else None)
            def diagnose_admin(check):
                print(json.dumps({'engine': ENGINE, 'check': check, 'url': admin.url, 'api': api_events[-30:], 'body': admin.locator('body').inner_text()[:6000], 'errors': errors}, ensure_ascii=False), flush=True)
            group_reads = []
            group_url = ROOT+base+'/groups?state=all'
            admin.on('response', lambda response: group_reads.append({'status': response.status}) if response.url == group_url else None)
            admin.on('requestfailed', lambda request: group_reads.append({'failure': request.failure}) if request.url == group_url else None)
            admin.get_by_label('Филиал', exact=True).select_option(club['id'])
            open_section(admin, 'Занятия')
            expect(admin.locator('#schedule-title')).to_be_visible()
            open_section(admin, 'Участники')
            expect(admin.locator('#registry-title')).to_be_visible()
            open_section(admin, 'Группы')
            groups = admin.locator('.groups')
            groups.get_by_role('button', name='Создать группу', exact=True).click()
            groups.get_by_label('Название группы', exact=True).fill('Synthetic PWA group')
            groups.get_by_role('button', name='Сохранить группу', exact=True).click()
            expect(groups).to_contain_text('Группа сохранена.')
            group = api(admin, 'GET', base+'/groups')[0]
            open_section(admin, 'Сотрудники')
            staff = admin.locator('.staff-management')
            staff.get_by_role('button', name='Добавить тренера', exact=True).click()
            staff.get_by_label('Имя тренера', exact=True).fill('Synthetic Trainer')
            staff.get_by_label('Логин тренера', exact=True).fill('trainer')
            staff.get_by_label('Первоначальный пароль', exact=True).fill(PASSWORD)
            staff.get_by_label('Повторите пароль', exact=True).fill(PASSWORD)
            staff.get_by_role('button', name='Создать тренера', exact=True).click()
            expect(staff).to_contain_text('Тренер создан.')
            trainer = api(admin, 'GET', base+'/trainers?available=true')[0]
            open_section(admin, 'Участники')
            registry = admin.locator('.registry').filter(has=admin.locator('#registry-title'))
            def select_initial_group():
                try:
                    registry.get_by_label('Начальная группа (необязательно)', exact=True).select_option(group['id'])
                except Exception:
                    # This fixture uses only disposable synthetic data. Keep failure
                    # evidence instead of silently retrying or forcing a disabled select.
                    state = registry.evaluate('''el=>({
                      alerts:[...el.querySelectorAll('[role="alert"]')].map(v=>v.textContent),
                      statuses:[...el.querySelectorAll('[role="status"]')].map(v=>v.textContent),
                      selects:[...el.querySelectorAll('select')].map(v=>({id:v.id,disabled:v.matches(':disabled'),options:v.options.length})),
                      busy:el.querySelector('form[aria-busy="true"]')!==null
                    })''')
                    print(json.dumps({'engine': ENGINE, 'check': 'initial-group-load', 'groupReads': group_reads[-12:], 'ui': state}, ensure_ascii=False), flush=True)
                    diagnose_admin('initial-group-load')
                    raise
            registry.get_by_role('button', name='Добавить участника', exact=True).click()
            registry.get_by_label('ФИО участника', exact=True).fill('Synthetic Participant A')
            registry.get_by_label('Дата рождения', exact=True).fill('2010-01-01')
            registry.get_by_label('Дата вступления', exact=True).fill('2020-01-01')
            registry.get_by_label('Номер телефона', exact=True).fill('+375291234567')
            select_initial_group()
            registry.get_by_label('Дата включения в группу', exact=True).fill('2020-01-01')
            registry.get_by_role('button', name='Сохранить участника', exact=True).click()
            expect(registry).to_contain_text('Участник добавлен.')
            dimensions(admin, 'admin-card')
            registry.get_by_role('button', name='Закрыть карточку', exact=True).click()
            # Five synthetic cards, all entered by the administrator through UI.
            for letter in 'BCDE':
                registry.get_by_role('button', name='Добавить участника', exact=True).click()
                registry.get_by_label('ФИО участника', exact=True).fill('Synthetic Participant '+letter)
                registry.get_by_label('Дата рождения', exact=True).fill('2010-01-01')
                registry.get_by_label('Дата вступления', exact=True).fill('2020-01-01')
                registry.get_by_label('Номер телефона', exact=True).fill('+375291234567')
                select_initial_group()
                registry.get_by_label('Дата включения в группу', exact=True).fill('2020-01-01')
                registry.get_by_role('button', name='Сохранить участника', exact=True).click()
                expect(registry).to_contain_text('Участник добавлен.')
                registry.get_by_role('button', name='Закрыть карточку', exact=True).click()
            open_section(admin, 'Занятия')
            schedule = admin.locator('.schedule')
            # Wait for the refresh initiated by this action before opening an
            # editor; the old enabled button can precede the loading effect.
            def schedule_read(response):
                return response.request.method == 'GET' and response.url == ROOT+base+'/schedule' and response.status == 200
            with admin.expect_response(schedule_read):
                schedule.get_by_role('button', name='Обновить расписание', exact=True).click()
            today = datetime.now(ZoneInfo('Europe/Minsk')).date()
            tomorrow = today+timedelta(days=1)
            until = tomorrow+timedelta(days=8)
            schedule_counts = {}
            for mode, hour in (('weekly', '18:00'), ('day_parity', '19:00'), ('date_list', '20:00'), ('once', '21:00')):
                try:
                    schedule.get_by_role('button', name='Создать расписание', exact=True).click()
                    schedule.get_by_label('Группа расписания', exact=True).select_option(group['id'])
                    schedule.get_by_label('Тренер занятия', exact=True).select_option(trainer['id'])
                    schedule.get_by_label('Режим расписания', exact=True).select_option(mode)
                    schedule.get_by_label('Время начала', exact=True).fill(hour)
                except Exception:
                    diagnose_admin('schedule-form-'+mode)
                    raise
                if mode in ('weekly', 'day_parity'):
                    schedule.get_by_label('Начало действия', exact=True).fill(str(tomorrow))
                    schedule.get_by_label('Конец действия (не включён, необязательно)', exact=True).fill(str(until))
                    if mode == 'weekly':
                        for i, label in enumerate(('Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс')):
                            schedule.get_by_label(label, exact=True).set_checked(i == tomorrow.weekday())
                        expected_dates = [str(tomorrow), str(tomorrow+timedelta(days=7))]
                    else:
                        schedule.get_by_label('Чётность числа месяца', exact=True).select_option('odd')
                        expected_dates = [str(tomorrow+timedelta(days=i)) for i in range(8) if (tomorrow+timedelta(days=i)).day % 2]
                else:
                    expected_dates = [str(tomorrow)] if mode == 'once' else [str(tomorrow), str(tomorrow+timedelta(days=3))]
                    for picked in expected_dates:
                        schedule.get_by_label('Дата в календаре', exact=True).fill(picked)
                        schedule.get_by_role('button', name='Выбрать дату', exact=True).click()
                schedule.get_by_role('button', name='Показать предпросмотр', exact=True).click()
                expect(schedule.locator('.schedule-preview')).to_be_visible()
                preview = schedule.locator('.schedule-preview .schedule-dates li').all_text_contents()
                assert [v.split(' · ')[0] for v in preview] == ['.'.join(reversed(d.split('-'))) for d in expected_dates], (mode, preview, expected_dates)
                dimensions(admin, 'admin-schedule-'+mode)
                calls = []
                def saved(request):
                    if request.method == 'POST' and request.url == ROOT+base+'/schedule':
                        calls.append(request.url)
                admin.on('request', saved)
                with admin.expect_response(schedule_read):
                    schedule.get_by_role('button', name='Подтвердить расписание', exact=True).evaluate('(b)=>{b.click();b.click()}')
                expect(schedule.locator('.schedule-editor')).to_have_count(0)
                admin.remove_listener('request', saved)
                assert len(calls) == 1, (mode, calls)
                sessions = api(admin, 'GET', base+'/schedule/sessions?from='+str(tomorrow)+'&until='+str(until))
                actual = [s for s in sessions if s['startsAt'][11:16] == f'{int(hour[:2])-3:02d}:00']
                assert [s['date'] for s in actual] == expected_dates, (mode, actual, expected_dates)
                assert all((s['ruleId'] is None) == (mode == 'once') for s in actual)
                schedule_counts[mode] = len(actual)
            check_registry(browser, page, signin, api, owner, ROOT, ENGINE, open_section, sql)
            check_runtime(browser, page, signin, api, ROOT, club, ENGINE, open_section)
            check_shell(browser, page, signin, api, owner, club, group, trainer, ROOT, ENGINE, sql, open_section, dimensions)
            # Historical synthetic session allows opening regardless of time of CI run.
            sid = sql(f"INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES('{club['id']}','{group['id']}','{trainer['id']}','{today}','Europe/Minsk','{today} 09:00+03','{today} 10:00+03') RETURNING id").splitlines()[0]
            # Cancelled disposable fixtures create a genuine long daily list.
            # They never contribute marks or the closed-session report ledger.
            sql(f"INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at,status,cancelled_at,cancellation_reason) SELECT '{club['id']}','{group['id']}','{trainer['id']}','{today}','Europe/Minsk','{today} 16:00+03'::timestamptz+n*interval '15 minutes','{today} 16:10+03'::timestamptz+n*interval '15 minutes','cancelled',now(),'Synthetic scroll fixture' FROM generate_series(1,12) AS n")
            # Playwright WebKit cannot route fetches from SW-controlled pages.
            # Network fault injection uses a separate context without a worker;
            # owner/admin contexts exercise the real worker/cache/update behavior.
            trainer_context = browser.new_context(viewport={'width': 390, 'height': 844}, has_touch=True, service_workers='block' if ENGINE == 'webkit' else 'allow')
            tp = page(trainer_context); signin(tp, 'trainer')
            expect(tp.get_by_label('Филиал', exact=True)).to_have_count(0)
            tp.goto(ROOT+'/clubs/'+club['id']+'/home')
            assert tp.locator('.sidebar').count() == 0
            assert tp.locator('.bottom-navigation a').all_text_contents() == ['Главная', 'Занятия', 'Журналы']
            open_section(tp, 'Журналы')
            expect(tp.locator('.trainer-session-list li')).to_have_count(13)
            tp.set_viewport_size({'width': 320, 'height': 480})
            tp.get_by_role('button', name='Открыть журнал', exact=True).click()
            confirmation = tp.get_by_role('button', name='Подтвердить открытие журнала', exact=True)
            expect(confirmation).to_be_visible()
            expect(tp.locator('.session-journal h3')).to_be_focused()
            expect(tp.locator('.session-journal h3')).to_be_in_viewport(ratio=1)
            expect(confirmation).to_be_in_viewport(ratio=1)
            assert confirmation.bounding_box()['y'] + confirmation.bounding_box()['height'] <= tp.locator('.bottom-navigation').bounding_box()['y']
            tp.screenshot(path=f'/tmp/rd183-{ENGINE}-journal-confirm-320.png')
            assert sql(f"SELECT status FROM pride.training_sessions WHERE id='{sid}'") == 'planned'
            assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}'") == '0'
            tp.get_by_role('button', name='Подтвердить открытие журнала', exact=True).click()
            expect(tp.locator('.attendance-card')).to_contain_text('Synthetic Participant A')
            dimensions(tp, 'trainer-card')
            tp.set_viewport_size({'width': 320, 'height': 480})
            tp.get_by_role('button', name='Отсутствует', exact=True).click()
            expect(tp.get_by_role('dialog')).to_be_visible()
            tp.get_by_role('button', name='Болезнь', exact=True).focus()
            assert tp.get_by_role('button', name='Болезнь', exact=True).evaluate('el=>el===document.activeElement')
            assert tp.evaluate('document.documentElement.scrollWidth <= innerWidth')
            # Every intermediate Tab must work even when macOS skips native buttons.
            for target in ([tp.get_by_role('dialog').get_by_role('button', name=name, exact=True) for name in ('Уважительная', 'Отмена')] + [tp.get_by_role('dialog').locator('.dialog-top-close')] + [tp.get_by_role('dialog').get_by_role('button', name=name, exact=True) for name in ('Неизвестна', 'Болезнь')]) * 2:
                tp.keyboard.press('Tab')
                expect(target).to_be_focused()
            tp.get_by_role('dialog').locator('[data-initial-focus]').focus()
            tp.keyboard.press('Tab')
            expect(tp.get_by_role('dialog').locator('.dialog-top-close')).to_be_focused()
            tp.keyboard.press('Tab')
            expect(tp.get_by_role('button', name='Неизвестна', exact=True)).to_be_focused()
            tp.get_by_role('dialog').locator('[data-initial-focus]').focus()
            for target in ([tp.get_by_role('dialog').get_by_role('button', name=name, exact=True) for name in ('Отмена', 'Уважительная', 'Болезнь', 'Неизвестна')] + [tp.get_by_role('dialog').locator('.dialog-top-close'), tp.get_by_role('dialog').get_by_role('button', name='Отмена', exact=True)]):
                tp.keyboard.press('Shift+Tab')
                expect(target).to_be_focused()
            tp.get_by_role('dialog').screenshot(path=f'/tmp/pwa161-{ENGINE}-sheet-320x480.png')
            tp.keyboard.press('Escape')
            expect(tp.get_by_role('dialog')).to_have_count(0)
            # Offline never sends a mutation or advances the card.
            trainer_context.set_offline(True)
            expect(tp.locator('.pwa-status [role="alert"]')).to_contain_text('Нет сети')
            tp.get_by_role('button', name='Присутствует', exact=True).click()
            expect(tp.locator('.session-journal [role="alert"]')).to_contain_text('Изменение не отправлено')
            assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND state IS NOT NULL") == '0'
            trainer_context.set_offline(False)
            held_mark = []
            tp.route('**/journal/records/*', lambda route: held_mark.append(route))
            tp.get_by_role('button', name='Повторить сохранение присутствия', exact=True).evaluate('(b)=>{b.click();b.click()}')
            expect(tp.get_by_role('button', name='Сохраняем…', exact=True)).to_be_disabled()
            wait_for_route(tp, held_mark)
            assert len(held_mark) == 1
            assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND state IS NOT NULL") == '0'
            held_mark[0].continue_()
            tp.unroute('**/journal/records/*')
            expect(tp.locator('.attendance-card')).to_contain_text('Synthetic Participant B')
            # Failed request before commit, then lost response after a real commit.
            tp.get_by_role('button', name='Отсутствует', exact=True).tap()
            tp.route('**/journal/records/*', lambda route: route.abort())
            tp.get_by_role('button', name='Неизвестна', exact=True).tap()
            expect(tp.get_by_role('dialog').get_by_role('alert')).to_contain_text('Нет соединения')
            assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND state IS NOT NULL") == '1'
            tp.unroute('**/journal/records/*')
            def lost_response(route):
                response = route.fetch()
                assert response.ok
                route.abort()
            tp.route('**/journal/records/*', lost_response)
            tp.get_by_role('button', name='Повторить сохранение: Неизвестна', exact=True).tap()
            expect(tp.get_by_role('dialog').get_by_role('alert')).to_contain_text('Нет соединения')
            expect(tp.locator('.attendance-card')).to_contain_text('Synthetic Participant B')
            assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND state IS NOT NULL") == '2'
            audit_count = sql(f"SELECT count(*) FROM pride.audit_events WHERE entity_type='attendance_record' AND after_state->>'sessionId'='{sid}'")
            tp.unroute('**/journal/records/*')
            tp.get_by_role('button', name='Повторить сохранение: Неизвестна', exact=True).evaluate('(b)=>{b.click();b.click()}')
            expect(tp.locator('.attendance-card')).to_contain_text('Synthetic Participant C')
            assert sql(f"SELECT count(*) FROM pride.audit_events WHERE entity_type='attendance_record' AND after_state->>'sessionId'='{sid}'") == audit_count
            for reason, next_name in (('Болезнь', 'Synthetic Participant D'), ('Уважительная', 'Synthetic Participant E')):
                tp.get_by_role('button', name='Отсутствует', exact=True).tap()
                tp.get_by_role('button', name=reason, exact=True).tap()
                expect(tp.locator('.attendance-card')).to_contain_text(next_name)
            tp.reload()
            tp.get_by_role('button', name='Посмотреть журнал', exact=True).click()
            expect(tp.locator('.attendance-card')).to_contain_text('Synthetic Participant E')
            tp.get_by_role('button', name='К сводке и предыдущим отметкам', exact=True).click()
            expect(tp.get_by_role('heading', name='Сводка посещаемости')).to_be_visible()
            tp.get_by_role('button', name='Завершить журнал', exact=True).click()
            # RD-06: transport failure before close leaves the server open;
            # loss of the committed response must not claim success in the UI.
            close_path = base+'/trainer-sessions/'+sid+'/journal/close'
            fault_fetch(tp, close_path)
            tp.get_by_role('button', name='Подтвердить завершение', exact=True).click()
            expect(tp.get_by_role('dialog').get_by_role('alert')).to_contain_text('Изменение не подтверждено')
            assert sql(f"SELECT status FROM pride.training_sessions WHERE id='{sid}'") == 'open'
            assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND origin='close_default'") == '0'
            restore_fault(tp)
            fault_fetch(tp, close_path, after_commit=True)
            tp.get_by_role('button', name='Подтвердить завершение', exact=True).click()
            expect(tp.get_by_role('dialog').get_by_role('alert')).to_contain_text('Изменение не подтверждено')
            assert sql(f"SELECT status FROM pride.training_sessions WHERE id='{sid}'") == 'closed'
            restore_fault(tp)
            tp.get_by_role('button', name='Продолжить отметки', exact=True).click()
            tp.reload()
            tp.get_by_role('button', name='Посмотреть журнал', exact=True).click()
            expect(tp.locator('.session-journal')).to_contain_text('Завершено')
            assert sql(f"SELECT status FROM pride.training_sessions WHERE id='{sid}'") == 'closed'
            assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND origin='close_default' AND absence_reason='unknown'") == '1'
            expect(tp.locator('.attendance-card')).to_have_count(0)
            open_section(admin, 'Журналы')
            journals = admin.locator('.admin-journals')
            journals.get_by_role('button', name='Обновить журналы', exact=True).click()
            journals.locator('[data-session-id="'+sid+'"]').get_by_role('button', name='Посмотреть журнал филиала', exact=True).click()
            # Correct the manually marked illness; independent ledger becomes 2/5=40%.
            journals.get_by_role('button', name='Исправить отметку', exact=True).nth(2).click()
            journals.get_by_label('Новая отметка', exact=True).select_option('present')
            journals.get_by_label('Причина исправления', exact=True).fill('Synthetic PWA correction')
            dimensions(admin, 'admin-correction')
            correction_id = sql(f"SELECT id FROM pride.attendance_records WHERE training_session_id='{sid}' AND absence_reason='illness'")
            correction_path = base+'/journals/'+sid+'/records/'+correction_id
            fault_fetch(admin, correction_path)
            journals.get_by_role('button', name='Сохранить исправление', exact=True).click()
            expect(journals.get_by_role('alert')).to_contain_text('Исправление не подтверждено')
            expect(journals.get_by_label('Причина исправления', exact=True)).to_have_value('Synthetic PWA correction')
            assert sql(f"SELECT state FROM pride.attendance_records WHERE id='{correction_id}'") == 'absent'
            assert sql(f"SELECT count(*) FROM pride.audit_events WHERE entity_id='{correction_id}' AND action='corrected'") == '0'
            restore_fault(admin)
            fault_fetch(admin, correction_path, after_commit=True)
            journals.get_by_role('button', name='Сохранить исправление', exact=True).click()
            expect(journals.get_by_role('alert')).to_contain_text('Исправление не подтверждено')
            expect(journals.get_by_label('Причина исправления', exact=True)).to_have_value('Synthetic PWA correction')
            expect(journals).not_to_contain_text('Отметка подтверждена сервером.')
            assert sql(f"SELECT state FROM pride.attendance_records WHERE id='{correction_id}'") == 'present'
            assert sql(f"SELECT count(*) FROM pride.audit_events WHERE entity_id='{correction_id}' AND action='corrected'") == '1'
            restore_fault(admin)
            # A stale correction retry must conflict, never append another event.
            journals.get_by_role('button', name='Сохранить исправление', exact=True).click()
            expect(journals.get_by_role('button', name='Перечитать журнал и сбросить ввод', exact=True)).to_be_visible()
            assert sql(f"SELECT count(*) FROM pride.audit_events WHERE entity_id='{correction_id}' AND action='corrected'") == '1'
            journals.get_by_role('button', name='Отмена исправления', exact=True).click()
            admin.reload()
            journals.locator('[data-session-id="'+sid+'"]').get_by_role('button', name='Посмотреть журнал филиала', exact=True).click()
            expect(journals.get_by_label('Причина исправления', exact=True)).to_have_count(0)
            expect(journals.locator('.admin-records > li').nth(2)).to_contain_text('Присутствует')
            owner.reload()
            open_section(owner, 'Статистика')
            owner.get_by_label('Филиал отчёта', exact=True).select_option(club['id'])
            owner.get_by_role('button', name='Обновить статистику', exact=True).click()
            expect(owner.locator('[data-metric="closed"]')).to_have_text('1')
            expect(owner.locator('[data-metric="present"]')).to_have_text('2')
            expect(owner.locator('[data-metric="absent"]')).to_have_text('3')
            expect(owner.locator('[data-metric="percentage"]')).to_have_text('40,0%')
            expect(owner.locator('[data-metric="reasons"]')).to_have_text('Причины пропусков: неизвестна — 2; болезнь — 0; уважительная — 1.')
            assert sql(f"SELECT count(*) FILTER (WHERE state='present') || '|' || count(*) FILTER (WHERE state='absent') || '|' || count(*) FILTER (WHERE absence_reason='unknown') || '|' || count(*) FILTER (WHERE absence_reason='illness') || '|' || count(*) FILTER (WHERE absence_reason='excused') FROM pride.attendance_records WHERE training_session_id='{sid}'") == '2|3|2|0|1'
            dimensions(owner, 'owner-report')
            # Real cancellation keeps an open snapshot unmarked and read-only.
            cancelled_id = sql(f"INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES('{club['id']}','{group['id']}','{trainer['id']}','{today}','Europe/Minsk','{today} 11:00+03','{today} 12:00+03') RETURNING id").splitlines()[0]
            tp.reload()
            tp.locator('.trainer-session-list li').filter(has_text='11:00').get_by_role('button', name='Открыть журнал', exact=True).click()
            tp.get_by_role('button', name='Подтвердить открытие журнала', exact=True).click()
            expect(tp.locator('.attendance-card')).to_contain_text('Synthetic Participant A')
            current = api(tp, 'GET', base+'/trainer-sessions/'+cancelled_id+'/journal')
            api(admin, 'POST', base+'/sessions/'+cancelled_id+'/cancel', {'version': current['session']['version'], 'confirmOpen': True, 'reason': 'Synthetic browser cancellation'})
            tp.get_by_role('button', name='Присутствует', exact=True).click()
            expect(tp.locator('.session-journal [role="alert"]')).to_contain_text('Журнал недоступен')
            tp.get_by_role('button', name='Загрузить актуальный журнал и сбросить ввод', exact=True).click()
            expect(tp.locator('.session-journal')).to_contain_text('Занятие отменено')
            expect(tp.locator('.attendance-card')).to_have_count(0)
            assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{cancelled_id}' AND state IS NULL") == '5'
            cache_a = cache_check(owner)
            cache_check(admin)
            if ENGINE == 'chromium':
                cache_check(tp)
            # An API navigation must not receive the shell when offline.
            offline = page(owner_context)
            if ENGINE == 'chromium':
                owner_context.set_offline(True)
            else:
                # WebKit's set_offline fails navigation before dispatching the SW.
                # A real disposable server outage exercises fetch rejection/fallback.
                command('docker', 'stop', APP)
            try:
                owner.reload()
                expect(owner.get_by_role('heading', name='Вход в аккаунт')).to_be_visible()
                expect(owner.locator('.owner-dashboard')).to_have_count(0)
                expect(owner.locator('.pwa-status [role="alert"]')).to_contain_text('Нет сети')
                try:
                    response = offline.goto(ROOT+'/api/auth/me')
                    assert response and 'text/html' not in response.headers.get('content-type', '')
                except Exception as cause:
                    assert any(message in str(cause) for message in ('net::ERR_', 'WebKit encountered an internal error', 'cancelled', 'Could not connect', 'Internet connection')), cause
            finally:
                if ENGINE == 'chromium':
                    owner_context.set_offline(False)
                else:
                    command('docker', 'start', APP)
                    for _ in range(30):
                        try:
                            with urllib.request.urlopen(ROOT+'/api/readyz', timeout=1) as response:
                                if response.status == 200:
                                    break
                        except OSError:
                            pass
                        time.sleep(.1)
                    else:
                        raise AssertionError('App did not recover after disposable outage')
            owner.get_by_role('button', name='Проверить подключение и доступ', exact=True).click()
            expect(owner.locator('.owner-dashboard')).to_be_visible()
            # Two tabs share cookies: logout/login invalidates personal DOM immediately.
            second = page(owner_context); second.goto(ROOT+'/clubs/'+club['id']+'/statistics')
            expect(second.locator('.owner-dashboard')).to_be_visible()
            hold_response(owner, '/api/reports/attendance', 'GET')
            owner.get_by_role('button', name='Обновить статистику', exact=True).click()
            wait_for_response(owner)
            signout(owner)
            expect(second.get_by_label('Логин', exact=True)).to_be_visible()
            expect(second.locator('.owner-dashboard')).to_have_count(0)
            signin(owner, 'admin', navigate=False)
            release_response(owner)
            expect(owner.locator('.owner-dashboard')).to_have_count(0)
            expect(owner.locator('.account')).to_contain_text('Synthetic Admin')
            expect(second.locator('.account')).to_contain_text('Synthetic Admin')
            expect(second.locator('.owner-dashboard')).to_have_count(0)
            # Back/forward snapshot: data is removed before pagehide, fresh me on pageshow.
            # A restored document is foreground. Background WebKit pages can
            # suspend work; dispatching synthetic pageshow does not activate them.
            second.bring_to_front()
            second.wait_for_function("document.visibilityState === 'visible'")
            second.get_by_label('Филиал', exact=True).select_option(club['id'])
            open_section(second, 'Участники')
            expect(second.locator('#registry-title')).to_be_visible()
            # Snapshot a settled page, rather than its initial heading while all
            # club reads are still starting. Keep the direct persisted-event
            # contract in both engines, with the same restoration deadline.
            second.wait_for_load_state('networkidle')
            second.evaluate("window.dispatchEvent(new PageTransitionEvent('pagehide',{persisted:true}))")
            expect(second.locator('#registry-title')).to_have_count(0)
            second.evaluate("window.dispatchEvent(new PageTransitionEvent('pageshow',{persisted:true}))")
            try:
                expect(second.locator('#registry-title')).to_be_visible()
            except Exception:
                print(json.dumps({'engine': ENGINE, 'check': 'settled-persisted-page', 'url': second.url,
                    'visibility': second.evaluate('document.visibilityState'),
                    'api': second.pwa_api_events[-40:], 'body': second.locator('body').inner_text()[:6000], 'errors': errors}, ensure_ascii=False), flush=True)
                raise
            # Dispatched events do not perform a native document/history restore.
            # Exercise an actual departure and return in both engines. The earlier WebKit
            # synthetic-event stall remains recorded, without claiming its cause.
            api_start = len(second.pwa_api_events)
            second.goto(ROOT+'/api/readyz', wait_until='domcontentloaded')
            second.go_back(wait_until='domcontentloaded')
            try:
                expect(second.locator('#registry-title')).to_be_visible()
                expect(second.locator('.account')).to_contain_text('Synthetic Admin')
                expect(second.locator('.owner-dashboard')).to_have_count(0)
                restored_api = second.pwa_api_events[api_start:]
                assert any(v['event'] == 'request' and v['path'] == '/api/auth/me' for v in restored_api), restored_api
                assert any(v['event'] == 'response' and v['path'] == '/api/auth/me' and v['status'] == 200 for v in restored_api), restored_api
            except Exception:
                print(json.dumps({'engine': ENGINE, 'check': 'native-restored-page', 'url': second.url,
                    'visibility': second.evaluate('document.visibilityState'),
                    'api': second.pwa_api_events[-40:], 'body': second.locator('body').inner_text()[:6000], 'errors': errors}, ensure_ascii=False), flush=True)
                raise
            # Explicit update cannot interrupt current response/body processing.
            if ENGINE == 'chromium':
                # Keep A alive in this separate context across later navigations.
                # This reproduces macOS's legitimate waiting B + two static caches.
                trainer_anchor = page(trainer_context)
                trainer_anchor.goto(ROOT)
                trainer_anchor.wait_for_function('navigator.serviceWorker.controller !== null')
            open_section(admin, 'Группы')
            hold_response(admin, base+'/groups', 'POST')
            groups.get_by_role('button', name='Создать группу', exact=True).click()
            groups.get_by_label('Название группы', exact=True).fill('Synthetic update group')
            groups.get_by_role('button', name='Сохранить группу', exact=True).click()
            wait_for_response(admin)
            expect(admin.get_by_role('dialog').get_by_role('button', name='Закрыть форму', exact=True)).to_be_disabled()
            admin.get_by_role('navigation', name='Разделы филиала').get_by_role('link', name='Занятия', exact=True).evaluate('(el)=>el.click()')
            expect(admin.locator('main h1')).to_have_text('Группы')
            expect(admin.locator('main')).to_contain_text('Дождитесь окончания сохранения')
            assert sql(f"SELECT count(*) FROM pride.training_groups WHERE club_id='{club['id']}' AND name='Synthetic update group'") == '1'
            for path in (source/'dist').rglob('*'):
                if path.is_file():
                    target = static/path.relative_to(source/'dist'); target.parent.mkdir(parents=True, exist_ok=True)
                    shutil.copy2(path, target)
            readable(static)
            command('docker', 'cp', f'{static}/.', f'{APP}:/app/static')
            admin.evaluate('navigator.serviceWorker.getRegistration().then(r => r.update())')
            expect(admin.get_by_role('button', name='Обновить приложение', exact=True)).to_be_visible(timeout=20_000)
            expect(admin.get_by_role('button', name='Обновить приложение', exact=True)).to_be_disabled()
            assert ' · B' not in admin.title()
            release_response(admin)
            expect(groups.get_by_label('Название группы', exact=True)).to_have_count(0)
            expect(admin.get_by_role('button', name='Обновить приложение', exact=True)).to_be_enabled()
            # Another tab has an outstanding save: activating B must not reload it.
            open_section(second, 'Группы')
            hold_response(second, base+'/groups', 'POST')
            second.locator('.groups').get_by_role('button', name='Создать группу', exact=True).click()
            second.get_by_label('Название группы', exact=True).fill('Synthetic other tab save')
            second.get_by_role('button', name='Сохранить группу', exact=True).click()
            wait_for_response(second)
            admin.get_by_role('button', name='Обновить приложение', exact=True).click()
            expect(admin).to_have_title('PRIDE — рабочее пространство · B')
            expect(admin.locator('.account')).to_contain_text('Synthetic Admin')
            assert ' · B' not in second.title()
            release_response(second)
            expect(second.get_by_label('Название группы', exact=True)).to_have_count(0)
            cache_b = cache_check(admin)
            assert cache_b != cache_a
            # Expiration clears the journal; next employee cannot recover it via reload.
            sql(f"UPDATE pride.staff_sessions SET expires_at=now()-interval '1 second' WHERE staff_user_id='{trainer['id']}'")
            tp.get_by_role('button', name='Обновить журнал', exact=True).click()
            expect(tp.get_by_label('Логин', exact=True)).to_be_visible()
            expect(tp.locator('.session-journal')).to_have_count(0)
            signin(tp, 'admin')
            expect(tp.locator('.trainer-sessions')).to_have_count(0)
            tp.reload(); expect(tp.locator('.trainer-sessions')).to_have_count(0)
            if ENGINE == 'chromium':
                expect(tp.locator('.account')).to_contain_text('Synthetic Admin')
                tp.evaluate('navigator.serviceWorker.getRegistration().then(r => r.update())')
                expect(tp.get_by_role('button', name='Обновить приложение', exact=True)).to_be_visible(timeout=20_000)
                assert tp.evaluate('navigator.serviceWorker.getRegistration().then(r => r.waiting?.state)') == 'installed'
                # Both versions are allowed while B waits; privacy checks still apply.
                cache_check(tp, expected_names={cache_a, cache_b})
                # HTML can already have title B while controller A is still active.
                # Wait for the actual reload, not just that already-visible title.
                with tp.expect_event('load'):
                    tp.get_by_role('button', name='Обновить приложение', exact=True).click()
                expect(tp).to_have_title('PRIDE — рабочее пространство · B')
                expect(tp.locator('.account')).to_contain_text('Synthetic Admin')
                assert cache_check(tp) == cache_b
                assert ' · B' not in trainer_anchor.title()
                trainer_anchor.close()
            assert not errors, errors
            # Live role withdrawal rejects a save and removes the displayed snapshot.
            signout(tp)
            revoked_id = sql(f"INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES('{club['id']}','{group['id']}','{trainer['id']}','{today}','Europe/Minsk','{today} 13:00+03','{today} 14:00+03') RETURNING id").splitlines()[0]
            signin(tp, 'trainer')
            expect(tp.get_by_label('Филиал', exact=True)).to_have_count(0)
            tp.goto(ROOT+'/clubs/'+club['id']+'/home')
            open_section(tp, 'Журналы')
            tp.locator('.trainer-session-list li').filter(has_text='13:00').get_by_role('button', name='Открыть журнал', exact=True).click()
            tp.get_by_role('button', name='Подтвердить открытие журнала', exact=True).click()
            expect(tp.locator('.attendance-card')).to_contain_text('Synthetic Participant A')
            sql(f"DELETE FROM pride.staff_club_roles WHERE club_id='{club['id']}' AND staff_user_id='{trainer['id']}' AND role='trainer'")
            tp.get_by_role('button', name='Присутствует', exact=True).click()
            expect(tp.locator('.session-journal')).to_contain_text('Доступ к журналу отозван')
            expect(tp.locator('.attendance-card')).to_have_count(0)
            assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{revoked_id}' AND state IS NOT NULL") == '0'
            assert not errors, errors
            owner_context.close()
            browser.close()
        print(json.dumps({'check': 'rendered-text-contrast', 'normalMinimum': contrast_floor[4.5], 'largeMinimum': contrast_floor[3]}, ensure_ascii=False))
        print(json.dumps({'engine': ENGINE, 'version': browser.version, 'scheduleCounts': schedule_counts, 'report': {'conducted': 1, 'present': 2, 'absent': 3, 'unknown': 2, 'illness': 0, 'excused': 1, 'percentage': 40}, 'nativeHistoryReturn': 'fresh session and role verified; BFCache not measured', 'checks': 'manifest/icons; admin UI/five participants/four schedules; cards/three reasons/defaults/correction/report; 320/390/768/900/1280/1440; rendered text contrast 4.5/3; double tap/delay/offline; mark/close/correct precommit error/lost response/SQL/reload; mark idempotent retry/correction stale retry; closed/cancelled/role revoke; static-only cache/logout/account swap/expiry/pagehide/native history return/fresh me; two builds/waiting/save guard/other-tab save', 'physicalInstallation': 'NOT TESTED'}, ensure_ascii=False))
    finally:
        subprocess.run(['docker', 'rm', '-f', APP, DB], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        subprocess.run(['docker', 'network', 'rm', NETWORK], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

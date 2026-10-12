"""Disposable production Go/PostgreSQL/Chromium PWA regression; docs/pwa-v1.md."""
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

REPO = Path(__file__).resolve().parents[2]
ROOT = 'http://127.0.0.1:18081'
PASSWORD = 'synthetic-browser-password'
PREFIX = 'pwa161-' + uuid.uuid4().hex[:8]
DB, APP, NETWORK = PREFIX+'-db', PREFIX+'-app', PREFIX+'-net'
IMAGE = os.environ.get('PWA_TEST_IMAGE', 'judeoslite:ci')
assert IMAGE.startswith('judeoslite:')

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
            path = os.environ.get('CHROMIUM_PATH') or ('/usr/bin/chromium' if Path('/usr/bin/chromium').exists() else None)
            browser = p.chromium.launch(headless=True, executable_path=path)
            errors = []
            def page(context):
                value = context.new_page()
                value.on('pageerror', lambda error: errors.append(str(error)))
                return value
            def signin(value, login='owner'):
                value.goto(ROOT)
                value.get_by_label('Логин', exact=True).fill(login)
                value.get_by_label('Пароль', exact=True).fill(PASSWORD)
                value.get_by_role('button', name='Войти', exact=True).click()
                expect(value.get_by_role('button', name='Выйти', exact=True)).to_be_visible()
            def api(value, method, path, data=None):
                result = value.evaluate('''async ([method,path,data]) => {
                  const me = await (await fetch('/api/auth/me', {cache:'no-store'})).json();
                  const r = await fetch(path, {method,cache:'no-store',headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrfToken},body:method==='GET'?undefined:JSON.stringify(data)});
                  return {status:r.status,body:await r.json(),cache:r.headers.get('cache-control')};
                }''', [method, path, data])
                assert result['status'] in (200, 201) and result['cache'] == 'no-store', result
                return result['body']
            def dimensions(value, role):
                for width in (320, 390, 1280):
                    value.set_viewport_size({'width': width, 'height': 900})
                    assert value.evaluate('document.documentElement.scrollWidth <= innerWidth'), (role, width)
                    for control in value.locator('button:visible,input:visible,select:visible,summary:visible').all():
                        if control.get_attribute('type') not in ('checkbox', 'radio'):
                            rect = control.bounding_box()
                            assert rect['height'] >= 44 and rect['width'] >= 44, (role, width, control.outer_html() if hasattr(control, 'outer_html') else control.get_attribute('class'), rect)
                    value.screenshot(path=f'/tmp/pwa161-{role}-{width}.png')
            def cache_check(value):
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
                assert len({v['name'] for v in caches}) == 1, caches
                assert not any(v['type'] and 'json' in v['type'] for v in caches), caches
                assert all('Synthetic' not in v['text'] and 'csrfToken' not in v['text'] for v in caches)
                return caches[0]['name']
            owner_context = p.chromium.launch_persistent_context(str(temp/'profile'), headless=True, executable_path=path, viewport={'width': 320, 'height': 900})
            owner = page(owner_context)
            owner.goto(ROOT)
            expect(owner.get_by_role('heading', name='Вход для сотрудников')).to_be_visible()
            dimensions(owner, 'login')
            owner.locator('.install-help summary').click()
            expect(owner.locator('.install-help')).to_contain_text('iPhone / Safari')
            expect(owner.locator('.install-help')).to_contain_text('Android / Chrome')
            owner.locator('.install-help summary').click()
            signin(owner)
            owner.wait_for_function('navigator.serviceWorker.controller !== null')
            cdp = owner_context.new_cdp_session(owner)
            manifest = cdp.send('Page.getAppManifest')
            assert not manifest.get('errors'), manifest
            parsed = json.loads(manifest['data'])
            assert parsed['display'] == 'standalone' and parsed['start_url'] == '/' and parsed['scope'] == '/'
            for icon in parsed['icons']:
                size = int(icon['sizes'].split('x')[0])
                assert owner.evaluate('''async ([src,size])=>{const img=new Image();img.src=src;await img.decode();return img.naturalWidth===size&&img.naturalHeight===size;}''', [icon['src'], size])
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
            admin.get_by_label('Филиал', exact=True).select_option(club['id'])
            groups = admin.locator('.groups')
            groups.get_by_role('button', name='Создать группу', exact=True).click()
            groups.get_by_label('Название группы', exact=True).fill('Synthetic PWA group')
            groups.get_by_role('button', name='Сохранить группу', exact=True).click()
            expect(groups).to_contain_text('Группа сохранена.')
            group = api(admin, 'GET', base+'/groups')[0]
            staff = admin.locator('.staff-management')
            staff.get_by_role('button', name='Добавить тренера', exact=True).click()
            staff.get_by_label('Имя тренера', exact=True).fill('Synthetic Trainer')
            staff.get_by_label('Логин тренера', exact=True).fill('trainer')
            staff.get_by_label('Первоначальный пароль', exact=True).fill(PASSWORD)
            staff.get_by_label('Повторите пароль', exact=True).fill(PASSWORD)
            staff.get_by_role('button', name='Создать тренера', exact=True).click()
            expect(staff).to_contain_text('Тренер создан.')
            trainer = api(admin, 'GET', base+'/trainers?available=true')[0]
            registry = admin.locator('.registry').filter(has=admin.locator('#registry-title'))
            registry.get_by_role('button', name='Добавить участника', exact=True).click()
            registry.get_by_label('ФИО участника', exact=True).fill('Synthetic PWA Participant')
            registry.get_by_label('Дата рождения', exact=True).fill('2010-01-01')
            registry.get_by_label('Дата вступления', exact=True).fill('2020-01-01')
            registry.get_by_label('Номер телефона', exact=True).fill('+375291234567')
            registry.get_by_label('Начальная группа (необязательно)', exact=True).select_option(group['id'])
            registry.get_by_label('Дата включения в группу', exact=True).fill('2020-01-01')
            registry.get_by_role('button', name='Сохранить участника', exact=True).click()
            expect(registry).to_contain_text('Участник добавлен.')
            dimensions(admin, 'admin-card')
            registry.get_by_role('button', name='Закрыть карточку', exact=True).click()
            schedule = admin.locator('.schedule')
            schedule.get_by_role('button', name='Обновить расписание', exact=True).click()
            schedule.get_by_role('button', name='Создать расписание', exact=True).click()
            schedule.get_by_label('Группа расписания', exact=True).select_option(group['id'])
            schedule.get_by_label('Тренер занятия', exact=True).select_option(trainer['id'])
            schedule.get_by_label('Режим расписания', exact=True).select_option('once')
            today = datetime.now(ZoneInfo('Europe/Minsk')).date()
            tomorrow = str(today+timedelta(days=1))
            schedule.get_by_label('Дата в календаре', exact=True).fill(tomorrow)
            schedule.get_by_role('button', name='Выбрать дату', exact=True).click()
            schedule.get_by_role('button', name='Показать предпросмотр', exact=True).click()
            expect(schedule.locator('.schedule-preview')).to_be_visible()
            dimensions(admin, 'admin-schedule')
            schedule.get_by_role('button', name='Подтвердить расписание', exact=True).click()
            expect(schedule).to_contain_text('Расписание сохранено.')
            # Historical synthetic session allows opening regardless of time of CI run.
            sid = sql(f"INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES('{club['id']}','{group['id']}','{trainer['id']}','{today}','Europe/Minsk','{today} 09:00+03','{today} 10:00+03') RETURNING id").splitlines()[0]
            trainer_context = browser.new_context(viewport={'width': 390, 'height': 844})
            tp = page(trainer_context); signin(tp, 'trainer')
            tp.get_by_label('Филиал', exact=True).select_option(club['id'])
            tp.get_by_role('button', name='Открыть журнал', exact=True).click()
            tp.get_by_role('button', name='Подтвердить открытие журнала', exact=True).click()
            expect(tp.locator('.attendance-card')).to_contain_text('Synthetic PWA Participant')
            dimensions(tp, 'trainer-card')
            tp.set_viewport_size({'width': 320, 'height': 480})
            tp.get_by_role('button', name='Отсутствует', exact=True).click()
            expect(tp.get_by_role('dialog')).to_be_visible()
            tp.get_by_role('button', name='Болезнь', exact=True).focus()
            assert tp.get_by_role('button', name='Болезнь', exact=True).evaluate('el=>el===document.activeElement')
            assert tp.evaluate('document.documentElement.scrollWidth <= innerWidth')
            tp.get_by_role('dialog').screenshot(path='/tmp/pwa161-sheet-320x480.png')
            tp.keyboard.press('Escape')
            expect(tp.get_by_role('dialog')).to_have_count(0)
            # Offline never sends a mutation or advances the card.
            trainer_context.set_offline(True)
            expect(tp.locator('.pwa-status [role="alert"]')).to_contain_text('Нет сети')
            tp.get_by_role('button', name='Присутствует', exact=True).click()
            expect(tp.locator('.session-journal [role="alert"]')).to_contain_text('Изменение не отправлено')
            assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND state IS NOT NULL") == '0'
            trainer_context.set_offline(False)
            tp.get_by_role('button', name='Повторить сохранение присутствия', exact=True).click()
            expect(tp.get_by_role('heading', name='Сводка посещаемости')).to_be_visible()
            tp.get_by_role('button', name='Завершить журнал', exact=True).click()
            tp.get_by_role('button', name='Подтвердить завершение', exact=True).click()
            expect(tp.locator('.session-journal')).to_contain_text('Завершено')
            assert sql(f"SELECT status FROM pride.training_sessions WHERE id='{sid}'") == 'closed'
            journals = admin.locator('.admin-journals')
            journals.get_by_role('button', name='Обновить журналы', exact=True).click()
            journals.locator('[data-session-id="'+sid+'"]').get_by_role('button', name='Посмотреть журнал филиала', exact=True).click()
            journals.get_by_role('button', name='Исправить отметку', exact=True).click()
            journals.get_by_label('Новая отметка', exact=True).select_option('illness')
            journals.get_by_label('Причина исправления', exact=True).fill('Synthetic PWA correction')
            dimensions(admin, 'admin-correction')
            journals.get_by_role('button', name='Сохранить исправление', exact=True).click()
            expect(journals.get_by_label('Причина исправления', exact=True)).to_have_count(0)
            owner.reload()
            owner.get_by_label('Филиал отчёта', exact=True).select_option(club['id'])
            owner.get_by_role('button', name='Обновить статистику', exact=True).click()
            expect(owner.locator('[data-metric="absent"]')).to_have_text('1')
            expect(owner.locator('[data-metric="percentage"]')).to_have_text('0,0%')
            dimensions(owner, 'owner-report')
            cache_a = cache_check(owner)
            cache_check(admin); cache_check(tp)
            # An API navigation must not receive the shell when offline.
            offline = page(owner_context)
            owner_context.set_offline(True)
            owner.reload()
            expect(owner.get_by_role('heading', name='Вход для сотрудников')).to_be_visible()
            expect(owner.locator('.owner-dashboard')).to_have_count(0)
            expect(owner.locator('.pwa-status [role="alert"]')).to_contain_text('Нет сети')
            try:
                response = offline.goto(ROOT+'/api/auth/me')
                assert response and 'text/html' not in response.headers.get('content-type', '')
            except Exception as cause:
                assert 'net::ERR_' in str(cause), cause
            owner_context.set_offline(False)
            owner.get_by_role('button', name='Проверить подключение и доступ', exact=True).click()
            expect(owner.locator('.owner-dashboard')).to_be_visible()
            # Two tabs share cookies: logout/login invalidates personal DOM immediately.
            second = page(owner_context); second.goto(ROOT+'/clubs/'+club['id'])
            expect(second.locator('.owner-dashboard')).to_be_visible()
            late_report = []
            owner.route('**/api/reports/attendance?*', lambda route: late_report.append((route, route.fetch())))
            owner.get_by_role('button', name='Обновить статистику', exact=True).click()
            owner.wait_for_timeout(200); assert late_report
            owner.get_by_role('button', name='Выйти', exact=True).click()
            expect(second.get_by_label('Логин', exact=True)).to_be_visible()
            expect(second.locator('.owner-dashboard')).to_have_count(0)
            signin(owner, 'admin')
            for route, response in late_report:
                route.fulfill(response=response)
            owner.unroute('**/api/reports/attendance?*')
            expect(owner.locator('.owner-dashboard')).to_have_count(0)
            expect(owner.locator('.account')).to_contain_text('Synthetic Admin')
            expect(second.locator('.account')).to_contain_text('Synthetic Admin')
            expect(second.locator('.owner-dashboard')).to_have_count(0)
            # Back/forward snapshot: data is removed before pagehide, fresh me on pageshow.
            second.get_by_label('Филиал', exact=True).select_option(club['id'])
            expect(second.locator('#registry-title')).to_be_visible()
            second.evaluate("window.dispatchEvent(new PageTransitionEvent('pagehide',{persisted:true}))")
            expect(second.locator('#registry-title')).to_have_count(0)
            second.evaluate("window.dispatchEvent(new PageTransitionEvent('pageshow',{persisted:true}))")
            expect(second.locator('#registry-title')).to_be_visible()
            # Explicit update cannot interrupt current response/body processing.
            held = []
            def hold(route):
                if route.request.method == 'POST':
                    held.append((route, route.fetch()))
                else:
                    route.continue_()
            admin.route('**/api/clubs/'+club['id']+'/groups', hold)
            groups.get_by_role('button', name='Создать группу', exact=True).click()
            groups.get_by_label('Название группы', exact=True).fill('Synthetic update group')
            groups.get_by_role('button', name='Сохранить группу', exact=True).click()
            admin.wait_for_timeout(200)
            assert held
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
            for route, response in held:
                route.fulfill(response=response)
            expect(groups.get_by_label('Название группы', exact=True)).to_have_count(0)
            expect(admin.get_by_role('button', name='Обновить приложение', exact=True)).to_be_enabled()
            # Another tab has an outstanding save: activating B must not reload it.
            held_second = []
            second.route('**/api/clubs/'+club['id']+'/groups', lambda route: held_second.append((route, route.fetch())) if route.request.method == 'POST' else route.continue_())
            second.locator('.groups').get_by_role('button', name='Создать группу', exact=True).click()
            second.get_by_label('Название группы', exact=True).fill('Synthetic other tab save')
            second.get_by_role('button', name='Сохранить группу', exact=True).click()
            second.wait_for_timeout(200); assert held_second
            admin.get_by_role('button', name='Обновить приложение', exact=True).click()
            expect(admin).to_have_title('PRIDE — рабочее пространство · B')
            expect(admin.locator('.account')).to_contain_text('Synthetic Admin')
            assert ' · B' not in second.title()
            for route, response in held_second:
                route.fulfill(response=response)
            expect(second.get_by_label('Название группы', exact=True)).to_have_count(0)
            assert cache_check(admin) != cache_a
            # Expiration clears the journal; next employee cannot recover it via reload.
            sql(f"UPDATE pride.staff_sessions SET expires_at=now()-interval '1 second' WHERE staff_user_id='{trainer['id']}'")
            tp.get_by_role('button', name='Обновить журнал', exact=True).click()
            expect(tp.get_by_label('Логин', exact=True)).to_be_visible()
            expect(tp.locator('.session-journal')).to_have_count(0)
            signin(tp, 'admin')
            expect(tp.locator('.trainer-sessions')).to_have_count(0)
            tp.reload(); expect(tp.locator('.trainer-sessions')).to_have_count(0)
            cache_check(tp)
            assert not errors, errors
            owner_context.close()
            browser.close()
        print('PWA: manifest/installability/icons; three roles UI→API→PostgreSQL; 320/390/1280; offline/no false save; static-only cache; logout/account swap/expiry/pagehide; two builds/waiting/save guard/other-tab save passed')
    finally:
        subprocess.run(['docker', 'rm', '-f', APP, DB], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        subprocess.run(['docker', 'network', 'rm', NETWORK], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

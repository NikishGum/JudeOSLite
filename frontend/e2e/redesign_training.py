from runtime_ui import observe_api, wait_for_api_idle, assert_page_targets, settle_shell
"""RD-03: real browser -> Go -> disposable PostgreSQL. No external club data."""
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

ROOT = 'http://127.0.0.1:18085'
REPO = Path(__file__).resolve().parents[2]
ENGINE = os.environ.get('PWA_BROWSER', 'chromium')
IMAGE = os.environ.get('PWA_TEST_IMAGE', 'judeoslite:ci')
assert ENGINE in ('chromium', 'webkit') and IMAGE.startswith('judeoslite:')
PREFIX = 'rd185-' + uuid.uuid4().hex[:8]
DB, APP, NET = PREFIX+'-db', PREFIX+'-app', PREFIX+'-net'
PASSWORD = 'synthetic-browser-password'
STATIC_TEMP = tempfile.TemporaryDirectory(prefix='rd185-static-')
static = Path(STATIC_TEMP.name)/'static'
shutil.copytree(REPO/'frontend/dist', static)
for file in (static, *static.rglob('*')):
    file.chmod(0o755 if file.is_dir() else 0o644)

def cmd(*args):
    return subprocess.check_output(args, text=True).strip()

def sql(statement):
    return cmd('docker', 'exec', DB, 'psql', '-U', 'test', '-d', 'rd185', '-Atc', statement)

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
    cmd('docker', 'run', '-d', '--name', DB, '--network', NET, '-e', 'POSTGRES_USER=test', '-e', 'POSTGRES_PASSWORD=test-only', '-e', 'POSTGRES_DB=rd185', 'postgres:18.6-bookworm')
    for _ in range(30):
        if subprocess.run(['docker', 'exec', DB, 'pg_isready', '-h', '127.0.0.1', '-U', 'test', '-d', 'rd185'], stdout=subprocess.DEVNULL).returncode == 0:
            break
        time.sleep(1)
    else:
        raise AssertionError('PostgreSQL not ready')
    env = ['-e', f'DATABASE_URL=postgres://test:test-only@{DB}:5432/rd185?sslmode=disable']
    cmd('docker', 'run', '--rm', '--network', NET, *env, IMAGE, '/app/migrate')
    owner(env)
    cmd('docker', 'create', '--name', APP, '--network', NET, *env, '-e', f'APP_ORIGIN={ROOT}', '-e', 'AUTH_INSECURE_LOCAL=true', '-p', '127.0.0.1:18085:8080', IMAGE)
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
            context = browser.new_context(viewport={'width': 1280, 'height': 900}, service_workers='block', reduced_motion='reduce')
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
            value.wait_for_load_state('networkidle')
            wait_for_api_idle(value)
            return value
        def api(value, method, path, data=None, status=None):
            result = value.evaluate('''async ([method,path,data])=>{
              const me=await (await fetch('/api/auth/me')).json();
              const r=await fetch(path,{method,headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrfToken},body:method==='GET'?undefined:JSON.stringify(data)});
              return {status:r.status,body:await r.json()};
            }''', [method, path, data])
            assert result['status'] == status if status else result['status'] in (200, 201), result
            return result['body']
        def shots(value, name):
            for width in (320, 390, 1280):
                value.set_viewport_size({'width': width, 'height': 900})
                settle_shell(value, width)
                value.wait_for_load_state('networkidle')
                assert value.evaluate('document.documentElement.scrollWidth <= innerWidth'), (name, width)
                assert_page_targets(value, width, extra_selector=',.session-link:visible')
                value.evaluate('window.scrollTo(0,0)')
                value.screenshot(path=f'/tmp/rd185-{ENGINE}-{name}-{width}.png', full_page=True)
        own = page('owner')
        a = api(own, 'POST', '/api/clubs', {'name': 'Контроль RD-03 · Север', 'timezone': 'Europe/Minsk'})
        b = api(own, 'POST', '/api/clubs', {'name': 'Контроль RD-03 · Юг', 'timezone': 'UTC'})
        base = '/api/clubs/'+a['id']
        group = api(own, 'POST', base+'/groups', {'name': 'Младшая группа'})
        other_group = api(own, 'POST', '/api/clubs/'+b['id']+'/groups', {'name': 'Младшая группа'})
        trainer = api(own, 'POST', base+'/trainers', {'login': 'trainer', 'displayName': 'Тренер · контроль', 'password': PASSWORD})
        replacement = api(own, 'POST', base+'/trainers', {'login': 'replacement', 'displayName': 'Тренер замены', 'password': PASSWORD})
        api(own, 'POST', '/api/staff', {'login': 'admin', 'displayName': 'Администратор · контроль', 'password': PASSWORD, 'clubIds': [a['id']]})
        for name in ('Алексей · контроль', 'Борис · контроль', 'Виктор · контроль'):
            api(own, 'POST', base+'/participants', {'identity': {'fullName': name, 'birthDate': '2010-01-01'}, 'joinedOn': '2020-01-01', 'contactKind': 'phone', 'contactValue': '+375291234567', 'initialGroup': {'groupId': group['id'], 'from': '2020-01-01'}})
        today = datetime.now(ZoneInfo('Europe/Minsk')).date()
        tomorrow = today+timedelta(days=1)
        sid = sql(f"INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES('{a['id']}','{group['id']}','{trainer['id']}','{today}','Europe/Minsk','{today} 09:00+03','{today} 10:00+03') RETURNING id").splitlines()[0]
        calendar = ROOT+'/clubs/'+a['id']+'/sessions?group='+group['id']
        admin = page('admin')
        admin.goto(ROOT+'/clubs/'+a['id']+'/groups?detail='+group['id'])
        admin.get_by_role('button', name='К занятиям группы', exact=True).click()
        expect(admin).to_have_url(calendar)
        expect(admin.get_by_label('Группа календаря', exact=True)).to_have_value(group['id'])
        expect(admin.locator('.session-list > li')).to_have_count(1)
        admin.get_by_role('radio', name='Неделя', exact=True).click()
        start = today-timedelta(days=today.weekday())
        expect(admin.get_by_label('С даты', exact=True)).to_have_value(str(start))
        expect(admin.get_by_label('По дату включительно', exact=True)).to_have_value(str(start+timedelta(days=6)))
        admin.get_by_role('radio', name='День', exact=True).click()
        admin.get_by_role('button', name='Сегодня', exact=True).click()
        admin.get_by_role('button', name='Все группы · сбросить фильтр', exact=True).click()
        expect(admin.get_by_label('Группа календаря', exact=True)).to_have_value('')
        admin.get_by_label('Группа календаря', exact=True).select_option(group['id'])
        admin.reload(); expect(admin.get_by_label('Группа календаря', exact=True)).to_have_value(group['id'])
        # Wrong-club group with an identical name must never select the local group.
        admin.goto(ROOT+'/clubs/'+a['id']+'/sessions?group='+other_group['id'])
        expect(admin.locator('.session-calendar [role="alert"]')).to_be_visible()
        expect(admin.locator('.session-list > li')).to_have_count(0)
        admin.goto(calendar)
        # Full admin exception path from the new daily/weekly list.
        admin.get_by_role('button', name='Добавить разовое занятие', exact=True).click()
        form = admin.locator('.session-form')
        form.get_by_label('Группа разового занятия', exact=True).select_option(group['id'])
        form.get_by_label('Фактический тренер', exact=True).select_option(trainer['id'])
        form.get_by_label('Дата занятия', exact=True).fill(str(tomorrow))
        form.get_by_role('button', name='Создать разовое занятие', exact=True).click()
        expect(form).to_have_count(0)
        admin.get_by_label('По дату включительно', exact=True).fill(str(tomorrow+timedelta(days=3)))
        admin.get_by_label('С даты', exact=True).fill(str(tomorrow))
        expect(admin.locator('.session-list > li')).to_have_count(1)
        row = admin.locator('.session-list > li')
        def session_action(name):
            # React can retain the native details state after a fast refresh.
            # Clicking an already-open summary would close the actions instead.
            details = row.locator('details')
            if not details.evaluate('(element) => element.open'):
                details.locator('summary').click()
            expect(details).to_have_attribute('open', '')
            details.get_by_role('button', name=name, exact=True).click()
        session_action('Перенести занятие')
        form.get_by_label('Дата занятия', exact=True).fill(str(tomorrow+timedelta(days=1)))
        form.get_by_role('button', name='Подтвердить изменение занятия', exact=True).click()
        expect(form).to_have_count(0); expect(row).to_contain_text('Перенесено с')
        session_action('Заменить тренера')
        form.get_by_label('Фактический тренер', exact=True).select_option(replacement['id'])
        form.get_by_role('button', name='Подтвердить изменение занятия', exact=True).click()
        expect(form).to_have_count(0); expect(row).to_contain_text('Тренер замены')
        session_action('Отменить занятие')
        form.get_by_label('Причина отмены', exact=True).fill('Контроль отмены')
        form.get_by_role('button', name='Подтвердить изменение занятия', exact=True).click()
        expect(form).to_have_count(0); expect(row).to_contain_text('Отменено')
        admin.get_by_label('С даты', exact=True).fill(str(today))
        expect(admin.locator('.session-list > li')).to_have_count(2)
        shots(admin, 'calendar')
        tr = page('trainer'); tr.goto(calendar)
        tr.locator(f'[data-session-id="{sid}"]').get_by_role('link', name='Открыть журнал', exact=True).click()
        expect(tr.get_by_role('button', name='Подтвердить открытие журнала', exact=True)).to_be_visible()
        assert sql(f"SELECT status FROM pride.training_sessions WHERE id='{sid}'") == 'planned'
        tr.get_by_role('button', name='Подтвердить открытие журнала', exact=True).click()
        expect(tr.locator('.attendance-card')).to_contain_text('Алексей')
        shots(tr, 'attendance')
        tr.set_viewport_size({'width': 320, 'height': 480})
        tr.locator('.attendance-card h4').evaluate("e=>e.scrollIntoView({block:'start'})")
        expect(tr.get_by_role('button', name='Присутствует', exact=True)).to_be_in_viewport(ratio=1)
        assert tr.get_by_role('button', name='Отсутствует', exact=True).bounding_box()['y'] + 56 <= tr.locator('.bottom-navigation').bounding_box()['y']
        tr.screenshot(path=f'/tmp/rd185-{ENGINE}-attendance-action-320.png')
        # Escape must restore the keyboard opener. Pointer click in macOS
        # WebKit does not focus native buttons; activate the focused control.
        absence = tr.get_by_role('button', name='Отсутствует', exact=True)
        absence.focus(); expect(absence).to_be_focused(); absence.press('Enter')
        dialog = tr.get_by_role('dialog')
        expect(dialog).to_be_visible(); tr.screenshot(path=f'/tmp/rd185-{ENGINE}-reasons-320.png'); tr.keyboard.press('Tab')
        expect(dialog.locator('.dialog-top-close')).to_be_focused()
        tr.keyboard.press('Tab')
        expect(dialog.get_by_role('button', name='Неизвестна', exact=True)).to_be_focused()
        tr.keyboard.press('Escape'); expect(dialog).to_have_count(0)
        expect(tr.get_by_role('button', name='Отсутствует', exact=True)).to_be_focused()
        assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND state IS NOT NULL") == '0'
        path = base+'/trainer-sessions/'+sid+'/journal'
        # Failed save keeps the current participant; double-tap produces one request.
        tr.route('**/journal/records/*', lambda route: route.abort())
        tr.get_by_role('button', name='Присутствует', exact=True).click()
        expect(tr.locator('.session-journal [role="alert"]').first).to_be_visible()
        expect(tr.locator('.attendance-card')).to_contain_text('Алексей')
        tr.unroute('**/journal/records/*')
        calls = []
        tr.on('request', lambda r: calls.append(r.url) if r.method == 'PUT' and '/journal/records/' in r.url else None)
        tr.get_by_role('button', name='Присутствует', exact=True).evaluate('(b)=>{b.click();b.click()}')
        expect(tr.locator('.attendance-card')).to_contain_text('Борис'); assert len(calls) == 1
        tr.reload(); expect(tr.locator('.attendance-card')).to_contain_text('Борис')
        tr.get_by_role('button', name='Предыдущий участник', exact=True).click()
        expect(tr.locator('.attendance-card')).to_contain_text('Алексей')
        tr.locator('.attendance-card h4').evaluate("e=>e.scrollIntoView({block:'start'})")
        expect(tr.get_by_role('button', name='Присутствует', exact=True)).to_be_in_viewport(ratio=1)
        assert tr.get_by_role('button', name='Отсутствует', exact=True).bounding_box()['y'] + 56 <= tr.locator('.bottom-navigation').bounding_box()['y']
        tr.screenshot(path=f'/tmp/rd185-{ENGINE}-attendance-action-320.png')
        tr.get_by_role('button', name='Отсутствует', exact=True).click()
        dialog.get_by_role('button', name='Болезнь', exact=True).click()
        expect(tr.locator('.attendance-card')).to_contain_text('Борис')
        tr.get_by_role('button', name='Завершить журнал', exact=True).click()
        expect(dialog).to_contain_text('Осталось отметить: 2')
        tr.keyboard.press('Escape')
        tr.get_by_role('button', name='Завершить журнал', exact=True).click()
        dialog.get_by_role('button', name='Подтвердить завершение', exact=True).evaluate('(b)=>{b.click();b.click()}')
        expect(tr.locator('.session-journal')).to_contain_text('Журнал завершён')
        assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND origin='close_default'") == '2'
        tr.reload(); expect(tr.locator('.attendance-card')).to_have_count(0)
        api(tr, 'GET', base+'/journals/'+sid, status=404)
        # Return/back/forward and direct deep link restore the exact session.
        tr.get_by_role('link', name='К списку занятий', exact=True).click()
        expect(tr.get_by_label('Группа календаря', exact=True)).to_have_value(group['id'])
        tr.go_back(); expect(tr.locator('.session-journal')).to_contain_text('Журнал завершён')
        tr.go_forward(); expect(tr.locator('.session-calendar')).to_be_visible()
        admin.goto(calendar+'&session='+sid)
        expect(admin.locator('.admin-records > li')).to_have_count(3)
        record = admin.locator('.admin-records > li').first
        record.get_by_role('button', name='Исправить отметку', exact=True).click()
        admin.get_by_label('Новая отметка', exact=True).select_option('present')
        expect(admin.get_by_role('button', name='Сохранить исправление', exact=True)).to_be_disabled()
        admin.get_by_label('Причина исправления', exact=True).fill('Проверка поштучной коррекции')
        shots(admin, 'correction')
        j = api(own, 'GET', base+'/journals/'+sid); rec = j['records'][0]
        api(own, 'PUT', base+'/journals/'+sid+'/records/'+rec['id'], {'version': rec['version'], 'state': 'absent', 'absenceReason': 'excused', 'changeReason': 'Конкурентная проверка владельца'})
        admin.get_by_role('button', name='Сохранить исправление', exact=True).click()
        expect(admin.get_by_role('button', name='Перечитать журнал и сбросить ввод', exact=True)).to_be_visible()
        expect(admin.get_by_role('button', name='Сохранить исправление', exact=True)).to_be_disabled()
        admin.get_by_role('button', name='Перечитать журнал и сбросить ввод', exact=True).click()
        record.get_by_role('button', name='Исправить отметку', exact=True).click()
        admin.get_by_label('Новая отметка', exact=True).select_option('present')
        admin.get_by_label('Причина исправления', exact=True).fill('Уточнение администратора')
        admin.get_by_role('button', name='Сохранить исправление', exact=True).click()
        expect(record.locator('.record-history')).to_contain_text('Уточнение администратора')
        expect(record.locator('.record-history')).to_contain_text('Конкурентная проверка владельца')
        admin.reload(); expect(admin.locator('.admin-records > li').first).to_contain_text('Присутствует')
        assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}'") == '3'
        assert not errors, errors
        print(json.dumps({'engine': ENGINE, 'version': browser.version, 'result': 'PASS', 'checks': 'day/week/period, ID scope/two clubs, create/reschedule/replace/cancel, direct journal/back/reload, Escape/focus, double tap/network/reload, correction conflict/history, 320/390/1280'}, ensure_ascii=False))
        browser.close()
finally:
    STATIC_TEMP.cleanup()
    subprocess.run(['docker', 'rm', '-f', APP, DB], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    subprocess.run(['docker', 'network', 'rm', NET], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

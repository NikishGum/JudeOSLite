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
            for width in (320, 390, 768, 1280, 1440):
                value.set_viewport_size({'width': width, 'height': 900})
                settle_shell(value, width)
                value.wait_for_load_state('networkidle')
                assert value.evaluate('document.documentElement.scrollWidth <= innerWidth'), (name, width)
                assert_page_targets(value, width, extra_selector=',.session-link:visible,.session-card-link:visible')
                for target in value.locator('.session-calendar .date-step:visible,.session-calendar .date-select-trigger:visible').all():
                    box = target.bounding_box()
                    assert box['width'] >= 44 and box['height'] >= 44, box
                if name == 'weekly-calendar':
                    layout = value.locator('.session-days').evaluate('''e => ({
                        columns: getComputedStyle(e).gridTemplateColumns.split(' ').length,
                        days: [...e.children].map(day => ({x: day.getBoundingClientRect().x, y: day.getBoundingClientRect().y,
                            background: getComputedStyle(day).backgroundColor})),
                    })''')
                    assert layout['columns'] == (7 if width >= 1280 else 1), (width, layout)
                    assert all(day['background'] != 'rgba(0, 0, 0, 0)' for day in layout['days']), layout
                    if width >= 1280:
                        assert max(day['y'] for day in layout['days']) - min(day['y'] for day in layout['days']) < 1, layout
                        assert len({round(day['x']) for day in layout['days']}) == 7, layout
                    if width <= 390:
                        toggle = value.locator('.session-filter-toggle')
                        expect(toggle).to_have_attribute('aria-expanded', 'false')
                        expect(value.get_by_label('Группа календаря', exact=True)).not_to_be_visible()
                        toggle.click()
                        expect(value.get_by_label('Группа календаря', exact=True)).to_be_visible()
                        toggle.click()
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
        expect(admin.get_by_role('radio', name='Неделя', exact=True)).to_be_checked()
        expect(admin.locator('.session-day')).to_have_count(7)
        # Week step and counters share the same filtered dataset; no athlete API.
        calendar_reads = []
        admin.on('request', lambda r: calendar_reads.append(r.url) if '/api/clubs/' in r.url else None)
        admin.locator('.session-calendar').get_by_role('button', name='Вперёд', exact=True).click()
        expect(admin.locator('.session-list > li')).to_have_count(0)
        admin.locator('.session-calendar').get_by_role('button', name='Назад', exact=True).click()
        expect(admin.locator('.session-list > li')).to_have_count(1)
        assert not any('/journal' in u or '/participants' in u for u in calendar_reads), calendar_reads
        expect(admin.locator('.session-calendar')).not_to_contain_text('Алексей · контроль')
        admin.get_by_label('Статус занятия', exact=True).select_option('cancelled')
        expect(admin.locator('.session-list > li')).to_have_count(0)
        expect(admin.locator('.session-list-heading')).to_contain_text('Занятий за период: 0')
        assert sum(int(x.rsplit(': ',1)[1]) for x in admin.locator('.session-day-heading small').all_text_contents()) == 0
        admin.get_by_label('Статус занятия', exact=True).select_option('')
        admin.get_by_label('Фактический тренер', exact=True).select_option(trainer['id'])
        expect(admin.locator('.session-list > li')).to_have_count(1)
        admin.get_by_label('Фактический тренер', exact=True).select_option('')
        start = today-timedelta(days=today.weekday())
        assert 'from='+str(start) in admin.url
        assert 'through='+str(start+timedelta(days=6)) in admin.url
        shots(admin, 'weekly-calendar')
        admin.get_by_role('radio', name='День', exact=True).click()
        admin.get_by_role('button', name='Сегодня', exact=True).click()
        admin.get_by_role('button', name='Сбросить фильтры · Все группы', exact=True).click()
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
        admin.get_by_role('radio', name='Период', exact=True).click()
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
        # Clicking the status area of the card must open the same permanent page as its time.
        tr.locator(f'[data-session-id="{sid}"] .training-status').click()
        expect(tr).to_have_url(__import__('re').compile('/sessions/'+sid))
        expect(tr.get_by_role('button', name='Подтвердить открытие журнала', exact=True)).to_be_visible()
        expect(tr.locator('.session-roster')).to_contain_text('Предварительный состав')
        expect(tr.locator('.session-roster-list > li')).to_have_count(3)
        expect(tr.locator('.session-detail-heading')).not_to_contain_text('Тренер:')
        tr.get_by_label('Поиск по составу', exact=True).fill('Борис')
        expect(tr.locator('.session-roster-list > li')).to_have_count(1)
        tr.get_by_label('Поиск по составу', exact=True).fill('')
        # The left roster switches the read-only athlete area on the right; it does not open the journal.
        tr.get_by_role('button', name='Посмотреть спортсмена: Борис · контроль', exact=True).click()
        expect(tr.locator('.session-member-panel h3')).to_have_text('Борис · контроль')
        expect(tr.locator('.session-member-panel')).to_contain_text('предварительном составе')
        expect(tr.get_by_role('button', name='Посмотреть спортсмена: Борис · контроль', exact=True)).to_have_attribute('aria-pressed', 'true')
        tr.get_by_role('button', name='Посмотреть спортсмена: Виктор · контроль', exact=True).click()
        expect(tr.locator('.session-member-panel h3')).to_have_text('Виктор · контроль')
        assert sql(f"SELECT (roster_frozen_at IS NULL)::text || '|' || version FROM pride.training_sessions WHERE id='{sid}'") == 'true|1'
        live_group = api(own, 'POST', base+'/groups', {'name': 'Текущая тренировка · контроль'})
        live_sid = sql(f"INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES('{a['id']}','{live_group['id']}','{trainer['id']}',((now()-interval '5 minutes') AT TIME ZONE 'Europe/Minsk')::date,'Europe/Minsk',now()-interval '5 minutes',now()+interval '55 minutes') RETURNING id").splitlines()[0]
        tr.reload()
        current_link = tr.get_by_role('link', name='К текущей тренировке', exact=True)
        expect(current_link).to_be_visible()
        current_link.click()
        expect(tr).to_have_url(ROOT+'/clubs/'+a['id']+'/sessions/'+live_sid+'?journal=trainer')
        expect(tr.locator('.session-detail-heading')).to_contain_text(live_group['name'])
        expect(tr.get_by_role('link', name='К текущей тренировке', exact=True)).to_have_count(0)
        assert sql(f"SELECT status || '|' || (roster_frozen_at IS NULL)::text FROM pride.training_sessions WHERE id='{live_sid}'") == 'planned|true'
        tr.go_back()
        expect(tr.locator('.session-roster-list > li')).to_have_count(3)
        tr.get_by_role('button', name='Посмотреть спортсмена: Виктор · контроль', exact=True).click()
        shots(tr, 'selected-athlete')
        layout = tr.locator('.session-detail-layout').evaluate('e => [...e.children].map(child => child.getBoundingClientRect().x)')
        assert layout[0] < layout[1], layout
        tr.get_by_role('button', name='Вернуться к журналу', exact=True).click()
        # Finishing an interval removes it from the current-training shortcut.
        live_journal = api(tr, 'POST', base+'/trainer-sessions/'+live_sid+'/journal/open', {'version': 1})
        api(tr, 'POST', base+'/trainer-sessions/'+live_sid+'/journal/close', {'version': live_journal['session']['version']})
        tr.reload()
        expect(tr.get_by_role('link', name='К текущей тренировке', exact=True)).to_have_count(0)
        expect(tr.locator('.session-roster-list > li')).to_have_count(3)
        assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}'") == '0'
        assert sql(f"SELECT (roster_frozen_at IS NULL)::text || '|' || version FROM pride.training_sessions WHERE id='{sid}'") == 'true|1'
        shots(tr, 'planned-session')
        assert sql(f"SELECT status FROM pride.training_sessions WHERE id='{sid}'") == 'planned'
        tr.get_by_role('button', name='Подтвердить открытие журнала', exact=True).click()
        expect(tr.locator('.attendance-card')).to_contain_text('Алексей')
        expect(tr.locator('.session-roster')).to_contain_text('Зафиксированный состав')
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
        tr.get_by_role('link', name='К расписанию', exact=True).click()
        expect(tr.get_by_label('Группа календаря', exact=True)).to_have_value(group['id'])
        tr.go_back(); expect(tr.locator('.session-journal')).to_contain_text('Журнал завершён')
        tr.go_forward(); expect(tr.locator('.session-calendar')).to_be_visible()
        # Legacy query link normalizes to the canonical page without selecting a fallback.
        admin.goto(calendar+'&session='+sid)
        expect(admin).to_have_url(__import__('re').compile('/sessions/'+sid))
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
        # Direct entry without a calendar uses the actual session week on return.
        direct = ROOT+'/clubs/'+a['id']+'/sessions/'+sid
        admin.goto(direct)
        expect(admin.locator('.admin-records > li')).to_have_count(3)
        admin.reload(); expect(admin.locator('.admin-records > li')).to_have_count(3)
        admin.get_by_role('link', name='К расписанию', exact=True).click()
        expect(admin.get_by_role('radio', name='Неделя', exact=True)).to_be_checked()
        admin.go_back(); expect(admin.locator('.admin-records > li')).to_have_count(3)
        admin.goto(ROOT+'/clubs/'+b['id']+'/sessions/'+sid)
        expect(admin.get_by_role('alert')).to_be_visible()
        expect(admin.locator('.admin-records > li')).to_have_count(0)
        # Forty-five participants are a stress case, not a product limit.
        for i in range(42):
            api(own, 'POST', base+'/participants', {'identity': {'fullName': f'Стресс участник {i:02} с длинным именем', 'birthDate': '2010-01-01'}, 'joinedOn': '2020-01-01', 'contactKind': 'phone', 'contactValue': '+375291234567', 'initialGroup': {'groupId': group['id'], 'from': '2020-01-01'}})
        stress = sql(f"INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES('{a['id']}','{group['id']}','{trainer['id']}','{tomorrow}','Europe/Minsk','{tomorrow} 20:00+03','{tomorrow} 21:30+03') RETURNING id").splitlines()[0]
        tr.goto(ROOT+'/clubs/'+a['id']+'/sessions/'+stress)
        expect(tr.locator('.session-roster-list > li')).to_have_count(45)
        expect(tr.get_by_role('button', name='Подтвердить открытие журнала', exact=True)).to_be_disabled()
        tr.get_by_label('Поиск по составу', exact=True).fill('участник 41')
        expect(tr.locator('.session-roster-list > li')).to_have_count(1)
        tr.get_by_label('Поиск по составу', exact=True).fill('такого имени нет')
        expect(tr.locator('.session-roster')).to_contain_text('По этому имени никого нет')
        assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{stress}'") == '0'
        # Cancellation before open is preliminary data, never an empty snapshot.
        stress_journal = api(own, 'GET', base+'/journals/'+stress)
        api(own, 'POST', base+'/sessions/'+stress+'/cancel', {'version': stress_journal['session']['version'], 'reason': 'Synthetic cancellation before freeze'})
        tr.reload()
        expect(tr.locator('.session-journal')).to_contain_text('Снимок состава не создавался')
        expect(tr.locator('.session-roster')).to_contain_text('не история посещения')
        expect(tr.locator('.session-roster-list > li')).to_have_count(45)
        expect(tr.locator('.attendance-cards')).to_have_count(0)
        assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{stress}'") == '0'
        # Calendar boundaries and the inclusive 366-day limit.
        admin.goto(ROOT+'/clubs/'+a['id']+'/sessions?from=2026-12-28&through=2027-01-03&view=week')
        expect(admin.locator('.session-day')).to_have_count(7)
        admin.locator('.session-calendar').get_by_role('button', name='Вперёд', exact=True).click()
        assert 'from=2027-01-04' in admin.url
        admin.locator('.session-calendar').get_by_role('button', name='Назад', exact=True).click()
        assert 'through=2027-01-03' in admin.url
        admin.goto(ROOT+'/clubs/'+a['id']+'/sessions?from=2025-01-01&through=2026-01-01&view=period')
        expect(admin.locator('.session-calendar [role="alert"]')).to_have_count(0)
        expect(admin.get_by_role('button', name='Добавить разовое занятие', exact=True)).to_be_enabled()
        admin.get_by_label('По дату включительно', exact=True).fill('2026-01-02')
        expect(admin.locator('.session-calendar [role="alert"]')).to_be_visible()
        admin.goto(ROOT+'/clubs/'+a['id']+'/sessions?status=invalid')
        expect(admin.locator('.session-calendar [role="alert"]')).to_be_visible()
        admin.get_by_role('button', name='Сбросить фильтры · Все группы', exact=True).click()
        expect(admin.locator('.session-calendar [role="alert"]')).to_have_count(0)
        expect(admin.get_by_role('button', name='Добавить разовое занятие', exact=True)).to_be_enabled()
        # Unknown IDs cannot fall back to an available session.
        admin.goto(ROOT+'/clubs/'+a['id']+'/sessions/'+str(uuid.uuid4()))
        expect(admin.locator('.session-detail [role="alert"]')).to_be_visible()
        expect(admin.locator('.admin-records > li')).to_have_count(0)
        assert not errors, errors
        print(json.dumps({'engine': ENGINE, 'version': browser.version, 'result': 'PASS', 'checks': 'day/week/period, ID scope/two clubs, create/reschedule/replace/cancel, direct journal/back/reload, Escape/focus, double tap/network/reload, correction conflict/history, preview/snapshot/search/45 participants, no calendar N+1, canonical/legacy links, 320/390/768/1280/1440'}, ensure_ascii=False))
        browser.close()
finally:
    STATIC_TEMP.cleanup()
    subprocess.run(['docker', 'rm', '-f', APP, DB], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    subprocess.run(['docker', 'network', 'rm', NET], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

from runtime_ui import observe_api, wait_for_api_idle, assert_page_targets, settle_shell, signout
from staff_ui import select_employee, add_assignment, discard_staff_form, toggle_access
from redesign_accessibility import contrast_audit
"""UX-STAFF #215: real browser -> Go -> disposable PostgreSQL. No external club data."""
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
from urllib.request import urlopen
from playwright.sync_api import sync_playwright, expect

ROOT = 'http://127.0.0.1:18089'
REPO = Path(__file__).resolve().parents[2]
ENGINE = os.environ.get('PWA_BROWSER', 'chromium')
IMAGE = os.environ.get('PWA_TEST_IMAGE', 'judeoslite:ci')
assert ENGINE in ('chromium', 'webkit') and IMAGE.startswith('judeoslite:')
PREFIX = 'staff215-' + uuid.uuid4().hex[:8]
DB, APP, NET = PREFIX+'-db', PREFIX+'-app', PREFIX+'-net'
PASSWORD = 'synthetic-browser-password'
STATIC_TEMP = tempfile.TemporaryDirectory(prefix='staff215-static-')
static = Path(STATIC_TEMP.name)/'static'
shutil.copytree(REPO/'frontend/dist', static)
for file in (static, *static.rglob('*')):
    file.chmod(0o755 if file.is_dir() else 0o644)

def cmd(*args):
    return subprocess.check_output(args, text=True).strip()

def sql(statement):
    return cmd('docker', 'exec', DB, 'psql', '-U', 'test', '-d', 'staff215', '-Atc', statement)

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
    cmd('docker', 'run', '-d', '--name', DB, '--network', NET, '-e', 'POSTGRES_USER=test', '-e', 'POSTGRES_PASSWORD=test-only', '-e', 'POSTGRES_DB=staff215', 'postgres:18.6-bookworm')
    for _ in range(30):
        if subprocess.run(['docker', 'exec', DB, 'pg_isready', '-U', 'test', '-d', 'staff215'], stdout=subprocess.DEVNULL).returncode == 0:
            break
        time.sleep(1)
    else:
        raise AssertionError('PostgreSQL not ready')
    env = ['-e', f'DATABASE_URL=postgres://test:test-only@{DB}:5432/staff215?sslmode=disable']
    cmd('docker', 'run', '--rm', '--network', NET, *env, IMAGE, '/app/migrate')
    owner(env)
    cmd('docker', 'create', '--name', APP, '--network', NET, *env, '-e', f'APP_ORIGIN={ROOT}', '-e', 'AUTH_INSECURE_LOCAL=true', '-p', '127.0.0.1:18089:8080', IMAGE)
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
    # Each run owns its database; parent-only is a real shared-auth identity.
    with sync_playwright() as p:
        browser = getattr(p, ENGINE).launch(headless=True, executable_path=os.environ.get('CHROMIUM_PATH') if ENGINE == 'chromium' else None)
        errors = []
        contexts = []
        def signin(value, login, password=PASSWORD):
            value.goto(ROOT)
            value.wait_for_load_state('networkidle'); wait_for_api_idle(value)
            value.get_by_label('Логин', exact=True).fill(login)
            value.get_by_label('Пароль', exact=True).fill(password)
            value.get_by_role('button', name='Войти', exact=True).click()
            expect(value.get_by_role('button', name='Меню профиля', exact=True)).to_be_visible()
            wait_for_api_idle(value)
        def page(login):
            context = browser.new_context(viewport={'width': 1440, 'height': 1000}, service_workers='block', reduced_motion='reduce')
            contexts.append(context)
            value = context.new_page(); observe_api(value)
            value.on('pageerror', lambda cause: errors.append(str(cause)))
            signin(value, login)
            return value
        def api(value, method, path, data=None, want=200):
            result = value.evaluate('''async ({method,path,data}) => {
              const me=await (await fetch('/api/auth/me',{cache:'no-store'})).json();
              const response=await fetch(path,{method,cache:'no-store',headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrfToken},body:data===null?undefined:JSON.stringify(data)});
              return {status:response.status,body:await response.json(),cache:response.headers.get('Cache-Control')};
            }''', dict(method=method, path=path, data=data))
            assert result['status'] == want, (method, path, result)
            assert result['cache'] == 'no-store', (path, result['cache'])
            return result['body']
        ui = page('owner')
        a = api(ui, 'POST', '/api/clubs', dict(name='Synthetic · Центральный', address='Синтетический зал', timezone='Europe/Minsk'), 201)
        b = api(ui, 'POST', '/api/clubs', dict(name='Synthetic · Восточный', address='', timezone='Europe/Minsk'), 201)
        parent_id = sql("INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT 'parent-only','Synthetic Parent Only',password_hash FROM pride.staff_users WHERE is_owner RETURNING id").splitlines()[0]
        family_id = sql(f"INSERT INTO pride.families(club_id,name) VALUES('{a['id']}','Synthetic private family') RETURNING id").splitlines()[0]
        sql(f"INSERT INTO pride.family_access(family_id,user_id) VALUES('{family_id}','{parent_id}')")
        ui.goto(ROOT+'/staff')
        expect(ui.get_by_text('Пока нет сотрудников', exact=True)).to_be_visible()
        assert api(ui, 'GET', '/api/staff-directory')['staff'] == []
        assert api(ui, 'GET', '/api/staff') == [] and api(ui, 'GET', '/api/trainer-accounts') == []
        panel = ui.get_by_role('dialog')
        def create_ui(login, name, club_id, role='trainer'):
            ui.get_by_role('button', name='Добавить сотрудника', exact=True).click()
            panel.get_by_label('Имя сотрудника', exact=True).fill(name)
            panel.get_by_label('Логин сотрудника', exact=True).fill(login)
            panel.get_by_label('Первоначальный пароль', exact=True).fill(PASSWORD)
            panel.get_by_label('Повторите пароль', exact=True).fill(PASSWORD)
            panel.get_by_label('Роль', exact=True).select_option(role)
            panel.get_by_label('Филиал назначения', exact=True).select_option(club_id)
            panel.get_by_role('button', name='Создать сотрудника', exact=True).click()
            expect(panel).to_have_count(0)
            expect(ui.locator('.staff-access-feedback')).to_contain_text('Сотрудник создан')
            return next(person for person in api(ui, 'GET', '/api/staff-directory')['staff'] if person['login'] == login)
        admin = create_ui('local-admin', 'Synthetic Администратор', a['id'], 'administrator')
        long_name = ('Synthetic Тренер с длинным именем и фамилией ' + 'Александрович ' * 8).strip()
        coach = create_ui('staff-coach', long_name, a['id'])
        trainer = page('staff-coach')
        api(trainer, 'GET', '/api/staff-directory', want=403)
        # Explicit existing selection adds one role, never a duplicate identity.
        ui.get_by_role('button', name='Добавить сотрудника', exact=True).click()
        panel.get_by_label('Способ добавления', exact=True).select_option('existing')
        panel.get_by_label('Аккаунт сотрудника', exact=True).select_option(coach['id'])
        panel.get_by_label('Роль', exact=True).select_option('administrator')
        panel.get_by_label('Филиал назначения', exact=True).select_option(b['id'])
        panel.get_by_role('button', name='Добавить назначение', exact=True).click()
        expect(panel).to_have_count(0)
        expect(ui.locator('.staff-access-assignments li')).to_have_count(2)
        assert sql('SELECT count(*) FROM pride.staff_users') == '4'
        # Role+club are evaluated in a single assignment.
        ui.locator('.staff-access-filters').get_by_label('Роль', exact=True).select_option('trainer')
        ui.locator('.staff-access-filters').get_by_label('Филиал сотрудника', exact=True).select_option(b['id'])
        expect(ui.locator('.staff-access-list li')).to_have_count(0)
        expect(ui.locator('.staff-access-filter-notice')).to_contain_text(long_name)
        expect(ui.locator('.staff-access-card h2')).to_have_text(long_name)
        ui.get_by_role('button', name='Показать в списке', exact=True).click()
        expect(ui.locator('.staff-access-list li')).to_have_count(2)
        # Duplicate login is an atomic failure, preserving the form and identity.
        ui.get_by_role('button', name='Добавить сотрудника', exact=True).click()
        panel.get_by_label('Имя сотрудника', exact=True).fill('Synthetic duplicate')
        panel.get_by_label('Логин сотрудника', exact=True).fill('staff-coach')
        panel.get_by_label('Первоначальный пароль', exact=True).fill(PASSWORD)
        panel.get_by_label('Повторите пароль', exact=True).fill(PASSWORD)
        panel.get_by_label('Филиал назначения', exact=True).select_option(a['id'])
        panel.get_by_role('button', name='Создать сотрудника', exact=True).click()
        expect(panel.get_by_role('alert')).to_contain_text('уже занят')
        expect(panel.get_by_label('Имя сотрудника', exact=True)).to_have_value('Synthetic duplicate')
        expect(panel.get_by_label('Первоначальный пароль', exact=True)).to_have_value('')
        assert sql('SELECT count(*) FROM pride.staff_users') == '4'
        discard_staff_form(ui)
        # Admin sees only allowed trainers; a mixed/multirole account is read-only.
        local = page('local-admin'); local.goto(ROOT+'/staff')
        expect(local.locator('.staff-access-person')).to_have_count(1)
        select_employee(local, coach['id'])
        expect(local.get_by_role('button', name='Сменить пароль', exact=True)).to_have_count(0)
        local_directory = api(local, 'GET', '/api/staff-directory')
        assert len(local_directory['clubs']) == 1 and len(local_directory['staff'][0]['assignments']) == 1
        assert b['id'] not in json.dumps(local_directory) and family_id not in json.dumps(local_directory)
        api(local, 'POST', f"/api/staff/{coach['id']}/administrator-assignments", dict(clubId=a['id'], version=2), 403)
        parent = page('parent-only')
        api(parent, 'GET', '/api/staff-directory', want=403)
        assert api(trainer, 'GET', '/api/staff-directory')['clubs'][0]['id'] == b['id']
        # Cancel preserves server state and returns focus to the same action.
        select_employee(ui, coach['id'])
        ui.get_by_role('button', name='Изменить данные', exact=True).click()
        panel.get_by_label('Имя сотрудника', exact=True).fill('Unsaved synthetic draft')
        panel.get_by_role('button', name='Отмена', exact=True).click()
        leave = ui.get_by_role('dialog', name='Есть несохранённые изменения', exact=True)
        expect(leave).to_be_visible()
        leave.get_by_role('button', name='Продолжить редактирование', exact=True).click()
        expect(panel.get_by_label('Имя сотрудника', exact=True)).to_have_value('Unsaved synthetic draft')
        discard_staff_form(ui)
        expect(ui.get_by_role('button', name='Изменить данные', exact=True)).to_be_focused()
        assert next(v for v in api(ui, 'GET', '/api/staff-directory')['staff'] if v['id'] == coach['id'])['displayName'] == long_name
        # Version conflict preserves a draft and requires explicit server reload.
        ui.get_by_role('button', name='Изменить данные', exact=True).click()
        panel.get_by_label('Имя сотрудника', exact=True).fill('Stale synthetic draft')
        current = next(v for v in api(ui, 'GET', '/api/staff-directory')['staff'] if v['id'] == coach['id'])
        api(ui, 'PUT', f"/api/staff/{coach['id']}/account", dict(login=current['login'], displayName=current['displayName'], disabled=False, version=current['version']))
        panel.get_by_role('button', name='Сохранить данные', exact=True).click()
        expect(panel.get_by_role('alert')).to_contain_text('уже изменена')
        expect(panel.get_by_label('Имя сотрудника', exact=True)).to_have_value('Stale synthetic draft')
        panel.get_by_role('button', name='Загрузить актуальные данные', exact=True).click()
        ui.get_by_role('dialog', name='Есть несохранённые изменения', exact=True).get_by_role('button', name='Перейти без сохранения', exact=True).click()
        expect(panel).to_have_count(0)
        # Login edit and password change revoke actual existing sessions.
        ui.get_by_role('button', name='Изменить данные', exact=True).click()
        panel.get_by_label('Логин сотрудника', exact=True).fill('renamed-coach')
        panel.get_by_role('button', name='Сохранить данные', exact=True).click()
        expect(panel).to_have_count(0)
        api(trainer, 'GET', '/api/auth/me', want=401)
        signin(trainer, 'renamed-coach')
        ui.get_by_role('button', name='Сменить пароль', exact=True).click()
        panel.get_by_label('Новый пароль', exact=True).fill(PASSWORD+'-new')
        panel.get_by_label('Повторите пароль', exact=True).fill(PASSWORD+'-new')
        panel.get_by_role('button', name='Сменить пароль', exact=True).click()
        expect(panel).to_have_count(0)
        api(trainer, 'GET', '/api/auth/me', want=401)
        signin(trainer, 'renamed-coach', PASSWORD+'-new')
        toggle_access(ui, coach['id'], False)
        api(trainer, 'GET', '/api/auth/me', want=401)
        # Disabling changes filtering, never the card's recipient.
        ui.locator('.staff-access-filters').get_by_label('Состояние доступа', exact=True).select_option('enabled')
        expect(ui.locator('.staff-access-filter-notice')).to_contain_text(long_name)
        expect(ui.locator('.staff-access-card h2')).to_have_text(long_name)
        ui.get_by_role('button', name='Показать в списке', exact=True).click()
        toggle_access(ui, coach['id'], True)
        api(trainer, 'GET', '/api/auth/me', want=401)  # enabling does not revive tokens
        # Removing one role preserves the other; last role preserves staff identity.
        revoke_admin = 'Снять назначение: Администратор · '+b['name']
        ui.get_by_role('button', name=revoke_admin, exact=True).click()
        panel.get_by_role('button', name='Отмена', exact=True).click()
        expect(ui.locator('.staff-access-assignments li')).to_have_count(2)
        ui.get_by_role('button', name=revoke_admin, exact=True).click()
        panel.get_by_role('button', name='Подтвердить снятие назначения', exact=True).click()
        expect(panel).to_have_count(0)
        expect(ui.locator('.staff-access-assignments li')).to_have_count(1)
        ui.get_by_role('button', name='Снять назначение: Тренер · '+a['name'], exact=True).click()
        panel.get_by_role('button', name='Подтвердить снятие назначения', exact=True).click()
        expect(panel).to_have_count(0)
        expect(ui.locator('.staff-access-card')).to_contain_text('Рабочие разделы недоступны')
        assert sql(f"SELECT count(*) FROM pride.staff_users WHERE id='{coach['id']}'") == '1'
        ui.reload(); expect(ui.locator(f'[data-person-id="{coach["id"]}"]')).to_be_visible()
        add_assignment(ui, coach['id'], a['id'])
        # A mixed staff+parent account exposes no family details and no local writer.
        sql(f"INSERT INTO pride.family_access(family_id,user_id) VALUES('{family_id}','{coach['id']}')")
        local.reload(); select_employee(local, coach['id'])
        expect(local.get_by_role('button', name='Изменить данные', exact=True)).to_have_count(0)
        assert family_id not in json.dumps(api(local, 'GET', '/api/staff-directory'))
        # Network failure before commit, then one submission despite double tap.
        select_employee(ui, coach['id'])
        ui.get_by_role('button', name='Изменить данные', exact=True).click()
        panel.get_by_label('Имя сотрудника', exact=True).fill('Synthetic Updated Coach')
        account_path = f"**/api/staff/{coach['id']}/account"
        ui.route(account_path, lambda route: route.abort('failed'))
        panel.get_by_role('button', name='Сохранить данные', exact=True).click()
        expect(panel.get_by_role('alert')).to_contain_text('Нет соединения')
        expect(panel.get_by_label('Имя сотрудника', exact=True)).to_have_value('Synthetic Updated Coach')
        assert sql(f"SELECT display_name FROM pride.staff_users WHERE id='{coach['id']}'") == long_name
        ui.unroute(account_path)
        requests = []
        ui.on('request', lambda request: requests.append(request.url) if request.method == 'PUT' and request.url.endswith(f"/staff/{coach['id']}/account") else None)
        panel.get_by_role('button', name='Сохранить данные', exact=True).evaluate('button=>{button.click();button.click()}')
        expect(panel).to_have_count(0)
        expect(ui.locator('.staff-access-card h2')).to_have_text('Synthetic Updated Coach')
        assert len(requests) == 1
        # A lost response after a committed write is not presented as success.
        ui.get_by_role('button', name='Изменить данные', exact=True).click()
        panel.get_by_label('Имя сотрудника', exact=True).fill(long_name)
        def lose_response(route):
            response = route.fetch(); assert response.status == 200; route.abort('failed')
        ui.route(account_path, lose_response)
        panel.get_by_role('button', name='Сохранить данные', exact=True).click()
        expect(panel.get_by_role('alert')).to_contain_text('Нет соединения')
        assert sql(f"SELECT display_name FROM pride.staff_users WHERE id='{coach['id']}'") == long_name
        assert ui.locator('.staff-access-feedback').count() == 0
        ui.unroute(account_path); discard_staff_form(ui)
        ui.get_by_role('button', name='Обновить сотрудников', exact=True).click()
        expect(ui.locator('.staff-access-card h2')).to_have_text(long_name)
        # Loading/error/retry is real: no stale-version writes after failed read.
        ui.route('**/api/staff-directory', lambda route: route.fulfill(status=503, content_type='application/json', body=json.dumps(dict(error=dict(code='not_ready', message='Synthetic directory unavailable')))))
        ui.get_by_role('button', name='Обновить сотрудников', exact=True).click()
        expect(ui.get_by_role('alert')).to_contain_text('Synthetic directory unavailable')
        expect(ui.get_by_role('button', name='Изменить данные', exact=True)).to_be_disabled()
        ui.unroute('**/api/staff-directory')
        ui.get_by_role('button', name='Повторить загрузку сотрудников', exact=True).click()
        expect(ui.get_by_role('button', name='Изменить данные', exact=True)).to_be_enabled()
        # Long list: mobile card always starts at top; back restores list context.
        for index in range(7):
            api(ui, 'POST', f"/api/clubs/{a['id']}/trainers", dict(login=f'synthetic-scroll-{index}', displayName=f'Я · Synthetic {index}', password=PASSWORD), 201)
        ui.get_by_role('button', name='Обновить сотрудников', exact=True).click()
        expect(ui.locator('.staff-access-person')).to_have_count(9)
        ui.set_viewport_size(dict(width=390, height=844)); settle_shell(ui, 390)
        back = ui.get_by_role('button', name='К списку сотрудников', exact=True)
        if back.is_visible(): back.click()
        ui.get_by_label('Поиск по имени или логину', exact=True).fill('Synthetic')
        last = ui.locator('.staff-access-person').last
        last.scroll_into_view_if_needed(); assert ui.evaluate('scrollY') > 400
        last.click()
        expect(ui.locator('.staff-access-card')).to_be_visible()
        expect(ui.locator('.staff-access-card h2')).to_be_focused()
        assert ui.evaluate('scrollY') == 0
        ui.get_by_role('button', name='К списку сотрудников', exact=True).click()
        expect(last).to_be_focused()
        expect(ui.get_by_label('Поиск по имени или логину', exact=True)).to_have_value('Synthetic')
        assert ui.evaluate('scrollY') > 400
        ui.get_by_label('Поиск по имени или логину', exact=True).fill('')
        select_employee(ui, coach['id'])
        # Phone secondary filters start collapsed and preserve active conditions.
        ui.get_by_role('button', name='К списку сотрудников', exact=True).click()
        filters_toggle = ui.get_by_role('button', name='Фильтры', exact=True)
        expect(filters_toggle).to_have_attribute('aria-expanded', 'false')
        filters_toggle.click()
        ui.locator('.staff-access-filters').get_by_label('Роль', exact=True).select_option('trainer')
        ui.locator('.staff-access-filters').get_by_label('Филиал сотрудника', exact=True).select_option(a['id'])
        ui.get_by_role('button', name='Фильтры · 2 активных', exact=True).click()
        expect(ui.locator('.staff-access-filters')).not_to_be_visible()
        select_employee(ui, coach['id'])
        ui.get_by_role('button', name='К списку сотрудников', exact=True).click()
        expect(ui.get_by_role('button', name='Фильтры · 2 активных', exact=True)).to_have_attribute('aria-expanded', 'false')
        ui.get_by_role('button', name='Сбросить фильтры', exact=True).click()
        select_employee(ui, coach['id'])
        # Keyboard focus is trapped; Escape respects a dirty draft.
        ui.get_by_role('button', name='Сменить пароль', exact=True).click()
        first = panel.locator('[data-initial-focus]'); expect(first).to_be_focused()
        panel.press('Tab'); assert ui.evaluate('!!document.activeElement.closest("dialog")')
        panel.get_by_label('Новый пароль', exact=True).fill('unsaved-password')
        panel.press('Escape')
        expect(ui.get_by_role('dialog', name='Есть несохранённые изменения', exact=True)).to_be_visible()
        ui.get_by_role('button', name='Перейти без сохранения', exact=True).click()
        expect(panel).to_have_count(0)
        # Parent-only remains outside the roster until the owner explicitly assigns it.
        evidence = REPO/'.impeccable/review/staff215'; evidence.mkdir(parents=True, exist_ok=True)
        captures = []
        ui.get_by_role('button', name='К списку сотрудников', exact=True).click()
        ui.get_by_role('button', name='Добавить сотрудника', exact=True).click()
        panel.get_by_label('Способ добавления', exact=True).select_option('existing')
        panel.get_by_label('Найти существующий аккаунт', exact=True).fill('parent-only')
        panel.get_by_role('button', name='Найти аккаунты', exact=True).click()
        expect(panel.get_by_role('status')).to_contain_text('По поиску найдено: 1')
        panel.get_by_label('Аккаунт сотрудника', exact=True).select_option(parent_id)
        expect(panel.locator('.staff-access-selected')).to_contain_text('Synthetic Parent Only')
        expect(panel.locator('.staff-access-selected')).to_contain_text('Логин: parent-only')
        panel.get_by_label('Филиал назначения', exact=True).select_option(a['id'])
        assert all(person['id'] != parent_id for person in api(ui, 'GET', '/api/staff-directory')['staff'])
        for width in (1440, 1280, 390, 320):
            ui.set_viewport_size(dict(width=width, height=1000 if width > 760 else 844)); settle_shell(ui, width)
            assert_page_targets(ui, width)
            contrast = contrast_audit(ui)
            assert not contrast['failures'] and not contrast['unmeasured'], ('existing', width, contrast)
            if os.environ.get('STAFF_CAPTURE') == '1':
                path = evidence/f'{ENGINE}-existing-{width}.png'; ui.screenshot(path=str(path)); captures.append(path.name)
        users_before = sql('SELECT count(*) FROM pride.staff_users')
        panel.get_by_role('button', name='Добавить назначение', exact=True).click()
        expect(panel).to_have_count(0)
        expect(ui.locator('.staff-access-card h2')).to_have_text('Synthetic Parent Only')
        assert sql('SELECT count(*) FROM pride.staff_users') == users_before
        assert sql(f"SELECT count(*) FROM pride.family_access WHERE user_id='{parent_id}'") == '1'
        assert any(person['id'] == parent_id for person in api(ui, 'GET', '/api/staff-directory')['staff'])
        select_employee(ui, coach['id'])
        # One batched visual round, at the changed device classes and widths.
        for width in (1440, 1280, 390, 320):
            ui.set_viewport_size(dict(width=width, height=1000 if width > 760 else 844)); settle_shell(ui, width)
            if ui.get_by_role('button', name='К списку сотрудников', exact=True).is_visible():
                ui.get_by_role('button', name='К списку сотрудников', exact=True).click()
            for state in ('list', 'card', 'form'):
                if state == 'card': select_employee(ui, coach['id'])
                if state == 'form': ui.get_by_role('button', name='Изменить данные', exact=True).click()
                wait_for_api_idle(ui); ui.evaluate('scrollTo(0,0)'); ui.mouse.move(0, 0)
                assert ui.evaluate('document.documentElement.scrollWidth <= innerWidth'), (width, state)
                assert_page_targets(ui, width)
                if state == 'list' and width <= 390:
                    first_row = ui.locator('.staff-access-person').first
                    expect(first_row).to_be_in_viewport(ratio=1)
                    assert first_row.bounding_box()['y'] + first_row.bounding_box()['height'] <= ui.locator('.bottom-navigation').bounding_box()['y']
                contrast = contrast_audit(ui)
                assert not contrast['failures'] and not contrast['unmeasured'], (width, state, contrast)
                if os.environ.get('STAFF_CAPTURE') == '1':
                    path = evidence/f'{ENGINE}-{state}-{width}.png'; ui.screenshot(path=str(path), full_page=state != 'form'); captures.append(path.name)
                    if state == 'list' and width <= 390:
                        ui.locator('.staff-access-filter-toggle').click()
                        filters = ui.locator('.staff-access-filters')
                        filters.get_by_label('Роль', exact=True).select_option('trainer')
                        filters.get_by_label('Филиал сотрудника', exact=True).select_option(a['id'])
                        for filter_state in ('expanded', 'active'):
                            if filter_state == 'active': ui.locator('.staff-access-filter-toggle').click()
                            path = evidence/f'{ENGINE}-filters-{filter_state}-{width}.png'
                            ui.evaluate('scrollTo(0,0)'); ui.screenshot(path=str(path), full_page=True); captures.append(path.name)
                        ui.get_by_role('button', name='Сбросить фильтры', exact=True).click()
                if state == 'form': discard_staff_form(ui)
        if captures: (evidence/f'{ENGINE}-screenshots.json').write_text(json.dumps(captures, indent=2)+'\n')
        # A late old-scope response after leaving cannot re-open the old registry.
        ui.set_viewport_size(dict(width=1440, height=1000)); settle_shell(ui, 1440)
        held = []
        ui.route('**/api/staff-directory', lambda route: held.append((route, route.fetch())))
        ui.get_by_role('button', name='Обновить сотрудников', exact=True).click()
        expect(ui.get_by_text('Загружаем сотрудников и назначения…', exact=True)).to_be_visible()
        ui.get_by_role('link', name='Профиль', exact=True).click()
        expect(ui.locator('.staff-access')).to_have_count(0)
        assert len(held) == 1
        try: held[0][0].fulfill(response=held[0][1])
        except Exception as cause:
            assert 'cancel' in str(cause).lower() or 'interception' in str(cause).lower() or 'closed' in str(cause).lower(), str(cause)
        ui.unroute('**/api/staff-directory'); wait_for_api_idle(ui)
        expect(ui.locator('.staff-access')).to_have_count(0)
        assert not errors, errors
        assert sql("SELECT count(*) FROM pride.audit_events WHERE entity_type='staff_user' AND (after_state::text LIKE '%synthetic-browser-password%' OR after_state::text LIKE '%renamed-coach%' OR after_state::text LIKE '%Synthetic%')") == '0'
        print(json.dumps(dict(result='PASS', engine=ENGINE, version=browser.version, roles='owner/admin/trainer/parent/mixed', staff=10, jsErrors=errors, captures=captures)))
        for context in contexts: context.close()
        browser.close()
finally:
    for name in (APP, DB): subprocess.run(['docker', 'rm', '-f', name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    subprocess.run(['docker', 'network', 'rm', NET], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    STATIC_TEMP.cleanup()

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
REPO = Path('/workspace/JudeOSLite')
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
        a = api(ui, 'POST', '/api/clubs', dict(name='Synthetic · Центральный', address='', timezone='Europe/Minsk'), 201)
        parent_id = sql("INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT 'parent-only','Synthetic Parent Only',password_hash FROM pride.staff_users WHERE is_owner RETURNING id").splitlines()[0]
        ui.goto(ROOT+'/staff')
        ui.get_by_role('button', name='Добавить сотрудника', exact=True).click()
        panel=ui.get_by_role('dialog')
        panel.get_by_label('Способ добавления', exact=True).select_option('existing')
        expect(panel.get_by_label('Способ добавления', exact=True).locator('option:checked')).to_have_text('Существующий аккаунт')
        panel.get_by_label('Найти существующий аккаунт', exact=True).fill('parent-only')
        panel.get_by_role('button', name='Найти аккаунты', exact=True).click()
        expect(panel.get_by_role('status')).to_contain_text('По поиску найдено: 1')
        panel.get_by_label('Аккаунт сотрудника', exact=True).select_option(parent_id)
        panel.get_by_label('Филиал назначения', exact=True).select_option(a['id'])
        expect(panel.locator('.staff-access-selected')).to_contain_text('Логин: parent-only')
        for width in (1440,1280,390,320):
            ui.set_viewport_size(dict(width=width,height=1000 if width>760 else 844));settle_shell(ui,width)
            assert ui.evaluate('document.documentElement.scrollWidth <= innerWidth')
            assert_page_targets(ui,width)
            contrast=contrast_audit(ui);assert not contrast['failures'] and not contrast['unmeasured'],contrast
            panel.evaluate('element=>element.scrollTop=0')
            ui.screenshot(path=str(REPO/f'.impeccable/review/staff215/chromium-existing-{width}.png'))
        discard_staff_form(ui)
        assert api(ui,'GET','/api/staff-directory')['staff']==[]
        assert not errors,errors
        print(json.dumps(dict(result='PASS',scope='existing-account copy + full recipient identity',engine=ENGINE,version=browser.version,jsErrors=errors)))
        browser.close()
finally:
    for name in (APP, DB): subprocess.run(['docker', 'rm', '-f', name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    subprocess.run(['docker', 'network', 'rm', NET], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    STATIC_TEMP.cleanup()

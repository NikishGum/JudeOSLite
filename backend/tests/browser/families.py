"""RD-09: real browser -> Go -> disposable PostgreSQL. No external club data."""
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

ROOT = 'http://127.0.0.1:18089'
REPO = Path(__file__).resolve().parents[3]
ENGINE = os.environ.get('PWA_BROWSER', 'chromium')
IMAGE = os.environ.get('PWA_TEST_IMAGE', 'judeoslite:ci')
assert ENGINE in ('chromium', 'webkit') and IMAGE.startswith('judeoslite:')
PREFIX = 'rd201-' + uuid.uuid4().hex[:8]
DB, APP, NET = PREFIX+'-db', PREFIX+'-app', PREFIX+'-net'
PASSWORD = 'synthetic-browser-password'
STATIC_TEMP = tempfile.TemporaryDirectory(prefix='rd201-static-')
static = Path(STATIC_TEMP.name)/'static'
shutil.copytree(REPO/'frontend/dist', static)
for file in (static, *static.rglob('*')):
    file.chmod(0o755 if file.is_dir() else 0o644)

def cmd(*args):
    return subprocess.check_output(args, text=True).strip()

def sql(statement):
    return cmd('docker', 'exec', DB, 'psql', '-U', 'test', '-d', 'rd201', '-Atc', statement)

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
    cmd('docker', 'run', '-d', '--name', DB, '--network', NET, '-e', 'POSTGRES_USER=test', '-e', 'POSTGRES_PASSWORD=test-only', '-e', 'POSTGRES_DB=rd201', 'postgres:18.6-bookworm')
    for _ in range(30):
        if subprocess.run(['docker', 'exec', DB, 'pg_isready', '-h', '127.0.0.1', '-U', 'test', '-d', 'rd201'], stdout=subprocess.DEVNULL).returncode == 0:
            break
        time.sleep(1)
    else:
        raise AssertionError('PostgreSQL not ready')
    env = ['-e', f'DATABASE_URL=postgres://test:test-only@{DB}:5432/rd201?sslmode=disable']
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
        raise AssertionError(cmd('docker','logs',APP))
    with sync_playwright() as p:
        chromium_path = os.environ.get('CHROMIUM_PATH') or ('/usr/bin/chromium' if Path('/usr/bin/chromium').exists() else None)
        browser = getattr(p, ENGINE).launch(headless=True, executable_path=chromium_path if ENGINE == 'chromium' else None)
        context = browser.new_context(service_workers='block')
        page = context.new_page()
        errors = []
        page.on('pageerror', lambda error: errors.append(str(error)))
        page.goto(ROOT)
        page.get_by_label('Логин', exact=True).fill('owner')
        page.get_by_label('Пароль', exact=True).fill(PASSWORD)
        page.get_by_role('button', name='Войти', exact=True).click()
        expect(page.get_by_role('button', name='Меню профиля', exact=True)).to_be_visible()

        def api(value, method, path, data=None, expected=200, csrf=True):
            result = value.evaluate('''async ([method,path,data,csrf]) => {
              const me=await fetch('/api/auth/me',{cache:'no-store'}).then(r=>r.json());
              const response=await fetch(path,{method,headers:{'Content-Type':'application/json',...(csrf?{'X-CSRF-Token':me.csrfToken}:{})},...(data===null?{}:{body:JSON.stringify(data)})});
              return {status:response.status,cache:response.headers.get('Cache-Control'),body:await response.json()};
            }''', [method, path, data, csrf])
            assert result['status'] == expected, result
            assert result['cache'] == 'no-store', result
            return result['body']

        club = api(page, 'POST', '/api/clubs', {'name':'Synthetic RD201 A','address':'','timezone':'Europe/Minsk','version':0},201)['id']
        other = api(page, 'POST', '/api/clubs', {'name':'Synthetic RD201 B','address':'','timezone':'Europe/Minsk','version':0},201)['id']
        children = []
        for i in range(3):
            value = api(page,'POST',f'/api/clubs/{club if i<2 else other}/participants',{
                'identity':{'fullName':'Synthetic Child','birthDate':'2015-01-01','version':0},
                'joinedOn':'2020-01-01','contactKind':'email','contactValue':'synthetic@example.test','contactName':'Synthetic Contact','version':0},201)
            children.append(value['id'])
        families = []
        for i,scope in enumerate([club,other]):
            value=api(page,'POST',f'/api/clubs/{scope}/families',{'name':f'Synthetic Family {i}','contactName':'Synthetic Parent','phone':'synthetic','email':'','membershipIds':children[:2] if i==0 else children[2:],'version':0},201)
            value=api(page,'POST',f"/api/clubs/{scope}/families/{value['id']}/accounts",{'login':f'parent{i}','displayName':'Synthetic Parent','password':PASSWORD,'version':value['version']},201)
            families.append(value)
        assert sql('SELECT count(*) FROM pride.family_children')=='3'
        assert sql('SELECT count(*) FROM pride.family_access')=='2'
        assert sql("SELECT count(*) FROM pride.staff_users WHERE login LIKE 'parent%' AND password_hash LIKE '$argon2id$%'")=='2'

        # A blank same-origin document permits isolated browser fetches while #200
        # owns parent UI. No routes, responses or API data are mocked.
        parent_context=browser.new_context(service_workers='block')
        parent=parent_context.new_page()
        parent.goto(ROOT+'/api/healthz')
        login=parent.evaluate('''async password=>{const r=await fetch('/api/auth/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({login:'parent0',password})});return r.status;}''',PASSWORD)
        assert login==200
        me=api(parent,'GET','/api/auth/me')
        assert me['user']['isParent'] and me['clubs'][0]['roles']==['parent'] and len(me['clubs'])==1
        listed=api(parent,'GET','/api/portal/families')
        assert listed['total']==1 and len(listed['items'][0]['children'])==2
        for member in children[:2]:
            profile=api(parent,'GET',f'/api/clubs/{club}/member-profiles/{member}')
            assert profile['familyId']==families[0]['id'] and profile['attendance']['rate'] is None
        api(parent,'GET',f'/api/clubs/{other}/member-profiles/{children[2]}',expected=404)
        api(parent,'GET',f'/api/clubs/{club}/participants',expected=404)
        family=families[0]
        endpoint=f"/api/clubs/{club}/families/{family['id']}"
        mutation={'name':family['name'],'contactName':family['contactName'],'phone':family['phone'],'email':family['email'],'membershipIds':children[1:2],'version':family['version']}
        api(page,'PUT',endpoint,mutation,expected=403,csrf=False)
        family=api(page,'PUT',endpoint,mutation)
        api(parent,'GET',f'/api/clubs/{club}/member-profiles/{children[0]}',expected=404)
        assert sql(f"SELECT count(*) FROM pride.family_children WHERE family_id='{family['id']}'")=='1'
        account=family['accounts'][0]['id']
        family=api(page,'POST',endpoint+f'/accounts/{account}/reset-password',{'password':'new-synthetic-browser-password','version':family['version']})
        api(parent,'GET','/api/auth/me',expected=401)
        assert sql(f"SELECT count(*) FROM pride.staff_sessions WHERE staff_user_id='{account}' AND revoked_at IS NULL")=='0'
        family=api(page,'POST',endpoint+f'/accounts/{account}/revoke',{'version':family['version']})
        assert not family['accounts']
        assert sql(f"SELECT count(*) FROM pride.family_access WHERE user_id='{account}'")=='0'
        assert not errors,errors
        print(json.dumps({'sampleFamily':families[0], 'sampleProfile':profile},ensure_ascii=False))
        print(json.dumps({'engine':ENGINE,'result':'PASS','transport':'real browser -> Go -> PostgreSQL18.6','checks':['owner UI login','explicit two-child family','parent login and isolated list','foreign UUID/staff denial','CSRF','unlink live denial','Argon2id storage','reset session revocation','revoke SQL']},ensure_ascii=False))
        browser.close()
finally:
    subprocess.run(['docker','rm','-f',APP,DB],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    subprocess.run(['docker','network','rm',NET],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    STATIC_TEMP.cleanup()

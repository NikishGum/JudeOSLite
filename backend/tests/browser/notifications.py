"""RD-12: real browser -> Go -> disposable PostgreSQL. No external club data."""
import base64
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
from urllib.request import urlopen
from playwright.sync_api import sync_playwright, expect

ROOT = 'http://127.0.0.1:18104'
REPO = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(REPO/'frontend/e2e'))
from runtime_ui import observe_api, wait_for_api_idle
ENGINE = os.environ.get('PWA_BROWSER', 'chromium')
IMAGE = os.environ.get('PWA_TEST_IMAGE', 'judeoslite:ci')
assert ENGINE in ('chromium', 'webkit') and IMAGE.startswith('judeoslite:')
PREFIX = 'rd204-' + uuid.uuid4().hex[:8]
DB, APP, NET = PREFIX+'-db', PREFIX+'-app', PREFIX+'-net'
PASSWORD = 'synthetic-browser-password'
STATIC_TEMP = tempfile.TemporaryDirectory(prefix='rd204-static-')
static = Path(STATIC_TEMP.name)/'static'
shutil.copytree(REPO/'frontend/dist', static)
for file in (static, *static.rglob('*')):
    file.chmod(0o755 if file.is_dir() else 0o644)

def cmd(*args):
    return subprocess.check_output(args, text=True).strip()

def sql(statement):
    return cmd('docker', 'exec', DB, 'psql', '-U', 'test', '-d', 'rd204', '-Atc', statement)

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
    cmd('docker', 'run', '-d', '--name', DB, '--network', NET, '-e', 'POSTGRES_USER=test', '-e', 'POSTGRES_PASSWORD=test-only', '-e', 'POSTGRES_DB=rd204', 'postgres:18.6-bookworm')
    for _ in range(30):
        if subprocess.run(['docker', 'exec', DB, 'pg_isready', '-h', '127.0.0.1', '-U', 'test', '-d', 'rd204'], stdout=subprocess.DEVNULL).returncode == 0:
            break
        time.sleep(1)
    else:
        raise AssertionError('PostgreSQL not ready')
    env = ['-e', f'DATABASE_URL=postgres://test:test-only@{DB}:5432/rd204?sslmode=disable']
    cmd('docker', 'run', '--rm', '--network', NET, *env, IMAGE, '/app/migrate')
    owner(env)
    cmd('docker', 'create', '--name', APP, '--network', NET, *env, '-e', f'APP_ORIGIN={ROOT}', '-e', 'AUTH_INSECURE_LOCAL=true', '-p', '127.0.0.1:18104:8080', IMAGE)
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
        chromium_path = os.environ.get('CHROMIUM_PATH')
        browser = getattr(p, ENGINE).launch(headless=True, executable_path=chromium_path if ENGINE == 'chromium' else None)
        context = browser.new_context(service_workers='block')
        page = context.new_page()
        errors = []
        def observe(value):
            observe_api(value)
            recent=[]
            value.on('requestfailed',lambda request:recent.append({'url':request.url,'failure':request.failure}))
            def failed(error):
                diagnostic={'engine':ENGINE,'url':value.url,'message':str(error),'stack':error.stack,'failedRequests':recent[-12:]}
                errors.append(diagnostic)
                print(json.dumps(diagnostic,ensure_ascii=False),flush=True)
            value.on('pageerror',failed)
        observe(page)
        def navigate(value,path):
            # A forced document navigation must not abandon reads from the
            # previous SPA view; use the same observer as the existing suites.
            wait_for_api_idle(value)
            value.goto(ROOT+path)
            wait_for_api_idle(value)
        def login(value, name):
            with value.expect_response(lambda r:r.url==ROOT+'/api/auth/me' and r.request.method=='GET'):
                value.goto(ROOT)
            wait_for_api_idle(value)
            value.get_by_label('Логин',exact=True).fill(name)
            value.get_by_label('Пароль',exact=True).fill(PASSWORD)
            expect(value.get_by_label('Логин',exact=True)).to_have_value(name)
            with value.expect_response(lambda r:r.url==ROOT+'/api/auth/login' and r.status==200):
                value.get_by_role('button',name='Войти',exact=True).click()
            expect(value.get_by_role('button',name='Меню профиля',exact=True)).to_be_visible()
            wait_for_api_idle(value)

        def api(value, method, path, data=None, expected=200, csrf=True):
            result=value.evaluate('''async ([method,path,data,csrf])=>{
                const me=await fetch('/api/auth/me',{cache:'no-store'}).then(r=>r.json());
                const r=await fetch(path,{method,headers:{'Content-Type':'application/json',...(csrf?{'X-CSRF-Token':me.csrfToken}:{})},...(data===null?{}:{body:JSON.stringify(data)})});
                return {status:r.status,cache:r.headers.get('Cache-Control'),body:await r.json()};
            }''',[method,path,data,csrf])
            assert result['status']==expected and result['cache']=='no-store',result
            return result['body']

        login(page,'owner')
        club=api(page,'POST','/api/clubs',{'name':'Synthetic Inbox A','address':'','timezone':'Europe/Minsk','version':0},201)['id']
        other=api(page,'POST','/api/clubs',{'name':'Synthetic Inbox B','address':'','timezone':'Europe/Minsk','version':0},201)['id']
        members=[];families=[]
        for i,scope in enumerate([club,club,other]):
            member=api(page,'POST',f'/api/clubs/{scope}/participants',{'identity':{'fullName':f'Private Child {i}','birthDate':'2015-01-01','version':0},'joinedOn':'2020-01-01','contactKind':'email','contactValue':'private@example.test','contactName':'Private Contact','version':0},201)
            members.append(member['id'])
            family=api(page,'POST',f'/api/clubs/{scope}/families',{'name':f'Private Family {i}','contactName':'Private Parent','phone':'private','email':'','membershipIds':[member['id']],'version':0},201)
            family=api(page,'POST',f"/api/clubs/{scope}/families/{family['id']}/accounts",{'login':f'parent{i}','displayName':f'Private Parent {i}','password':PASSWORD,'version':family['version']},201)
            families.append(family)

        contexts=[]
        def person(name,width=390):
            context=browser.new_context(service_workers='block',viewport={'width':width,'height':844})
            contexts.append(context)
            value=context.new_page();observe(value)
            login(value,name)
            return value

        parent=person('parent0');parent_b=person('parent1');parent_other=person('parent2')
        for value in [parent,parent_b,parent_other]:api(value,'POST','/api/notifications/read-all',{})
        base=f'/api/clubs/{club}'
        payment_body={'requestId':str(uuid.uuid4()),'membershipId':members[0],'description':'Private invoice','amountMinor':12345,'method':None,'paidOn':None,'planId':None,'validFrom':None}
        payment=api(page,'POST',base+'/payments',payment_body,201)
        api(page,'POST',base+'/payments',payment_body,201)
        inbox=api(parent,'GET','/api/notifications?status=unread')
        assert inbox['total']==inbox['unreadCount']==1 and len(inbox['items'])==1,inbox
        assert inbox['items'][0]['title']=='Выставлен новый счёт' and inbox['items'][0]['href']=='/payments'
        assert sql(f"SELECT count(*) FROM pride.notifications WHERE user_id=(SELECT id FROM pride.staff_users WHERE login='parent0') AND kind='payment'")=='1'
        for value in [parent_b,parent_other]:assert api(value,'GET','/api/notifications?status=unread')['total']==0
        owner_note=api(page,'GET','/api/notifications?status=unread')['items'][0]['id']
        api(parent,'POST',f'/api/notifications/{owner_note}/read',{},404)
        api(parent,'POST',f"/api/notifications/{inbox['items'][0]['id']}/read",{},403,csrf=False)

        navigate(parent,'/notifications')
        expect(parent.get_by_role('button',name='Уведомления, непрочитанных: 1',exact=True)).to_be_visible()
        expect(parent.get_by_text('Выставлен новый счёт',exact=True)).to_be_visible()
        # The existing UI follows the parent-safe route, without staff endpoints.
        parent.locator('.notification-list').get_by_role('button',name='Перейти',exact=True).first.click()
        expect(parent).to_have_url(ROOT+'/payments')
        expect(parent.get_by_text('Private invoice',exact=True)).to_be_visible()
        navigate(parent,'/notifications')
        with parent.expect_response(lambda r:r.url.endswith('/read') and r.request.method=='POST' and r.status==200):
            parent.get_by_role('button',name='Прочитано: Выставлен новый счёт',exact=True).click()
        expect(parent.get_by_role('button',name='Прочитано: Выставлен новый счёт',exact=True)).to_have_count(0)
        assert api(parent,'GET','/api/notifications?status=unread')['unreadCount']==0
        wait_for_api_idle(parent)
        parent.reload()
        wait_for_api_idle(parent)
        expect(parent.get_by_text('Выставлен новый счёт',exact=True)).to_be_visible()
        assert sql(f"SELECT count(*) FROM pride.notifications WHERE id='{inbox['items'][0]['id']}' AND read_at IS NOT NULL")=='1'
        print('PASS personal bell/UI read/reload/parent route/SQL/idempotency/IDOR/CSRF',flush=True)

        api(page,'POST',base+f"/payments/{payment['id']}/confirm",{'version':1,'method':'cash','paidOn':'2026-10-19'})
        docbase=base+f'/participants/{members[0]}/documents'
        api(page,'POST',docbase,{'name':'Private medical title','kind':'medical','filename':'private.pdf','contentBase64':base64.b64encode(b'%PDF-1.7\n%%EOF').decode(),'expiresOn':None},201)
        assert api(parent,'GET','/api/notifications?status=unread')['unreadCount']==2
        navigate(parent,'/notifications')
        with parent.expect_response(lambda r:r.url.endswith('/read-all') and r.status==200):
            parent.get_by_role('button',name='Прочитать все',exact=True).click()
        expect(parent.get_by_role('button',name='Прочитать все',exact=True)).to_be_disabled()
        assert api(parent,'GET','/api/notifications?status=unread')['total']==0
        raw=json.dumps(api(parent,'GET','/api/notifications'),ensure_ascii=False)
        assert all(value not in raw for value in ['Private','private.pdf','medical','12345','private@example.test']),raw

        # Real family writes create enough personal events for both UI pages.
        family=families[0]
        before=api(parent,'GET','/api/notifications')['total']
        for i in range(26):
            family=api(page,'PUT',base+f"/families/{family['id']}",{'name':f'Private Family revision {i}','contactName':'Private Parent','phone':'private','email':'','membershipIds':[members[0]],'version':family['version']})
        navigate(parent,'/notifications')
        pager=parent.get_by_role('navigation',name='Страницы списка',exact=True)
        expect(pager).to_contain_text(f'Страница 1 · всего {before+26}')
        expect(parent.locator('.notification-list > li')).to_have_count(25)
        pager.get_by_role('button',name='Далее',exact=True).click()
        expect(pager).to_contain_text(f'Страница 2 · всего {before+26}')
        expect(parent.locator('.notification-list > li')).to_have_count(before+1)
        assert parent.evaluate('document.documentElement.scrollWidth<=innerWidth')
        api(parent,'POST','/api/notifications/read-all',{})
        assert api(parent_b,'GET','/api/notifications?status=unread')['total']==0
        print('PASS confirmation/document events, privacy, read-all and mobile pagination',flush=True)

        # Real trainer assignment -> inbox -> calendar detail, then immediate loss
        # of all prior session messages after replacement.
        trainer=api(page,'POST',base+'/trainers',{'login':'trainer','displayName':'Synthetic Trainer','password':PASSWORD},201)
        replacement=api(page,'POST',base+'/trainers',{'login':'replacement','displayName':'Synthetic Replacement','password':PASSWORD},201)
        group=api(page,'POST',base+'/groups',{'name':'Synthetic Inbox Group'},201)
        session=api(page,'POST',base+'/sessions',{'groupId':group['id'],'trainerId':trainer['id'],'date':'2030-02-10','startsTime':'10:00','durationMinutes':60},201)
        coach=person('trainer')
        navigate(coach,'/notifications')
        expect(coach.get_by_text('Назначено занятие',exact=True)).to_be_visible()
        notes=api(coach,'GET','/api/notifications')
        assert notes['total']==1 and notes['items'][0]['kind']=='training_session'
        coach.locator('.notification-list').get_by_role('button',name='Перейти',exact=True).click()
        expect(coach).to_have_url(ROOT+notes['items'][0]['href'])
        expect(coach.get_by_role('heading',name='Synthetic Inbox Group',exact=False).first).to_be_visible()
        wait_for_api_idle(coach)
        api(page,'POST',base+f"/sessions/{session['id']}/replace-trainer",{'version':1,'trainerId':replacement['id']})
        assert api(coach,'GET','/api/notifications')['total']==0
        api(coach,'POST',f"/api/notifications/{notes['items'][0]['id']}/read",{},404)
        substitute=person('replacement')
        assert api(substitute,'GET','/api/notifications')['total']==1
        api(page,'POST',base+f"/sessions/{session['id']}/cancel",{'version':2,'reason':'Private reason'})
        assert api(substitute,'GET','/api/notifications')['total']==2
        assert all(n['kind']!='training_session' for n in api(parent,'GET','/api/notifications')['items'])

        # Unlink hides child messages while retaining legitimate generic family
        # messages. Revoke then removes the remainder from lists and counters.
        family=api(page,'PUT',base+f"/families/{family['id']}",{'name':family['name'],'contactName':family['contactName'],'phone':family['phone'],'email':family['email'],'membershipIds':[],'version':family['version']})
        available=api(parent,'GET','/api/notifications?limit=100')
        assert all(n['kind']=='family' for n in available['items'])
        api(parent,'POST',f"/api/notifications/{inbox['items'][0]['id']}/read",{},404)
        account=next(a for a in family['accounts'] if a['login']=='parent0')
        api(page,'POST',base+f"/families/{family['id']}/accounts/{account['id']}/revoke",{'version':family['version']})
        assert api(parent,'GET','/api/notifications')['total']==api(parent,'GET','/api/notifications')['unreadCount']==0
        navigate(page,f'/clubs/{club}/notifications')
        expect(page.locator('.notification-list > li').first).to_be_visible()
        page.set_viewport_size({'width':1280,'height':900})
        try:
            page.wait_for_function('document.documentElement.scrollWidth<=innerWidth',timeout=5000)
        except Exception:
            print(page.evaluate('''() => ({width:innerWidth,scroll:document.documentElement.scrollWidth,overflow:[...document.querySelectorAll('*')].map(el=>({tag:el.tagName,cls:el.className,left:el.getBoundingClientRect().left,right:el.getBoundingClientRect().right})).filter(r=>r.right>innerWidth+1).slice(-15)})'''),flush=True)
            page.screenshot(path='/tmp/rd204-notifications-overflow.png',full_page=True)
            raise
        assert page.evaluate('document.documentElement.scrollWidth<=innerWidth')
        api(parent,'POST','/api/auth/logout',{})
        api(parent,'GET','/api/notifications',expected=401)
        assert not errors,errors
        print(json.dumps({'engine':ENGINE,'browser':browser.version,'result':'PASS','transport':'real frontend -> Go -> PostgreSQL18.6','checks':['personal counts and IDOR','UI read/read-all/reload + independent SQL','payment/document/family events','mobile pagination and desktop inbox','trainer route/replacement/cancellation/revocation','parent unlink/revoke/logout','CSRF/no-store/privacy']},ensure_ascii=False),flush=True)
        browser.close()
finally:
    subprocess.run(['docker','rm','-f',APP,DB],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    subprocess.run(['docker','network','rm',NET],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    STATIC_TEMP.cleanup()

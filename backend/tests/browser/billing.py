"""RD-10: real browser -> Go -> disposable PostgreSQL. No external club data."""
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

ROOT = 'http://127.0.0.1:18102'
REPO = Path(__file__).resolve().parents[3]
ENGINE = os.environ.get('PWA_BROWSER', 'chromium')
IMAGE = os.environ.get('PWA_TEST_IMAGE', 'judeoslite:ci')
assert ENGINE in ('chromium', 'webkit') and IMAGE.startswith('judeoslite:')
PREFIX = 'rd202-' + uuid.uuid4().hex[:8]
DB, APP, NET = PREFIX+'-db', PREFIX+'-app', PREFIX+'-net'
PASSWORD = 'synthetic-browser-password'
STATIC_TEMP = tempfile.TemporaryDirectory(prefix='rd202-static-')
static = Path(STATIC_TEMP.name)/'static'
shutil.copytree(REPO/'frontend/dist', static)
for file in (static, *static.rglob('*')):
    file.chmod(0o755 if file.is_dir() else 0o644)

def cmd(*args):
    return subprocess.check_output(args, text=True).strip()

def sql(statement):
    return cmd('docker', 'exec', DB, 'psql', '-U', 'test', '-d', 'rd202', '-qAtc', statement)

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
    cmd('docker', 'run', '-d', '--name', DB, '--network', NET, '-e', 'POSTGRES_USER=test', '-e', 'POSTGRES_PASSWORD=test-only', '-e', 'POSTGRES_DB=rd202', 'postgres:18.6-bookworm')
    for _ in range(30):
        if subprocess.run(['docker', 'exec', DB, 'pg_isready', '-h', '127.0.0.1', '-U', 'test', '-d', 'rd202'], stdout=subprocess.DEVNULL).returncode == 0:
            break
        time.sleep(1)
    else:
        raise AssertionError('PostgreSQL not ready')
    env = ['-e', f'DATABASE_URL=postgres://test:test-only@{DB}:5432/rd202?sslmode=disable']
    cmd('docker', 'run', '--rm', '--network', NET, *env, IMAGE, '/app/migrate')
    owner(env)
    cmd('docker', 'create', '--name', APP, '--network', NET, *env, '-e', f'APP_ORIGIN={ROOT}', '-e', 'AUTH_INSECURE_LOCAL=true', '-p', '127.0.0.1:18102:8080', IMAGE)
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

        today = datetime.now(ZoneInfo('Europe/Minsk')).date()
        day = today.isoformat()
        until = (today+timedelta(days=30)).isoformat()
        club = api(page,'POST','/api/clubs',{'name':'Synthetic Billing A','address':'','timezone':'Europe/Minsk','version':0},201)['id']
        other = api(page,'POST','/api/clubs',{'name':'Synthetic Billing B','address':'','timezone':'Europe/Minsk','version':0},201)['id']
        children=[]
        for i in range(3):
            scope=club if i<2 else other
            children.append(api(page,'POST',f'/api/clubs/{scope}/participants',{'identity':{'fullName':f'Synthetic Billing Child {i}','birthDate':'2015-01-01','version':0},'joinedOn':'2020-01-01','contactKind':'email','contactValue':'synthetic@example.test','contactName':'Synthetic Contact','version':0},201)['id'])
        families=[]
        for i,member in enumerate(children):
            scope=club if i<2 else other
            family=api(page,'POST',f'/api/clubs/{scope}/families',{'name':f'Synthetic Billing Family {i}','contactName':'Synthetic Parent','phone':'','email':'','membershipIds':[member],'version':0},201)
            family=api(page,'POST',f"/api/clubs/{scope}/families/{family['id']}/accounts",{'login':f'parent{i}','displayName':'Synthetic Parent','password':PASSWORD,'version':family['version']},201)
            families.append(family)
        base=f'/api/clubs/{club}'
        # Existing production UI -> Go -> PG: create tariff, invoice, confirm, void.
        page.goto(ROOT+f'/clubs/{club}/payments')
        expect(page.get_by_role('button',name='Добавить платёж',exact=True)).to_be_enabled()
        page.get_by_role('tab',name='Тарифы',exact=True).click()
        page.get_by_role('button',name='Создать тариф',exact=True).click()
        dialog=page.get_by_role('dialog',name='Новый тариф',exact=True)
        dialog.get_by_label('Название',exact=True).fill('Synthetic Browser Plan')
        dialog.get_by_label('Стоимость, BYN',exact=True).fill('123.45')
        dialog.get_by_label('Занятий',exact=True).fill('1')
        dialog.get_by_label('Срок, дней',exact=True).fill('30')
        with page.expect_response(lambda r:r.request.method=='POST' and r.url.endswith('/billing/plans') and r.status==201) as saved:
            dialog.get_by_role('button',name='Сохранить тариф',exact=True).click()
        plan=saved.value.json()
        expect(dialog).not_to_be_visible()
        assert sql(f"SELECT amount_minor||':'||visits||':'||days FROM pride.billing_plans WHERE id='{plan['id']}'")=='12345:1:30'
        page.get_by_role('tab',name='Платежи',exact=True).click()
        page.get_by_role('button',name='Добавить платёж',exact=True).click()
        dialog=page.get_by_role('dialog',name='Новый платёж / абонемент',exact=True)
        expect(dialog.get_by_role('combobox',name='Клиент',exact=True)).to_be_enabled()
        dialog.get_by_role('combobox',name='Клиент',exact=True).select_option(children[0])
        dialog.get_by_role('combobox',name='Тип оплаты',exact=True).select_option('pending')
        dialog.get_by_role('combobox',name='Тариф / абонемент',exact=True).select_option(plan['id'])
        dialog.get_by_label('Абонемент действует с',exact=True).fill(day)
        with page.expect_response(lambda r:r.request.method=='POST' and r.url.endswith('/payments') and r.status==201) as saved:
            dialog.get_by_role('button',name='Выставить счёт',exact=True).click()
        payment=saved.value.json()
        expect(dialog).not_to_be_visible()
        assert payment['status']=='pending' and payment['method'] is None and payment['paidOn'] is None
        assert sql(f"SELECT count(*) FROM pride.subscriptions WHERE payment_id='{payment['id']}' AND valid_from='{day}' AND valid_until='{until}' AND days=30")=='1'
        row=page.get_by_role('row').filter(has_text='Synthetic Billing Child 0')
        expect(row.locator('.status-badge')).to_have_text('Ожидает оплаты')
        row.get_by_role('button').click()
        dialog=page.get_by_role('dialog',name='Платёж',exact=True)
        dialog.get_by_role('combobox',name='Способ оплаты',exact=True).select_option('erip')
        with page.expect_response(lambda r:r.request.method=='POST' and r.url.endswith('/confirm') and r.status==200) as saved:
            dialog.get_by_role('button',name='Подтвердить оплату',exact=True).click()
        payment=saved.value.json()
        expect(dialog).not_to_be_visible()
        assert payment['status']=='paid' and payment['version']==2 and payment['method']=='erip'
        assert sql(f"SELECT status||':'||version||':'||amount_minor FROM pride.payments WHERE id='{payment['id']}'")=='paid:2:12345'
        summary=api(page,'GET',base+f'/billing/summary?from={day}&until={day}')
        assert summary['paidMinor']==12345 and summary['paidCount']==1 and summary['pendingMinor']==0
        print('PASS UI tariff/invoice/confirm and SQL money',flush=True)
        # Live attendance, date boundaries and correction use existing journals API.
        owner_id=sql("SELECT id FROM pride.staff_users WHERE login='owner'")
        trainer=sql("INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT 'trainer','Synthetic Trainer',password_hash FROM pride.staff_users WHERE login='owner' RETURNING id")
        sql(f"INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES('{trainer}','{club}','trainer')")
        group=sql(f"INSERT INTO pride.training_groups(club_id,name) VALUES('{club}','Synthetic Billing Group') RETURNING id")
        source=sql(f"INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from) VALUES('{club}','{group}','{children[0]}','2020-01-01') RETURNING id")
        records=[]
        for offset,status in [(-1,'closed'),(0,'closed'),(29,'closed'),(30,'closed'),(1,'cancelled'),(2,'open')]:
            session_day=(today+timedelta(days=offset)).isoformat()
            session=sql(f"INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES('{club}','{group}','{trainer}','{session_day}','Europe/Minsk','{session_day} 18:00+03','{session_day} 19:00+03') RETURNING id")
            record=sql(f"INSERT INTO pride.attendance_records(club_id,training_session_id,group_id,club_membership_id,source_group_membership_id) VALUES('{club}','{session}','{group}','{children[0]}','{source}') RETURNING id")
            sql(f"UPDATE pride.training_sessions SET status='open',roster_frozen_at=now() WHERE id='{session}'")
            sql(f"UPDATE pride.attendance_records SET state='present',marked_by='{owner_id}',marked_at=now(),origin='manual' WHERE id='{record}'")
            if status=='closed':sql(f"UPDATE pride.training_sessions SET status='closed',closed_at=now() WHERE id='{session}'")
            elif status=='cancelled':sql(f"UPDATE pride.training_sessions SET status='cancelled',cancelled_at=now(),cancellation_reason='Synthetic' WHERE id='{session}'")
            records.append((session,record))
        sub=api(page,'GET',base+'/subscriptions')['items'][0]
        assert sub['used']==2 and sub['visits']==1 and sub['status']=='exhausted'
        session,record=records[1]
        api(page,'PUT',base+f'/journals/{session}/records/{record}',{'version':1,'state':'absent','absenceReason':'excused','changeReason':'Synthetic billing correction'})
        assert api(page,'GET',base+'/subscriptions')['items'][0]['used']==1
        # Updating the source tariff must leave all snapshot fields unchanged.
        api(page,'PUT',base+f"/billing/plans/{plan['id']}",{'name':'Changed source','amountMinor':20000,'visits':9,'days':10,'archived':False,'version':1})
        sub=api(page,'GET',base+'/subscriptions')['items'][0]
        assert sub['planName']=='Synthetic Browser Plan' and sub['visits']==1 and sub['validUntil']==until
        print('PASS interval usage/HTTP correction/snapshot',flush=True)
        # Concurrent request UUID retries are genuine concurrent browser HTTP calls.
        def body(member,amount=50):
            return {'requestId':str(uuid.uuid4()),'membershipId':member,'description':'Synthetic concurrent','amountMinor':amount,'method':None,'paidOn':None,'planId':None,'validFrom':None}
        repeated=body(children[0])
        batch=page.evaluate('''async ([path,data])=>{const me=await fetch('/api/auth/me').then(r=>r.json());return Promise.all(Array.from({length:8},async()=>{const r=await fetch(path,{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrfToken},body:JSON.stringify(data)});return {status:r.status,cache:r.headers.get('Cache-Control'),body:await r.json()};}));}''',[base+'/payments',repeated])
        assert all(v['status']==201 and v['cache']=='no-store' for v in batch),batch
        assert len({v['body']['id'] for v in batch})==1
        retry=batch[0]['body']
        assert sql(f"SELECT count(*) FROM pride.payments WHERE request_id='{repeated['requestId']}'")=='1'
        assert sql(f"SELECT count(*) FROM pride.audit_events WHERE entity_id='{retry['id']}'")=='1'
        repeated['amountMinor']=51
        api(page,'POST',base+'/payments',repeated,409)
        api(page,'POST',base+f"/payments/{retry['id']}/confirm",{'version':1,'method':'cash','paidOn':day})
        api(page,'POST',base+f"/payments/{retry['id']}/confirm",{'version':1,'method':'cash','paidOn':day},409)
        api(page,'POST',base+'/payments',body(children[1]),201)
        api(page,'POST',f'/api/clubs/{other}/payments',body(children[2]),201)
        # Family A/B (same club and other club), unlink and revoke are live checks.
        parent_context=browser.new_context(service_workers='block')
        parent=parent_context.new_page();parent.goto(ROOT+'/api/healthz')
        def parent_login():
            result=parent.evaluate('''async password=>{const r=await fetch('/api/auth/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({login:'parent0',password})});return r.status;}''',PASSWORD)
            assert result==200
        parent_login()
        own=api(parent,'GET','/api/portal/payments')
        assert own['total']==2 and {p['membershipId'] for p in own['items']}=={children[0]}
        for child in children[1:]:
            assert api(parent,'GET','/api/portal/payments?membershipId='+child)['total']==0
            assert api(parent,'GET','/api/portal/subscriptions?membershipId='+child)['total']==0
        assert api(parent,'GET','/api/portal/subscriptions')['items'][0]['used']==1
        api(parent,'GET',base+'/billing/summary',expected=404)
        api(parent,'POST',base+'/payments',body(children[0]),404)
        api(page,'POST',base+'/payments',body(children[0]),403,csrf=False)
        invalid=body(children[0],1.5);api(page,'POST',base+'/payments',invalid,400)
        family=families[0];endpoint=base+f"/families/{family['id']}"
        mutation={'name':family['name'],'contactName':family['contactName'],'phone':family['phone'],'email':family['email'],'membershipIds':[],'version':family['version']}
        family=api(page,'PUT',endpoint,mutation)
        assert api(parent,'GET','/api/portal/payments')['total']==0
        assert api(parent,'GET','/api/portal/subscriptions')['total']==0
        mutation['membershipIds']=[children[0]];mutation['version']=family['version'];family=api(page,'PUT',endpoint,mutation)
        account=family['accounts'][0]['id']
        family=api(page,'POST',endpoint+f'/accounts/{account}/revoke',{'version':family['version']})
        assert api(parent,'GET','/api/portal/payments')['total']==0
        assert api(parent,'GET','/api/portal/subscriptions')['total']==0
        print('PASS concurrent retries and parent isolation/revoke',flush=True)
        # Void through existing real UI, preserving the received amount and audit.
        page.reload();row=page.get_by_role('row').filter(has_text='Synthetic Browser Plan')
        expect(row.locator('.status-badge')).to_have_text('Оплачен')
        row.get_by_role('button').click();dialog=page.get_by_role('dialog',name='Платёж',exact=True)
        dialog.get_by_label('Причина аннулирования',exact=False).fill('Synthetic browser cancellation')
        with page.expect_response(lambda r:r.request.method=='POST' and r.url.endswith('/void') and r.status==200):
            dialog.get_by_role('button',name='Аннулировать запись',exact=True).click()
        expect(dialog).not_to_be_visible()
        assert sql(f"SELECT status||':'||version||':'||amount_minor FROM pride.payments WHERE id='{payment['id']}'")=='void:3:12345'
        assert api(page,'GET',base+'/subscriptions')['items'][0]['status']=='cancelled'
        assert sql(f"SELECT count(*) FROM pride.audit_events WHERE entity_id='{payment['id']}'")=='3'
        # Trainer session and actual assignment revocation cannot expose finances.
        trainer_context=browser.new_context(service_workers='block');trainer_page=trainer_context.new_page();trainer_page.goto(ROOT+'/api/healthz')
        assert trainer_page.evaluate('''async password=>(await fetch('/api/auth/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({login:'trainer',password})})).status''',PASSWORD)==200
        for suffix in ['/payments','/subscriptions','/billing/plans','/billing/summary']:
            api(trainer_page,'GET',base+suffix,expected=404)
        sql(f"UPDATE pride.staff_users SET disabled_at=now() WHERE id='{trainer}'")
        api(trainer_page,'GET','/api/portal/payments',expected=401)
        cmd('docker','run','--rm','--network',NET,*env,IMAGE,'/app/migrate')
        assert not errors,errors
        print(json.dumps({'engine':ENGINE,'browser':browser.version,'result':'PASS','transport':'real UI/browser -> Go -> PostgreSQL18.6','checks':['UI tariff/invoice/confirm/void','integer BYN and SQL audit','eight concurrent UUID retries','version conflict','snapshot','exclusive interval/closed-only usage/HTTP correction','family A/B and club isolation','unlink/revoke/disabled session','CSRF/no-store','migration rerun']},ensure_ascii=False))
        browser.close()
finally:
    subprocess.run(['docker','rm','-f',APP,DB],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    subprocess.run(['docker','network','rm',NET],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    STATIC_TEMP.cleanup()

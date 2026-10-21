"""UX-HISTORY: production browser -> Go -> disposable PostgreSQL, synthetic data only."""
import json, os, pty, select, shutil, subprocess, tempfile, time, uuid
from datetime import datetime, timedelta
from pathlib import Path
from urllib.parse import urlparse, parse_qs
from urllib.request import urlopen
from zoneinfo import ZoneInfo
from playwright.sync_api import sync_playwright, expect
from runtime_ui import observe_api, wait_for_api_idle, signout
from redesign_accessibility import contrast_audit

ENGINE = os.environ.get('PWA_BROWSER', 'chromium')
EXISTING = os.environ.get('HISTORY_TEST_ORIGIN')
ROOT = EXISTING or 'http://127.0.0.1:18082'
assert urlparse(ROOT).hostname in ('127.0.0.1', 'localhost')
PREFIX = 'history220-' + uuid.uuid4().hex[:8]
DB, APP, NET = (os.environ.get('HISTORY_TEST_DB', PREFIX+'-db'), PREFIX+'-app', PREFIX+'-net')
DATABASE = 'audit_browser' if EXISTING else 'history220'
PASSWORD = 'synthetic-browser-password'
IMAGE = os.environ.get('PWA_TEST_IMAGE', 'judeoslite:ci')
SHOTS = Path(os.environ.get('HISTORY_EVIDENCE', '/tmp/history220-evidence'))
SHOTS.mkdir(parents=True, exist_ok=True)

def cmd(*args): return subprocess.check_output(args, text=True).strip()
def sql(statement):
    return cmd('docker', 'exec', DB, 'psql', '-v', 'ON_ERROR_STOP=1', '-U', 'test', '-d', DATABASE, '-Atc', statement).splitlines()[0]
STATIC_TEMP = tempfile.TemporaryDirectory(prefix='history220-static-')
static = Path(STATIC_TEMP.name)/'static'
shutil.copytree(Path(__file__).resolve().parents[1]/'dist', static)
for file in (static, *static.rglob('*')):
    file.chmod(0o755 if file.is_dir() else 0o644)

def bootstrap(env):
    master, slave = pty.openpty()
    process = subprocess.Popen(['docker', 'run', '--rm', '-it', '--network', NET, *env, IMAGE, '/app/create-owner', '--login', 'owner', '--name', 'Synthetic Owner'], stdin=slave, stdout=slave, stderr=slave)
    os.close(slave)
    for prompt in (b'Password (', b'Repeat password:'):
        output=b''; deadline=time.monotonic()+30
        while prompt not in output:
            assert time.monotonic()<deadline
            if select.select([master],[],[],1)[0]: output+=os.read(master,4096)
        os.write(master,(PASSWORD+'\n').encode())
    assert process.wait(timeout=30)==0
    os.close(master)

def hold(page, target):
    page.evaluate('''target=>{const original=window.fetch;window.held=[];window.restore=()=>window.fetch=original;
      window.fetch=async(...args)=>{const response=await original(...args);
        if(String(args[0]).includes(target))await new Promise(resolve=>window.held.push(resolve));return response;};}''', target)
def release(page): page.evaluate('window.restore();window.held.forEach(resolve=>resolve())')

try:
    if not EXISTING:
        assert IMAGE.startswith('judeoslite:')
        cmd('docker','network','create',NET)
        cmd('docker','run','-d','--name',DB,'--network',NET,'-e','POSTGRES_USER=test','-e','POSTGRES_PASSWORD=test-only','-e','POSTGRES_DB='+DATABASE,'postgres:18.6-bookworm')
        for _ in range(30):
            # initdb temporarily accepts Unix sockets before the final server
            # enables TCP. Migrations run in another container and require TCP.
            if subprocess.run(['docker','exec',DB,'pg_isready','-h','127.0.0.1','-U','test','-d',DATABASE],stdout=subprocess.DEVNULL).returncode==0: break
            time.sleep(1)
        else:
            raise AssertionError('PostgreSQL TCP endpoint did not become ready')
        env=['-e',f'DATABASE_URL=postgres://test:test-only@{DB}:5432/{DATABASE}?sslmode=disable']
        cmd('docker','run','--rm','--network',NET,*env,IMAGE,'/app/migrate'); bootstrap(env)
        cmd('docker','create','--name',APP,'--network',NET,*env,'-e','APP_ORIGIN='+ROOT,'-e','AUTH_INSECURE_LOCAL=true','-p','127.0.0.1:18082:8080',IMAGE)
        cmd('docker','cp',str(static)+'/.',APP+':/app/static')
        cmd('docker','start',APP)
        for _ in range(30):
            try:
                urlopen(ROOT+'/api/readyz',timeout=2).close(); break
            except OSError: time.sleep(1)
    ownerid=sql('SELECT id FROM pride.staff_users WHERE is_owner LIMIT 1')
    a=sql(f"INSERT INTO pride.clubs(name) VALUES('Брест · Ёлка длинное название филиала {PREFIX}') RETURNING id")
    b=sql(f"INSERT INTO pride.clubs(name) VALUES('Чужой филиал {PREFIX}') RETURNING id")
    users={}
    for name in ('admin','former','namesake','foreign'):
        label='Ольга Калиновская с длинным именем' if name in ('former','namesake') else name
        users[name]=sql(f"INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT '{PREFIX}-{name}','{label}',password_hash FROM pride.staff_users WHERE id='{ownerid}' RETURNING id")
    sql(f"INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES('{users['admin']}','{a}','administrator')")
    sql(f"UPDATE pride.staff_users SET disabled_at=now() WHERE id='{users['former']}' RETURNING id")
    today=datetime.now(ZoneInfo('Europe/Minsk')).date(); start=today-timedelta(days=6)
    sql(f"INSERT INTO pride.audit_events(actor_id,club_id,entity_type,entity_id,action,created_at) SELECT '{ownerid}','{a}','training_group',gen_random_uuid(),'created','{today} 15:00+03' FROM generate_series(1,55) RETURNING id")
    target=sql(f"INSERT INTO pride.audit_events(actor_id,club_id,entity_type,entity_id,action,created_at,change_reason) VALUES('{users['former']}','{a}','attendance_record',gen_random_uuid(),'corrected','{today} 00:00+03','PRIVATE REASON') RETURNING id")
    sql(f"INSERT INTO pride.audit_events(actor_id,club_id,entity_type,entity_id,action,created_at) VALUES('{users['namesake']}','{a}','future_type',gen_random_uuid(),'future_action','{today} 12:00+03') RETURNING id")
    sql(f"INSERT INTO pride.audit_events(actor_id,club_id,entity_type,entity_id,action,created_at) VALUES('{users['foreign']}','{b}','training_group',gen_random_uuid(),'foreign_action','{today} 12:00+03') RETURNING id")
    with sync_playwright() as p:
        browser=getattr(p,ENGINE).launch(headless=True)
        errors=[]; requests=[]
        context=browser.new_context(viewport={'width':1280,'height':900},timezone_id='Pacific/Honolulu',service_workers='block',reduced_motion='reduce')
        page=context.new_page(); observe_api(page); page.on('pageerror',lambda error:errors.append(str(error)))
        page.on('request', lambda request: requests.append(request.url) if '/api/audit/events?' in request.url else None)
        def login(name):
            page.goto(ROOT+'/history');page.wait_for_load_state('networkidle');wait_for_api_idle(page)
            page.get_by_label('Логин',exact=True).fill(name);page.get_by_label('Пароль',exact=True).fill(PASSWORD)
            page.get_by_role('button',name='Войти',exact=True).click()
            expect(page.get_by_role('button',name='Меню профиля',exact=True)).to_be_visible();wait_for_api_idle(page)
        login('owner'); audit=page.locator('.audit-history')
        expect(audit.get_by_role('combobox',name='Период',exact=True)).to_have_value('week')
        assert any(f'from={start}' in request and f'until={today}' in request for request in requests),requests
        audit.get_by_role('combobox',name='Филиал',exact=True).select_option(a)
        expect(audit.locator('.audit-event')).to_have_count(50)
        expect(audit.get_by_role('status')).to_have_text('Найдено событий: 57')
        page.screenshot(path=str(SHOTS/f'{ENGINE}-1280-default.png'))
        first=audit.locator('.audit-event').evaluate_all('(rows)=>rows.map(row=>row.querySelector("button").getAttribute("aria-controls"))')
        audit.get_by_role('button',name='Показать ещё',exact=True).click();expect(audit.locator('.audit-event')).to_have_count(57)
        ids=audit.locator('.audit-event').evaluate_all('(rows)=>rows.map(row=>row.querySelector("button").getAttribute("aria-controls"))')
        assert ids[:50]==first and len(set(ids))==57
        search=audit.get_by_role('searchbox',name='Поиск по истории',exact=True)
        def find(value):
            search.fill(value);search.press('Enter');wait_for_api_idle(page)
        before=len(requests)
        search.fill('Кали');search.fill('Калин');search.fill('Калиновская исправлена')
        expect(audit.locator('.audit-event')).to_have_count(1);wait_for_api_idle(page)
        assert [parse_qs(urlparse(value).query).get('q',[''])[0] for value in requests[before:]]==['Калиновская исправлена']
        for value in ('Калиновская исправлена','ИСПРАВЛЕНА КАЛИНОВСКАЯ','елка исправлена','#'+target[:8]):
            find(value);expect(audit.locator('.audit-event')).to_have_count(1)
            expect(audit.get_by_role('button',name='Подробнее о событии #'+target,exact=True)).to_be_visible()
        find('Калиновская');expect(audit.locator('.audit-event')).to_have_count(2)
        assert audit.locator('mark').count()>0
        page.screenshot(path=str(SHOTS/f'{ENGINE}-1280-search.png'))
        audit.get_by_role('button',name='Фильтры',exact=True).click()
        audit.get_by_label('Сотрудник',exact=True).select_option(users['former'])
        audit.get_by_label('Действие',exact=True).select_option('corrected')
        expect(audit.locator('.audit-event')).to_have_count(1)
        assert all(audit.get_by_label('Сотрудник',exact=True).locator(f'option[value="{users[name]}"]').count()==1 for name in ('former','namesake'))
        audit.get_by_role('button',name='Фильтры',exact=False).filter(has_text='Фильтры').first.click()
        expect(audit.get_by_label('Действие',exact=True)).not_to_be_visible()
        audit.get_by_role('button',name='Снять фильтр сотрудника',exact=True).click()
        expect(audit.locator('#audit-toggle')).to_be_focused();expect(audit.locator('.audit-event')).to_have_count(1)
        search.press('Escape');expect(search).to_have_value('');expect(audit.locator('.audit-event')).to_have_count(1)
        find('ничего-не-найдено');expect(audit.get_by_role('heading',name='Изменений не найдено')).to_be_visible()
        audit.locator('.audit-empty').get_by_role('button',name='Очистить поиск',exact=True).click();expect(audit.locator('.audit-event')).to_have_count(1)
        # All dates are live civil dates in Minsk even with a Honolulu browser.
        audit.get_by_role('combobox',name='Период',exact=True).select_option('custom')
        expect(audit.get_by_label('С даты',exact=True)).to_have_value(str(start))
        audit.get_by_label('С даты',exact=True).fill(str(today+timedelta(days=1)));wait_for_api_idle(page)
        before=len(requests);expect(audit.get_by_role('alert')).to_contain_text('Дата начала');page.wait_for_timeout(350);assert len(requests)==before
        audit.get_by_label('С даты',exact=True).fill(str(today));audit.get_by_label('По дату',exact=True).fill(str(today))
        expect(audit.locator('.audit-event')).to_have_count(1)
        audit.get_by_role('button',name='Подробнее о событии #'+target,exact=True).click()
        expect(audit.locator('.audit-detail-row')).to_contain_text(target)
        assert 'PRIVATE' not in audit.inner_text()
        assert search.evaluate('n=>getComputedStyle(n).borderTopWidth')=='0px'
        assert search.locator('..').evaluate('n=>n.getBoundingClientRect().height')==48
        assert audit.locator('.audit-details-toggle svg').first.evaluate('n=>getComputedStyle(n).transitionDuration')=='0s'
        # One batched visual/a11y pass, open/closed filters at all required widths.
        metrics=[]
        for width in (1280,980,390,320):
            page.set_viewport_size({'width':width,'height':900});page.evaluate('scrollTo(0,0)')
            for is_open in (True,False):
                toggle=audit.locator('#audit-toggle')
                if toggle.get_attribute('aria-expanded')!=str(is_open).lower():toggle.click()
                if not page.evaluate('document.documentElement.scrollWidth<=innerWidth'):
                    print('OVERFLOW',width,page.evaluate('({inner:innerWidth,doc:document.documentElement.scrollWidth,body:document.body.scrollWidth})'),page.locator('body *:visible').evaluate_all('(nodes)=>nodes.filter(n=>n.getBoundingClientRect().right>innerWidth || n.scrollWidth>n.clientWidth+1).map(n=>({tag:n.tagName,cls:n.className,w:n.getBoundingClientRect().width,right:n.getBoundingClientRect().right}))'),flush=True)
                assert page.evaluate('document.documentElement.scrollWidth<=innerWidth'),width
                boxes=audit.locator('button:visible,select:visible,input:visible').evaluate_all('(nodes)=>nodes.map(n=>({tag:n.tagName,h:n.getBoundingClientRect().height,w:n.getBoundingClientRect().width,appearance:getComputedStyle(n).appearance}))')
                assert all(item['h']>=44 and item['w']>=44 for item in boxes),(width,boxes)
                assert all(item['h']==48 and item['appearance']=='none' for item in boxes if item['tag']=='SELECT'),boxes
                page.screenshot(path=str(SHOTS/f'{ENGINE}-{width}-{"open" if is_open else "closed"}.png'),full_page=True)
            result=contrast_audit(page);assert not result['failures'],result
            metrics.append({'width':width,'controls':boxes,'contrast':result})
        page.set_viewport_size({'width':1280,'height':900})
        # Timer/focus preserve the mounted rows and expansion, including transient errors.
        row=audit.locator('.audit-event').element_handle();detail=audit.locator('.audit-detail-row').element_handle()
        page.evaluate("dispatchEvent(new Event('focus'))");wait_for_api_idle(page)
        assert row.evaluate('n=>n.isConnected') and detail.evaluate('n=>n.isConnected')
        page.route('**/api/audit/events?*',lambda route:route.abort())
        page.evaluate("dispatchEvent(new Event('focus'))");expect(audit.get_by_role('alert')).to_be_visible()
        assert row.evaluate('n=>n.isConnected') and detail.evaluate('n=>n.isConnected')
        page.unroute('**/api/audit/events?*');audit.get_by_role('button',name='Повторить загрузку истории').click();wait_for_api_idle(page)
        # A held response for the prior query must not replace the current result.
        hold(page,'q=')
        search.fill('елка');search.press('Enter');page.wait_for_function('window.held.length>0')
        search.fill('zz-no-match');search.press('Enter')
        # Restore fetch before issuing the current query; release the old one last.
        page.evaluate('window.restore()');search.fill('другой-no-match');search.press('Enter')
        expect(audit.get_by_role('heading',name='Изменений не найдено')).to_be_visible()
        page.evaluate('window.held.forEach(resolve=>resolve())');wait_for_api_idle(page)
        expect(audit.get_by_role('heading',name='Изменений не найдено')).to_be_visible()
        audit.get_by_role('button',name='Сбросить всё',exact=True).click();expect(search).to_have_value('')
        expect(audit.get_by_role('combobox',name='Период',exact=True)).to_have_value('week')
        expect(audit.get_by_role('combobox',name='Филиал',exact=True)).to_have_value('')
        signout(page);login(PREFIX+'-admin');audit=page.locator('.audit-history')
        expect(audit.locator('.audit-event')).to_have_count(50)
        assert not audit.get_by_label('Сотрудник',exact=True).locator(f'option[value="{users["foreign"]}"]').count()
        sql(f"DELETE FROM pride.staff_club_roles WHERE staff_user_id='{users['admin']}' RETURNING staff_user_id")
        page.evaluate("dispatchEvent(new Event('focus'))");expect(page.locator('.audit-event')).to_have_count(0)
        assert not errors,errors
        (SHOTS/f'{ENGINE}-results.json').write_text(json.dumps({'browser':browser.version,'metrics':metrics,'errors':errors,'checks':'search beyond page, AND/order/case/ё/#ID, exact UUID filters, namesakes/disabled, live Minsk/custom dates, total/keyset, empty/clear, focus/late/background/error/revoke, 1280/980/390/320 and reduced motion'},ensure_ascii=False,indent=2))
        print('UX-HISTORY PASS',ENGINE,browser.version,'jsErrors=',errors)
        browser.close()
finally:
    STATIC_TEMP.cleanup()
    if not EXISTING:
        subprocess.run(['docker','rm','-f',APP,DB],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        subprocess.run(['docker','network','rm',NET],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)

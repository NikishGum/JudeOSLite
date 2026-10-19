"""RD-11: real browser -> Go -> disposable PostgreSQL. No external club data."""
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

ROOT = 'http://127.0.0.1:18093'
REPO = Path(__file__).resolve().parents[3]
ENGINE = os.environ.get('PWA_BROWSER', 'chromium')
IMAGE = os.environ.get('PWA_TEST_IMAGE', 'judeoslite:ci')
assert ENGINE in ('chromium', 'webkit') and IMAGE.startswith('judeoslite:')
PREFIX = 'rd203-' + uuid.uuid4().hex[:8]
DB, APP, NET = PREFIX+'-db', PREFIX+'-app', PREFIX+'-net'
PASSWORD = 'synthetic-browser-password'
STATIC_TEMP = tempfile.TemporaryDirectory(prefix='rd203-static-')
static = Path(STATIC_TEMP.name)/'static'
shutil.copytree(REPO/'frontend/dist', static)
for file in (static, *static.rglob('*')):
    file.chmod(0o755 if file.is_dir() else 0o644)

def cmd(*args):
    return subprocess.check_output(args, text=True).strip()

def sql(statement):
    return cmd('docker', 'exec', DB, 'psql', '-U', 'test', '-d', 'rd203', '-Atc', statement)

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
    cmd('docker', 'run', '-d', '--name', DB, '--network', NET, '-e', 'POSTGRES_USER=test', '-e', 'POSTGRES_PASSWORD=test-only', '-e', 'POSTGRES_DB=rd203', 'postgres:18.6-bookworm')
    for _ in range(30):
        if subprocess.run(['docker', 'exec', DB, 'pg_isready', '-h', '127.0.0.1', '-U', 'test', '-d', 'rd203'], stdout=subprocess.DEVNULL).returncode == 0:
            break
        time.sleep(1)
    else:
        raise AssertionError('PostgreSQL not ready')
    env = ['-e', f'DATABASE_URL=postgres://test:test-only@{DB}:5432/rd203?sslmode=disable']
    cmd('docker', 'run', '--rm', '--network', NET, *env, IMAGE, '/app/migrate')
    owner(env)
    cmd('docker', 'create', '--name', APP, '--network', NET, *env, '-e', f'APP_ORIGIN={ROOT}', '-e', 'AUTH_INSECURE_LOCAL=true', '-p', '127.0.0.1:18093:8080', IMAGE)
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

        files = [("proof.pdf", "application/pdf", b"%PDF-1.7\nSynthetic private document\n%%EOF\n"),
                 ("proof.png", "image/png", base64.b64decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4//8/AAX+Av4N70a4AAAAAElFTkSuQmCC")),
                 ("proof.jpg", "image/jpeg", base64.b64decode("/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/2wBDAQkJCQwLDBgNDRgyIRwhMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjL/wAARCAABAAEDASIAAhEBAxEB/8QAHwAAAQUBAQEBAQEAAAAAAAAAAAECAwQFBgcICQoL/8QAtRAAAgEDAwIEAwUFBAQAAAF9AQIDAAQRBRIhMUEGE1FhByJxFDKBkaEII0KxwRVS0fAkM2JyggkKFhcYGRolJicoKSo0NTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqDhIWGh4iJipKTlJWWl5iZmqKjpKWmp6ipqrKztLW2t7i5usLDxMXGx8jJytLT1NXW19jZ2uHi4+Tl5ufo6erx8vP09fb3+Pn6/8QAHwEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoL/8QAtREAAgECBAQDBAcFBAQAAQJ3AAECAxEEBSExBhJBUQdhcRMiMoEIFEKRobHBCSMzUvAVYnLRChYkNOEl8RcYGRomJygpKjU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6goOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4uPk5ebn6Onq8vP09fb3+Pn6/9oADAMBAAIRAxEAPwD3+iiigD//2Q=="))]
        club=api(page,'POST','/api/clubs',{'name':'Synthetic RD203 A','address':'','timezone':'Europe/Minsk','version':0},201)['id']
        other=api(page,'POST','/api/clubs',{'name':'Synthetic RD203 B','address':'','timezone':'Europe/Minsk','version':0},201)['id']
        members=[]
        for i,scope in enumerate([club,club,other]):
            child=api(page,'POST',f'/api/clubs/{scope}/participants',{'identity':{'fullName':f'Synthetic Child {i}','birthDate':'2015-01-01','version':0},'joinedOn':'2020-01-01','contactKind':'email','contactValue':'synthetic@example.test','contactName':'Synthetic Contact','version':0},201)
            members.append(child['id'])
        family=api(page,'POST',f'/api/clubs/{club}/families',{'name':'Synthetic Documents Family','contactName':'Synthetic Parent','phone':'synthetic','email':'','membershipIds':members[:1],'version':0},201)
        family=api(page,'POST',f"/api/clubs/{club}/families/{family['id']}/accounts",{'login':'parent','displayName':'Synthetic Parent','password':PASSWORD,'version':family['version']},201)
        base=f'/api/clubs/{club}/participants/{members[0]}/documents'

        def open_documents(value, portal=False):
            value.goto(ROOT+('/families' if portal else f"/clubs/{club}/families?detail={family['id']}"))
            value.get_by_role('button',name='Synthetic Child 0',exact=False).click()
            value.get_by_role('tab',name='Документы',exact=True).click()
            expect(value.get_by_role('heading',name='Документы участника')).to_be_visible()

        open_documents(page)
        stored=[]
        for i,(filename,media,content) in enumerate(files):
            page.get_by_role('button',name='Добавить документ',exact=True).click()
            dialog=page.get_by_role('dialog',name='Новый документ',exact=True)
            dialog.get_by_label('Название',exact=True).fill(f'Synthetic Document {i}')
            dialog.get_by_label('Файл',exact=False).set_input_files({'name':filename,'mimeType':media,'buffer':content})
            with page.expect_response(lambda response:response.url==ROOT+base and response.request.method=='POST' and response.status==201):
                dialog.get_by_role('button',name='Загрузить документ',exact=True).click()
            expect(dialog).not_to_be_visible()
            link=page.get_by_role('link',name=f'Скачать Synthetic Document {i}',exact=True)
            expect(link).to_be_visible()
            with page.expect_download() as info:
                link.click()
            download=info.value
            assert download.suggested_filename==filename
            assert Path(download.path()).read_bytes()==content
            listed=api(page,'GET',base)
            stored.append(next(item for item in listed['items'] if item['filename']==filename))
            assert sql(f"SELECT encode(content,'hex') FROM pride.member_documents WHERE id='{stored[-1]['id']}'")==content.hex()
        # A full browser reload must reconstruct metadata and the working download.
        page.reload()
        page.get_by_role('button',name='Synthetic Child 0',exact=False).click()
        page.get_by_role('tab',name='Документы',exact=True).click()
        expect(page.get_by_role('link',name='Скачать Synthetic Document 0',exact=True)).to_be_visible()
        assert api(page,'GET',base+'?limit=2')['hasMore']
        assert api(page,'GET',base+'?limit=2&page=1')['total']==3

        parent_context=browser.new_context(service_workers='block',viewport={'width':390,'height':844})
        parent=parent_context.new_page()
        parent.on('pageerror',lambda error: errors.append(str(error)))
        parent.goto(ROOT)
        parent.get_by_label('Логин',exact=True).fill('parent')
        parent.get_by_label('Пароль',exact=True).fill(PASSWORD)
        parent.get_by_role('button',name='Войти',exact=True).click()
        expect(parent.get_by_role('button',name='Меню профиля',exact=True)).to_be_visible()
        open_documents(parent,True)
        expect(parent.get_by_role('button',name='Добавить документ',exact=True)).to_have_count(0)
        with parent.expect_download() as info:
            parent.get_by_role('link',name='Скачать Synthetic Document 0',exact=True).click()
        assert Path(info.value.path()).read_bytes()==files[0][2]
        assert parent.evaluate('document.documentElement.scrollWidth<=innerWidth')
        upload={'name':'Denied','kind':'other','filename':'proof.pdf','contentBase64':base64.b64encode(files[0][2]).decode(),'expiresOn':None}
        api(parent,'POST',base,upload,404)
        api(parent,'POST',base+f"/{stored[0]['id']}/delete",{'version':1},404)
        api(parent,'GET',f'/api/clubs/{club}/participants/{members[1]}/documents',expected=404)
        api(parent,'GET',f'/api/clubs/{other}/participants/{members[2]}/documents',expected=404)
        api(page,'POST',base,upload,403,csrf=False)
        api(page,'POST',base,{**upload,'contentBase64':base64.b64encode(b'<svg/>').decode()},400)
        api(page,'POST',base,{**upload,'filename':'../proof.pdf'},400)
        api(page,'POST',base+f"/{stored[0]['id']}/delete",{'version':2},409)
        page.get_by_role('button',name='Удалить Synthetic Document 0',exact=True).click()
        confirm=page.get_by_role('dialog',name='Удалить документ?',exact=True)
        with page.expect_response(lambda response:response.url.endswith('/delete') and response.status==200):
            confirm.get_by_role('button',name='Удалить документ',exact=True).click()
        expect(confirm).not_to_be_visible()
        expect(page.get_by_role('link',name='Скачать Synthetic Document 0',exact=True)).to_have_count(0)
        api(parent,'GET',base+f"/{stored[0]['id']}/download",expected=404)
        assert sql('SELECT count(*) FROM pride.member_documents')=='2'
        assert sql("SELECT count(*) FROM pride.audit_events WHERE entity_type='member_document'")=='4'
        assert sql("SELECT count(*) FROM pride.audit_events WHERE entity_type='member_document' AND (after_state::text LIKE '%Synthetic Document%' OR after_state::text LIKE '%filename%' OR after_state::text LIKE '%medical%')")=='0'
        # An already saved download link must lose access after unlink, without
        # relying on hiding the button in the frontend.
        family=api(page,'PUT',f"/api/clubs/{club}/families/{family['id']}",{'name':family['name'],'contactName':family['contactName'],'phone':family['phone'],'email':family['email'],'membershipIds':[],'version':family['version']})
        api(parent,'GET',base+f"/{stored[1]['id']}/download",expected=404)
        api(parent,'GET',base,expected=404)
        assert not errors,errors
        print(json.dumps({'engine':ENGINE,'browser':browser.version,'result':'PASS','transport':'real frontend -> Go -> PostgreSQL18.6','sampleDocument':stored[0],'checks':['PDF/PNG/JPEG UI upload','binary byte-for-byte browser download + SQL','reload','parent read-only mobile UI','CSRF/foreign UUID/magic/filename/version denial','UI deletion + SQL audit privacy','live unlink on saved URL']},ensure_ascii=False))
        browser.close()
finally:
    subprocess.run(['docker','rm','-f',APP,DB],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    subprocess.run(['docker','network','rm',NET],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    STATIC_TEMP.cleanup()

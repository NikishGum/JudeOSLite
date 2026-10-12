"""Disposable loopback server + attendance157-* PostgreSQL only; see attendance-v1.md."""
import json
import os
import subprocess
import uuid
from playwright.sync_api import sync_playwright, expect

ROOT = 'http://127.0.0.1:8080'
CONTAINER = os.environ.get('ATTENDANCE_TEST_DB_CONTAINER', 'attendance157-db')
assert CONTAINER.startswith('attendance157-')

def sql(statement):
    return subprocess.check_output(['docker', 'exec', CONTAINER, 'psql', '-U', 'test', '-d', 'browser157', '-Atc', statement], text=True).strip()

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True, executable_path=os.environ.get('CHROMIUM_PATH', '/usr/bin/chromium'))
    owner = browser.new_page(viewport={'width': 1280, 'height': 900})
    errors = []
    def signin(page, login, password):
        page.on('pageerror', lambda e: errors.append(str(e)))
        page.goto(ROOT)
        page.get_by_label('Логин', exact=True).fill(login)
        page.get_by_label('Пароль', exact=True).fill(password)
        page.get_by_role('button', name='Войти', exact=True).click()
        expect(page.get_by_role('button', name='Выйти', exact=True)).to_be_visible()
    def api(page, method, path, data=None):
        return page.evaluate('''async ([method,path,data])=>{
          const me=await (await fetch('/api/auth/me')).json();
          const r=await fetch(path,{method,headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrfToken},body:method==='GET'?undefined:JSON.stringify(data)});
          if(!r.ok)throw Error(await r.text());return r.json();
        }''', [method, path, data])
    signin(owner, 'owner', 'synthetic-browser-password')
    suffix = uuid.uuid4().hex[:8]
    club = api(owner, 'POST', '/api/clubs', {'name': 'Attendance '+suffix, 'timezone': 'Europe/Minsk'})
    base = '/api/clubs/'+club['id']
    group = api(owner, 'POST', base+'/groups', {'name': 'Journal group '+suffix})
    empty = api(owner, 'POST', base+'/groups', {'name': 'Empty group '+suffix})
    trainer = api(owner, 'POST', base+'/trainers', {'login': 'attendance-'+suffix, 'displayName': 'Synthetic Trainer', 'password': 'synthetic-trainer-password'})
    replacement = api(owner, 'POST', base+'/trainers', {'login': 'next-'+suffix, 'displayName': 'Synthetic Replacement', 'password': 'synthetic-trainer-password'})
    day = owner.evaluate("new Intl.DateTimeFormat('en-CA',{timeZone:'Europe/Minsk',year:'numeric',month:'2-digit',day:'2-digit'}).format(new Date())")
    names = ['Synthetic A '+suffix, 'Synthetic B '+suffix, 'Synthetic C '+suffix]
    for name in names:
        api(owner, 'POST', base+'/participants', {'identity': {'fullName': name, 'birthDate': '2015-01-01'}, 'joinedOn': day, 'contactKind': 'phone', 'contactValue': '+375291234567', 'initialGroup': {'groupId': group['id'], 'from': day}})
    def session(group_id, hour):
        return sql(f"INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES('{club['id']}','{group_id}','{trainer['id']}','{day}','Europe/Minsk','{day} {hour}:00+03','{day} {hour}:45+03') RETURNING id") .splitlines()[0]
    sid = session(group['id'], '18')
    cancel_id = session(group['id'], '19')
    empty_id = session(empty['id'], '20')
    jbase = base+'/trainer-sessions/'+sid+'/journal'
    page = browser.new_page(viewport={'width': 390, 'height': 844})
    signin(page, trainer['login'], 'synthetic-trainer-password')
    page.get_by_label('Филиал', exact=True).select_option(club['id'])
    screen = page.locator('.trainer-sessions')
    expect(screen).to_contain_text('Мои занятия и журнал')
    expect(screen.get_by_role('button', name='Открыть журнал', exact=True)).to_have_count(3)
    screen.get_by_role('button', name='Открыть журнал', exact=True).first.click()
    journal = screen.locator('.session-journal')
    calls = []
    page.on('request', lambda req: calls.append(req.url) if req.method == 'POST' and req.url.endswith('/open') else None)
    journal.get_by_role('button', name='Подтвердить открытие журнала', exact=True).evaluate('(b)=>{b.click();b.click()}')
    expect(journal).to_contain_text('Не отмечено: 3 из 3')
    assert len(calls) == 1
    assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}'") == '3'
    forms = journal.locator('.journal-mark')
    forms.first.locator('select').select_option('illness')
    page.route('**/journal/records/*', lambda route: route.abort() if route.request.method == 'PUT' else route.continue_())
    forms.first.get_by_role('button', name='Сохранить отметку', exact=True).click()
    expect(journal.get_by_role('alert')).to_contain_text('Нет соединения')
    expect(forms.first.locator('select')).to_have_value('illness')
    assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND state IS NOT NULL") == '0'
    page.unroute('**/journal/records/*')
    forms.first.get_by_role('button', name='Сохранить отметку', exact=True).evaluate('(b)=>{b.click();b.click()}')
    expect(journal).to_contain_text('Не отмечено: 2 из 3')
    assert sql(f"SELECT count(*) FROM pride.audit_events WHERE entity_type='attendance_record' AND after_state->>'sessionId'='{sid}'") == '1'
    # Another tab updates a different card. A stale correction remains unsaved and demands reload.
    current = api(page, 'GET', jbase)
    api(page, 'PUT', jbase+'/records/'+current['records'][1]['id'], {'version': current['records'][1]['version'], 'sessionVersion': current['session']['version'], 'state': 'absent', 'absenceReason': 'excused'})
    forms.first.locator('select').select_option('present')
    forms.first.get_by_role('button', name='Сохранить отметку', exact=True).click()
    expect(journal.get_by_role('alert')).to_contain_text('Журнал изменился')
    journal.get_by_role('button', name='Загрузить актуальный журнал и сбросить ввод', exact=True).click()
    expect(journal).to_contain_text('Не отмечено: 1 из 3')
    forms.first.locator('select').select_option('present')
    forms.first.get_by_role('button', name='Сохранить отметку', exact=True).click()
    expect(journal.locator('.journal-records li').first).to_contain_text('Присутствует')
    # All responsive widths; no overflow, visible touch targets or JS errors.
    for width in (320, 390, 1280):
        page.set_viewport_size({'width': width, 'height': 900})
        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'), width
        for button in journal.locator('button:visible').all():
            assert button.bounding_box()['height'] >= 44
        page.screenshot(path=f'/tmp/attendance157-{width}.png', full_page=True)
    journal.get_by_role('button', name='Завершить журнал', exact=True).click()
    expect(journal).to_contain_text('Не отмечено: 1. При завершении')
    journal.get_by_role('button', name='Продолжить отметки', exact=True).click()
    assert sql(f"SELECT status FROM pride.training_sessions WHERE id='{sid}'") == 'open'
    journal.get_by_role('button', name='Завершить журнал', exact=True).click()
    journal.get_by_role('button', name='Подтвердить завершение', exact=True).click()
    expect(journal).to_contain_text('Журнал завершён')
    expect(journal.locator('.journal-mark')).to_have_count(0)
    assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND state IS NULL") == '0'
    assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND origin='close_default' AND absence_reason='unknown'") == '1'
    page.reload()
    screen.get_by_role('button', name='Посмотреть журнал', exact=True).first.click()
    expect(journal).to_contain_text('При завершении журнала')
    # Cancellation after real snapshot preserves manual authors and remaining NULLs.
    other = api(page, 'POST', base+'/trainer-sessions/'+cancel_id+'/journal/open', {'version': 1})
    other = api(page, 'PUT', base+'/trainer-sessions/'+cancel_id+'/journal/records/'+other['records'][0]['id'], {'version': 1, 'sessionVersion': other['session']['version'], 'state': 'present', 'absenceReason': None})
    api(owner, 'POST', base+'/sessions/'+cancel_id+'/cancel', {'version': other['session']['version'], 'confirmOpen': True, 'reason': 'Synthetic cancellation'})
    screen.get_by_role('button', name='Обновить занятия', exact=True).click()
    expect(screen).to_contain_text('Synthetic cancellation')
    assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{cancel_id}' AND state IS NULL") == '2'
    # Empty roster can close. Replacement invalidates current trainer writes/reads.
    screen.get_by_role('button', name='Открыть журнал', exact=True).click()
    journal.get_by_role('button', name='Подтвердить открытие журнала', exact=True).click()
    expect(journal).to_contain_text('В снимке состава нет участников')
    j = api(page, 'GET', base+'/trainer-sessions/'+empty_id+'/journal')
    api(owner, 'POST', base+'/sessions/'+empty_id+'/replace-trainer', {'version': j['session']['version'], 'trainerId': replacement['id']})
    journal.get_by_role('button', name='Завершить журнал', exact=True).click()
    journal.get_by_role('button', name='Подтвердить завершение', exact=True).click()
    expect(journal.get_by_role('alert')).to_contain_text('недоступны')
    expect(journal.locator('.journal-records li')).to_have_count(0)
    page.get_by_role('button', name='Выйти', exact=True).click()
    expect(page.get_by_label('Логин', exact=True)).to_be_visible()
    assert page.locator('.trainer-sessions').count() == 0
    assert not errors, errors
    print(json.dumps({'browser': browser.version, 'widths': [320,390,1280], 'flow': 'open/mark/retry/conflict/close/reload/cancel/replacement/logout', 'database': 'PostgreSQL 18.6'}, ensure_ascii=False))
    browser.close()

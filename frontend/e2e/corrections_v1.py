from runtime_ui import signout
"""Synthetic loopback server + disposable corrections159-* database only; corrections-v1.md."""
import json
import os
import subprocess
import uuid
from playwright.sync_api import sync_playwright, expect

ROOT = 'http://127.0.0.1:8080'
CONTAINER = os.environ.get('CORRECTIONS_TEST_DB_CONTAINER', 'corrections159-db')
assert CONTAINER.startswith('corrections159-')

def sql(statement):
    return subprocess.check_output(['docker', 'exec', CONTAINER, 'psql', '-U', 'test', '-d', 'browser159', '-Atc', statement], text=True).strip()

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True, executable_path=os.environ.get('CHROMIUM_PATH', '/usr/bin/chromium'))
    errors = []
    def signin(page, login, password):
        page.on('pageerror', lambda e: errors.append(str(e)))
        page.goto(ROOT)
        page.get_by_label('Логин', exact=True).fill(login)
        page.get_by_label('Пароль', exact=True).fill(password)
        page.get_by_role('button', name='Войти', exact=True).click()
        expect(page.get_by_role('button', name='Меню профиля', exact=True)).to_be_visible()
    def api(page, method, path, data=None, status=200):
        result = page.evaluate('''async ([method,path,data])=>{
          const me=await (await fetch('/api/auth/me')).json();
          const r=await fetch(path,{method,headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrfToken},body:method==='GET'?undefined:JSON.stringify(data)});
          return {status:r.status,body:await r.json()};
        }''', [method, path, data])
        assert result['status'] == status, result
        return result['body']
    owner = browser.new_page(viewport={'width': 1280, 'height': 900})
    signin(owner, 'owner', 'synthetic-browser-password')
    suffix = uuid.uuid4().hex[:8]
    club = api(owner, 'POST', '/api/clubs', {'name': 'Corrections '+suffix, 'timezone': 'Europe/Minsk'}, 201)
    other = api(owner, 'POST', '/api/clubs', {'name': 'Other '+suffix, 'timezone': 'Europe/Minsk'}, 201)
    base = '/api/clubs/'+club['id']
    group = api(owner, 'POST', base+'/groups', {'name': 'Historical group '+suffix}, 201)
    empty = api(owner, 'POST', base+'/groups', {'name': 'Other group '+suffix}, 201)
    trainer = api(owner, 'POST', base+'/trainers', {'login': 'ct-'+suffix, 'displayName': 'Synthetic Trainer', 'password': 'synthetic-trainer-password'}, 201)
    admin = api(owner, 'POST', '/api/staff', {'login': 'ca-'+suffix, 'displayName': 'Synthetic Administrator', 'password': 'synthetic-admin-password', 'clubIds': [club['id']]}, 201)
    members = []
    for name in ('A Synthetic '+suffix, 'B Synthetic '+suffix):
        members.append(api(owner, 'POST', base+'/participants', {'identity': {'fullName': name, 'birthDate': '1990-01-01'}, 'joinedOn': '1999-01-01', 'contactKind': 'phone', 'contactValue': '+375291234567', 'initialGroup': {'groupId': group['id'], 'from': '2000-01-01'}}, 201))
    sid = sql(f"INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES('{club['id']}','{group['id']}','{trainer['id']}','2000-02-01','Europe/Minsk','2000-02-01 18:00+03','2000-02-01 19:00+03') RETURNING id").splitlines()[0]
    tpage = browser.new_page()
    signin(tpage, trainer['login'], 'synthetic-trainer-password')
    trainer_path = base+'/trainer-sessions/'+sid+'/journal'
    j = api(tpage, 'POST', trainer_path+'/open', {'version': 1})
    j = api(tpage, 'PUT', trainer_path+'/records/'+j['records'][0]['id'], {'version': 1, 'sessionVersion': j['session']['version'], 'state': 'absent', 'absenceReason': 'illness'})
    j = api(tpage, 'POST', trainer_path+'/close', {'version': j['session']['version']})
    rid = j['records'][0]['id']
    sql(f"UPDATE pride.club_memberships SET archived_at=now() WHERE id='{j['records'][0]['membershipId']}'; UPDATE pride.group_memberships SET valid_until='2000-02-02' WHERE club_membership_id='{j['records'][0]['membershipId']}'; UPDATE pride.training_groups SET archived_at=now() WHERE id='{group['id']}'")
    page = browser.new_page(viewport={'width': 390, 'height': 844})
    signin(page, admin['login'], 'synthetic-admin-password')
    page.get_by_label('Филиал', exact=True).select_option(club['id'])
    screen = page.locator('.admin-journals')
    screen.get_by_label('Дата журнала', exact=True).fill('2000-02-01')
    expect(screen).to_contain_text('Занятий на дату: 1')
    screen.get_by_label('Группа журнала', exact=True).select_option(empty['id'])
    expect(screen).to_contain_text('Занятий на дату: 0')
    screen.get_by_label('Группа журнала', exact=True).select_option(group['id'])
    expect(screen).to_contain_text('Занятий на дату: 1')
    screen.get_by_role('button', name='Посмотреть журнал филиала', exact=True).click()
    journal = screen.locator('.admin-journal')
    row = journal.locator('[data-record-id="'+rid+'"]')
    expect(row).to_contain_text('Карточка в архиве')
    expect(row).to_contain_text('Отсутствует · Болезнь')
    row.get_by_role('button', name='История отметки', exact=True).click()
    expect(row.locator('.record-history')).to_contain_text('Synthetic Trainer')
    row.get_by_role('button', name='Исправить отметку', exact=True).click()
    form = row.locator('form')
    expect(form.get_by_role('button', name='Сохранить исправление', exact=True)).to_be_disabled()
    form.get_by_label('Новая отметка', exact=True).select_option('present')
    form.get_by_label('Причина исправления', exact=True).fill('   ')
    expect(form.get_by_role('button', name='Сохранить исправление', exact=True)).to_be_disabled()
    form.get_by_label('Причина исправления', exact=True).fill('Синтетическая проверка давней отметки')
    path = base+'/journals/'+sid+'/records/'+rid
    page.route('**/journals/*/records/*', lambda route: route.abort() if route.request.method == 'PUT' else route.continue_())
    form.get_by_role('button', name='Сохранить исправление', exact=True).click()
    expect(journal.get_by_role('alert')).to_contain_text('Нет соединения')
    expect(form.get_by_label('Причина исправления', exact=True)).to_have_value('Синтетическая проверка давней отметки')
    assert sql(f"SELECT state||':'||absence_reason FROM pride.attendance_records WHERE id='{rid}'") == 'absent:illness'
    page.unroute('**/journals/*/records/*')
    calls = []
    page.on('request', lambda req: calls.append(req.url) if req.method == 'PUT' and req.url.endswith('/records/'+rid) else None)
    form.get_by_role('button', name='Сохранить исправление', exact=True).evaluate('(b)=>{b.click();b.click()}')
    expect(row.locator('form')).to_have_count(0)
    expect(row).to_contain_text('Присутствует')
    expect(row.locator('.record-history')).to_contain_text('Синтетическая проверка давней отметки')
    expect(row.locator('.record-history')).to_contain_text('Synthetic Administrator')
    assert len(calls) == 1
    assert sql(f"SELECT state||':'||coalesce(absence_reason,'NULL')||':'||origin FROM pride.attendance_records WHERE id='{rid}'") == 'present:NULL:correction'
    assert sql(f"SELECT count(*) FROM pride.audit_events WHERE entity_id='{rid}' AND action='corrected'") == '1'
    # A second administrator's correction must cause a conflict in the stale form.
    row.get_by_role('button', name='Исправить отметку', exact=True).click()
    form.get_by_label('Новая отметка', exact=True).select_option('excused')
    form.get_by_label('Причина исправления', exact=True).fill('Устаревший ввод сохраняется')
    current = api(owner, 'GET', base+'/journals/'+sid)
    record = next(r for r in current['records'] if r['id'] == rid)
    api(owner, 'PUT', path, {'version': record['version'], 'state': 'absent', 'absenceReason': 'unknown', 'changeReason': 'Параллельная правка владельца'})
    form.get_by_role('button', name='Сохранить исправление', exact=True).click()
    expect(journal.get_by_role('alert')).to_contain_text('Отметка уже изменена')
    expect(form.get_by_label('Причина исправления', exact=True)).to_have_value('Устаревший ввод сохраняется')
    expect(form.get_by_role('button', name='Сохранить исправление', exact=True)).to_be_disabled()
    journal.get_by_role('button', name='Перечитать журнал и сбросить ввод', exact=True).click()
    expect(row).to_contain_text('Отсутствует · Причина неизвестна')
    expect(row.locator('form')).to_have_count(0)
    row.get_by_role('button', name='История отметки', exact=True).click()
    expect(row.locator('.record-history')).to_contain_text('Параллельная правка владельца')
    row.get_by_role('button', name='Исправить отметку', exact=True).click()
    form.get_by_label('Новая отметка', exact=True).select_option('excused')
    form.get_by_label('Причина исправления', exact=True).fill('Подтверждённая уважительная причина')
    form.get_by_role('button', name='Сохранить исправление', exact=True).click()
    expect(row.locator('form')).to_have_count(0)
    expect(row).to_contain_text('Отсутствует · Уважительная причина')
    assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}'") == '2'
    assert sql(f"SELECT status FROM pride.training_sessions WHERE id='{sid}'") == 'closed'
    for width in (320, 390, 1280):
        page.set_viewport_size({'width': width, 'height': 900})
        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'), width
        for button in journal.locator('button:visible').all():
            assert button.bounding_box()['height'] >= 44
        page.screenshot(path=f'/tmp/corrections159-{width}.png', full_page=True)
    page.reload()
    expect(page.locator('.admin-journals')).to_be_visible()
    screen.get_by_label('Дата журнала', exact=True).fill('2000-02-01')
    expect(screen).to_contain_text('Занятий на дату: 1')
    screen.get_by_role('button', name='Посмотреть журнал филиала', exact=True).click()
    expect(row).to_contain_text('Подтверждённая уважительная причина')
    api(tpage, 'PUT', path, {'version': 5, 'state': 'present', 'changeReason': 'trainer forbidden'}, 404)
    api(page, 'GET', '/api/clubs/'+other['id']+'/journals/'+sid, status=404)
    tpage.get_by_label('Филиал', exact=True).select_option(club['id'])
    expect(tpage.locator('.admin-journals')).to_have_count(0)
    owner.get_by_label('Филиал', exact=True).select_option(club['id'])
    owner_screen = owner.locator('.admin-journals')
    owner_screen.get_by_label('Дата журнала', exact=True).fill('2000-02-01')
    expect(owner_screen).to_contain_text('Занятий на дату: 1')
    owner_screen.get_by_role('button', name='Посмотреть журнал филиала', exact=True).click()
    expect(owner_screen.locator('.admin-journal')).to_contain_text('Подтверждённая уважительная причина')
    owner.get_by_label('Филиал', exact=True).select_option(other['id'])
    expect(owner.locator('[data-record-id="'+rid+'"]')).to_have_count(0)
    sql(f"DELETE FROM pride.staff_club_roles WHERE staff_user_id='{admin['id']}' AND role='administrator'")
    journal.get_by_role('button', name='Обновить журнал', exact=True).click()
    expect(page.locator('[data-record-id="'+rid+'"]')).to_have_count(0)
    signout(page)
    expect(page.get_by_label('Логин', exact=True)).to_be_visible()
    expect(page.locator('.admin-journals')).to_have_count(0)
    assert not errors, errors
    browser.close()
    print('Corrections: UI/API/PostgreSQL, historical archive, filters, history, mandatory reason, network retry, double submit, 409/reload, role isolation/revoke, owner, reload, 320/390/1280 passed')

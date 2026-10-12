"""Synthetic loopback server + disposable reports160-* database only; reports-v1.md."""
import os
import subprocess
import uuid
from playwright.sync_api import sync_playwright, expect

ROOT = 'http://127.0.0.1:8080'
CONTAINER = os.environ.get('REPORTS_TEST_DB_CONTAINER', 'reports160-db')
assert CONTAINER.startswith('reports160-')

def sql(statement):
    return subprocess.check_output(['docker', 'exec', CONTAINER, 'psql', '-U', 'test', '-d', 'browser160', '-Atc', statement], text=True).strip()

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True, executable_path=os.environ.get('CHROMIUM_PATH', '/usr/bin/chromium'))
    errors = []
    def signin(page, login, password):
        page.on('pageerror', lambda e: errors.append(str(e)))
        page.goto(ROOT)
        page.get_by_label('Логин', exact=True).fill(login)
        page.get_by_label('Пароль', exact=True).fill(password)
        page.get_by_role('button', name='Войти', exact=True).click()
        expect(page.get_by_role('button', name='Выйти', exact=True)).to_be_visible()
    def api(page, method, path, data=None, status=200):
        result = page.evaluate('''async ([method,path,data])=>{
          const me=await (await fetch('/api/auth/me')).json();
          const r=await fetch(path,{method,headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrfToken},body:method==='GET'?undefined:JSON.stringify(data)});
          return {status:r.status,body:await r.json(),cache:r.headers.get('Cache-Control')};
        }''', [method, path, data])
        assert result['status'] == status and result['cache'] == 'no-store', result
        return result['body']
    owner = browser.new_page(viewport={'width': 390, 'height': 844})
    signin(owner, 'owner', 'synthetic-browser-password')
    suffix = uuid.uuid4().hex[:8]
    clubs, groups, trainers, tpages = [], [], [], []
    for name, count in [('A', 4), ('B', 3)]:
        c = api(owner, 'POST', '/api/clubs', {'name': 'Report '+name+' '+suffix, 'timezone': 'Europe/Minsk'}, 201)
        base = '/api/clubs/'+c['id']
        g = api(owner, 'POST', base+'/groups', {'name': 'Same group '+suffix}, 201)
        trainer = api(owner, 'POST', base+'/trainers', {'login': 'rt-'+name.lower()+'-'+suffix, 'displayName': 'Synthetic Trainer '+name, 'password': 'synthetic-trainer-password'}, 201)
        for i in range(count):
            api(owner, 'POST', base+'/participants', {'identity': {'fullName': f'{i} Synthetic {suffix}', 'birthDate': '1990-01-01'}, 'joinedOn': '1999-01-01', 'contactKind': 'phone', 'contactValue': '+375291234567', 'initialGroup': {'groupId': g['id'], 'from': '1999-01-01'}}, 201)
        tp = browser.new_page()
        signin(tp, trainer['login'], 'synthetic-trainer-password')
        clubs.append(c); groups.append(g); trainers.append(trainer); tpages.append(tp)
    a, b = clubs; ga, gb = groups; ta, tb = tpages
    admin = api(owner, 'POST', '/api/staff', {'login': 'ra-'+suffix, 'displayName': 'Synthetic Administrator', 'password': 'synthetic-admin-password', 'clubIds': [a['id'], b['id']]}, 201)
    empty = api(owner, 'POST', '/api/clubs/'+a['id']+'/groups', {'name': 'Empty '+suffix}, 201)
    def new_session(index, group, time, date='2000-02-01', source='2000-02-01'):
        # AT TIME ZONE applies historical Europe/Minsk offset (UTC+02 in 2000).
        return sql(f"INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES('{clubs[index]['id']}','{group['id']}','{trainers[index]['id']}','{source}','Europe/Minsk',('{date} {time}'::timestamp AT TIME ZONE 'Europe/Minsk'),('{date} {time}'::timestamp AT TIME ZONE 'Europe/Minsk')+interval '1 hour') RETURNING id").splitlines()[0]
    def journal(index, sid, marks, close=True):
        path = '/api/clubs/'+clubs[index]['id']+'/trainer-sessions/'+sid+'/journal'
        j = api(tpages[index], 'POST', path+'/open', {'version': 1})
        for i, state, reason in marks:
            j = api(tpages[index], 'PUT', path+'/records/'+j['records'][i]['id'], {'version': j['records'][i]['version'], 'sessionVersion': j['session']['version'], 'state': state, 'absenceReason': reason})
        return api(tpages[index], 'POST', path+'/close', {'version': j['session']['version']}) if close else j
    sa = new_session(0, ga, '00:30', source='2000-01-31')
    ja = journal(0, sa, [(0, 'present', None), (1, 'present', None), (2, 'absent', 'illness')])
    sb = new_session(1, gb, '18:00')
    journal(1, sb, [(0, 'present', None), (1, 'absent', 'illness'), (2, 'absent', 'excused')])
    journal(0, new_session(0, empty, '19:00'), [])
    journal(0, new_session(0, ga, '18:00'), [(0, 'present', None)], close=False)
    new_session(1, gb, '19:00')
    sc = new_session(0, ga, '20:00')
    jc = journal(0, sc, [(0, 'absent', 'illness')], close=False)
    api(owner, 'POST', '/api/clubs/'+a['id']+'/sessions/'+sc+'/cancel', {'version': jc['session']['version'], 'reason': 'synthetic cancel', 'confirmOpen': True})
    journal(0, new_session(0, empty, '00:00', date='2000-02-02', source='2000-02-02'), [])
    mid = ja['records'][0]['membershipId']
    sql(f"UPDATE pride.group_memberships SET valid_until='2000-02-02' WHERE club_membership_id='{mid}'; INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from) VALUES('{a['id']}','{empty['id']}','{mid}','2000-02-02'); UPDATE pride.club_memberships SET archived_at=now() WHERE id='{mid}'; UPDATE pride.training_groups SET archived_at=now() WHERE id='{ga['id']}'")
    report_path = '/api/reports/attendance?from=2000-02-01&until=2000-02-01'
    r = api(owner, 'GET', report_path)
    assert [r['total'][k] for k in ('closed', 'planned', 'open', 'present', 'absent', 'unknown', 'illness', 'excused')] == [3, 1, 1, 3, 4, 1, 2, 1], r
    assert len(r['clubs']) == 2 and len(r['groups']) == 3
    screen = owner.locator('.owner-dashboard')
    def period():
        screen.get_by_label('Период с', exact=True).fill('2000-02-01')
        screen.get_by_label('Период по', exact=True).fill('2000-02-01')
    def metric(name, text):
        expect(screen.locator('[data-metric="'+name+'"]')).to_have_text(text)
    period(); metric('closed', '3'); metric('present', '3'); metric('absent', '4'); metric('percentage', '42,9%'); metric('planned', '1'); metric('open', '1')
    expect(screen.locator('[data-metric="reasons"]')).to_contain_text('неизвестна — 1; болезнь — 2; уважительная — 1')
    expect(screen.locator('tr[data-group-id="'+ga['id']+'"]')).to_contain_text(a['name'])
    expect(screen.locator('tr[data-group-id="'+gb['id']+'"]')).to_contain_text(b['name'])
    screen.get_by_label('Филиал отчёта', exact=True).select_option(a['id']); metric('closed', '2'); metric('percentage', '50,0%')
    screen.get_by_label('Группа отчёта', exact=True).select_option(empty['id']); metric('closed', '1'); metric('percentage', '—')
    screen.get_by_label('Филиал отчёта', exact=True).select_option(b['id']); metric('percentage', '33,3%')
    assert screen.get_by_label('Группа отчёта', exact=True).input_value() == ''
    screen.get_by_label('Филиал отчёта', exact=True).select_option(''); metric('percentage', '42,9%')
    # UI correction in this window triggers a fresh SQL report immediately.
    owner.get_by_label('Филиал', exact=True).select_option(a['id'])
    journals = owner.locator('.admin-journals')
    journals.get_by_label('Дата журнала', exact=True).fill('2000-02-01')
    journals.locator('[data-session-id="'+sa+'"]').get_by_role('button', name='Посмотреть журнал филиала', exact=True).click()
    row = journals.locator('[data-record-id="'+ja['records'][2]['id']+'"]')
    row.get_by_role('button', name='Исправить отметку', exact=True).click()
    row.get_by_label('Новая отметка', exact=True).select_option('present')
    row.get_by_label('Причина исправления', exact=True).fill('Synthetic report correction')
    row.get_by_role('button', name='Сохранить исправление', exact=True).click()
    metric('present', '4'); metric('absent', '3'); metric('percentage', '57,1%')
    assert sql(f"SELECT count(*) FROM pride.audit_events WHERE entity_id='{ja['records'][2]['id']}' AND action='corrected'") == '1'
    # A failed request clears totals; retry retains the chosen period.
    owner.route('**/api/reports/attendance?*', lambda route: route.abort())
    screen.get_by_role('button', name='Обновить статистику', exact=True).click()
    expect(screen.get_by_role('alert')).to_contain_text('Нет соединения')
    expect(screen.locator('[data-metric="present"]')).to_have_count(0)
    owner.unroute('**/api/reports/attendance?*')
    screen.get_by_role('button', name='Повторить загрузку статистики', exact=True).click(); metric('percentage', '57,1%')
    # A late response from A cannot replace the subsequently selected B.
    held = []
    def delay(route):
        if 'clubId='+a['id'] in route.request.url:
            held.append((route, route.fetch()))
        else:
            route.continue_()
    owner.route('**/api/reports/attendance?*', delay)
    screen.get_by_label('Филиал отчёта', exact=True).select_option(a['id'])
    owner.wait_for_timeout(200)
    screen.get_by_label('Филиал отчёта', exact=True).select_option(b['id']); metric('percentage', '33,3%')
    assert held
    for route, response in held:
        route.fulfill(response=response)
    metric('percentage', '33,3%')
    owner.unroute('**/api/reports/attendance?*')
    screen.get_by_label('Филиал отчёта', exact=True).select_option(''); metric('percentage', '57,1%')
    screen.get_by_label('Период с', exact=True).fill('2000-02-02'); expect(screen.get_by_role('alert')).to_contain_text('начало периода')
    expect(screen.locator('[data-metric="closed"]')).to_have_count(0)
    screen.get_by_label('Период по', exact=True).fill('2000-02-02'); metric('closed', '1'); metric('percentage', '—')
    period(); metric('percentage', '57,1%')
    for width in (320, 390, 1280):
        owner.set_viewport_size({'width': width, 'height': 900})
        assert owner.evaluate('document.documentElement.scrollWidth <= innerWidth'), width
        for control in screen.locator('button:visible,input:visible,select:visible').all():
            assert control.bounding_box()['height'] >= 44
        screen.screenshot(path=f'/tmp/reports160-{width}.png')
    owner.reload(); period(); metric('percentage', '57,1%')
    ap = browser.new_page(); signin(ap, admin['login'], 'synthetic-admin-password')
    expect(ap.locator('.owner-dashboard')).to_have_count(0)
    api(ap, 'GET', report_path, status=403); api(ta, 'GET', '/api/reports/filters', status=403)
    owner.get_by_role('button', name='Выйти', exact=True).click()
    expect(owner.locator('.owner-dashboard')).to_have_count(0)
    signin(owner, admin['login'], 'synthetic-admin-password')
    expect(owner.locator('.owner-dashboard')).to_have_count(0)
    assert not errors, errors
    browser.close()
    print('Reports: manual 3/7=42.9%, corrected 4/7=57.1%; UI/API/PostgreSQL, two branches/same names, period, zero denominator, transfer/archive, cancellation/open, immediate correction refresh, failure/retry, stale response, logout/roles, 320/390/1280 passed')

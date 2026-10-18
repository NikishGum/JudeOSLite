from runtime_ui import signout
"""V1-09/V1-10 browser -> real API -> disposable PostgreSQL; see attendance-mobile-v1.md."""
import json
import os
import subprocess
import uuid
from playwright.sync_api import sync_playwright, expect

ROOT = 'http://127.0.0.1:8080'
CONTAINER = os.environ.get('ATTENDANCE_TEST_DB_CONTAINER', 'attendance158-db')
assert CONTAINER.startswith('attendance158-')


def sql(statement):
    return subprocess.check_output(['docker', 'exec', CONTAINER, 'psql', '-U', 'test', '-d', 'browser158', '-Atc', statement], text=True).strip()


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
        expect(page.get_by_role('button', name='Меню профиля', exact=True)).to_be_visible()

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
    names = ['Synthetic '+letter+' '+suffix for letter in 'ABCDEF']
    names[0] += ' ДлинноеИмяДляПроверкиПереносаБезГоризонтальнойПрокрутки'*2
    for name in names:
        api(owner, 'POST', base+'/participants', {'identity': {'fullName': name, 'birthDate': '2015-01-01'}, 'joinedOn': day, 'contactKind': 'phone', 'contactValue': '+375291234567', 'initialGroup': {'groupId': group['id'], 'from': day}})

    def session(group_id, hour):
        return sql(f"INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES('{club['id']}','{group_id}','{trainer['id']}','{day}','Europe/Minsk','{day} {hour}:00+03','{day} {hour}:45+03') RETURNING id").splitlines()[0]

    sid = session(group['id'], '18')
    cancel_id = session(group['id'], '19')
    empty_id = session(empty['id'], '20')
    replace_id = session(group['id'], '21')
    revoke_id = session(group['id'], '22')
    retry_id = session(group['id'], '17')
    jbase = base+'/trainer-sessions/'+sid+'/journal'
    context = browser.new_context(viewport={'width': 390, 'height': 844}, is_mobile=True, has_touch=True)
    page = context.new_page()
    signin(page, trainer['login'], 'synthetic-trainer-password')
    page.get_by_label('Филиал', exact=True).select_option(club['id'])
    screen = page.locator('.trainer-sessions')
    journal = screen.locator('.session-journal')
    card = journal.locator('.attendance-card')
    dialog = page.get_by_role('dialog')

    def open_session(session_id):
        group_name = empty['name'] if session_id == empty_id else group['name']
        # Use the displayed hour to disambiguate sessions of the same group.
        hour = {sid:'18:00', cancel_id:'19:00', empty_id:'20:00', replace_id:'21:00', revoke_id:'22:00', retry_id:'17:00'}[session_id]
        row = screen.locator('.trainer-session-list li').filter(has_text=hour).filter(has_text=group_name)
        row.get_by_role('button').click()
        return row

    expect(screen).to_contain_text('Мои занятия и журнал')
    open_session(sid)
    calls = []
    page.on('request', lambda req: calls.append(req.url) if req.method == 'POST' and req.url.endswith('/open') else None)
    journal.get_by_role('button', name='Подтвердить открытие журнала', exact=True).evaluate('(b)=>{b.click();b.click()}')
    expect(journal).to_contain_text('Не отмечено: 6 из 6')
    expect(card).to_contain_text(names[0])
    assert len(calls) == 1
    assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}'") == '6'

    # Failed presence request leaves the card and count unchanged, with an explicit retry.
    page.route('**/journal/records/*', lambda route: route.abort())
    card.get_by_role('button', name='Присутствует', exact=True).click()
    expect(journal.get_by_role('alert')).to_contain_text('Нет соединения')
    expect(card).to_contain_text(names[0])
    page.unroute('**/journal/records/*')
    # No advancing or optimistic success while a real retry is held; double click sends once.
    held = []
    page.route('**/journal/records/*', lambda route: held.append(route))
    card.get_by_role('button', name='Повторить сохранение присутствия', exact=True).evaluate('(b)=>{b.click();b.click()}')
    expect(card.get_by_role('button', name='Сохраняем…', exact=True)).to_be_disabled()
    expect(card).to_contain_text(names[0])
    expect(journal).to_contain_text('Отмечено: 0 из 6')
    assert len(held) == 1
    assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND state IS NOT NULL") == '0'
    held[0].continue_()
    page.unroute('**/journal/records/*')
    expect(card).to_contain_text(names[1])
    expect(journal).to_contain_text('Отмечено: 1 из 6')

    # Cancellation, focus trapping/restoration and keyboard reason selection.
    absent = card.get_by_role('button', name='Отсутствует', exact=True)
    absent.click()
    expect(dialog).to_be_visible()
    expect(dialog.locator('h4')).to_be_focused()
    for _ in range(8):
        page.keyboard.press('Tab')
        assert dialog.evaluate('(d)=>d.contains(document.activeElement)')
    for _ in range(8):
        page.keyboard.press('Shift+Tab')
        assert dialog.evaluate('(d)=>d.contains(document.activeElement)')
    page.keyboard.press('Escape')
    expect(dialog).to_have_count(0)
    expect(absent).to_be_focused()
    assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND state IS NOT NULL") == '1'
    absent.tap()
    dialog.get_by_role('button', name='Отмена', exact=True).tap()
    expect(dialog).to_have_count(0)
    absent.click()
    dialog.get_by_role('button', name='Неизвестна', exact=True).focus()
    page.keyboard.press('Enter')
    expect(card).to_contain_text(names[2])
    assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND absence_reason='unknown'") == '1'

    # Lost response after the DB commit: stay on this card; identical retry creates no second audit.
    card.get_by_role('button', name='Отсутствует', exact=True).tap()
    def lost_response(route):
        response = route.fetch()
        assert response.ok
        route.abort()
    page.route('**/journal/records/*', lost_response)
    dialog.get_by_role('button', name='Болезнь', exact=True).tap()
    expect(dialog.get_by_role('alert')).to_contain_text('Нет соединения')
    expect(card).to_contain_text(names[2])
    expect(journal).to_contain_text('Отмечено: 2 из 6')
    assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND absence_reason='illness'") == '1'
    audits = sql(f"SELECT count(*) FROM pride.audit_events WHERE entity_type='attendance_record' AND after_state->>'sessionId'='{sid}'")
    page.unroute('**/journal/records/*')
    dialog.get_by_role('button', name='Повторить сохранение: Болезнь', exact=True).evaluate('(b)=>{b.click();b.click()}')
    expect(card).to_contain_text(names[3])
    assert sql(f"SELECT count(*) FROM pride.audit_events WHERE entity_type='attendance_record' AND after_state->>'sessionId'='{sid}'") == audits
    card.get_by_role('button', name='Отсутствует', exact=True).tap()
    dialog.get_by_role('button', name='Уважительная', exact=True).tap()
    expect(card).to_contain_text(names[4])

    # Reload resumes from server state; return to a prior card and correct it.
    page.reload()
    open_session(sid)
    expect(card).to_contain_text(names[4])
    card.get_by_role('button', name='К сводке и предыдущим отметкам', exact=True).click()
    summary = journal.locator('.attendance-summary')
    expect(summary.locator('li')).to_have_count(6)
    expect(summary.locator('li').first).to_contain_text('Присутствует')
    summary.get_by_role('button', name='Открыть карточку 1: '+names[0], exact=True).click()
    expect(card).to_contain_text(names[0])
    for width in (320, 390, 1280):
        page.set_viewport_size({'width': width, 'height': 900})
        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'), width
        for button in journal.locator('button:visible').all():
            assert button.bounding_box()['height'] >= 44
        page.screenshot(path=f'/tmp/attendance158-card-{width}.png', full_page=True)
    page.set_viewport_size({'width': 390, 'height': 844})
    card.get_by_role('button', name='Отсутствует', exact=True).tap()
    # Short landscape viewport keeps every control reachable, including cancel.
    page.set_viewport_size({'width': 320, 'height': 480})
    assert dialog.evaluate('(d)=>d.getBoundingClientRect().right <= innerWidth && d.getBoundingClientRect().left >= 0')
    dialog.get_by_role('button', name='Отмена', exact=True).scroll_into_view_if_needed()
    expect(dialog.get_by_role('button', name='Отмена', exact=True)).to_be_in_viewport()
    page.screenshot(path='/tmp/attendance158-sheet-320.png')
    page.set_viewport_size({'width': 390, 'height': 844})
    dialog.get_by_role('button', name='Болезнь', exact=True).tap()
    expect(card).to_contain_text(names[4])

    # Actual concurrent update forces explicit reload, and disables stale commands.
    current = api(page, 'GET', jbase)
    item = current['records'][4]
    api(page, 'PUT', jbase+'/records/'+item['id'], {'version': item['version'], 'sessionVersion': current['session']['version'], 'state': 'present', 'absenceReason': None})
    card.get_by_role('button', name='Отсутствует', exact=True).click()
    dialog.get_by_role('button', name='Неизвестна', exact=True).click()
    expect(journal.get_by_role('alert')).to_contain_text('Журнал изменился')
    expect(card).to_have_count(0)
    expect(journal.get_by_role('button', name='Завершить журнал', exact=True)).to_have_count(0)
    journal.get_by_role('button', name='Загрузить актуальный журнал и сбросить ввод', exact=True).click()
    expect(card).to_contain_text(names[5])

    # Completion cancellation, failed close, retry and automatic absence for the remainder.
    journal.get_by_role('button', name='Завершить журнал', exact=True).click()
    expect(dialog).to_contain_text('Не отмечено: 1.')
    dialog.get_by_role('button', name='Продолжить отметки', exact=True).click()
    assert sql(f"SELECT status FROM pride.training_sessions WHERE id='{sid}'") == 'open'
    journal.get_by_role('button', name='Завершить журнал', exact=True).click()
    page.route('**/journal/close', lambda route: route.abort())
    dialog.get_by_role('button', name='Подтвердить завершение', exact=True).click()
    expect(dialog.get_by_role('alert')).to_contain_text('Нет соединения')
    assert sql(f"SELECT status FROM pride.training_sessions WHERE id='{sid}'") == 'open'
    page.unroute('**/journal/close')
    dialog.get_by_role('button', name='Подтвердить завершение', exact=True).evaluate('(b)=>{b.click();b.click()}')
    expect(journal).to_contain_text('Журнал завершён')
    expect(card).to_have_count(0)
    expect(summary.locator('li')).to_have_count(6)
    assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND state IS NULL") == '0'
    assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{sid}' AND origin='close_default' AND absence_reason='unknown'") == '1'
    page.reload()
    open_session(sid)
    expect(journal).to_contain_text('При завершении журнала')

    # An idempotent retry returns newer server state: skip the card marked in another window.
    open_session(retry_id)
    journal.get_by_role('button', name='Подтвердить открытие журнала', exact=True).click()
    expect(card).to_contain_text(names[0])
    page.route('**/journal/records/*', lost_response)
    card.get_by_role('button', name='Присутствует', exact=True).click()
    expect(journal.get_by_role('alert')).to_contain_text('Нет соединения')
    page.unroute('**/journal/records/*')
    retry_base = base+'/trainer-sessions/'+retry_id+'/journal'
    current = api(page, 'GET', retry_base)
    second = current['records'][1]
    api(page, 'PUT', retry_base+'/records/'+second['id'], {'version': second['version'], 'sessionVersion': current['session']['version'], 'state': 'present', 'absenceReason': None})
    card.get_by_role('button', name='Повторить сохранение присутствия', exact=True).click()
    expect(card).to_contain_text(names[2])
    expect(journal).to_contain_text('Отмечено: 2 из 6')
    assert sql(f"SELECT count(*) FROM pride.audit_events WHERE entity_type='attendance_record' AND after_state->>'sessionId'='{retry_id}'") == '2'

    # Cancel a real open snapshot: no default absences, reload shows read-only cancellation.
    open_session(cancel_id)
    journal.get_by_role('button', name='Подтвердить открытие журнала', exact=True).click()
    expect(card).to_contain_text(names[0])
    other = api(page, 'GET', base+'/trainer-sessions/'+cancel_id+'/journal')
    api(owner, 'POST', base+'/sessions/'+cancel_id+'/cancel', {'version': other['session']['version'], 'confirmOpen': True, 'reason': 'Synthetic cancellation'})
    card.get_by_role('button', name='Присутствует', exact=True).click()
    expect(journal.get_by_role('alert')).to_contain_text('Журнал недоступен')
    journal.get_by_role('button', name='Загрузить актуальный журнал и сбросить ввод', exact=True).click()
    expect(journal).to_contain_text('Занятие отменено')
    expect(card).to_have_count(0)
    assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{cancel_id}' AND state IS NULL") == '6'

    # Empty journal can complete; replacement immediately removes the old trainer's data.
    open_session(empty_id)
    journal.get_by_role('button', name='Подтвердить открытие журнала', exact=True).click()
    expect(journal).to_contain_text('В снимке состава нет участников')
    journal.get_by_role('button', name='Завершить журнал', exact=True).click()
    expect(dialog).to_contain_text('Не отмечено: 0.')
    dialog.get_by_role('button', name='Подтвердить завершение', exact=True).click()
    expect(journal).to_contain_text('Журнал завершён')
    open_session(replace_id)
    journal.get_by_role('button', name='Подтвердить открытие журнала', exact=True).click()
    expect(card).to_contain_text(names[0])
    other = api(page, 'GET', base+'/trainer-sessions/'+replace_id+'/journal')
    api(owner, 'POST', base+'/sessions/'+replace_id+'/replace-trainer', {'version': other['session']['version'], 'trainerId': replacement['id']})
    card.get_by_role('button', name='Присутствует', exact=True).click()
    expect(journal.get_by_role('alert')).to_contain_text('недоступны')
    expect(journal).to_contain_text('Доступ к журналу отозван')
    expect(journal.locator('.attendance-cards')).to_have_count(0)
    signout(page)
    expect(page.get_by_label('Логин', exact=True)).to_be_visible()
    # Actual session revocation returns 401 and removes the journal/account entirely.
    signin(page, trainer['login'], 'synthetic-trainer-password')
    page.get_by_label('Филиал', exact=True).select_option(club['id'])
    open_session(revoke_id)
    journal.get_by_role('button', name='Подтвердить открытие журнала', exact=True).click()
    expect(card).to_contain_text(names[0])
    sql(f"UPDATE pride.staff_sessions SET revoked_at=now() WHERE staff_user_id='{trainer['id']}' AND revoked_at IS NULL")
    card.get_by_role('button', name='Отсутствует', exact=True).click()
    dialog.get_by_role('button', name='Болезнь', exact=True).click()
    expect(page.get_by_label('Логин', exact=True)).to_be_visible()
    expect(page.locator('.trainer-sessions')).to_have_count(0)
    expect(dialog).to_have_count(0)
    assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{revoke_id}' AND state IS NOT NULL") == '0'
    assert page.locator('.trainer-sessions').count() == 0
    # Revocation of the current role also clears the open journal immediately on request.
    signin(page, replacement['login'], 'synthetic-trainer-password')
    page.get_by_label('Филиал', exact=True).select_option(club['id'])
    open_session(replace_id)
    expect(card).to_contain_text(names[0])
    sql(f"DELETE FROM pride.staff_club_roles WHERE club_id='{club['id']}' AND staff_user_id='{replacement['id']}' AND role='trainer'")
    card.get_by_role('button', name='Присутствует', exact=True).click()
    expect(journal.get_by_role('alert')).to_contain_text('недоступны')
    expect(journal.locator('.attendance-cards')).to_have_count(0)
    signout(page)
    expect(page.get_by_label('Логин', exact=True)).to_be_visible()
    assert not errors, errors
    print(json.dumps({'browser': browser.version, 'widths': [320,390,1280], 'flow': 'cards/present/three-reasons/keyboard/touch/cancel/double-click/delayed-response/lost-response/retry/correction/reload/conflict/close/empty/cancelled/replacement/role-revoke/session-revoke/logout', 'database': 'PostgreSQL 18.6'}, ensure_ascii=False))
    browser.close()

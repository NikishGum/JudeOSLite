# Disposable loopback DB/server only. See docs/sessions-v1.md.
import json
import os
import subprocess
import uuid
from datetime import date, timedelta
from playwright.sync_api import sync_playwright, expect

ROOT = 'http://127.0.0.1:8080'
CONTAINER = os.environ.get('SESSIONS_TEST_DB_CONTAINER', 'sessions156-db')
assert CONTAINER.startswith('sessions156-'), 'Disposable sessions156-* container required'

def sql(statement):
    return subprocess.check_output(['docker', 'exec', CONTAINER, 'psql', '-U', 'test', '-d', 'browser156', '-Atc', statement], text=True).strip()

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    page = browser.new_page(viewport={'width': 390, 'height': 844})
    errors = []
    page.on('pageerror', lambda e: errors.append(str(e)))

    def signin(target, login, password):
        target.goto(ROOT)
        target.get_by_label('Логин', exact=True).fill(login)
        target.get_by_label('Пароль', exact=True).fill(password)
        target.get_by_role('button', name='Войти', exact=True).click()
        expect(target.get_by_role('button', name='Выйти', exact=True)).to_be_visible()

    signin(page, 'owner', 'synthetic-browser-password')

    def api(method, path, data=None):
        return page.evaluate('''async ([method,path,data]) => {
            const me=await (await fetch('/api/auth/me')).json();
            const r=await fetch(path,{method,headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrfToken},body:method==='GET'?undefined:JSON.stringify(data)});
            if(!r.ok)throw Error(await r.text());return r.json();
        }''', [method, path, data])

    suffix = uuid.uuid4().hex[:8]
    a = api('POST', '/api/clubs', {'name': 'Sessions A '+suffix, 'timezone': 'Europe/Minsk'})
    b = api('POST', '/api/clubs', {'name': 'Sessions B '+suffix, 'timezone': 'UTC'})
    group = api('POST', '/api/clubs/'+a['id']+'/groups', {'name': 'Calendar group '+suffix})
    trainer_login, replacement_login = 'sessions-'+suffix, 'replacement-'+suffix
    trainer = api('POST', '/api/clubs/'+a['id']+'/trainers', {'login': trainer_login, 'displayName': 'Original Trainer', 'password': 'synthetic-trainer-password'})
    replacement = api('POST', '/api/clubs/'+a['id']+'/trainers', {'login': replacement_login, 'displayName': 'Replacement Trainer', 'password': 'synthetic-trainer-password'})
    base = '/api/clubs/'+a['id']+'/sessions'
    tomorrow = date.fromisoformat(page.evaluate("new Intl.DateTimeFormat('sv-SE',{timeZone:'Europe/Minsk'}).format(new Date())")) + timedelta(days=1)
    day = tomorrow.isoformat()
    page.reload()
    page.get_by_label('Филиал', exact=True).select_option(a['id'])
    screen = page.locator('section.session-calendar')
    expect(screen.get_by_role('button', name='Добавить разовое занятие', exact=True)).to_be_enabled()
    screen.get_by_label('Группа календаря', exact=True).select_option(group['id'])
    expect(screen.get_by_role('button', name='Добавить разовое занятие', exact=True)).to_be_enabled()

    def create_form(at):
        screen.get_by_role('button', name='Добавить разовое занятие', exact=True).click()
        form = screen.locator('.session-form')
        form.get_by_label('Группа разового занятия', exact=True).select_option(group['id'])
        form.get_by_label('Фактический тренер', exact=True).select_option(trainer['id'])
        form.get_by_label('Дата занятия', exact=True).fill(at)
        form.get_by_label('Время занятия', exact=True).fill('18:00')
        return form

    form = create_form(day)
    page.route(ROOT+base, lambda route: route.abort() if route.request.method == 'POST' else route.continue_())
    form.get_by_role('button', name='Создать разовое занятие', exact=True).click()
    expect(form.get_by_role('alert')).to_contain_text('Нет соединения')
    expect(form.get_by_label('Дата занятия', exact=True)).to_have_value(day)
    page.unroute(ROOT+base)
    calls = []
    def record(req):
        if req.method == 'POST' and req.url == ROOT+base: calls.append(req.url)
    page.on('request', record)
    form.get_by_role('button', name='Создать разовое занятие', exact=True).evaluate('(b) => {b.click();b.click();}')
    expect(screen.get_by_role('status').filter(has_text='Занятие сохранено')).to_be_visible()
    expect(screen.get_by_role('button', name='Добавить разовое занятие', exact=True)).to_be_enabled()
    page.remove_listener('request', record)
    assert len(calls) == 1, calls
    listing = api('GET', base+'?from='+day+'&until='+(tomorrow+timedelta(days=1)).isoformat())
    v = listing[0]
    assert v['ruleId'] is None and v['version'] == 1
    card = screen.locator('[data-session-id="'+v['id']+'"]')
    expect(card).to_contain_text('Original Trainer')

    # An already-open trainer view picks up replacements/cancellations on refresh.
    old_page = browser.new_page(viewport={'width': 390, 'height': 844})
    old_page.on('pageerror', lambda e: errors.append(str(e)))
    signin(old_page, trainer_login, 'synthetic-trainer-password')
    old_page.get_by_label('Филиал', exact=True).select_option(a['id'])
    old_screen = old_page.locator('section.session-calendar')
    expect(old_screen.locator('[data-session-id="'+v['id']+'"]')).to_be_visible()
    expect(old_screen.get_by_role('button', name='Заменить тренера', exact=True)).to_have_count(0)

    card.get_by_role('button', name='Заменить тренера', exact=True).click()
    form = screen.locator('.session-form')
    form.get_by_label('Фактический тренер', exact=True).select_option(replacement['id'])
    form.get_by_role('button', name='Подтвердить изменение занятия', exact=True).click()
    expect(card).to_contain_text('Replacement Trainer')
    old_screen.get_by_role('button', name='Обновить календарь', exact=True).click()
    expect(old_screen.locator('[data-session-id="'+v['id']+'"]')).to_have_count(0)

    new_page = browser.new_page(viewport={'width': 390, 'height': 844})
    new_page.on('pageerror', lambda e: errors.append(str(e)))
    signin(new_page, replacement_login, 'synthetic-trainer-password')
    new_page.get_by_label('Филиал', exact=True).select_option(a['id'])
    new_screen = new_page.locator('section.session-calendar')
    expect(new_screen.locator('[data-session-id="'+v['id']+'"]')).to_contain_text('Replacement Trainer')

    # Preserve a stale form until the user deliberately reloads.
    card.get_by_role('button', name='Отменить занятие', exact=True).click()
    form = screen.locator('.session-form')
    form.get_by_label('Причина отмены', exact=True).fill('Saved reason on conflict')
    api('POST', base+'/'+v['id']+'/replace-trainer', {'version': 2, 'trainerId': trainer['id']})
    form.get_by_role('button', name='Подтвердить изменение занятия', exact=True).click()
    expect(form.get_by_role('alert')).to_contain_text('уже изменено')
    expect(form.get_by_label('Причина отмены', exact=True)).to_have_value('Saved reason on conflict')
    form.get_by_role('button', name='Загрузить актуальные занятия и сбросить форму', exact=True).click()
    expect(card).to_contain_text('Original Trainer')
    card.get_by_role('button', name='Отменить занятие', exact=True).click()
    form = screen.locator('.session-form')
    form.get_by_label('Причина отмены', exact=True).fill('Synthetic planned cancellation')
    form.get_by_role('button', name='Подтвердить изменение занятия', exact=True).click()
    expect(card).to_contain_text('Отменено')
    expect(card).to_contain_text('Synthetic planned cancellation')
    expect(card.get_by_role('button')).to_have_count(0)
    old_screen.get_by_role('button', name='Обновить календарь', exact=True).click()
    expect(old_screen.locator('[data-session-id="'+v['id']+'"]')).to_contain_text('Отменено')

    # V1-09 consumer fixture: opened/closed journals and authors survive V1-08.
    opened = api('POST', base, {'groupId': group['id'], 'trainerId': replacement['id'], 'date': (tomorrow+timedelta(days=1)).isoformat(), 'startsTime': '18:00', 'durationMinutes': 60})
    closed = api('POST', base, {'groupId': group['id'], 'trainerId': trainer['id'], 'date': (tomorrow+timedelta(days=2)).isoformat(), 'startsTime': '18:00', 'durationMinutes': 60})
    for ident in [opened['id'], closed['id'], replacement['id']]: uuid.UUID(ident)
    sql("CREATE TABLE IF NOT EXISTS pride.browser_session_history(session_id uuid PRIMARY KEY REFERENCES pride.training_sessions(id),marked_by uuid REFERENCES pride.staff_users(id),value text)")
    sql("INSERT INTO pride.browser_session_history VALUES('"+opened['id']+"','"+replacement['id']+"','snapshot/present/unmarked')")
    sql("UPDATE pride.training_sessions SET status='open',roster_frozen_at=now(),version=version+1 WHERE id='"+opened['id']+"'")
    sql("UPDATE pride.training_sessions SET status='closed',roster_frozen_at=now(),closed_at=now(),version=version+1 WHERE id='"+closed['id']+"'")
    screen.get_by_role('button', name='Обновить календарь', exact=True).click()
    opened_card = screen.locator('[data-session-id="'+opened['id']+'"]')
    expect(opened_card).to_contain_text('Журнал открыт')
    expect(screen.locator('[data-session-id="'+closed['id']+'"]')).to_contain_text('Завершено')
    expect(screen.locator('[data-session-id="'+closed['id']+'"] button')).to_have_count(0)
    opened_card.get_by_role('button', name='Отменить занятие', exact=True).click()
    form = screen.locator('.session-form')
    expect(form).to_contain_text('снимок состава и уже сохранённые отметки')
    form.get_by_label('Причина отмены', exact=True).fill('Synthetic opened cancellation')
    form.get_by_label('Подтверждаю отмену открытого журнала', exact=True).check()
    form.get_by_role('button', name='Подтвердить изменение занятия', exact=True).click()
    expect(opened_card).to_contain_text('Отменено')
    assert sql("SELECT status||'|'||(closed_at IS NOT NULL)::text||'|'||(roster_frozen_at IS NOT NULL)::text FROM pride.training_sessions WHERE id='"+opened['id']+"'") == 'cancelled|true|true'
    assert sql("SELECT marked_by::text||'|'||value FROM pride.browser_session_history WHERE session_id='"+opened['id']+"'") == replacement['id']+'|snapshot/present/unmarked'
    new_screen.get_by_role('button', name='Обновить календарь', exact=True).click()
    expect(new_screen.locator('[data-session-id="'+opened['id']+'"]')).to_contain_text('Отменено')

    # Manual move displays the actual day and leaves source starts_on untouched.
    movable = api('POST', base, {'groupId': group['id'], 'trainerId': trainer['id'], 'date': (tomorrow+timedelta(days=3)).isoformat(), 'startsTime': '18:00', 'durationMinutes': 60})
    screen.get_by_role('button', name='Обновить календарь', exact=True).click()
    move_card = screen.locator('[data-session-id="'+movable['id']+'"]')
    expect(move_card).to_be_visible()
    move_card.get_by_role('button', name='Перенести занятие', exact=True).click()
    form = screen.locator('.session-form')
    form.get_by_label('Дата занятия', exact=True).fill((tomorrow+timedelta(days=4)).isoformat())
    form.get_by_label('Время занятия', exact=True).fill('23:30')
    form.get_by_label('Длительность занятия, минут', exact=True).fill('90')
    form.get_by_role('button', name='Подтвердить изменение занятия', exact=True).click()
    expect(move_card).to_contain_text('Перенесено с')
    expect(move_card).to_contain_text('окончание')
    assert sql("SELECT starts_on::text FROM pride.training_sessions WHERE id='"+movable['id']+"'") == movable['sourceDate']

    page.reload()
    screen = page.locator('section.session-calendar')
    expect(screen.locator('[data-session-id="'+v['id']+'"]')).to_contain_text('Отменено')
    for width in [320, 390, 1280]:
        page.set_viewport_size({'width': width, 'height': 900})
        screen.scroll_into_view_if_needed()
        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'), width
        assert screen.get_by_role('button').evaluate_all('(bs)=>bs.every(b=>b.getBoundingClientRect().height>=44)'), width
        page.screenshot(path=f'/tmp/sessions156-{width}.png', full_page=True)
    page.get_by_label('Филиал', exact=True).select_option(b['id'])
    expect(page.locator('section.session-calendar')).to_have_count(1)
    expect(page.locator('section.session-calendar')).not_to_contain_text('Synthetic planned cancellation')
    expect(page.locator('section.session-calendar')).not_to_contain_text(group['name'])
    assert not errors, errors
    print(json.dumps({'browser': browser.version, 'scenarios': 'create retry/double submit, replace/actual trainer refresh, stale form, planned/open cancel, snapshot/authors, closed guards, move/source date, reload/club switch, widths 320/390/1280', 'sessions': [v['id'], opened['id'], closed['id'], movable['id']], 'jsErrors': errors}))
    browser.close()

# Disposable loopback DB/server only. See docs/schedule-v1.md.
import json
import uuid
from datetime import date, timedelta
from playwright.sync_api import sync_playwright, expect

ROOT = 'http://127.0.0.1:8080'
with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    page = browser.new_page(viewport={'width': 390, 'height': 844})
    errors = []
    page.on('pageerror', lambda e: errors.append(str(e)))
    page.goto(ROOT)
    page.get_by_label('Логин', exact=True).fill('owner')
    page.get_by_label('Пароль', exact=True).fill('synthetic-browser-password')
    page.get_by_role('button', name='Войти', exact=True).click()
    expect(page.get_by_role('heading', name='Synthetic Owner', exact=True)).to_be_visible()

    def api(method, path, data=None):
        return page.evaluate('''async ([method,path,data]) => {
            const me=await (await fetch('/api/auth/me')).json();
            const r=await fetch(path,{method,headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrfToken},body:method==='GET'?undefined:JSON.stringify(data)});
            if(!r.ok)throw Error(await r.text());return r.json();
        }''', [method, path, data])

    suffix = uuid.uuid4().hex[:8]
    a = api('POST', '/api/clubs', {'name': 'Schedule A '+suffix, 'timezone': 'Europe/Minsk'})
    b = api('POST', '/api/clubs', {'name': 'Schedule B '+suffix, 'timezone': 'UTC'})
    base = '/api/clubs/'+a['id']+'/schedule'
    groups = [api('POST', '/api/clubs/'+a['id']+'/groups', {'name': name}) for name in ['Weekly', 'Parity', 'Dates', 'Once']]
    trainer = api('POST', '/api/clubs/'+a['id']+'/trainers', {'login': 'schedule-'+suffix, 'displayName': 'Schedule Trainer', 'password': 'synthetic-trainer-password'})
    page.reload()
    page.get_by_label('Филиал', exact=True).select_option(a['id'])
    expect(page.locator('.club-summary').get_by_role('heading', name=a['name'], exact=True)).to_be_visible()
    screen = page.locator('section.schedule')
    expect(screen.get_by_role('button', name='Создать расписание', exact=True)).to_be_enabled()
    tomorrow = date.fromisoformat(page.evaluate("new Intl.DateTimeFormat('sv-SE',{timeZone:'Europe/Minsk'}).format(new Date())")) + timedelta(days=1)
    counts = []

    def new_form(group, mode):
        screen.get_by_role('button', name='Создать расписание', exact=True).click()
        form = screen.locator('.schedule-editor')
        expect(form).to_be_visible()
        form.get_by_label('Группа расписания', exact=True).select_option(group['id'])
        form.get_by_label('Тренер занятия', exact=True).select_option(trainer['id'])
        form.get_by_label('Режим расписания', exact=True).select_option(mode)
        return form

    def check_preview_save(form, group, from_date, until_date):
        form.get_by_role('button', name='Показать предпросмотр', exact=True).click()
        expect(form.locator('.schedule-preview')).to_be_visible()
        expected = form.locator('.schedule-preview .schedule-dates').first.locator('li').all_text_contents()
        calls = []
        def record(req):
            if req.method == 'POST' and req.url == ROOT+base:
                calls.append(req.url)
        page.on('request', record)
        form.get_by_role('button', name='Подтвердить расписание', exact=True).evaluate('(button) => {button.click();button.click();}')
        expect(screen.get_by_role('status').filter(has_text='Расписание сохранено')).to_be_visible()
        expect(screen.get_by_role('button', name='Создать расписание', exact=True)).to_be_enabled()
        page.remove_listener('request', record)
        assert len(calls) == 1, calls
        sessions = [s for s in api('GET', base+'/sessions?from='+from_date+'&until='+until_date) if s['groupId'] == group['id']]
        assert len(sessions) == len(expected), (sessions, expected)
        actual = page.evaluate('''sessions => sessions.map(s => {
            const fmt=new Intl.DateTimeFormat('ru',{timeZone:'Europe/Minsk',hour:'2-digit',minute:'2-digit'});
            return s.date.split('-').reverse().join('.')+' · '+fmt.format(new Date(s.startsAt))+'–'+fmt.format(new Date(s.endsAt));
        })''', sessions)
        assert actual == expected, (actual, expected)
        counts.append(len(sessions))
        return sessions

    form = new_form(groups[0], 'weekly')
    form.get_by_label('Начало действия', exact=True).fill(tomorrow.isoformat())
    # A failed preview keeps input, then retry follows the same flow.
    page.route('**/schedule/preview', lambda route: route.abort())
    form.get_by_role('button', name='Показать предпросмотр', exact=True).click()
    expect(form.get_by_role('alert')).to_contain_text('Нет соединения')
    expect(form.get_by_label('Начало действия', exact=True)).to_have_value(tomorrow.isoformat())
    page.unroute('**/schedule/preview')
    horizon = date(tomorrow.year+1, 1, 1) if tomorrow.month == 12 else date(tomorrow.year, tomorrow.month+1, 1)
    horizon = date(horizon.year+1, 1, 1) if horizon.month == 12 else date(horizon.year, horizon.month+1, 1)
    check_preview_save(form, groups[0], tomorrow.isoformat(), horizon.isoformat())

    form = new_form(groups[1], 'day_parity')
    form.get_by_label('Начало действия', exact=True).fill(tomorrow.isoformat())
    form.get_by_label('Чётность числа месяца', exact=True).select_option('odd')
    sessions = check_preview_save(form, groups[1], tomorrow.isoformat(), horizon.isoformat())
    assert all(int(s['date'][-2:]) % 2 == 1 for s in sessions)

    form = new_form(groups[2], 'date_list')
    for picked in ['2032-02-29', '2032-03-31']:
        form.get_by_label('Дата в календаре', exact=True).fill(picked)
        form.get_by_role('button', name='Выбрать дату', exact=True).click()
    check_preview_save(form, groups[2], '2032-02-01', '2032-04-01')

    form = new_form(groups[3], 'once')
    form.get_by_label('Дата в календаре', exact=True).fill(tomorrow.isoformat())
    form.get_by_role('button', name='Выбрать дату', exact=True).click()
    one = check_preview_save(form, groups[3], tomorrow.isoformat(), (tomorrow+timedelta(days=1)).isoformat())
    assert one[0]['ruleId'] is None

    # Dated edit: real affected plans are visible before mutation.
    row = screen.locator('.registry-list > li').filter(has=page.get_by_role('heading', name='Weekly', exact=True))
    row.get_by_role('button', name='Изменить правило с даты', exact=True).click()
    form = screen.locator('.schedule-editor')
    effective = (tomorrow+timedelta(days=7)).isoformat()
    form.get_by_label('Применить изменения с даты', exact=True).fill(effective)
    form.get_by_label('Время начала', exact=True).fill('20:00')
    form.get_by_role('button', name='Показать предпросмотр', exact=True).click()
    expect(form.locator('.schedule-preview').get_by_text('Заменяемые будущие плановые занятия:', exact=False)).to_be_visible()
    rule = next(r for r in api('GET', base) if r['groupId'] == groups[0]['id'])
    change = {k: rule[k] for k in ['groupId', 'trainerId', 'mode', 'weekdays', 'startsTime', 'durationMinutes', 'from', 'until', 'version']}
    change['from'] = effective
    change['startsTime'] = '21:00'
    pr = api('POST', base+'/'+rule['id']+'/preview', change)
    change['previewToken'] = pr['token']
    api('PUT', base+'/'+rule['id'], change)
    form.get_by_role('button', name='Подтвердить расписание', exact=True).click()
    expect(form.get_by_role('alert')).to_contain_text('Правило уже изменено')
    expect(form.get_by_label('Время начала', exact=True)).to_have_value('20:00')
    form.get_by_role('button', name='Загрузить актуальные правила и сбросить форму', exact=True).click()
    expect(screen.get_by_role('button', name='Создать расписание', exact=True)).to_be_enabled()
    screen.get_by_label('Показать историю версий', exact=True).check()
    expect(screen.get_by_text('Историческая версия', exact=True)).to_have_count(1)

    # Open a form/preview for responsive layout evidence.
    form = new_form(groups[3], 'once')
    form.get_by_label('Дата в календаре', exact=True).fill((tomorrow+timedelta(days=2)).isoformat())
    form.get_by_role('button', name='Выбрать дату', exact=True).click()
    form.get_by_role('button', name='Показать предпросмотр', exact=True).click()
    expect(form.locator('.schedule-preview')).to_be_visible()
    for width in [320, 390, 1280]:
        page.set_viewport_size({'width': width, 'height': 900})
        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'), width
        assert screen.locator('button:visible').evaluate_all('(buttons) => buttons.every(b => b.getBoundingClientRect().height >= 44)'), width
        screen.screenshot(path=f'/tmp/schedule155-{width}.png')
    page.get_by_label('Филиал', exact=True).select_option(b['id'])
    expect(page.get_by_role('heading', name='Schedule B '+suffix, exact=True)).to_be_visible()
    expect(page.locator('section.schedule')).to_have_count(1)
    expect(screen.get_by_role('heading', name='Weekly', exact=True)).to_have_count(0)
    expect(screen.locator('.schedule-editor')).to_have_count(0)
    page.get_by_label('Филиал', exact=True).select_option(a['id'])
    expect(screen.get_by_role('heading', name='Weekly', exact=True)).to_be_visible()
    page.reload()
    expect(screen.get_by_role('heading', name='Weekly', exact=True)).to_be_visible()
    assert not errors, errors
    print(json.dumps({'browser': browser.version, 'counts': counts, 'clubId': a['id'], 'checks': 'four forms, preview=DB dates, far leap dates, once without rule, retry, double submit, dated edit/stale form/history, widths/isolation/reload', 'errors': errors}))
    browser.close()

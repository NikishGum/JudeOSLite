"""RD-02: real UI/API/PG, isolated synthetic fixtures in the PWA harness."""
from pathlib import Path
from playwright.sync_api import expect
from staff_ui import select_employee, add_assignment, discard_staff_form, toggle_access


def check_registry(browser, page, signin, api, owner, root, engine, open_section, sql):
    ctx = browser.new_context(viewport={'width': 1280, 'height': 900}, reduced_motion='reduce')
    ui = page(ctx); signin(ui, 'owner')
    evidence = Path('/tmp') / ('rd184-'+engine)
    evidence.mkdir(exist_ok=True)
    def shots(name):
        ui.wait_for_load_state('networkidle')
        for width in (1280, 390, 320):
            ui.set_viewport_size({'width': width, 'height': 900})
            if name == 'staff-list' and ui.get_by_role('button', name='К списку сотрудников', exact=True).is_visible():
                ui.get_by_role('button', name='К списку сотрудников', exact=True).click()
            ui.evaluate('window.scrollTo(0,0)')
            if not ui.evaluate('document.documentElement.scrollWidth <= innerWidth'):
                ui.screenshot(path=f'/tmp/rd08-registry-overflow-{name}-{width}.png')
                overflow=ui.evaluate('''()=>[...document.querySelectorAll('body *')].filter(e=>{const r=e.getBoundingClientRect();return r.width&&r.right>innerWidth+1&&!e.closest('[data-slot="sidebar"]')}).slice(0,12).map(e=>({tag:e.tagName,cls:e.className,text:e.textContent.slice(0,60),rect:e.getBoundingClientRect().toJSON()}))''')
                raise AssertionError((name,width,overflow))
            for control in ui.locator('dialog[open] button,dialog[open] input:not([type=checkbox]),dialog[open] select' if ui.locator('dialog[open]').count() else 'main button:visible,main input:visible,main select:visible').all():
                r = control.bounding_box()
                if r and control.is_visible():
                    assert r['height'] >= 44 and r['width'] >= 44, (name, width, r)
                    if width <= 390:
                        assert float(control.evaluate('el=>parseFloat(getComputedStyle(el).fontSize)')) >= 16
            if engine == 'chromium':
                ui.screenshot(path=str(evidence/f'{name}-{width}.png'), full_page=not bool(ui.locator('dialog[open]').count()))
        ui.set_viewport_size({'width': 1280, 'height': 900})
    # Club creation has all v1 fields; cancel sends no command.
    open_section(ui, 'Филиалы')
    ui.get_by_role('button', name='Добавить филиал', exact=True).click()
    panel = ui.get_by_role('dialog')
    panel.get_by_label('Название филиала', exact=True).fill('Synthetic cancelled club')
    panel.get_by_role('button', name='Отмена', exact=True).click()
    assert not any(c['name'] == 'Synthetic cancelled club' for c in api(ui, 'GET', '/api/clubs'))
    clubs = []
    for name in ('Synthetic RD F1', 'Synthetic RD F2'):
        ui.get_by_role('button', name='Добавить филиал', exact=True).click()
        panel.get_by_label('Название филиала', exact=True).fill(name)
        panel.get_by_label('Адрес зала', exact=True).fill('Синтетический адрес зала')
        panel.get_by_label('Часовой пояс', exact=True).fill('Europe/Minsk')
        if not clubs:
            shots('club-form')
        panel.get_by_role('button', name='Сохранить филиал', exact=True).click()
        expect(panel).to_have_count(0)
        clubs.append(next(c for c in api(ui, 'GET', '/api/clubs') if c['name'] == name))
    a, b = clubs
    shots('clubs-list')
    # Owner administrators and multiple assignments: UI create, stale version,
    # explicit recovery, cancel archive; API enforces actual IDs independently.
    open_section(ui, 'Сотрудники')
    ui.get_by_role('button', name='Добавить сотрудника', exact=True).click()
    panel.get_by_label('Имя сотрудника', exact=True).fill('Synthetic RD Admin')
    panel.get_by_label('Логин сотрудника', exact=True).fill('rdadmin')
    panel.get_by_label('Первоначальный пароль', exact=True).fill('synthetic-browser-password')
    panel.get_by_label('Повторите пароль', exact=True).fill('synthetic-browser-password')
    panel.get_by_label('Роль', exact=True).select_option('administrator')
    panel.get_by_label('Филиал назначения', exact=True).select_option(a['id'])
    shots('admin-form')
    panel.get_by_role('button', name='Создать сотрудника', exact=True).click()
    expect(panel).to_have_count(0)
    employee = next(s for s in api(ui, 'GET', '/api/staff') if s['login'] == 'rdadmin')
    add_assignment(ui, employee['id'], b['id'], 'administrator')
    employee = next(s for s in api(ui, 'GET', '/api/staff') if s['login'] == 'rdadmin')
    assert set(employee['clubIds']) == {a['id'], b['id']}
    select_employee(ui, employee['id'])
    ui.get_by_role('button', name='Изменить данные', exact=True).click()
    panel.get_by_label('Имя сотрудника', exact=True).fill('Synthetic stale admin draft')
    api(ui, 'PUT', '/api/staff/'+employee['id'], {'displayName': employee['displayName'], 'clubIds': employee['clubIds'], 'disabled': False, 'version': employee['version']})
    panel.get_by_role('button', name='Сохранить данные', exact=True).click()
    expect(panel.get_by_role('alert')).to_contain_text('уже изменена')
    expect(panel.get_by_label('Имя сотрудника', exact=True)).to_have_value('Synthetic stale admin draft')
    discard_staff_form(ui)
    # Administrator works only in a concrete club, never mixed network registry.
    admin_ctx = browser.new_context(viewport={'width': 1280, 'height': 900})
    admin = page(admin_ctx); signin(admin, 'rdadmin')
    admin.goto(root+'/participants')
    expect(admin.locator('main')).to_contain_text('Выберите филиал')
    assert admin.locator('.registry').count() == 0
    admin.get_by_label('Филиал', exact=True).select_option(a['id'])
    open_section(admin, 'Группы')
    for name in ('Synthetic младшая', 'Synthetic старшая'):
        admin.get_by_role('button', name='Создать группу', exact=True).click()
        admin.get_by_label('Название группы', exact=True).fill(name)
        admin.get_by_role('button', name='Сохранить группу', exact=True).click()
        expect(admin.get_by_role('dialog')).to_have_count(0)
    groups = api(admin, 'GET', f"/api/clubs/{a['id']}/groups")
    g1 = next(g for g in groups if g['name'] == 'Synthetic младшая'); g2 = next(g for g in groups if g['name'] == 'Synthetic старшая')
    # Trainer creation and password validation/cancel; multi assignment stays server driven.
    open_section(admin, 'Сотрудники')
    admin.get_by_role('button', name='Добавить сотрудника', exact=True).click()
    admin.get_by_label('Имя сотрудника', exact=True).fill('Synthetic RD Trainer')
    admin.get_by_label('Логин сотрудника', exact=True).fill('rdtrainer')
    admin.get_by_label('Первоначальный пароль', exact=True).fill('synthetic-browser-password')
    admin.get_by_label('Повторите пароль', exact=True).fill('synthetic-other-password')
    admin.get_by_label('Филиал назначения', exact=True).select_option(a['id'])
    admin.get_by_role('button', name='Создать сотрудника', exact=True).click()
    expect(admin.get_by_role('alert')).to_contain_text('Пароли не совпадают')
    admin.get_by_label('Повторите пароль', exact=True).fill('synthetic-browser-password')
    admin.get_by_role('button', name='Создать сотрудника', exact=True).click()
    expect(admin.get_by_role('dialog')).to_have_count(0)
    trainer = api(admin, 'GET', f"/api/clubs/{a['id']}/trainers")[0]
    ui.get_by_label('Филиал', exact=True).select_option(b['id'])
    add_assignment(ui, trainer['id'], b['id'])
    expect(ui.locator('.staff-access')).to_contain_text('Назначение добавлено')
    # Show the list, then the selected card at each responsive width.
    back = ui.get_by_role('button', name='К списку сотрудников', exact=True)
    if back.is_visible(): back.click()
    shots('staff-list')
    select_employee(ui, trainer['id'])
    ui.get_by_role('button', name='Изменить данные', exact=True).click()
    shots('trainer-form')
    discard_staff_form(ui)
    # Same-name participants sharing a family contact remain separate.
    open_section(admin, 'Участники')
    for _ in range(2):
        admin.get_by_role('button', name='Добавить участника', exact=True).click()
        admin.get_by_label('ФИО участника', exact=True).fill('Synthetic однофамилец')
        admin.get_by_label('Дата рождения', exact=True).fill('2014-02-28')
        admin.get_by_label('Дата вступления', exact=True).fill('2025-09-01')
        admin.get_by_label('Номер телефона', exact=True).fill('+375291234567')
        admin.get_by_label('Имя контактного лица (необязательно)', exact=True).fill('Synthetic родитель')
        admin.get_by_label('Начальная группа (необязательно)', exact=True).select_option(g1['id'])
        admin.get_by_label('Дата включения в группу', exact=True).fill('2025-09-01')
        if _ == 0:
            admin.get_by_label('Номер телефона', exact=True).fill('invalid-phone')
            admin.get_by_role('button', name='Сохранить участника', exact=True).click()
            expect(admin.get_by_role('dialog').get_by_role('alert')).to_be_visible()
            expect(admin.get_by_label('ФИО участника', exact=True)).to_have_value('Synthetic однофамилец')
            assert api(admin, 'GET', f"/api/clubs/{a['id']}/participants")['items'] == []
            admin.get_by_label('Номер телефона', exact=True).fill('+375291234567')
        admin.get_by_role('button', name='Сохранить участника', exact=True).click()
        expect(admin.get_by_role('status').filter(has_text='Участник добавлен')).to_be_visible()
        # Creation opens the real card and starts a separate history read.
        # Verify the initial membership before closing its still-loading view.
        expect(admin.locator('.membership-history .registry-list > li')).to_have_count(1)
        expect(admin.locator('.membership-history')).to_contain_text('Synthetic младшая')
        expect(admin.locator('.participant-rows > li')).to_have_count(_ + 1)
        admin.get_by_role('button', name='Закрыть карточку', exact=True).click()
    cards = api(admin, 'GET', f"/api/clubs/{a['id']}/participants")['items']
    assert len(cards) == 2 and len({c['participantId'] for c in cards}) == 2
    card = cards[0]
    # v1 permits local identity editing; only a genuinely shared identity is owner-only.
    sql(f"INSERT INTO pride.club_memberships(club_id,participant_id,joined_on,contact_kind,contact_value,contact_name) VALUES('{b['id']}','{card['participantId']}','2025-09-01','phone','+375291234570','Synthetic second-club contact')")
    admin.get_by_label('Поиск по ФИО или контакту', exact=True).fill('375291234567')
    admin.get_by_role('button', name='Найти', exact=True).click()
    expect(admin.locator('.participant-rows > li')).to_have_count(2)
    admin.goto(root+f"/clubs/{a['id']}/participants?detail={card['id']}")
    admin.get_by_role('button', name='Редактировать участника', exact=True).click()
    expect(admin.get_by_label('ФИО участника', exact=True)).to_be_disabled()
    expect(admin.get_by_label('Дата рождения', exact=True)).to_be_disabled()
    admin.get_by_label('Номер телефона', exact=True).fill('+375291234568')
    live = api(ui, 'GET', f"/api/clubs/{a['id']}/participants/{card['id']}")
    api(ui, 'PUT', f"/api/clubs/{a['id']}/participants/{card['id']}", {'joinedOn': live['joinedOn'], 'contactKind': 'phone', 'contactValue': '+375291234569', 'contactName': live['contactName'], 'version': live['version']})
    admin.get_by_role('button', name='Сохранить участника', exact=True).click()
    expect(admin.get_by_role('dialog').get_by_role('alert')).to_contain_text('уже изменена')
    expect(admin.get_by_label('Номер телефона', exact=True)).to_have_value('+375291234568')
    admin.get_by_role('button', name='Загрузить актуальную карточку', exact=True).click()
    expect(admin.get_by_role('dialog', name='Редактирование участника', exact=True)).to_have_count(0)
    expect(admin.locator('dialog.participant-detail')).to_be_visible()
    # Membership: transfer, exclude, add, interval conflict, archive/restore.
    history = admin.locator('.membership-history')
    history.get_by_role('button', name='Перевести с даты', exact=True).click()
    history.get_by_label('Новая группа', exact=True).select_option(g2['id'])
    history.get_by_label('Дата действия', exact=True).fill('2025-10-01')
    history.get_by_role('button', name='Сохранить перевод', exact=True).click()
    expect(history.locator('li')).to_have_count(2)
    history.locator('li').filter(has_text='Synthetic старшая').get_by_role('button', name='Исключить с даты', exact=True).click()
    history.get_by_label('Дата действия', exact=True).fill('2025-11-01')
    history.get_by_role('button', name='Сохранить исключение', exact=True).click()
    expect(history.locator('li').filter(has_text='до 01.11.2025')).to_be_visible()
    history.get_by_label('Группа', exact=True).select_option(g1['id'])
    history.get_by_label('Дата действия', exact=True).fill('2025-12-01')
    history.get_by_role('button', name='Добавить в группу', exact=True).click()
    expect(history.locator('li')).to_have_count(3)
    last = api(ui, 'GET', f"/api/clubs/{a['id']}/participants/{card['id']}/groups")[-1]
    history.locator('li').filter(has_text='С 01.12.2025').get_by_role('button', name='Перевести с даты', exact=True).click()
    history.get_by_label('Новая группа', exact=True).select_option(g2['id'])
    history.get_by_label('Дата действия', exact=True).fill('2026-01-01')
    api(ui, 'POST', f"/api/clubs/{a['id']}/participants/{card['id']}/groups/{last['id']}/exclude", {'date': '2026-02-01', 'version': last['version']})
    history.get_by_role('button', name='Сохранить перевод', exact=True).click()
    expect(history.get_by_role('alert')).to_contain_text('уже изменена')
    expect(history.get_by_label('Дата действия', exact=True)).to_have_value('2026-01-01')
    history.get_by_role('button', name='Обновить историю и сбросить форму', exact=True).click()
    expect(history.locator('li').filter(has_text='до 01.02.2026')).to_be_visible()
    admin.get_by_role('button', name='В архив', exact=True).click()
    admin.get_by_role('button', name='Отмена', exact=True).click()
    assert not api(ui, 'GET', f"/api/clubs/{a['id']}/participants/{card['id']}")['archived']
    admin.get_by_role('button', name='В архив', exact=True).click()
    admin.get_by_role('button', name='Подтвердить архивирование', exact=True).click()
    expect(admin.get_by_role('button', name='Вернуть из архива', exact=True)).to_be_visible()
    expect(history.locator('.registry-list > li')).to_have_count(3)
    admin.get_by_role('button', name='Вернуть из архива', exact=True).click()
    expect(admin.get_by_role('button', name='В архив', exact=True)).to_be_visible()
    expect(history.locator('.registry-list > li')).to_have_count(3)
    # Owner identity can be edited, forms and card/list paired captures.
    ui.goto(root+f"/clubs/{a['id']}/participants?detail={card['id']}")
    expect(ui.locator('.registry-card')).to_contain_text(card['fullName'])
    shots('participant-card')
    ui.get_by_role('button', name='Редактировать участника', exact=True).click()
    expect(panel.get_by_label('ФИО участника', exact=True)).to_be_enabled()
    shots('participant-form')
    panel.locator('[data-initial-focus]').focus(); ui.keyboard.press('Shift+Tab')
    expect(panel.get_by_role('button', name='Закрыть форму', exact=True)).to_be_focused()
    ui.keyboard.press('Tab'); expect(panel.locator('.dialog-top-close')).to_be_focused()
    ui.keyboard.press('Tab'); expect(panel.get_by_label('ФИО участника', exact=True)).to_be_focused()
    panel.get_by_label('Дата рождения', exact=True).fill('2014-03-01')
    panel.get_by_role('button', name='Сохранить участника', exact=True).click()
    expect(ui.get_by_role('dialog', name='Редактирование участника', exact=True)).to_have_count(0)
    expect(ui.locator('dialog.participant-detail')).to_be_visible()
    assert api(ui, 'GET', f"/api/clubs/{a['id']}/participants/{card['id']}")['birthDate'] == '2014-03-01'
    expect(ui.locator('.membership-history .registry-list > li')).to_have_count(3)
    ui.get_by_role('button', name='Закрыть карточку', exact=True).click()
    shots('participants-list')
    # Real group detail / history navigation, date boundaries, archive and ID adapter.
    open_section(ui, 'Группы')
    for gid, date, count in ((g1['id'], '2025-09-30', 2), (g1['id'], '2025-10-01', 1), (g2['id'], '2025-10-01', 1)):
        ui.goto(root+f"/clubs/{a['id']}/groups?detail={gid}")
        ui.get_by_label('Состав на дату', exact=True).fill(date)
        expect(ui.locator('.group-roster')).to_contain_text(': '+str(count)+' участников.')
    ui.reload(); expect(ui.get_by_label('Состав на дату', exact=True)).to_be_visible()
    ui.get_by_role('button', name='К занятиям группы', exact=True).click()
    assert '/sessions?group='+g2['id'] in ui.url, ui.url
    ui.go_back(); expect(ui.get_by_label('Состав на дату', exact=True)).to_be_visible()
    open_section(ui, 'Группы')
    assert '?detail=' not in ui.url
    ui.locator('.group-choice').filter(has_text='Synthetic младшая').click()
    settings = ui.locator('.group-header details')
    if settings.get_attribute('open') is None: settings.locator('summary').click()
    expect(settings).to_have_attribute('open', '')
    ui.locator('.group-header').get_by_role('button', name='В архив', exact=True).click()
    ui.get_by_role('button', name='Отмена архива группы', exact=True).click()
    assert not next(g for g in api(ui, 'GET', f"/api/clubs/{a['id']}/groups?state=all") if g['id'] == g1['id'])['archived']
    ui.locator('.group-choice').filter(has_text='Synthetic младшая').click()
    settings = ui.locator('.group-header details')
    if settings.get_attribute('open') is None: settings.locator('summary').click()
    expect(settings).to_have_attribute('open', '')
    ui.locator('.group-header').get_by_role('button', name='В архив', exact=True).click()
    ui.get_by_role('button', name='Подтвердить архив группы', exact=True).click()
    expect(ui.get_by_role('status').filter(has_text='Группа в архиве')).to_be_visible()
    ui.get_by_label('Состояние групп', exact=True).select_option('all')
    ui.locator('.group-choice').filter(has_text='Synthetic младшая').click()
    ui.get_by_label('Состав на дату', exact=True).fill('2025-09-30')
    expect(ui.locator('.group-roster')).to_contain_text('2 участников.')
    shots('groups-list')
    ui.get_by_role('button', name='Создать группу', exact=True).click(); shots('group-form')
    panel.get_by_role('button', name='Закрыть форму группы', exact=True).click()
    # Scope change and trainer-only/foreign API never expose contacts.
    with ui.expect_response(lambda response: f"/api/clubs/{b['id']}/groups?state=all" in response.url and response.status == 200) as changed_scope:
        ui.get_by_label('Филиал', exact=True).select_option(b['id'])
    expect(ui.locator('.group-roster')).to_have_count(0)
    # The fixture deliberately shares this identity with B above. Its name is
    # legitimate here; the local membership ID and contacts must belong to B.
    assert changed_scope.value.json() == []
    expect(ui.locator('.group-header')).to_have_count(0)
    b_cards = api(ui, 'GET', f"/api/clubs/{b['id']}/participants?state=all")['items']
    assert len(b_cards) == 1 and b_cards[0]['participantId'] == card['participantId']
    assert b_cards[0]['id'] != card['id']
    open_section(ui, 'Участники')
    expect(ui.locator('.participant-rows')).to_contain_text('Synthetic однофамилец')
    assert ui.evaluate("async p=>(await fetch(p)).status", f"/api/clubs/{b['id']}/participants/{card['id']}") == 404
    forbidden = admin.evaluate("async p=>(await fetch(p)).status", f"/api/clubs/{b['id']}/participants/{card['id']}")
    assert forbidden == 404
    trainer_ctx = browser.new_context(); t = page(trainer_ctx); signin(t, 'rdtrainer')
    t.goto(root+f"/clubs/{a['id']}/participants?detail={card['id']}")
    expect(t.locator('main')).to_contain_text('Раздел или филиал недоступен')
    assert t.evaluate("async p=>(await fetch(p)).status", f"/api/clubs/{a['id']}/participants/{card['id']}") == 404
    # Close the new modal card before navigating to another section.
    admin.get_by_role('button', name='Закрыть карточку', exact=True).click()
    # Revocation preserves the second assignment; disabling is a separate global operation.
    open_section(admin, 'Сотрудники')
    select_employee(admin, trainer['id'])
    revoke_name = 'Снять назначение: Тренер · '+a['name']
    admin.get_by_role('button', name=revoke_name, exact=True).click()
    admin.get_by_role('button', name='Отмена', exact=True).click()
    assert len(api(ui, 'GET', f"/api/clubs/{a['id']}/trainers")) == 1
    admin.get_by_role('button', name=revoke_name, exact=True).click()
    admin.get_by_role('button', name='Подтвердить снятие назначения', exact=True).click()
    expect(admin.locator('.staff-access-list li')).to_have_count(0)
    live_trainer = next(v for v in api(ui, 'GET', '/api/trainer-accounts') if v['id'] == trainer['id'])
    assert live_trainer['trainerClubIds'] == [b['id']]
    t.goto(root+f"/clubs/{a['id']}/journals")
    expect(t.locator('main')).to_contain_text('Раздел или филиал недоступен')
    trainer_ctx.close()
    # Owner UI global disable/re-enable and admin revocation preserve remaining assignment.
    open_section(ui, 'Сотрудники')
    toggle_access(ui, employee['id'], False)
    admin.evaluate("window.dispatchEvent(new Event('focus'))")
    expect(admin.get_by_label('Логин', exact=True)).to_be_visible()
    toggle_access(ui, employee['id'], True)
    ui.get_by_role('button', name='Снять назначение: Администратор · '+a['name'], exact=True).click()
    panel.get_by_role('button', name='Подтвердить снятие назначения', exact=True).click()
    expect(panel).to_have_count(0)
    live_employee = next(s for s in api(ui, 'GET', '/api/staff') if s['id'] == employee['id'])
    assert not live_employee['disabled'] and live_employee['clubIds'] == [b['id']]
    signin(admin, 'rdadmin')
    admin.goto(root+f"/clubs/{a['id']}/participants")
    expect(admin.locator('main')).to_contain_text('Раздел или филиал недоступен')
    admin.goto(root+f"/clubs/{b['id']}/participants")
    expect(admin.locator('#registry-title')).to_be_visible()
    # Cancel and confirm club archive through owner UI; permitted history remains in DB.
    open_section(ui, 'Филиалы')
    ui.get_by_role('button', name='Архивировать филиал '+a['name'], exact=True).click()
    ui.get_by_role('button', name='Отмена', exact=True).click()
    assert not next(c for c in api(ui, 'GET', '/api/clubs') if c['id'] == a['id'])['archived']
    ui.get_by_role('button', name='Архивировать филиал '+a['name'], exact=True).click()
    ui.get_by_role('button', name='Подтвердить архивирование', exact=True).click()
    expect(ui.get_by_role('button', name='Архивировать филиал '+a['name'], exact=True)).to_have_count(0)
    assert next(c for c in api(ui, 'GET', '/api/clubs') if c['id'] == a['id'])['archived']
    assert sql(f"SELECT count(*) FROM pride.group_memberships WHERE club_membership_id='{card['id']}'") == '3'
    admin_ctx.close(); ctx.close()
    print('RD-02 PASS: full fields/identity owner; separate same-name records/family contact; UI cancel/conflicts/archive/restore; dated transfer/exclude/add/history/roster; multi assignments/revoke/foreign API; group ID URL; keyboard/panels/list/card/forms1280/390/overflow320', flush=True)

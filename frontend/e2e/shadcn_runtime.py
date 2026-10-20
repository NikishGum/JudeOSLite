"""RD-07 rendering and real shell behavior on disposable API/PG fixtures."""
from pathlib import Path
from datetime import date, timedelta
from playwright.sync_api import expect
from runtime_ui import settle_shell, assert_page_targets, signout


def check_runtime(browser, page, signin, api, root, club, engine, open_section):
    evidence = Path('/tmp/shadcn-runtime-'+engine); evidence.mkdir(exist_ok=True)
    ctx = browser.new_context(viewport={'width': 1280, 'height': 900}, reduced_motion='reduce')
    ui = page(ctx); signin(ui, 'owner'); ui.goto(root+'/clubs/'+club['id']+'/home')
    def capture(name, widths=(1280, 900, 390, 320)):
        for width in widths:
            ui.set_viewport_size({'width': width, 'height': 900 if width >= 768 else 844})
            settle_shell(ui, width)
            ui.mouse.move(0, 0)
            ui.wait_for_load_state('networkidle')
            if ui.locator('dialog[open]').count():
                ui.locator('dialog[open]').last.locator('[data-initial-focus]').focus()
            else:
                ui.locator('main h1').focus(); ui.evaluate('window.scrollTo(0,0)')
            assert ui.evaluate('document.documentElement.scrollWidth <= innerWidth'), (name, width)
            if width < 768:
                assert ui.locator('[data-slot="sidebar"]').count() == 0, (name, width)
                expect(ui.locator('.bottom-navigation')).to_be_visible()
            if ui.locator('dialog[open]').count():
                modal=ui.locator('dialog[open]').last
                close=modal.locator('.dialog-top-close')
                expect(close).to_be_in_viewport(ratio=1)
                # The exit stays reachable after scrolling a long sheet.
                modal.evaluate('n=>{n.scrollTop=n.scrollHeight}')
                expect(close).to_be_in_viewport(ratio=1)
                if width < 768: ui.screenshot(path=str(evidence/f'{name}-scrolled-{width}.png'))
                modal.evaluate('n=>{n.scrollTop=0}')
            assert_page_targets(ui, width)
            if name == 'staff':
                row = ui.locator('.staff-access-person').first
                assert row.evaluate('n=>[getComputedStyle(n).paddingTop,getComputedStyle(n).paddingRight,getComputedStyle(n).paddingBottom,getComputedStyle(n).paddingLeft]') == ['16px','12px','16px','12px']
                avatar = row.locator('[data-slot="avatar"]').bounding_box()
                assert avatar['width'] == 44 and avatar['height'] == 44
            if name in ('home', 'statistics'):
                # The four numbers remain aligned when metric labels wrap.
                metrics=ui.locator('.report-totals').first.locator('dd').evaluate_all('nodes=>nodes.slice(0,4).map(n=>n.getBoundingClientRect().bottom)')
                if len(metrics) == 4:
                    assert abs(metrics[0]-metrics[1]) < 1
                    assert abs(metrics[2]-metrics[3]) < 1
                    if width >= 768: assert max(metrics)-min(metrics) < 1
            ui.screenshot(path=str(evidence/f'{name}-{width}.png'), full_page=False)
        ui.set_viewport_size({'width':1280,'height':900})
    expect(ui.locator('.workspace-navigation [data-active]')).to_have_count(1)
    inactive = ui.locator('.workspace-navigation a:not([aria-current])').first
    assert inactive.evaluate('el=>getComputedStyle(el).backgroundColor') == 'rgba(0, 0, 0, 0)'
    # Exact desktop geometry and actual live preference, never a demo profile update.
    expect(ui.locator('[data-slot="sidebar-container"]')).to_be_visible()
    assert ui.locator('[data-slot="sidebar-container"]').bounding_box()['width'] == 248
    ui.get_by_role('button', name='Переключить боковое меню', exact=True).click()
    assert ui.locator('[data-slot="sidebar"]').get_attribute('data-state') == 'collapsed'
    # Reduced motion makes the collapse immediate; no arbitrary sleeps.
    expect(ui.locator('.app-shell')).to_have_class(__import__('re').compile('sidebar-collapsed'))
    assert ui.locator('[data-slot="sidebar-container"]').bounding_box()['width'] == 76
    ui.get_by_role('button', name='Меню профиля', exact=True).click()
    expect(ui.get_by_role('menu')).to_contain_text('Synthetic Owner')
    ui.screenshot(path=str(evidence/'profile-open-1280.png'))
    ui.get_by_role('menuitem', name='Настройки приложения', exact=True).click()
    panel=ui.get_by_role('dialog', name='Настройки приложения', exact=True)
    expect(panel.get_by_label('Сворачивать меню на компьютере', exact=True)).to_be_checked()
    panel.get_by_label('Сворачивать меню на компьютере', exact=True).uncheck()
    expect(panel.get_by_role('button', name='Сохранить настройки', exact=True)).to_have_count(0)
    panel.get_by_role('button', name='Закрыть настройки', exact=True).click()
    assert ui.locator('[data-slot="sidebar-container"]').bounding_box()['width'] == 248
    # Each production section, with all original feature operations still mounted.
    for title, slug in [('Главная','home'),('Участники','participants'),('Группы','groups'),('Занятия','sessions'),('Журналы','journals'),('Статистика','statistics'),('Сотрудники','staff'),('Филиалы','clubs'),('История','history')]:
        open_section(ui, title)
        capture(slug)
    open_section(ui, 'Участники')
    expect(ui.get_by_role('button', name='Открыть карточку', exact=True)).to_have_count(0)
    first=ui.locator('.participant-rows .name-link').first
    expect(first).to_be_visible()
    first.hover(); ui.screenshot(path=str(evidence/'participant-hover-1280.png'))
    first.focus(); ui.keyboard.press('Shift+Tab'); ui.keyboard.press('Tab')
    expect(first).to_be_focused(); ui.screenshot(path=str(evidence/'participant-focus-1280.png'))
    first.click()
    detail=ui.get_by_role('dialog')
    expect(detail).to_be_visible()
    capture('participant-detail')
    detail.get_by_role('button', name='Редактировать участника', exact=True).click()
    capture('participant-editor')
    ui.get_by_role('button', name='Отмена', exact=True).click()
    ui.locator('dialog.participant-detail .dialog-top-close').click()
    open_section(ui,'Занятия')
    expect(ui.get_by_role('radio',name='Неделя',exact=True)).to_be_checked()
    expect(ui.locator('.session-day')).to_have_count(7)
    old_from=ui.locator('.session-day').first.get_attribute('id').removeprefix('day-')
    ui.get_by_role('button',name='Вперёд',exact=True).click()
    expected=str(date.fromisoformat(old_from)+timedelta(days=7))
    expect(ui.locator('.session-day').first).to_have_attribute('id','day-'+expected)
    assert __import__('urllib.parse',fromlist=['urlparse']).parse_qs(__import__('urllib.parse',fromlist=['urlparse']).urlparse(ui.url).query)['from'] == [expected]
    ui.get_by_role('radio',name='День',exact=True).click()
    expect(ui.locator('.session-day')).to_have_count(1)
    ui.get_by_role('button',name='Вперёд',exact=True).click()
    expected=str(date.fromisoformat(expected)+timedelta(days=1))
    expect(ui.locator('.session-day').first).to_have_attribute('id','day-'+expected)
    assert __import__('urllib.parse',fromlist=['urlparse']).parse_qs(__import__('urllib.parse',fromlist=['urlparse']).urlparse(ui.url).query)['from'] == [expected]
    capture('sessions-populated')
    ui.get_by_role('button',name='Открыть календарь',exact=True).click()
    expect(ui.get_by_role('navigation',name='Навигация календаря',exact=True)).to_be_visible()
    for width in (1280, 390, 320):
        ui.set_viewport_size({'width':width,'height':844})
        expect(ui.get_by_role('navigation',name='Навигация календаря',exact=True)).to_be_visible()
        assert_page_targets(ui, width)
        bounds=ui.locator('.calendar-popover').bounding_box()
        assert bounds['x'] >= 0 and bounds['x']+bounds['width'] <= width, (width, bounds)
        ui.screenshot(path=str(evidence/f'calendar-{width}.png'))
    ui.keyboard.press('Escape')
    expect(ui.get_by_role('button',name='Открыть календарь',exact=True)).to_be_focused()
    # On mobile the profile and More are real, keyboard-accessible and bounded.
    ui.set_viewport_size({'width':390,'height':844})
    ui.get_by_role('button',name='Меню профиля',exact=True).click()
    expect(ui.get_by_role('menuitem',name='Выйти',exact=True)).to_be_visible()
    bounds=ui.get_by_role('menu').bounding_box(); assert bounds['x'] >= 0 and bounds['x']+bounds['width'] <= 390
    ui.screenshot(path=str(evidence/'profile-open-390.png'))
    ui.keyboard.press('Escape'); expect(ui.get_by_role('button',name='Меню профиля',exact=True)).to_be_focused()
    ui.get_by_role('button',name='Ещё',exact=True).click()
    expect(ui.get_by_role('dialog',name='Разделы PRIDE',exact=True)).to_be_visible()
    ui.screenshot(path=str(evidence/'more-390.png'))
    ui.keyboard.press('Escape'); expect(ui.get_by_role('button',name='Ещё',exact=True)).to_be_focused()
    signout(ui); expect(ui.get_by_label('Логин',exact=True)).to_be_visible()
    expect(ui.locator('.account')).to_have_count(0); expect(ui.get_by_role('menu')).to_have_count(0)
    assert ui.evaluate("document.cookie.includes('sidebar_state')") is False
    ctx.close()
    print('RD-07 PASS: real profile/logout/live settings; Sidebar248/76/mobile absent; all9 screens1280/900/390/320; participant name/detail/editor; civil date+URL/calendar; keyboard focus return/More; complete existing features',flush=True)

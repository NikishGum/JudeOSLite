# Read-only UI regression against a disposable loopback app and synthetic audit fixture.
# Fixture: owner/admin/trainer/archived-admin; synthetic-browser-password; Audit A/B/Archive;
# 55 A created, 3 A updated, 2 B b_only, 1 archived, 1 network_only at 2026-10-12T12:00Z.
import json, os
from pathlib import Path
from urllib.parse import urlparse
from playwright.sync_api import sync_playwright, expect
ROOT=os.environ.get('AUDIT_TEST_ORIGIN','http://127.0.0.1:8082')
assert urlparse(ROOT).hostname in ('127.0.0.1','localhost'), 'Disposable loopback app only'
with sync_playwright() as p:
 browser=p.chromium.launch(headless=True)
 page=browser.new_page(viewport={'width':390,'height':844})
 errors=[]; page.on('pageerror',lambda error:errors.append(str(error)))
 page.goto(ROOT)
 def login(name):
  page.get_by_label('Логин',exact=True).fill(name)
  page.get_by_label('Пароль',exact=True).fill('synthetic-browser-password')
  page.get_by_role('button',name='Войти',exact=True).click()
  expect(page.get_by_role('button',name='Выйти',exact=True)).to_be_visible()
 def logout():
  page.get_by_role('button',name='Выйти',exact=True).click()
  expect(page.get_by_label('Логин',exact=True)).to_be_visible()
  expect(page.locator('.audit-event')).to_have_count(0)
 login('owner')
 audit=page.locator('section.audit-history')
 expect(audit.locator('.audit-event')).to_have_count(50)
 first=audit.locator('.audit-event').all_text_contents()
 audit.get_by_role('button',name='Следующая страница',exact=True).click()
 expect(audit.locator('.audit-event')).to_have_count(12)
 assert not set(first).intersection(audit.locator('.audit-event').all_text_contents())
 audit.get_by_role('button',name='Предыдущая страница',exact=True).click()
 expect(audit.locator('.audit-event')).to_have_count(50)
 audit.get_by_label('Филиал истории').select_option('network')
 audit.get_by_role('button',name='Показать историю',exact=True).click()
 expect(audit.locator('.audit-event')).to_have_count(1)
 expect(audit.locator('.audit-event')).to_contain_text('Сеть · без филиала')
 audit.get_by_label('Филиал истории').select_option(label='Audit A')
 audit.get_by_label('Действие',exact=True).select_option('updated')
 audit.get_by_label('С даты',exact=True).fill('2026-10-12')
 audit.get_by_label('По дату',exact=True).fill('2026-10-12')
 audit.get_by_role('button',name='Показать историю',exact=True).click()
 expect(audit.locator('.audit-event')).to_have_count(3)
 expect(audit.locator('.audit-event').first).to_contain_text('15:00:00')
 assert 'Audit B' not in '\n'.join(audit.locator('.audit-event').all_text_contents())
 audit.get_by_label('С даты',exact=True).fill('2026-10-13')
 audit.get_by_role('button',name='Показать историю',exact=True).click()
 expect(audit.get_by_role('alert')).to_contain_text('Дата начала')
 audit.get_by_label('С даты',exact=True).fill('2026-10-10')
 audit.get_by_label('По дату',exact=True).fill('2026-10-11')
 audit.get_by_role('button',name='Показать историю',exact=True).click()
 expect(audit.get_by_role('status')).to_contain_text('изменений нет')
 # Error clears events, retains entered filters and supports retry.
 audit.get_by_label('С даты',exact=True).fill('2026-10-12')
 audit.get_by_label('По дату',exact=True).fill('2026-10-12')
 page.route('**/api/audit/events?**',lambda route:route.abort())
 audit.get_by_role('button',name='Показать историю',exact=True).click()
 expect(audit.get_by_role('alert')).to_be_visible()
 expect(audit.locator('.audit-event')).to_have_count(0)
 expect(audit.get_by_label('Действие',exact=True)).to_have_value('updated')
 page.unroute('**/api/audit/events?**')
 audit.get_by_role('button',name='Повторить загрузку истории',exact=True).click()
 expect(audit.locator('.audit-event')).to_have_count(3)
 for width in (320,390,1280):
  page.set_viewport_size({'width':width,'height':900})
  assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'),f'overflow {width}'
  assert audit.locator('button:visible').evaluate_all('(buttons) => buttons.every(b => b.getBoundingClientRect().height >= 44)'),f'target {width}'
  page.screenshot(path=f'/tmp/audit170-{width}.png',full_page=True)
 audit.get_by_role('button',name='Сбросить фильтры',exact=True).click()
 expect(audit.locator('.audit-event')).to_have_count(50)
 # Changing the app club remounts history, removing the previous scope immediately.
 page.get_by_label('Филиал',exact=True).select_option(label='Audit B')
 expect(audit.locator('.audit-event')).to_have_count(2)
 page.get_by_label('Филиал',exact=True).select_option(label='Audit A')
 expect(audit.locator('.audit-event')).to_have_count(50)
 assert audit.count()==1
 logout();login('admin')
 expect(audit.locator('.audit-event')).to_have_count(50)
 options=audit.get_by_label('Филиал истории').locator('option').all_text_contents()
 assert 'Audit B' not in options and 'Audit Archive · в архиве' not in options and 'Сеть · без филиала' not in options,options
 assert not any('b_only' in item or 'network_only' in item for item in audit.get_by_label('Действие',exact=True).locator('option').all_text_contents())
 logout();login('trainer')
 expect(audit).to_have_count(0)
 logout();login('archived-admin')
 expect(audit.locator('.audit-event')).to_have_count(1)
 expect(audit.locator('.audit-event')).to_contain_text('Audit Archive · в архиве')
 logout()
 assert not errors,errors
 print(json.dumps({'browser':browser.version,'checks':'owner/admin/trainer/archived-admin, scope filters, inclusive dates, tied pages, empty/error/retry, account/club reset, widths 320/390/1280','errors':errors},ensure_ascii=False))
 browser.close()

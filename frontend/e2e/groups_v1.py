# Run only against a disposable loopback server with the synthetic owner fixture.
# Requires Python Playwright + Chromium; see docs/groups-v1.md.
import json, re, uuid
from pathlib import Path
from playwright.sync_api import sync_playwright, expect
ROOT='http://127.0.0.1:8080'
suffix=uuid.uuid4().hex[:8]
club_a='Browser A '+suffix
club_b='Browser B '+suffix
with sync_playwright() as p:
 b=p.chromium.launch(headless=True)
 page=b.new_page(viewport={'width':390,'height':844})
 errors=[];page.on('pageerror',lambda e:errors.append(str(e)))
 page.goto(ROOT)
 page.get_by_label('Логин',exact=True).fill('owner');page.get_by_label('Пароль',exact=True).fill('synthetic-browser-password');page.get_by_role('button',name='Войти',exact=True).click()
 expect(page.get_by_role('heading',name='Synthetic Owner',exact=True)).to_be_visible()
 # Set up clubs through real scoped API; all group/card/transfer operations below use forms.
 def post(path,data):
  return page.evaluate('''async ([path,data]) => { const me=await (await fetch('/api/auth/me')).json(); const r=await fetch(path,{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrfToken},body:JSON.stringify(data)}); if(!r.ok)throw Error(await r.text());return await r.json(); }''',[path,data])
 a=post('/api/clubs',{'name':club_a,'timezone':'Europe/Minsk'}); bb=post('/api/clubs',{'name':club_b,'timezone':'UTC'})
 page.reload();page.get_by_label('Филиал',exact=True).select_option(a['id'])
 gm=page.locator('section.groups');registry=page.locator('section.registry').filter(has=page.get_by_role('heading',name='Участники филиала',exact=True))
 for name in ['Младшая','Старшая']:
  gm.get_by_role('button',name='Создать группу',exact=True).click();gm.get_by_label('Название группы',exact=True).fill(name);gm.get_by_role('button',name='Сохранить группу',exact=True).click();expect(gm.get_by_role('heading',name=name,exact=True)).to_be_visible()
 registry.get_by_role('button',name='Добавить участника',exact=True).click()
 registry.get_by_label('ФИО участника',exact=True).fill('Браузерный Участник');registry.get_by_label('Дата рождения',exact=True).fill('2014-02-28');registry.get_by_label('Дата вступления',exact=True).fill('2025-09-01');registry.get_by_label('Номер телефона',exact=True).fill('+375291234567');registry.get_by_label('Начальная группа (необязательно)').select_option(label='Младшая');registry.get_by_label('Дата включения в группу',exact=True).fill('2025-09-01');registry.get_by_role('button',name='Сохранить участника',exact=True).click()
 expect(registry.get_by_role('status').filter(has_text='Участник добавлен')).to_be_visible()
 hist=registry.locator('.membership-history')
 expect(hist.locator('li').filter(has_text='Младшая')).to_be_visible()
 hist.get_by_role('button',name='Перевести с даты',exact=True).click();hist.get_by_label('Новая группа').select_option(label='Старшая');hist.get_by_label('Дата действия',exact=True).fill('2025-10-01');hist.get_by_role('button',name='Сохранить перевод',exact=True).click();expect(hist.locator('li')).to_have_count(2)
 for group,date,count in [('Младшая','2025-09-30',1),('Младшая','2025-10-01',0),('Старшая','2025-10-01',1)]:
  gm.get_by_role('button',name='Показать состав '+group,exact=True).click();gm.get_by_label('Состав на дату',exact=True).fill(date);expect(gm.get_by_text('На '+'.'.join(reversed(date.split('-')))+': '+str(count)+' участников.',exact=True)).to_be_visible()
 # Return, then exclude. Each operation must leave old periods visible.
 row=hist.locator('li').filter(has_text='Старшая');row.get_by_role('button',name='Перевести с даты',exact=True).click();hist.get_by_label('Новая группа').select_option(label='Младшая');hist.get_by_label('Дата действия',exact=True).fill('2025-11-01');hist.get_by_role('button',name='Сохранить перевод',exact=True).click();expect(hist.locator('li')).to_have_count(3)
 hist.locator('li').filter(has_text='С 01.11.2025').get_by_role('button',name='Исключить с даты',exact=True).click();hist.get_by_label('Дата действия',exact=True).fill('2025-12-01');hist.get_by_role('button',name='Сохранить исключение',exact=True).click();expect(hist.locator('li').filter(has_text='до 01.12.2025')).to_be_visible()
 # Network failure retains form, then retry and rapid double submit creates one interval.
 hist.get_by_label('Группа',exact=True).select_option(label='Старшая');hist.get_by_label('Дата действия',exact=True).fill('2026-01-01')
 page.route('**/participants/*/groups',lambda route:route.abort() if route.request.method=='POST' else route.continue_())
 hist.get_by_role('button',name='Добавить в группу',exact=True).click();expect(hist.get_by_role('alert')).to_be_visible();expect(hist.get_by_label('Дата действия',exact=True)).to_have_value('2026-01-01');page.unroute('**/participants/*/groups')
 calls=[];page.on('request',lambda r:calls.append(r.url) if r.method=='POST' and re.search(r'/participants/[^/]+/groups$',r.url) else None)
 hist.get_by_role('button',name='Добавить в группу',exact=True).evaluate('(button) => { button.click();button.click(); }');expect(hist.locator('li')).to_have_count(4);assert len(calls)==1,calls
 # Stale interval: another request excludes the period before submitting old form.
 items=page.evaluate('''async id => (await fetch(`/api/clubs/${id}/participants`)).json()''',a['id']);card=items['items'][0]
 intervals=page.evaluate('''async ([club,id]) => (await fetch(`/api/clubs/${club}/participants/${id}/groups`)).json()''',[a['id'],card['id']]);last=intervals[-1]
 hist.locator('li').filter(has_text='С 01.01.2026').get_by_role('button',name='Перевести с даты',exact=True).click();hist.get_by_label('Новая группа').select_option(label='Младшая');hist.get_by_label('Дата действия',exact=True).fill('2026-02-01')
 post(f"/api/clubs/{a['id']}/participants/{card['id']}/groups/{last['id']}/exclude",{'date':'2026-03-01','version':last['version']})
 hist.get_by_role('button',name='Сохранить перевод',exact=True).click();expect(hist.get_by_role('alert')).to_contain_text('Запись уже изменена');expect(hist.get_by_label('Дата действия',exact=True)).to_have_value('2026-02-01');hist.get_by_role('button',name='Обновить историю и сбросить форму',exact=True).click();expect(hist.locator('li').filter(has_text='до 01.03.2026')).to_be_visible()
 # Archive retains dated roster.
 gm.get_by_role('button',name='В архив Младшая',exact=True).click();gm.get_by_role('button',name='Подтвердить архив группы',exact=True).click();expect(gm.get_by_role('status').filter(has_text='Группа в архиве')).to_be_visible();gm.get_by_label('Состояние групп').select_option('all');gm.get_by_role('button',name='Показать состав Младшая',exact=True).click();gm.get_by_label('Состав на дату',exact=True).fill('2025-09-30');expect(gm.get_by_text('На 30.09.2025: 1 участников.',exact=True)).to_be_visible()
 for width in [320,390,1280]:
  page.set_viewport_size({'width':width,'height':900});assert page.evaluate('document.documentElement.scrollWidth<=innerWidth'),f'overflow {width}'
  assert page.locator('button:visible').evaluate_all('(buttons) => buttons.every(b => b.getBoundingClientRect().height >= 44)'),f'target {width}'
  page.screenshot(path=f'/tmp/groups154-{width}.png',full_page=True)
 page.reload();expect(page.get_by_role('heading',name='Группы филиала',exact=True)).to_be_visible();registry.get_by_role('button',name=re.compile('^Открыть карточку')).click();expect(registry.locator('.membership-history li')).to_have_count(4)
 assert page.locator('section.groups').count()==1
 page.get_by_label('Филиал',exact=True).select_option(bb['id']);expect(page.get_by_role('heading',name=club_b,exact=True)).to_be_visible();expect(page.get_by_text('Браузерный Участник',exact=True)).to_have_count(0)
 expect(page.locator('section.groups')).to_have_count(1)
 expect(page.get_by_role('heading',name='Участники филиала',exact=True)).to_have_count(1)
 page.get_by_label('Филиал',exact=True).select_option(a['id']);expect(page.locator('.club-summary').get_by_role('heading',name=club_a,exact=True)).to_be_visible();expect(registry.get_by_text('Браузерный Участник',exact=True)).to_be_visible()
 page.get_by_label('Филиал',exact=True).select_option(bb['id']);expect(page.get_by_text('Браузерный Участник',exact=True)).to_have_count(0)
 assert not errors,errors
 print(json.dumps({'browser':b.version,'checks':'create initial group, transfer/return/exclude, date boundaries, network retry, double submit, stale conflict, archive/history, reload/isolation, widths 320/390/1280','card':card,'club':a['id'],'errors':errors},ensure_ascii=False))
 b.close()

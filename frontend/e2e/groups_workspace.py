"""#214 targeted groups UI -> existing Go API -> disposable PostgreSQL.
Run on loopback:8080, migrated groups214 DB, synthetic owner; see groups-workspace.md.
No PWA or other-module acceptance. Fixtures are synthetic and uniquely named.
"""
import json
import os
import subprocess
import uuid
from datetime import datetime, timedelta
from pathlib import Path
from zoneinfo import ZoneInfo
from playwright.sync_api import sync_playwright, expect
from runtime_ui import observe_api, wait_for_api_idle, assert_page_targets
from redesign_accessibility import contrast_audit

ROOT = 'http://127.0.0.1:8080'
OUT = Path(os.environ.get('GROUPS214_EVIDENCE', '/tmp/groups214-evidence'))
OUT.mkdir(parents=True, exist_ok=True)
PASSWORD = 'synthetic-browser-password'
suffix = uuid.uuid4().hex[:7]
today = datetime.now(ZoneInfo('Europe/Minsk')).date()
def date(days=0): return (today + timedelta(days=days)).isoformat()
def sql(statement):
    return subprocess.check_output(['docker','exec','groups214-db','psql','-U','test','-d','groups214','-Atc',statement],text=True).strip()

def run():
 with sync_playwright() as p:
  browser=p.chromium.launch()
  errors=[]
  def login(name='owner', width=1440):
   context=browser.new_context(viewport={'width':width,'height':1000},reduced_motion='reduce',timezone_id='America/Los_Angeles',service_workers='block')
   page=context.new_page();observe_api(page);page.on('pageerror',lambda e:errors.append(str(e)))
   page.goto(ROOT);expect(page.get_by_label('Логин',exact=True)).to_be_visible();wait_for_api_idle(page)
   page.get_by_label('Логин',exact=True).fill(name);page.get_by_label('Пароль',exact=True).fill(PASSWORD);page.get_by_role('button',name='Войти',exact=True).click();expect(page.locator('.app-shell')).to_be_visible();wait_for_api_idle(page)
   return page
  owner=login()
  def api(page,method,path,body=None,status=200):
   result=page.evaluate('''async ([method,path,body])=>{const me=await(await fetch('/api/auth/me')).json();const r=await fetch(path,{method,headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrfToken},body:body===null?undefined:JSON.stringify(body)});return {status:r.status,body:await r.json()}}''',[method,path,body])
   assert result['status']==status,(method,path,result)
   return result['body']
  club=api(owner,'POST','/api/clubs',{'name':'Учебный филиал · '+suffix,'timezone':'Europe/Minsk'},201)
  other=api(owner,'POST','/api/clubs',{'name':'Второй филиал · '+suffix,'timezone':'Europe/Minsk'},201)
  base='/api/clubs/'+club['id']
  g=api(owner,'POST',base+'/groups',{'name':'Дзюдо · младшая группа'},201)
  target=api(owner,'POST',base+'/groups',{'name':'Дзюдо · старшая группа с длинным названием'},201)
  foreign=api(owner,'POST','/api/clubs/'+other['id']+'/groups',{'name':g['name']},201)
  trainer=api(owner,'POST',base+'/trainers',{'login':'tr214-'+suffix,'displayName':'Орлов Сергей · учебный тренер','password':PASSWORD},201)
  trainer2=api(owner,'POST',base+'/trainers',{'login':'rep214-'+suffix,'displayName':'Коваль Анна · учебный тренер','password':PASSWORD},201)
  admin_login='ad214-'+suffix
  api(owner,'POST','/api/staff',{'login':admin_login,'displayName':'Учебный администратор','password':PASSWORD,'clubIds':[club['id']]},201)
  cards=[]
  for name in ['Иванов Максим','Орлова София','Бондарь Артём']:
   cards.append(api(owner,'POST',base+'/participants',{'identity':{'fullName':name,'birthDate':'2014-02-28'},'joinedOn':'2020-01-01','contactKind':'phone','contactValue':'+375291234567'},201))
  api(owner,'POST',base+'/participants/'+cards[-1]['id']+'/groups',{'groupId':target['id'],'from':'2020-01-01'},201)
  admin=login(admin_login)
  def go(group=g):
   wait_for_api_idle(admin);admin.goto(ROOT+'/clubs/'+club['id']+'/groups?detail='+group['id']);expect(admin.locator('.group-header')).to_contain_text(group['name']);wait_for_api_idle(admin)
  def leave():
   admin.evaluate('() => new Promise(r => requestAnimationFrame(r))')
   dialog=admin.get_by_role('dialog').filter(has=admin.get_by_role('heading',name='Есть несохранённые изменения',exact=True))
   if dialog.is_visible():dialog.get_by_role('button',name='Перейти без сохранения',exact=True).click()
  go()
  expect(admin.locator('.group-header')).to_contain_text('Регулярное расписание не задано')
  expect(admin.get_by_role('button',name='Добавить участников',exact=True)).to_be_visible()
  admin.get_by_role('button',name='Добавить участников',exact=True).click()
  expect(admin.locator('.group-add h4').first).to_be_focused()
  for c in cards:admin.locator('.group-pick-list label').filter(has_text=c['fullName']).get_by_role('checkbox').check()
  admin.get_by_label('Дата зачисления',exact=True).fill(date(-2))
  admin.get_by_role('tab',name='Расписание',exact=True).click()
  expect(admin.get_by_role('heading',name='Есть несохранённые изменения',exact=True)).to_be_visible()
  admin.get_by_role('button',name='Продолжить редактирование',exact=True).click()
  requests=[];admin.on('request',lambda r:requests.append(r.url) if r.method=='POST' and '/participants/' in r.url and r.url.endswith('/groups') else None)
  admin.get_by_role('button',name='Добавить выбранных (3)',exact=True).evaluate('b=>{b.click();b.click()}')
  expect(admin.locator('.group-roster')).to_contain_text('Результат добавления')
  expect(admin.locator('.group-people > li')).to_have_count(2)
  expect(admin.locator('.group-add')).to_contain_text('Выбрано: 1')
  assert len(requests)==3,requests
  expect(admin.locator('.group-add')).to_contain_text('Добавлен')
  expect(admin.locator('.group-add')).to_contain_text('На эти даты участник уже состоит в группе')
  assert sql(f"SELECT count(*) FROM pride.group_memberships WHERE group_id='{g['id']}'")=='2'
  admin.get_by_role('button',name='Закрыть добавление',exact=True).click();leave();expect(admin.locator('.group-add')).to_have_count(0);expect(admin.get_by_role('button',name='Добавить участников',exact=True)).to_be_focused()
  # Transfer dates are independent of the historical roster date.
  row=admin.locator('.group-people > li').filter(has_text=cards[0]['fullName']);row.locator('summary').click();row.get_by_role('button',name='Перевести',exact=True).click()
  dialog=admin.get_by_role('dialog');dialog.get_by_label('Новая группа',exact=True).select_option(target['id']);dialog.get_by_label('Дата действия',exact=True).fill(date(1));expect(dialog).to_contain_text('Последний день в прежней группе');dialog.get_by_role('button',name='Сохранить перевод',exact=True).click()
  expect(admin.get_by_role('dialog')).to_have_count(0)
  admin.get_by_label('Состав на дату',exact=True).fill(date(1));expect(admin.locator('.group-people > li')).to_have_count(1)
  admin.get_by_label('Состав на дату',exact=True).fill(date());expect(admin.locator('.group-people > li')).to_have_count(2)
  row=admin.locator('.group-people > li').filter(has_text=cards[1]['fullName']);row.locator('summary').click();row.get_by_role('button',name='Завершить участие',exact=True).click()
  admin.get_by_role('dialog').get_by_label('Дата действия',exact=True).fill(date(1));admin.get_by_role('button',name='Завершить участие с даты',exact=True).click();expect(admin.get_by_role('dialog')).to_have_count(0)
  # Context-locked weekly rule, server preview, double submit.
  admin.get_by_role('tab',name='Расписание',exact=True).click();leave()
  admin.get_by_role('button',name='Настроить расписание',exact=True).click();editor=admin.locator('.group-rule-editor')
  assert editor.get_by_label('Группа расписания',exact=True).count()==0
  editor.get_by_label('Тренер занятия',exact=True).select_option(trainer['id']);editor.get_by_label('Начало действия',exact=True).fill(date(-3));editor.get_by_label('Время начала',exact=True).fill('18:00')
  editor.get_by_role('button',name='Показать предпросмотр',exact=True).click();expect(editor.locator('.schedule-preview')).to_be_visible();editor.get_by_role('button',name='Подтвердить расписание',exact=True).evaluate('b=>{b.click();b.click()}');expect(editor).to_have_count(0)
  rules=api(owner,'GET',base+'/schedule');rule=rules[0];assert len(rules)==1
  expect(admin.locator('.group-header')).to_contain_text('Пн, Ср, Пт · 18:00–19:00')
  sessions=api(owner,'GET',base+'/sessions?from='+date()+'&until='+date(28)+'&groupId='+g['id']);first=next(s for s in sessions if s['date']>date());original=first['date']
  # Move one actual session without editing its rule; new period follows a far date.
  row=admin.locator(f'.group-sessions > li[data-session-id="{first["id"]}"]');row.locator('summary').click();row.get_by_role('button',name='Перенести занятие',exact=True).click()
  form=admin.locator('.group-session-editor');form.get_by_label('Дата занятия',exact=True).fill(date(32));form.get_by_label('Время занятия',exact=True).fill('16:30');form.get_by_role('button',name='Подтвердить изменение занятия',exact=True).click();expect(form).to_have_count(0)
  expect(admin.locator('.group-sessions')).to_contain_text('Перенесено с '+'.'.join(reversed(original.split('-'))))
  expect(admin.get_by_label('Период с даты',exact=True)).to_have_value(date(32))
  # Cancellation reason, network failure preserves input, then save on server.
  row=admin.locator(f'.group-sessions > li[data-session-id="{first["id"]}"]');row.locator('summary').click();row.get_by_role('button',name='Отменить занятие',exact=True).click();form=admin.locator('.group-session-editor');form.get_by_label('Причина отмены',exact=True).fill('Тренер недоступен')
  pattern='**/sessions/'+first['id']+'/cancel';admin.route(pattern,lambda r:r.abort());form.get_by_role('button',name='Подтвердить изменение занятия',exact=True).click();expect(form.get_by_role('alert')).to_be_visible();expect(form.get_by_label('Причина отмены',exact=True)).to_have_value('Тренер недоступен');admin.unroute(pattern)
  form.get_by_role('button',name='Подтвердить изменение занятия',exact=True).click();expect(form).to_have_count(0);expect(admin.locator('.group-sessions')).to_contain_text('Отменено');expect(admin.locator('.group-sessions')).to_contain_text('Тренер недоступен')
  # Rule change preserves the moved/cancelled exception and splits effective/future summaries.
  admin.get_by_role('button',name='Изменить расписание',exact=True).click();editor=admin.locator('.group-rule-editor');editor.get_by_label('Применить изменения с даты',exact=True).fill(date(2));editor.get_by_label('Время начала',exact=True).fill('19:30');editor.get_by_label('Тренер занятия',exact=True).select_option(trainer2['id']);editor.get_by_role('button',name='Показать предпросмотр',exact=True).click();expect(editor.locator('.schedule-preview')).to_contain_text('Сохраняемые исключения и журналы');editor.get_by_role('button',name='Подтвердить расписание',exact=True).click();expect(editor).to_have_count(0)
  expect(admin.locator('.group-header')).to_contain_text('18:00–19:00');expect(admin.locator('.group-header')).to_contain_text('19:30–20:30');expect(admin.locator('.group-header')).to_contain_text('Коваль Анна')
  assert sql(f"SELECT status||'|'||starts_on||'|'||version FROM pride.training_sessions WHERE id='{first['id']}'").startswith('cancelled|'+original+'|')
  # Initial one-off setup stays within the group and creates no rule.
  admin.get_by_role('button',name='Добавить разовое занятие',exact=True).click();form=admin.locator('.group-session-editor');expect(form.get_by_label('Группа разового занятия',exact=True)).to_have_value(g['id']);form.get_by_label('Фактический тренер',exact=True).select_option(trainer['id']);form.get_by_label('Дата занятия',exact=True).fill(date(1));form.get_by_label('Время занятия',exact=True).fill('12:00');form.get_by_role('button',name='Создать разовое занятие',exact=True).click();expect(form).to_have_count(0)
  assert len(api(owner,'GET',base+'/schedule'))==2
  once=next(s for s in api(owner,'GET',base+'/sessions?from='+date(1)+'&until='+date(2)+'&groupId='+g['id']) if s['ruleId'] is None)
  print('PASS roster, dated transfer/exclude, weekly preview/change, far move, planned cancel/network, once',flush=True)
  # Real open snapshot/marks, explicit cancellation, no automatic absences.
  sql(f"UPDATE pride.training_sessions SET starts_at=now()-interval '1 minute',ends_at=now()+interval '59 minute' WHERE id='{once['id']}'")
  tr=login(trainer['login']);journal=api(tr,'POST',base+'/trainer-sessions/'+once['id']+'/journal/open',{'version':once['version']})
  assert len(journal['records'])==2
  marked=journal['records'][0]
  journal=api(tr,'PUT',base+'/trainer-sessions/'+once['id']+'/journal/records/'+marked['id'],{'version':marked['version'],'sessionVersion':journal['session']['version'],'state':'present','absenceReason':None})
  go();admin.get_by_role('tab',name='Расписание',exact=True).click();row=admin.locator(f'.group-sessions > li[data-session-id="{once["id"]}"]');row.locator('summary').click();assert row.get_by_role('button',name='Перенести занятие',exact=True).count()==0;row.get_by_role('button',name='Отменить занятие',exact=True).click();form=admin.locator('.group-session-editor');expect(form).to_contain_text('Журнал открыт');form.get_by_label('Причина отмены',exact=True).fill('Зал недоступен');form.get_by_label('Подтверждаю отмену открытого журнала',exact=True).check();form.get_by_role('button',name='Подтвердить изменение занятия',exact=True).click();expect(form).to_have_count(0)
  assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{once['id']}' AND state IS NOT NULL")=='1'
  assert sql(f"SELECT count(*) FROM pride.attendance_records WHERE training_session_id='{once['id']}'")==str(len(journal['records']))
  # Closed journals retain their existing restrictions; no controls that promise a forbidden change.
  closed=api(owner,'POST',base+'/sessions',{'groupId':g['id'],'trainerId':trainer['id'],'date':date(1),'startsTime':'13:00','durationMinutes':60},201)
  sql(f"UPDATE pride.training_sessions SET starts_at=now()-interval '2 hours',ends_at=now()-interval '1 hour' WHERE id='{closed['id']}'")
  closed_j=api(tr,'POST',base+'/trainer-sessions/'+closed['id']+'/journal/open',{'version':closed['version']})
  closed_j=api(tr,'POST',base+'/trainer-sessions/'+closed['id']+'/journal/close',{'version':closed_j['session']['version']})
  before=sql(f"SELECT row_to_json(s) FROM pride.training_sessions s WHERE id='{closed['id']}'")
  api(admin,'POST',base+'/sessions/'+closed['id']+'/cancel',{'version':closed_j['session']['version'],'reason':'Forbidden'},409)
  assert sql(f"SELECT row_to_json(s) FROM pride.training_sessions s WHERE id='{closed['id']}'")==before
  go();admin.get_by_role('tab',name='Расписание',exact=True).click();expect(admin.locator(f'.group-sessions > li[data-session-id="{closed["id"]}"]')).to_contain_text('Завершено')
  assert admin.locator(f'.group-sessions > li[data-session-id="{closed["id"]}"] summary').count()==0
  # Other regular modes are configured in the same fixed group workspace.
  go(target);admin.get_by_role('tab',name='Расписание',exact=True).click()
  for mode,clock in [('day_parity','08:00'),('date_list','10:00')]:
   admin.get_by_role('button',name='Настроить расписание' if mode=='day_parity' else 'Добавить интервал',exact=True).click();editor=admin.locator('.group-rule-editor')
   editor.get_by_label('Тренер занятия',exact=True).select_option(trainer['id']);editor.get_by_label('Режим расписания',exact=True).select_option(mode);editor.get_by_label('Время начала',exact=True).fill(clock)
   if mode=='day_parity':editor.get_by_label('Чётность числа месяца',exact=True).select_option('odd')
   else:
    for day in [date(1),date(75)]:
     editor.get_by_label('Дата в календаре',exact=True).fill(day);editor.get_by_role('button',name='Выбрать дату',exact=True).click()
   editor.get_by_role('button',name='Показать предпросмотр',exact=True).click();expect(editor.locator('.schedule-preview')).to_be_visible();editor.get_by_role('button',name='Подтвердить расписание',exact=True).click();expect(editor).to_have_count(0)
  assert sorted(r['mode'] for r in api(owner,'GET',base+'/schedule'))==['date_list','day_parity','weekly','weekly']
  planned=next(s for s in api(owner,'GET',base+'/sessions?from='+date()+'&until='+date(28)+'&groupId='+target['id']) if s['status']=='planned')
  row=admin.locator(f'.group-sessions > li[data-session-id="{planned["id"]}"]');row.locator('summary').click();row.get_by_role('button',name='Заменить тренера',exact=True).click();form=admin.locator('.group-session-editor');form.get_by_label('Фактический тренер',exact=True).select_option(trainer2['id']);form.get_by_role('button',name='Подтвердить изменение занятия',exact=True).click();expect(form).to_have_count(0);expect(admin.locator(f'.group-sessions > li[data-session-id="{planned["id"]}"]')).to_contain_text(trainer2['displayName'])
  print('PASS open cancellation preserves marks/snapshot, closed restriction, parity/date-list, trainer replacement',flush=True)
  go();admin.get_by_role('tab',name='Расписание',exact=True).click()
  # Future date-list picker stays valid initially and after advancing effective date.
  go(target);admin.get_by_role('tab',name='Расписание',exact=True).click()
  dates_row=admin.locator('.group-rules > li').filter(has_text='10:00–11:00');dates_row.get_by_role('button',name='Изменить расписание',exact=True).click();editor=admin.locator('.group-rule-editor');assert editor.locator('form').evaluate('f=>f.checkValidity()')
  editor.get_by_label('Применить изменения с даты',exact=True).fill(date(3));assert editor.locator('form').evaluate('f=>f.checkValidity()');expect(editor.get_by_label('Дата в календаре',exact=True)).to_have_value(date(3));editor.get_by_role('button',name='Показать предпросмотр',exact=True).click();expect(editor.locator('.schedule-preview')).to_be_visible();editor.get_by_role('button',name='Отменить форму расписания',exact=True).click();leave();expect(editor).to_have_count(0)
  # A lost successful batch response is reconciled from the dated history.
  probe=api(owner,'POST',base+'/participants',{'identity':{'fullName':'Проверка потерянного ответа','birthDate':'2014-01-01'},'joinedOn':'2020-01-01','contactKind':'phone','contactValue':'+375291234568'},201)
  go(target);admin.get_by_role('button',name='Добавить участников',exact=True).click();admin.locator('.group-pick-list label').filter(has_text=probe['fullName']).get_by_role('checkbox').check()
  pattern='**/participants/'+probe['id']+'/groups'
  def lost_response(route):
   if route.request.method=='POST':
    response=route.fetch();assert response.status==201;route.abort()
   else:route.continue_()
  admin.route(pattern,lost_response);admin.get_by_role('button',name='Добавить выбранных (1)',exact=True).click();expect(admin.locator('.group-add')).to_contain_text('Добавление подтверждено по истории');admin.unroute(pattern)
  assert sql(f"SELECT count(*) FROM pride.group_memberships WHERE club_membership_id='{probe['id']}'")=='1'
  admin.get_by_role('button',name='Закрыть добавление',exact=True).click();leave()
  # Stale session edits keep the reason and require an explicit server reload.
  admin.get_by_role('tab',name='Расписание',exact=True).click();leave()
  planned=next(s for s in api(owner,'GET',base+'/sessions?from='+date()+'&until='+date(28)+'&groupId='+target['id']) if s['status']=='planned')
  row=admin.locator(f'.group-sessions > li[data-session-id="{planned["id"]}"]');row.locator('summary').click();row.get_by_role('button',name='Отменить занятие',exact=True).click();form=admin.locator('.group-session-editor');form.get_by_label('Причина отмены',exact=True).fill('Сохранённая причина')
  api(owner,'POST',base+'/sessions/'+planned['id']+'/replace-trainer',{'version':planned['version'],'trainerId':trainer2['id'] if planned['trainerId']==trainer['id'] else trainer['id']})
  form.get_by_role('button',name='Подтвердить изменение занятия',exact=True).click();expect(form.get_by_role('alert')).to_be_visible();expect(form.get_by_label('Причина отмены',exact=True)).to_have_value('Сохранённая причина');expect(form.get_by_role('button',name='Подтвердить изменение занятия',exact=True)).to_be_disabled();form.get_by_role('button',name='Загрузить актуальные занятия и сбросить форму',exact=True).click();leave();expect(form).to_have_count(0)
  go();admin.get_by_role('tab',name='Расписание',exact=True).click()
  print('PASS lost response reconciliation and stale version input',flush=True)
  # Addressed branch/role checks use current real API, not UI filtering.
  api(admin,'GET','/api/clubs/'+other['id']+'/groups',status=404)
  api(tr,'GET',base+'/groups',status=404)
  api(admin,'POST','/api/clubs/'+other['id']+'/groups',{'name':'Forbidden'},status=404)
  # Error, empty, retry and keyboard tab behavior.
  admin.get_by_label('Период с даты',exact=True).fill(date(180));expect(admin.locator('.group-exceptions')).to_contain_text('В этом периоде занятий нет')
  admin.get_by_label('Период с даты',exact=True).fill(date());expect(admin.locator('.group-sessions > li').first).to_be_visible()
  admin.get_by_role('tab',name='Расписание',exact=True).focus();admin.keyboard.press('ArrowLeft');expect(admin.get_by_role('tab',name='Состав',exact=True)).to_be_focused()
  admin.route('**/schedule',lambda r:r.abort() if r.request.method=='GET' else r.continue_())
  go();expect(admin.locator('.group-header')).to_contain_text('Не удалось загрузить расписание');admin.unroute('**/schedule');admin.locator('.group-header').get_by_role('button',name='Повторить загрузку расписания',exact=True).click();expect(admin.locator('.group-header')).to_contain_text('19:30');admin.get_by_role('tab',name='Состав',exact=True).focus()
  # One batched visual pass: roster/rules/setup on desktop and mobile.
  for tab in ['Состав','Расписание']:
   admin.get_by_role('tab',name=tab,exact=True).click();wait_for_api_idle(admin)
   for width in [1440,1280,390,320]:
    admin.set_viewport_size({'width':width,'height':1000 if width>767 else 844});assert_page_targets(admin,width)
    assert admin.evaluate('document.documentElement.scrollWidth<=innerWidth'),(tab,width)
    audit=contrast_audit(admin);assert not audit['failures'],(tab,width,audit['failures'])
    admin.evaluate('window.scrollTo(0,0)')
    admin.screenshot(path=str(OUT/f'{tab}-{width}.png'),full_page=True,animations='disabled')
  admin.set_viewport_size({'width':390,'height':844});admin.get_by_role('button',name='К списку групп',exact=True).click();expect(admin.locator('.group-index')).to_be_visible();admin.locator('.group-choice').filter(has_text=target['name']).click();expect(admin.locator('.group-header')).to_contain_text(target['name']);admin.get_by_role('button',name='К списку групп',exact=True).click();admin.locator('.group-choice').filter(has_text=g['name']).click();expect(admin.get_by_role('tab',name='Расписание',exact=True)).to_have_attribute('aria-selected','true')
  admin.goto(ROOT+'/clubs/'+club['id']+'/groups?detail='+str(uuid.uuid4()));expect(admin.locator('.group-workarea')).to_contain_text('Группа недоступна');admin.get_by_role('button',name='К списку групп',exact=True).click();expect(admin.locator('.group-index')).to_be_visible()
  admin.set_viewport_size({'width':1440,'height':1000});admin.locator('.group-choice').first.hover();assert not contrast_audit(admin)['failures']
  print('PASS focus handoff, future date-list validity, unavailable mobile recovery, contrast and touch sizes',flush=True)
  assert not errors,errors
  print(json.dumps({'browser':browser.version,'club':club['id'],'group':g['id'],'checks':'roster multi partial/double submit, transfer/exclude/date boundaries, contextual weekly preview/change, moved exception/cancel/history, once/open snapshot, role/branch, dirty/keyboard, 1440/1280/390/320','screenshots':str(OUT),'errors':errors},ensure_ascii=False))
  browser.close()
if __name__=='__main__':run()

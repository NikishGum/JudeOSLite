"""RD-08 frontend contract tests. ALL responses here are synthetic; this does not verify backend #201–204."""
import base64
import copy
import json
import os
from datetime import datetime, timedelta
from pathlib import Path
from urllib.parse import urlparse, parse_qs
from zoneinfo import ZoneInfo
from playwright.sync_api import sync_playwright, expect
from redesign_accessibility import contrast_audit
from runtime_ui import observe_api, wait_for_api_idle

ROOT = os.environ.get('RD08_ROOT', 'http://127.0.0.1:4173')
ENGINE = os.environ.get('PWA_BROWSER', 'chromium')
OUT = Path(os.environ.get('RD08_EVIDENCE', '/tmp/rd08-contract-'+ENGINE)); OUT.mkdir(parents=True, exist_ok=True)
TODAY = datetime.now(ZoneInfo('Europe/Minsk')).date().isoformat()
def uid(n): return f'00000000-0000-4000-8000-{n:012d}'
CLUB, USER, GROUP, FAMILY, MEMBER, SESSION = [uid(n) for n in (1,2,3,4,5,6)]
def listing(items): return {'items':items,'total':len(items),'hasMore':False}
CHILD = dict(id=MEMBER, clubId=CLUB, fullName='Иванов Максим', birthDate='2014-03-12', groupName='Дзюдо · 8–12 лет', trainerName='Петров Сергей', archived=False)
FAMILY_DATA = dict(id=FAMILY,clubId=CLUB,name='Семья Ивановых',contactName='Иванова Анна',phone='+375 29 123-45-67',email='family@example.test',version=1,children=[CHILD,dict(CHILD,id=uid(7),fullName='Иванова София',birthDate='2017-06-04',groupName='Дзюдо · 4–7 лет')],accounts=[dict(id=uid(8),login='ivanova',displayName='Иванова Анна')])
PLAN = dict(id=uid(9),clubId=CLUB,name='12 занятий',amountMinor=15000,visits=12,days=30,archived=False,version=1)
PAYMENT = dict(id=uid(10),clubId=CLUB,membershipId=MEMBER,fullName=CHILD['fullName'],description='Абонемент · октябрь',amountMinor=15000,currency='BYN',status='paid',method='card',paidOn=TODAY,createdAt=TODAY+'T09:00:00+03:00',version=1,voidReason=None)
SUB = dict(id=uid(11),membershipId=MEMBER,paymentId=PAYMENT['id'],planName=PLAN['name'],visits=12,used=7,validFrom=TODAY,validUntil=(datetime.fromisoformat(TODAY)+timedelta(days=30)).date().isoformat(),status='active')
DOC = dict(id=uid(12),membershipId=MEMBER,name='Справка для занятий спортом',kind='medical',filename='medical.pdf',mediaType='application/pdf',size=284000,expiresOn='2027-03-12',createdAt=PAYMENT['createdAt'],version=1)
SESSION_DATA = dict(id=SESSION,clubId=CLUB,groupId=GROUP,groupName=CHILD['groupName'],trainerId=USER,trainerName='Петров Сергей',ruleId=None,date=TODAY,sourceDate=TODAY,timezone='Europe/Minsk',startsAt=TODAY+'T16:00:00+03:00',endsAt=TODAY+'T17:00:00+03:00',status='open',manualOverride=False,rosterFrozen=True,closedAt=None,cancelledAt=None,cancellationReason=None,supersededAt=None,version=1)
TOTAL = dict(closed=24,planned=12,open=2,present=186,absent=32,unknown=5,illness=19,excused=8,percentage=85.3)

class Fixture:
    def __init__(self, role='owner'):
        self.role=role; self.available=True; self.fail_next=False; self.mutations=[]; self.parent_two=False; self.revoked=set(); self.reads=[]
        self.families=[copy.deepcopy(FAMILY_DATA),dict(copy.deepcopy(FAMILY_DATA),id=uid(14),name='Семья Орловых',contactName='Орлова Елена',children=[dict(CHILD,id=uid(15),fullName='Орлов Артём')],accounts=[])]
        self.payments=[copy.deepcopy(PAYMENT),dict(PAYMENT,id=uid(16),fullName='Иванова София',status='pending',method=None,paidOn=None,description='Счёт за абонемент')]
        self.plans=[copy.deepcopy(PLAN),dict(PLAN,id=uid(17),name='8 занятий',visits=8,amountMinor=11000)]
        self.docs=[copy.deepcopy(DOC)]; self.notes=[dict(id=uid(18),kind='payment',title='Оплата подтверждена',body='Абонемент Максима оплачен. Хороших тренировок!',href=f'/clubs/{CLUB}/payments',read=False,createdAt=PAYMENT['createdAt']),dict(id=uid(19),kind='training_session',title='Занятие перенесено',body='Проверьте время ближайшей тренировки.',href=f'/clubs/{CLUB}/sessions',read=False,createdAt=PAYMENT['createdAt'])]
        self.records=[dict(id=uid(20+i),membershipId=uid(5+i),sourceGroupMembershipId=uid(40+i),fullName=name,archived=False,state=None,absenceReason=None,markedBy=None,markedAt=None,origin=None,version=1) for i,name in enumerate(['Иванов Максим','Иванова София'])]
    def route(self, route):
        request=route.request; url=urlparse(request.url); path=url.path; query=parse_qs(url.query); method=request.method
        if method=='GET': self.reads.append(path)
        def send(body,status=200): route.fulfill(status=status,json=body,headers={'Cache-Control':'no-store'})
        if method not in ('GET','HEAD'):
            body=request.post_data_json or {}; self.mutations.append((path,body))
            assert request.headers.get('x-csrf-token')=='A'*43, request.headers
            if self.fail_next: self.fail_next=False; send({'error':{'code':'version_conflict','message':'Запись уже изменена. Обновите данные.'}},409); return
        else: body={}
        new_module=any(x in path for x in ('/families','/member-profiles','/billing','/payments','/subscriptions','/documents','/notifications','/portal'))
        if new_module and not self.available: send({'error':{'code':'not_found','message':'Адрес не найден.'}},404); return
        if path=='/api/auth/me':
            roles={'owner':['owner'],'trainer':['trainer'],'parent':['parent']}[self.role]
            send(dict(user=dict(id=USER,displayName='Анна Петрова' if self.role=='owner' else 'Петров Сергей' if self.role=='trainer' else 'Иванова Анна',isOwner=self.role=='owner',isParent=self.role=='parent'),clubs=[dict(id=CLUB,name='PRIDE · Центральный',timezone='Europe/Minsk',archived=False,roles=roles)],csrfToken='A'*43)); return
        if path.startswith('/api/auth/clubs/'): send(dict(id=CLUB,name='PRIDE · Центральный',timezone='Europe/Minsk',archived=False,roles=['owner'] if self.role=='owner' else [self.role])); return
        if path=='/api/dashboard':
            item=dict(SESSION_DATA,canMark=self.role=='trainer',roster=12,marked=7,branchDate=TODAY)
            send(dict(mode=self.role,readAt=PAYMENT['createdAt'],clubs=[dict(id=CLUB,name='PRIDE · Центральный',today=TODAY,timezone='Europe/Minsk',canManage=self.role=='owner')],counters=dict(participants=142 if self.role=='owner' else None,groups=8,today=4,open=2),next=item,today=[item,dict(item,id=uid(27),groupName='Взрослая группа',startsAt=TODAY+'T18:00:00+03:00')],unfinished=[item],groups=[dict(id=GROUP,clubId=CLUB,name=CHILD['groupName'],members=12)])); return
        if path=='/api/reports/filters': send(dict(clubs=[dict(id=CLUB,name='PRIDE · Центральный',archived=False)],groups=[])); return
        if path=='/api/reports/attendance': send(dict(**{'from':TODAY[:7]+'-01'},until=TODAY,total=TOTAL,clubs=[dict(TOTAL,clubId=CLUB,clubName='PRIDE · Центральный',clubArchived=False,groupId=None,groupName=None,groupArchived=False)],groups=[])); return
        if path=='/api/audit/events': send(listing([])); return
        if '/billing/summary' in path: send(dict(paidMinor=1248000,pendingMinor=45000,paidCount=84,pendingCount=3,currency='BYN',byMonth=[dict(month='2026-'+str(i).zfill(2),paidMinor=n) for i,n in [(5,250000),(6,300000),(7,280000),(8,420000),(9,450000),(10,540000)]])); return
        if '/billing/plans' in path:
            if method=='POST': item=dict(body,id=uid(100+len(self.plans)),clubId=CLUB,archived=False,version=1); self.plans.append(item); send(item,201)
            else: send(listing(self.plans))
            return
        if path.endswith('/families'):
            if method=='POST': item=dict(body,id=uid(110),clubId=CLUB,version=1,children=[],accounts=[]); self.families.append(item);send(item,201)
            else: send(listing([f for f in (self.families[:1] if self.role=='parent' and not self.parent_two else self.families) if f['id'] not in self.revoked]))
            return
        if '/families/' in path:
            if '/accounts' in path: self.families[0]['version']+=1; send(self.families[0]); return
            send(self.families[0]); return
        if '/member-profiles/' in path:
            if MEMBER in self.revoked: send({'error':{'code':'not_found','message':'Участник недоступен.'}},404); return
            send(dict(CHILD,joinedOn='2024-09-01',contactName='Иванова Анна',contactValue='+375 29 123-45-67',familyId=FAMILY,familyName=FAMILY_DATA['name'],groups=[dict(id=GROUP,name=CHILD['groupName'],trainerName=CHILD['trainerName'],**{'from':'2026-09-01','until':None})],attendance=dict(present=18,absent=3,rate=85.7,history=[dict(id=uid(30),sessionId=SESSION,date=TODAY,time='16:00',groupName=CHILD['groupName'],state='present',absenceReason=None)]),upcoming=[dict(id=SESSION,date=TODAY,time='16:00',groupName=CHILD['groupName'],trainerName=CHILD['trainerName'])])); return
        if path.endswith('/documents'):
            if method=='POST': assert base64.b64decode(body['contentBase64']); item=dict(DOC,id=uid(121),name=body['name']);self.docs.append(item);send(item,201)
            else:send(listing(self.docs))
            return
        if path.endswith('/download'): route.fulfill(body=b'%PDF-1.4 synthetic',content_type='application/pdf',headers={'Content-Disposition':'attachment; filename="medical.pdf"','Cache-Control':'no-store'});return
        if '/documents/' in path and path.endswith('/delete'): self.docs=[];send({'status':'ok'});return
        if path.endswith('/subscriptions'): send(listing([SUB]));return
        if path.endswith('/payments'):
            if method=='POST': item=dict(PAYMENT,**{k:v for k,v in body.items() if k in PAYMENT},id=uid(120+len(self.payments)),fullName=CHILD['fullName']);self.payments.append(item);send(item,201)
            else:send(listing([p for p in self.payments if query.get('status',['all'])[0] in ('all',p['status'])]))
            return
        if '/notifications' in path:
            if method=='POST':
                for note in self.notes:
                    if path.endswith('/read-all') or note['id'] in path:note['read']=True
                send({'status':'ok'})
            else:
                unread=sum(not note['read'] for note in self.notes); items=[note for note in self.notes if query.get('status',['all'])[0]!='unread' or not note['read']];send(dict(listing(items),unreadCount=unread))
            return
        if '/journal/records/' in path:
            record=next(r for r in self.records if r['id'] in path);record.update(state=body['state'],absenceReason=body['absenceReason'],version=record['version']+1);send(dict(session=SESSION_DATA,records=self.records));return
        if path.endswith('/journal'):send(dict(session=SESSION_DATA,records=self.records));return
        if path.endswith('/trainer-sessions'):send([SESSION_DATA]);return
        if path.endswith('/sessions'):send([SESSION_DATA]);return
        if path.endswith('/participants'):send(listing([dict(CHILD,participantId=MEMBER,participantVersion=1,version=1,joinedOn='2024-09-01',contactName=FAMILY_DATA['contactName'],contactKind='phone',contactValue=FAMILY_DATA['phone'],canEditIdentity=True)]));return
        if path.endswith('/sessions/groups') or path.endswith('/groups'):send([dict(id=GROUP,name=CHILD['groupName'],clubId=CLUB,archived=False,version=1)]);return
        if path.endswith('/trainers') or path.endswith('/schedule-rules') or path.endswith('/schedule'):send([]);return
        send(listing([]))

def main():
    errors=[]; results=[]
    with sync_playwright() as p:
        browser=getattr(p,ENGINE).launch()
        def setup(role='owner',motion='reduce'):
            fixture=Fixture(role);context=browser.new_context(viewport={'width':1440,'height':1000},reduced_motion=motion,service_workers='block',accept_downloads=True);context.add_init_script('delete Navigator.prototype.serviceWorker');context.route('**/api/**',fixture.route);page=context.new_page();observe_api(page);page.on('pageerror',lambda error:errors.append(str(error)));return context,page,fixture
        ctx,ui,fixture=setup()
        def go(section):
            # Commands can close their editor before their refresh requests finish.
            # Settle those reads before replacing the document (WebKit reports
            # cancelled intercepted requests as access-control page errors).
            wait_for_api_idle(ui);ui.goto(f'{ROOT}/clubs/{CLUB}/{section}');expect(ui.locator('main h1')).to_be_visible();ui.wait_for_load_state('networkidle');wait_for_api_idle(ui)
        def close_context():
            wait_for_api_idle(ui);ctx.close()
        def capture(name,widths=(1440,390,320)):
            for width in widths:
                ui.set_viewport_size({'width':width,'height':1000 if width>767 else 844});ui.wait_for_load_state('networkidle');ui.evaluate('document.fonts.ready');ui.mouse.move(0,0)
                if name=='member-Документы':
                    ui.wait_for_function('''()=>{const t=document.querySelector('.member-profile [role=tab][data-state=active]');const l=t.closest('[role=tablist]');const r=t.getBoundingClientRect(),b=l.getBoundingClientRect();return r.left>=b.left-1&&r.right<=b.right+1}''')
                overflow=ui.evaluate('''() => [...document.querySelectorAll('body *')].filter(e=>{const r=e.getBoundingClientRect();return r.width&&r.right>innerWidth+.5&&!e.closest('[data-slot="sidebar"]')}).slice(0,12).map(e=>({tag:e.tagName,cls:e.className,text:e.textContent.slice(0,50),r:e.getBoundingClientRect().toJSON()}))''')
                if os.environ.get('RD08_CAPTURE')=='1':ui.screenshot(path=str(OUT/f'{name}-{width}.png'),full_page=False)
                assert ui.evaluate('document.documentElement.scrollWidth<=innerWidth'),(name,width,overflow)
                audit=contrast_audit(ui);assert not audit['failures'],(name,width,audit['failures'])
            results.append(name);ui.set_viewport_size({'width':1440,'height':1000});wait_for_api_idle(ui)
        def pointer_tab(label):
            trigger=ui.get_by_role('tab',name=label,exact=True);trigger.click()
            assert trigger.evaluate('e=>getComputedStyle(e).outlineStyle')=='none',(label,'pointer focus rectangle')
        go('home');capture('home')
        go('families');capture('families');ui.get_by_role('button',name='Семья Ивановых',exact=True).click();capture('family')
        ui.get_by_role('button',name='Иванов Максим',exact=False).click();capture('member')
        for label in ('Группы','Посещения','Платежи','Документы'):
            pointer_tab(label);capture('member-'+label)
        assert ui.get_by_role('link',name='Скачать '+DOC['name']).get_attribute('href')==f'/api/clubs/{CLUB}/participants/{MEMBER}/documents/{DOC["id"]}/download'
        ui.get_by_role('button',name='Добавить документ',exact=True).click();ui.get_by_label('Название',exact=True).fill('Согласие');ui.locator('input[type=file]').set_input_files({'name':'consent.pdf','mimeType':'application/pdf','buffer':b'%PDF-1.4 test'});ui.get_by_role('button',name='Загрузить документ',exact=True).click();expect(ui.get_by_text('Согласие',exact=True)).to_be_visible()
        go('payments');capture('payments');pointer_tab('Абонементы');capture('subscriptions');pointer_tab('Тарифы');capture('plans')
        ui.get_by_role('button',name='Создать тариф',exact=True).click();ui.get_by_label('Название',exact=True).fill('Разовый визит');ui.get_by_label('Стоимость, BYN',exact=True).fill('20,50');ui.get_by_role('button',name='Сохранить тариф',exact=True).click();expect(ui.get_by_text('Разовый визит',exact=True)).to_be_visible();assert fixture.mutations[-1][1]['amountMinor']==2050
        ui.get_by_role('button',name='Добавить платёж',exact=True).click();capture('payment-editor');ui.get_by_role('combobox',name='Участник',exact=True).select_option(MEMBER);ui.get_by_label('Назначение платежа',exact=True).fill('Разовая тренировка');ui.get_by_label('Сумма, BYN',exact=True).fill('20,50');fixture.fail_next=True;ui.get_by_role('button',name='Зарегистрировать оплату',exact=True).click();expect(ui.get_by_role('alert')).to_contain_text('Запись уже изменена');expect(ui.get_by_label('Сумма, BYN',exact=True)).to_have_value('20,50');key=fixture.mutations[-1][1]['requestId'];ui.get_by_role('button',name='Зарегистрировать оплату',exact=True).click();expect(ui.get_by_label('Назначение платежа',exact=True)).to_have_count(0);assert fixture.mutations[-1][1]['requestId']==key
        go('statistics');pointer_tab('Финансы');ui.get_by_role('combobox',name='Филиал отчёта',exact=True).select_option(CLUB);capture('finance-report');go('profile');capture('profile');go('notifications');capture('notifications');ui.get_by_role('button',name='Прочитать все',exact=True).click();expect(ui.get_by_role('button',name='Прочитать все',exact=True)).to_be_disabled()
        go('settings');capture('settings');ui.get_by_role('tab',name='Интерфейс',exact=True).click();expect(ui.get_by_label('Сворачивать меню на компьютере')).to_be_visible();ui.set_viewport_size({'width':390,'height':844});expect(ui.get_by_label('Сворачивать меню на компьютере')).to_have_count(0)
        go('sessions');capture('sessions');buttons=ui.get_by_role('radiogroup',name='Вид календаря',exact=True)
        for name in ('Неделя','Период','День'):
            button=buttons.get_by_role('radio',name=name,exact=True);button.click();expect(button).to_have_attribute('aria-checked','true');assert button.evaluate('e=>getComputedStyle(e).backgroundColor')=='rgb(200, 16, 46)'
        date=ui.locator('.date-picker-group');centers=date.locator('button').evaluate_all('nodes=>nodes.map(n=>{const r=n.getBoundingClientRect();return r.y+r.height/2})');assert max(centers)-min(centers)<1,centers
        fixture.available=False;go('families');expect(ui.get_by_text('Раздел готовится к запуску',exact=True)).to_be_visible();capture('unavailable');fixture.available=True
        # Keyboard focus remains visible; programmatically focused headings have no rectangular outline.
        go('families');ui.get_by_role('button',name='Семья Ивановых',exact=True).click();heading=ui.get_by_role('dialog').locator('[data-initial-focus]');assert heading.evaluate('e=>getComputedStyle(e).outlineStyle')=='none';ui.keyboard.press('Tab');focused=ui.locator(':focus');assert focused.evaluate('e=>getComputedStyle(e).outlineStyle')!='none' or focused.evaluate('e=>getComputedStyle(e).boxShadow')!='none'
        # Background refresh retains nested editor state; keyboard tabs retain a ring.
        pointer_tab('Доступ родителей');ui.keyboard.press('ArrowLeft');focused=ui.locator(':focus');assert focused.get_attribute('role')=='tab';assert focused.evaluate('e=>getComputedStyle(e).outlineStyle')!='none'
        pointer_tab('Доступ родителей');ui.get_by_role('button',name='Создать аккаунт',exact=True).click();ui.get_by_label('Логин',exact=False).fill('parent.draft');before=len(fixture.reads);ui.evaluate('window.dispatchEvent(new Event("focus"))');ui.wait_for_load_state('networkidle');assert len(fixture.reads)>before;expect(ui.get_by_label('Логин',exact=False)).to_have_value('parent.draft')
        close_context()
        ctx,ui,fixture=setup('parent');go('families');capture('parent-family');assert ui.get_by_label('Филиал',exact=True).count()==0;assert ui.get_by_role('button',name='Добавить семью',exact=True).count()==0;go('payments');capture('parent-payments');expect(ui.get_by_role('button',name='Оплатить онлайн',exact=True)).to_be_disabled()
        # Same-club access keys stay unchanged: remove A while family B remains.
        fixture.parent_two=True;go('families');expect(ui.get_by_role('heading',name='Семья Орловых',exact=True)).to_be_visible();ui.get_by_role('button',name='Иванов Максим',exact=False).click();expect(ui.locator('.member-profile')).to_contain_text('Иванов Максим');fixture.revoked={FAMILY,MEMBER};ui.evaluate('window.dispatchEvent(new Event("focus"))');expect(ui.locator('.member-profile')).to_have_count(0);expect(ui.get_by_role('heading',name='Семья Ивановых',exact=True)).to_have_count(0);expect(ui.get_by_role('heading',name='Семья Орловых',exact=True)).to_be_visible();close_context()
        ctx,ui,fixture=setup('trainer');go('journals');expect(ui.get_by_label('Филиал',exact=True)).to_have_count(0);ui.get_by_role('button',name='Посмотреть журнал',exact=True).click();expect(ui.get_by_role('button',name='Присутствует',exact=True)).to_be_visible();capture('attendance')
        for i in range(2):
            button=ui.get_by_role('button',name='Присутствует',exact=True);button.click()
            if i==0:
                expect(ui.locator('.attendance-card h4')).to_have_text('Иванова София');expect(button).to_be_enabled();assert button.evaluate('e=>getComputedStyle(e).backgroundColor') in ('rgb(36, 102, 78)','rgb(28, 80, 61)')
        ui.get_by_role('button',name='Завершить журнал',exact=True).click();expect(ui.get_by_role('dialog')).to_contain_text('Все участники отмечены');expect(ui.get_by_role('dialog')).not_to_contain_text('эти участники получат');capture('journal-complete');close_context()
        # Motion is a geometry check across the transition, and labels stay on one line.
        ctx,ui,fixture=setup(motion='no-preference');go('home')
        # Sample collapsing in the renderer task. The expanding endpoint is
        # checked separately: WebKit automation can return after its transition.
        samples=ui.evaluate('''async()=>{
          const sidebar=document.querySelector('[data-slot="sidebar-container"]');
          const result=[]; sidebar.getBoundingClientRect();
          document.querySelector('[data-slot="sidebar-trigger"]').click();
          for(let i=0;i<20;i++){
            await new Promise(requestAnimationFrame);
            result.push({w:sidebar.getBoundingClientRect().width,
              m:parseFloat(getComputedStyle(document.querySelector('main')).marginLeft),
              h:document.querySelector('.workspace-navigation a>span').getBoundingClientRect().height});
          }
          return result;
        }''')
        assert any(76<x['w']<248 for x in samples),samples
        assert all(abs(x['w']-x['m'])<2 for x in samples),samples
        assert max(x['h'] for x in samples)-min(x['h'] for x in samples)<1,samples
        ui.wait_for_function('document.querySelector("[data-slot=sidebar-container]").getBoundingClientRect().width===76')
        ui.get_by_role('button',name='Переключить боковое меню',exact=True).click()
        ui.wait_for_function('document.querySelector("[data-slot=sidebar-container]").getBoundingClientRect().width===248')
        assert ui.locator('main').evaluate('e=>parseFloat(getComputedStyle(e).marginLeft)')==248
        close_context()
        assert not errors,errors
        (OUT/'results.json').write_text(json.dumps({'engine':ENGINE,'source':'synthetic contract fixtures; not backend acceptance','screens':results,'mutationsChecked':['moneyMinor','requestId retry','document upload/download URL (binary backend deferred)' ,'read-all','attendance next/complete'],'pageErrors':errors},ensure_ascii=False,indent=2))
        browser.close();print('RD-08 frontend contract PASS:',ENGINE,len(results),'screen/state groups; synthetic responses; NOT backend acceptance',flush=True)
if __name__=='__main__':main()

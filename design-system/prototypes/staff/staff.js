'use strict';
const $ = (selector) => document.querySelector(selector);
const icons = {
 plus: '<path d="M12 5v14M5 12h14"/>', search: '<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 4 4"/>',
 arrow: '<path d="m9 5 7 7-7 7"/>', close: '<path d="m6 6 12 12M6 18 18 6"/>',
 key: '<circle cx="8" cy="9" r="5"/><path d="m12 12 9 9m-4-4 3-3m-6 0 3-3"/>',
 lock: '<rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/>',
 minus: '<circle cx="12" cy="12" r="9"/><path d="M7 12h10"/>', back: '<path d="m10 5-7 7 7 7M3 12h18"/>'
};
const icon = (name) => `<svg viewBox="0 0 24 24" aria-hidden="true">${icons[name]}</svg>`;
const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const branches = {east:'Брест · Восток', center:'Брест · Центр', kobrin:'Кобрин'};
const roleNames = {trainer:'Тренер', admin:'Администратор'};
let people = [
 {id:1,name:'Адамович Иван Андреевич',login:'trainer.ivan.adamovich',disabled:false,assignments:[{branch:'east',role:'trainer'},{branch:'center',role:'trainer'}]},
 {id:2,name:'Гурский Павел Дмитриевич',login:'trainer.pavel.gurskiy',disabled:false,assignments:[{branch:'east',role:'trainer'}]},
 {id:3,name:'Дубовик Роман Николаевич',login:'trainer.roman.dubovik',disabled:false,assignments:[{branch:'east',role:'trainer'}]},
 {id:4,name:'Станкевич Михаил Александрович',login:'trainer.mikhail.stankevich',disabled:false,assignments:[{branch:'east',role:'trainer'},{branch:'kobrin',role:'trainer'}]},
 {id:5,name:'Шевцов Виктор Михайлович',login:'trainer.viktor.shevtsov',disabled:false,assignments:[{branch:'east',role:'trainer'}]},
 {id:6,name:'Ковалёва Анна Сергеевна',login:'admin.anna.kovaleva',disabled:false,assignments:[{branch:'east',role:'admin'},{branch:'center',role:'admin'}]},
 {id:7,name:'Лис Андрей Викторович',login:'andrey.lis',disabled:false,assignments:[{branch:'center',role:'admin'},{branch:'center',role:'trainer'}]},
 {id:8,name:'Мороз Елена Павловна',login:'elena.moroz',disabled:true,assignments:[{branch:'kobrin',role:'trainer'}]},
 {id:9,name:'Новик Алексей Игоревич',login:'aleksey.novik',disabled:false,assignments:[]}
];
let concept = new URLSearchParams(location.search).get('concept') === 'b' ? 'b' : 'a';
let selectedId = 6, toastTimer, returnFocus;
const personById = (id) => people.find((p) => p.id === Number(id));
const initials = (p) => p.name.split(' ').slice(0,2).map((s) => s[0]).join('');
const avatar = (p) => `<span class="avatar" aria-hidden="true">${esc(initials(p))}</span>`;
const roles = (p) => [...new Set(p.assignments.map((a) => roleNames[a.role]))].join(', ') || 'Роль не назначена';
const badge = (p) => p.disabled ? '<span class="badge">Доступ отключён</span>' : !p.assignments.length ? '<span class="badge waiting">Без доступа</span>' : '<span class="badge active">Доступ открыт</span>';
function notify(message) { clearTimeout(toastTimer); $('#toast').textContent = message; $('#toast').classList.add('visible'); toastTimer = setTimeout(() => $('#toast').classList.remove('visible'), 5500); }
function setConcept(next) {
 concept = next; document.body.dataset.concept = concept; document.body.classList.remove('detail-open');
 document.querySelectorAll('[data-concept]').forEach((b) => b.setAttribute('aria-pressed',String(b.dataset.concept === concept)));
 $('#concept-description').innerHTML = concept === 'a' ? '<strong>А · Быстрый реестр.</strong> Вся команда на одном экране. Откройте сотрудника, чтобы изменить доступ.' : '<strong>Б · Карточка доступа.</strong> Список для поиска, одна карточка для аккаунта, ролей и доступа.';
 const url = new URL(location.href); url.searchParams.set('concept',concept); history.replaceState(null,'',url); render();
}
function filteredPeople() {
 const query = $('#search').value.trim().toLocaleLowerCase('ru');
 return people.filter((p) =>
 (!query || `${p.name} ${p.login}`.toLocaleLowerCase('ru').includes(query)) &&
 (p.assignments.some((a) => ($('#role-filter').value === 'all' || a.role === $('#role-filter').value) && ($('#branch').value === 'all' || a.branch === $('#branch').value)) || ($('#role-filter').value === 'all' && $('#branch').value === 'all')) &&
 ($('#status-filter').value === 'all' || ($('#status-filter').value === 'disabled' ? p.disabled : $('#status-filter').value === 'unassigned' ? !p.assignments.length : !p.disabled && p.assignments.length)))
 .sort((a,b) => a.name.localeCompare(b.name,'ru') * ($('#sort').value === 'asc' ? 1 : -1));
}
function render() {
 const items = filteredPeople();
 if (concept === 'b' && !items.some((p) => p.id === selectedId)) selectedId = items[0]?.id;
 $('#results').textContent = `Показано ${items.length} из ${people.length}`;
 $('#reset-filters').hidden = !($('#search').value || $('#role-filter').value !== 'all' || $('#branch').value !== 'all' || $('#status-filter').value !== 'all');
 $('#add-staff').innerHTML = icon('plus') + 'Добавить сотрудника';
 if (!items.length) $('#roster-content').innerHTML = '<div class="empty"><h3>Сотрудники не найдены</h3><p>Попробуйте другое имя или снимите ограничения поиска.</p><button data-action="reset">Сбросить фильтры</button></div>';
 else if (concept === 'a') $('#roster-content').innerHTML = `<table><thead><tr><th scope="col">Сотрудник</th><th scope="col" class="role-cell">Роли и филиалы</th><th scope="col">Доступ</th><th scope="col"><span class="sr-only">Действия</span></th></tr></thead><tbody>${items.map((p) => `<tr><td class="person-cell"><div class="person">${avatar(p)}<div class="person-text"><button class="name-button" data-open="${p.id}">${esc(p.name)}</button><small>${esc(p.login)}</small><span class="mobile-role">${esc(roles(p))}</span></div></div></td><td class="role-cell"><div class="assignment-summary">${p.assignments.map((a) => `<span>${roleNames[a.role]}<small>${esc(branches[a.branch])}</small></span>`).join('') || '<span>Роль не назначена</span>'}</div></td><td class="status-cell">${badge(p)}</td><td class="action-cell"><button class="icon-button" data-open="${p.id}" aria-label="Открыть сотрудника: ${esc(p.name)}">${icon('arrow')}</button></td></tr>`).join('')}</tbody></table>`;
 else $('#roster-content').innerHTML = `<ul class="compact-list">${items.map((p) => `<li><button class="compact-person" data-open="${p.id}" aria-current="${p.id === selectedId}"><span class="person">${avatar(p)}<span class="person-text"><strong>${esc(p.name)}</strong><small>${esc(roles(p))} · ${p.assignments.length ? [...new Set(p.assignments.map((a) => a.branch))].length + ' фил.' : 'Без назначений'}</small>${p.disabled ? '<small>Доступ отключён</small>' : ''}</span></span>${icon('arrow')}</button></li>`).join('')}</ul>`;
 $('#detail').hidden = concept !== 'b';
 if (concept === 'b') {
  $('#detail').innerHTML = selectedId ? detailMarkup(personById(selectedId)) : '<h2>Выберите сотрудника</h2><p class="notice">Измените фильтры, чтобы увидеть сотрудников.</p>';
 }
}
function detailMarkup(p) {
 return `<button class="back-button text-button" data-action="back">${icon('back')}К списку сотрудников</button><div class="detail-head">${avatar(p)}<div><h2 tabindex="-1" id="person-heading">${esc(p.name)}</h2>${badge(p)}</div></div>
 <div class="account-data"><div><p>Логин</p><strong>${esc(p.login)}</strong></div><button data-action="edit" data-id="${p.id}">Изменить</button></div>
 <section class="detail-section"><h3>Управление доступом</h3><button class="security-action" data-action="password" data-id="${p.id}"><span>${icon('key')}Сменить пароль</span>${icon('arrow')}</button><button class="security-action ${p.disabled ? '' : 'danger'}" data-action="disable" data-id="${p.id}"><span>${icon('lock')}${p.disabled ? 'Включить доступ' : 'Отключить весь доступ'}</span>${icon('arrow')}</button><p>${p.disabled ? 'Вход запрещён во всех филиалах. Назначения сохранены.' : 'Отключение запретит вход во все филиалы. Сотрудник и история сохранятся.'}</p></section>
 <section class="detail-section"><h3>Роли и филиалы <span class="badge">${p.assignments.length}</span></h3><p>Каждое назначение даёт доступ к одному филиалу.</p>
 ${p.assignments.map((a,i) => `<div class="assignment"><div><span>${esc(branches[a.branch])}</span><small>${roleNames[a.role]}</small></div><button class="icon-button" data-action="revoke" data-id="${p.id}" data-index="${i}" aria-label="Снять роль ${roleNames[a.role]}: ${esc(branches[a.branch])}">${icon('minus')}</button></div>`).join('') || '<p class="notice">Назначений нет. Аккаунт сохранён, но рабочие разделы недоступны.</p>'}
 <button class="full" data-action="assign" data-id="${p.id}">${icon('plus')} Добавить назначение</button></section>`;
}
function openDialog(title,html,wide=false) {
 if (!$('#dialog').open) returnFocus = document.activeElement;
 $('#dialog-title').textContent = title; $('#dialog-body').innerHTML = html; $('#dialog').classList.toggle('wide',wide);
 if (!$('#dialog').open) $('#dialog').showModal();
}
function closeDialog() { $('#dialog').close(); if (returnFocus?.isConnected) returnFocus.focus(); }
function openPerson(id) {
 selectedId = Number(id); const p = personById(id);
 if (concept === 'a') { openDialog('Доступ сотрудника',detailMarkup(p),true); }
 else { render(); document.body.classList.add('detail-open'); $('#person-heading')?.focus({preventScroll:true}); if (matchMedia('(max-width:800px)').matches) $('#detail').scrollIntoView({block:'start',behavior:'instant'}); }
}
function formActions(label='Сохранить') { return `<p class="form-error" id="form-error" role="alert" hidden></p><div class="form-actions"><button type="button" data-action="cancel">Отмена</button><button type="submit" class="primary">${label}</button></div>`; }
function branchOptions(value=$('#branch').value === 'all' ? 'east' : $('#branch').value) { return Object.entries(branches).map(([id,name]) => `<option value="${id}" ${value===id?'selected':''}>${name}</option>`).join(''); }
function assignmentFields(p, fixedTrainer=false) { return `<div class="form-grid"><label class="form-field">Роль<select name="role">${fixedTrainer?'<option value="trainer">Тренер</option>':'<option value="trainer">Тренер</option><option value="admin">Администратор</option>'}</select></label><label class="form-field">Филиал<select name="branch">${branchOptions()}</select></label></div><div class="notice" id="role-help"><strong>Тренер</strong>Доступ к назначенным занятиям и журналам филиала.</div>${p?.disabled ? '<p class="notice">Аккаунт отключён. Назначение сохранится, но для входа потребуется отдельно включить доступ.</p>' : ''}`; }
function addStaff(mode='new') {

 openDialog('Добавить сотрудника',`<div class="choice-tabs"><button data-create="new" aria-pressed="${mode==='new'}">Новый аккаунт</button><button data-create="existing" aria-pressed="${mode==='existing'}">Уже есть в сети</button></div><p class="dialog-intro">${mode==='new' ? 'Создайте аккаунт и назначьте роль в филиале.' : 'Выберите существующий аккаунт. Дублировать сотрудника не нужно.'}</p><form id="create-form" data-mode="${mode}">${mode==='new' ? `<label class="form-field">ФИО<input name="name" required maxlength="200" autocomplete="off" placeholder="Фамилия Имя Отчество"></label><label class="form-field">Логин<input name="login" required minlength="3" pattern="[a-zA-Z0-9._-]+" autocomplete="off" placeholder="ivan.petrov"><small>Латинские буквы, цифры, точка, дефис и подчёркивание.</small></label><div class="form-grid"><label class="form-field">Первый пароль<input name="password" type="password" required minlength="8" autocomplete="new-password"></label><label class="form-field">Повторите пароль<input name="confirm" type="password" required minlength="8" autocomplete="new-password"></label></div><p class="notice">Используйте вымышленный пароль. Это демонстрация, аккаунт создаётся только в памяти страницы.</p>` : `<label class="form-field">Сотрудник сети<select name="employee" required><option value="">Выберите сотрудника</option>${people.map((p) => `<option value="${p.id}">${esc(p.name)}${p.disabled ? ' · отключён' : ''}</option>`).join('')}</select></label>`}${assignmentFields(null)}${formActions(mode==='new'?'Создать и выдать доступ':'Добавить назначение')}</form>`);
}
function error(message) { $('#form-error').textContent = message; $('#form-error').hidden = false; }
function performAction(action,id,index) {
 const p = personById(id);
 if (action==='reset') { $('#search').value=''; $('#role-filter').value='all'; $('#branch').value='all'; $('#status-filter').value='all'; render(); return; }
 if (action==='back') { if ($('#dialog').open) { closeDialog(); return; } document.body.classList.remove('detail-open'); document.querySelector(`[data-open="${selectedId}"]`)?.focus(); return; }
 if (action==='cancel') { closeDialog(); return; }
 if (!p) return;

 if (action==='edit') openDialog('Изменить аккаунт',`<p class="dialog-intro">Данные общие для всех филиалов.</p><form id="edit-form" data-id="${id}"><label class="form-field">ФИО<input name="name" required maxlength="200" value="${esc(p.name)}"></label><label class="form-field">Логин<input name="login" required minlength="3" pattern="[a-zA-Z0-9._-]+" value="${esc(p.login)}"></label><p class="notice">При смене логина прежние сессии завершатся. Для входа потребуется новый логин.</p>${formActions()}</form>`);
 if (action==='assign') openDialog('Добавить назначение',`<p class="dialog-intro">${esc(p.name)}</p><form id="assign-form" data-id="${id}">${assignmentFields(p)}${formActions('Добавить назначение')}</form>`);
 if (action==='password') openDialog('Сменить пароль',`<p class="dialog-intro">${esc(p.name)}<br>${esc(p.login)}</p><form id="password-form" data-id="${id}"><label class="form-field">Новый пароль<input name="password" type="password" required minlength="8" autocomplete="new-password"><small>Минимум 8 символов. Введите вымышленный пароль для проверки.</small></label><label class="form-field">Повторите пароль<input name="confirm" type="password" required minlength="8" autocomplete="new-password"></label><p class="notice">Прежние сессии завершатся. Передайте новый пароль сотруднику лично или по защищённому каналу.</p>${formActions('Сменить пароль')}</form>`);
 if (action==='disable') openDialog(p.disabled?'Включить доступ?':'Отключить весь доступ?',`<p class="dialog-intro">${esc(p.name)}</p><div class="notice"><strong>${p.disabled ? 'Сотрудник сможет снова войти' : 'Вход будет запрещён во всех филиалах'}</strong>${p.disabled ? 'После включения потребуется новый вход. Доступные разделы определяются сохранёнными назначениями.' : 'Все сессии завершатся. Аккаунт, роли и история сохранятся. Позже вы сможете включить доступ.'}</div><form id="disable-form" data-id="${id}">${formActions(p.disabled?'Включить доступ':'Отключить весь доступ')}</form>`);
 if (action==='revoke') { const a=p.assignments[Number(index)]; openDialog('Снять назначение?',`<p class="dialog-intro">${esc(p.name)}</p><div class="notice"><strong>${roleNames[a.role]} · ${esc(branches[a.branch])}</strong>Сотрудник потеряет эту роль в филиале. Аккаунт, другие назначения и история сохранятся.${p.assignments.length===1?' Это последнее назначение: рабочие разделы станут недоступны.':''}</div><form id="revoke-form" data-id="${id}" data-index="${index}">${formActions('Снять назначение')}</form>`); }
}
function addAssignment(p,branch,role) {
 if (p.assignments.some((a) => a.branch===branch && a.role===role)) { error('Эта роль в филиале уже назначена. Выберите другой филиал или роль.'); return false; }
 p.assignments.push({branch,role}); return true;
}
document.addEventListener('click',(event) => {
 const b=event.target.closest('button'); if (!b) return;
 if (b.dataset.concept) setConcept(b.dataset.concept);
 else if (b.dataset.open) openPerson(b.dataset.open);
 else if (b.dataset.create) addStaff(b.dataset.create);
 else if (b.dataset.action) performAction(b.dataset.action,b.dataset.id,b.dataset.index);
});
document.addEventListener('submit',(event) => {
 event.preventDefault(); const form=event.target; const data=new FormData(form); const p=personById(form.dataset.id); let message='';
 if (form.id==='create-form' || form.id==='edit-form') {
  if (form.id==='edit-form' || form.dataset.mode==='new') {
   const login=data.get('login').trim().toLowerCase(), name=data.get('name').trim();
   if (!name) return error('Введите имя сотрудника.');
   if (people.some((other) => other.id!==p?.id && other.login.toLowerCase()===login)) return error('Этот логин занят. Выберите вкладку «Уже есть в сети» или укажите другой логин.');
   if (form.id==='edit-form') { p.name=name; p.login=login; message='Данные аккаунта сохранены в прототипе.'; }
   else {
    if (data.get('password')!==data.get('confirm')) return error('Пароли не совпадают. Повторите новый пароль.');
    const created={id:Math.max(...people.map((x)=>x.id))+1,name,login,disabled:false,assignments:[{branch:data.get('branch'),role:data.get('role')}]};
    people.push(created); selectedId=created.id; message='Сотрудник создан, роль и доступ к филиалу назначены.';
   }
  } else {
   const existing=personById(data.get('employee')); if (!existing) return error('Выберите сотрудника сети.');
   if (!addAssignment(existing,data.get('branch'),data.get('role'))) return;
   selectedId=existing.id; message='Назначение добавлено существующему сотруднику.';
  }
 } else if (form.id==='assign-form') { if (!addAssignment(p,data.get('branch'),data.get('role'))) return; message='Назначение добавлено. Другие роли сохранены.'; }
 else if (form.id==='password-form') { if (data.get('password')!==data.get('confirm')) return error('Пароли не совпадают. Повторите новый пароль.'); message='Смена пароля показана в прототипе. Реальный пароль не изменялся.'; }
 else if (form.id==='disable-form') { p.disabled=!p.disabled; message=p.disabled?'Весь доступ отключён. Аккаунт и история сохранены.':'Доступ включён. Сотрудник сможет войти заново.'; }
 else if (form.id==='revoke-form') { p.assignments.splice(Number(form.dataset.index),1); message='Назначение снято. Аккаунт и история сохранены.'; }
 else return;
 form.querySelectorAll('input[type="password"]').forEach((input)=>input.value='');
 closeDialog(); render();
 const focusTarget = concept === 'b'
  ? ($('#person-heading') || $('#search'))
  : (document.querySelector(`.name-button[data-open="${selectedId}"]`) || $('#search'));
 focusTarget.focus();
 notify(message);
});
document.addEventListener('change',(event) => {
 if (event.target.name==='role' && $('#role-help')) $('#role-help').innerHTML = event.target.value==='admin' ? '<strong>Администратор</strong>Управление участниками, группами, расписанием, журналами и тренерами выбранного филиала.' : '<strong>Тренер</strong>Доступ к назначенным занятиям и журналам филиала.';
 if (['branch','role-filter','status-filter','sort'].includes(event.target.id)) { document.body.classList.remove('detail-open'); render(); }
});
$('#search').addEventListener('input',render);
$('#reset-filters').addEventListener('click',()=>performAction('reset'));
$('#add-staff').addEventListener('click',()=>addStaff());
$('#close-dialog').innerHTML=icon('close'); $('#search-icon').innerHTML=icon('search');
$('#close-dialog').addEventListener('click',closeDialog);
$('#dialog').addEventListener('close',()=>{ $('#dialog-body').querySelectorAll('input[type="password"]').forEach((input)=>input.value=''); if(returnFocus?.isConnected) returnFocus.focus(); });
document.addEventListener('keydown',(event)=>{ if(event.key==='/' && !$('#dialog').open && !['INPUT','SELECT','TEXTAREA'].includes(document.activeElement.tagName)) { event.preventDefault(); $('#search').focus(); } });
setConcept(concept);

/* Synthetic, local-only demo. There is deliberately no API or storage dependency. */
'use strict';
const TODAY = '2026-10-20';
const groups = {
  first: { name: 'Первые шаги', age: '6–9 лет', size: 14 },
  base: { name: 'Основы дзюдо', age: '10–13 лет', size: 16 },
  team: { name: 'Сборная', age: '14–17 лет', size: 12 },
};
const trainers = { viktor: { name: 'Виктор Шевцов', initials: 'ВШ' }, anna: { name: 'Анна Станкевич', initials: 'АС' } };
const statuses = { planned: 'Запланировано', open: 'Журнал открыт', closed: 'Завершено', cancelled: 'Отменено' };
const paths = {
  plus: '<path d="M12 5v14M5 12h14"/>',
  left: '<path d="m14 6-6 6 6 6"/>', right: '<path d="m10 6 6 6-6 6"/>',
  chevron: '<path d="m7 10 5 5 5-5"/>', close: '<path d="m6 6 12 12M6 18 18 6"/>',
  clock: '<circle cx="12" cy="12" r="8.5"/><path d="M12 7v5l3 2"/>',
  group: '<circle cx="9" cy="8" r="3"/><path d="M3 20v-2a6 6 0 0 1 12 0v2M16 5a3 3 0 0 1 0 6m2 3a5 5 0 0 1 3 4v2"/>',
  person: '<circle cx="12" cy="7" r="3.5"/><path d="M5 21v-3a7 7 0 0 1 14 0v3"/>',
  more: '<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
  check: '<path d="m5 12 4 4L19 6"/>',
  book: '<path d="M5 4h14v17H5a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2Zm0 13h14M8 8h7M8 11h5"/>',
  move: '<path d="M4 8h15m-4-4 4 4-4 4M20 16H5m4-4-4 4 4 4"/>',
  calendar: '<rect x="4" y="5" width="16" height="16" rx="2"/><path d="M8 3v4m8-4v4M4 11h16"/>',
};
function icon(name) { return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths[name] || ''}</svg>`; }
function hydrateIcons(root = document) { root.querySelectorAll('[data-icon]').forEach(el => { el.innerHTML = icon(el.dataset.icon); }); }
function esc(value) { return String(value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c])); }
function date(value) { return new Date(`${value}T12:00:00`); }
function iso(value) { return `${value.getFullYear()}-${String(value.getMonth()+1).padStart(2,'0')}-${String(value.getDate()).padStart(2,'0')}`; }
function addDays(value, n) { const d = date(value); d.setDate(d.getDate()+n); return iso(d); }
function monday(value) { const d = date(value); return addDays(value, -(d.getDay()+6)%7); }
function fmt(value, options) { return date(value).toLocaleDateString('ru-RU',options); }
function minutes(time) { const [h,m] = time.split(':').map(Number); return h*60+m; }
function endTime(s) { const t = minutes(s.time)+s.duration; return `${String(Math.floor(t/60)).padStart(2,'0')}:${String(t%60).padStart(2,'0')}`; }
function lessonWord(n) { return n%10===1 && n%100!==11 ? 'занятие' : n%10>=2 && n%10<=4 && (n%100<12 || n%100>14) ? 'занятия' : 'занятий'; }
let sessions = [];
let seq = 0;
function seed(day, time, group, trainer, status = 'planned', extra = {}) { sessions.push({ id: ++seq, date: day, time, duration: 75, group, trainer, status, ...extra }); }
['2026-10-19','2026-10-21','2026-10-23'].forEach(day => {
  seed(day,'14:00','first','anna',day<TODAY?'closed':'planned');
  seed(day,'15:30','base','viktor',day<TODAY?'closed':'planned');
  seed(day,'18:00','team','viktor',day<TODAY?'closed':'planned', {duration:90});
});
seed(TODAY,'13:00','first','viktor','closed',{present:12});
seed(TODAY,'14:30','base','viktor','closed',{present:14});
seed(TODAY,'16:00','first','anna','open',{marked:9,change:'Замена тренера',replacedFrom:'viktor'});
seed(TODAY,'17:30','base','anna','planned');
seed(TODAY,'19:00','team','viktor','planned',{duration:60,change:'Перенесено с 21 октября'});
seed(TODAY,'20:15','team','viktor','cancelled',{duration:45,reason:'Тренер на соревнованиях'});
seed('2026-10-22','13:00','first','viktor'); seed('2026-10-22','14:30','base','viktor');
seed('2026-10-22','16:00','first','anna'); seed('2026-10-22','17:30','base','anna');
seed('2026-10-24','13:00','team','anna','planned',{duration:90,once:true});
seed('2026-10-24','15:00','base','viktor');
let concept = new URLSearchParams(location.search).get('concept') === 'week' ? 'week' : 'day';
let selected = TODAY;
let weekStart = monday(selected);
let filters = { group:'all', trainer:'all', status:'all' };
const dialog = document.querySelector('#lesson-dialog');
let opener;
let openerSelector;
let toastTimer;
function visible(s) { return Object.entries(filters).every(([key,value]) => value === 'all' || s[key] === value); }
function daySessions(day, applyFilters=true) { return sessions.filter(s=>s.date===day && (!applyFilters || visible(s))).sort((a,b)=>a.time.localeCompare(b.time)); }
function badge(s) { return `<span class="status ${s.status}">${s.status==='closed'?icon('check'):s.status==='open'?icon('book'):''}${statuses[s.status]}</span>`; }
function notify(message) { const t = document.querySelector('#toast'); clearTimeout(toastTimer); t.textContent=message; t.hidden=false; toastTimer=setTimeout(()=>t.hidden=true,4500); }
function render() {
  document.body.dataset.concept=concept;
  document.querySelectorAll('[data-concept]').forEach(a => { if(a.dataset.concept===concept) a.setAttribute('aria-current','page'); else a.removeAttribute('aria-current'); });
  document.querySelector('[data-action="prev"]').setAttribute('aria-label',concept==='day'?'Предыдущий день':'Предыдущая неделя');
  document.querySelector('[data-action="next"]').setAttribute('aria-label',concept==='day'?'Следующий день':'Следующая неделя');
  document.querySelector('#date-title').textContent=concept==='day' ? fmt(selected,{day:'numeric',month:'long'}) : `${fmt(weekStart,{day:'numeric'})}–${fmt(addDays(weekStart,6),{day:'numeric',month:'long'})}`;
  document.querySelector('#day-strip').innerHTML=Array.from({length:7},(_,i)=>{
    const d=addDays(weekStart,i), count=daySessions(d).length;
    return `<button class="day-button" data-day="${d}" aria-pressed="${selected===d}" aria-label="${esc(fmt(d,{weekday:'long',day:'numeric',month:'long'}))}, ${count} ${lessonWord(count)}${d===TODAY?', сегодня':''}"><span class="day-top">${fmt(d,{weekday:'short'})}${d===TODAY?'<span class="today-label">Сегодня</span>':''}</span><span class="day-number">${date(d).getDate()}</span><span class="day-count">${count?`${count} <span class="word">${lessonWord(count)}</span>`:'—'}</span></button>`;
  }).join('');
  const activeFilters=Object.values(filters).some(v=>v!=='all');
  document.querySelector('.reset').hidden=!activeFilters;
  const count=concept==='day' ? daySessions(selected).length : sessions.filter(s=>s.date>=weekStart && s.date<=addDays(weekStart,6) && visible(s)).length;
  document.querySelector('#result-count').textContent=`${count} ${lessonWord(count)}${concept==='week'?' за неделю':''}`;
  document.querySelector('#schedule-content').innerHTML=concept==='day' ? agenda(selected) : week();
  document.querySelector('#concept-description').textContent=concept==='day' ? 'Повестка дня · быстрые действия и журналы в одном списке.' : 'Обзор недели · время, интервалы и изменения на одной сетке.';
  hydrateIcons();
}
function agenda(day) {
  const list=daySessions(day);
  if(!list.length) return `<div class="empty-state"><h2>${Object.values(filters).some(v=>v!=='all')?'Нет занятий по этим фильтрам':'На этот день занятий нет'}</h2><p>${Object.values(filters).some(v=>v!=='all')?'Сбросьте фильтры, чтобы увидеть всё расписание.':'Выберите другой день или добавьте разовое занятие.'}</p><button class="button" data-action="${Object.values(filters).some(v=>v!=='all')?'reset':'add'}">${Object.values(filters).some(v=>v!=='all')?'Сбросить фильтры':'Добавить занятие'}</button></div>`;
  const completed=list.filter(s=>s.status==='closed');
  const collapse=day===TODAY && filters.status==='all' && completed.length>0 && completed.length<list.length;
  const displayed=collapse?list.filter(s=>s.status!=='closed'):list;
  const finished=completed.length;
  return `<div class="list-title"><h2>${esc(fmt(day,{weekday:'long',day:'numeric',month:'long'}))}${day===TODAY?' · Сегодня':''}</h2><span>${finished?`${finished} завершено`:'Расписание дня'}</span></div><div class="column-head" aria-hidden="true"><span>Время</span><span>Группа</span><span class="trainer">Тренер</span><span>Состояние журнала</span><span></span></div><ol class="lesson-list">${displayed.map(row).join('')}</ol>${collapse?`<details class="completed-lessons"><summary>Ранее сегодня · ${finished} завершено</summary><ol class="lesson-list">${completed.map(row).join('')}</ol></details>`:''}`;
}
function row(s) {
  const g=groups[s.group], t=trainers[s.trainer];
  const current=s.status==='open' && s.date===TODAY && minutes(s.time)<=980 && minutes(s.time)+s.duration>980;
  const sub=s.status==='closed'?`${s.present??g.size-2} из ${g.size} присутствовали`:s.status==='open'?`${s.marked??0} из ${g.size} отмечено`:s.status==='cancelled'?s.reason:'Журнал ещё не открыт';
  return `<li><article class="lesson-row ${current?'current':''} ${s.status==='cancelled'?'cancelled':''}" aria-label="${esc(`${s.time}, ${g.name}, ${t.name}, ${statuses[s.status]}`)}"><div class="lesson-time"><strong>${s.time}</strong><span>${endTime(s)}</span>${current?'<span class="now-label">Сейчас</span>':''}</div><div><h3 class="lesson-name">${g.name}</h3><div class="lesson-sub"><span>${g.age}</span>${s.once?'<span>· Разовое</span>':''}<span class="mobile-trainer">${t.name}</span></div>${s.change?`<span class="change-label">${icon('move')}${esc(s.change)}</span>`:''}</div><div class="trainer"><span class="avatar" aria-hidden="true">${t.initials}</span><div><div class="trainer-name">${t.name}</div>${s.replacedFrom?`<div class="trainer-sub">Вместо ${trainers[s.replacedFrom].name}</div>`:''}</div></div><div class="journal-info">${badge(s)}<span class="journal-sub">${esc(sub)}</span></div><div class="row-actions">${s.status!=='cancelled'?`<button class="button ${current?'primary':''}" data-journal="${s.id}" aria-label="${current?'Продолжить':'Открыть'} журнал ${s.time}">${current?'Продолжить журнал':'Журнал'}</button>`:`<button class="button" data-detail="${s.id}">Причина отмены</button>`}<button class="icon-button" data-detail="${s.id}" aria-label="Действия с занятием ${s.time}">${icon('more')}</button></div></article></li>`;
}
function week() {
  const days=Array.from({length:7},(_,i)=>addDays(weekStart,i));
  return `<div class="week-scroll" tabindex="0" role="region" aria-label="Недельная сетка занятий. Можно прокручивать по горизонтали."><div class="week-calendar"><div class="week-head"><span></span>${days.map(d=>`<button class="week-day-heading ${d===TODAY?'today-column':''}" data-open-day="${d}" aria-label="Открыть ${fmt(d,{weekday:'long',day:'numeric',month:'long'})}"><span>${fmt(d,{weekday:'short'})}</span><strong>${date(d).getDate()}</strong></button>`).join('')}</div><div class="week-body"><div class="time-axis" aria-hidden="true">${Array.from({length:8},(_,i)=>`<span>${i+13}:00</span>`).join('')}</div>${days.map(d=>`<div class="week-column ${d===TODAY?'today-column':''}">${daySessions(d).map(s=>`<button class="week-event ${s.status}" data-detail="${s.id}" style="top:${(minutes(s.time)-780)*1.4}px;height:${Math.max(s.duration*1.4-6,58)}px" aria-label="${esc(`${fmt(d,{weekday:'long'})}, ${s.time}, ${groups[s.group].name}, ${trainers[s.trainer].name}, ${statuses[s.status]}`)}"><span class="event-time">${s.time}–${endTime(s)}</span><strong>${groups[s.group].name}</strong><span class="event-trainer">${trainers[s.trainer].name}</span>${s.status!=='planned'?badge(s):''}</button>`).join('')}${d===TODAY?'<div class="now-line" aria-hidden="true"></div>':''}</div>`).join('')}</div></div></div><div class="calendar-foot"><div class="legend"><span>Нажмите занятие, чтобы открыть журнал или изменить его</span></div><span>Сегодня · 16:20</span></div><div class="mobile-week">${agenda(selected)}</div>`;
}
function openDialog(title, content) {
  if(!dialog.open) {
    opener=document.activeElement;
    openerSelector=opener?.dataset.journal?`[data-journal="${opener.dataset.journal}"]`:opener?.dataset.detail?`[data-detail="${opener.dataset.detail}"]`:null;
  }
  document.querySelector('#dialog-content').innerHTML=`<div class="dialog-header"><h2 id="dialog-title" tabindex="-1">${esc(title)}</h2><button class="icon-button" data-action="close" aria-label="Закрыть окно">${icon('close')}</button></div>${content}`;
  if(!dialog.open) dialog.showModal();
  else document.querySelector('#dialog-title').focus();
}
function closeDialog() { dialog.close(); }
dialog.addEventListener('close',()=> { if(opener?.isConnected) opener.focus(); else (openerSelector && document.querySelector(openerSelector) || document.querySelector('[data-action="add"]')).focus(); });
dialog.addEventListener('click',e=>{ if(e.target===dialog) {const r=dialog.getBoundingClientRect(); if(e.clientX<r.left || e.clientX>r.right || e.clientY<r.top || e.clientY>r.bottom) closeDialog();} });
function details(id) {
  const s=sessions.find(s=>s.id===id), g=groups[s.group];
  openDialog(g.name,`<p class="dialog-intro">${g.age} · ${fmt(s.date,{weekday:'long',day:'numeric',month:'long'})}</p>${badge(s)}<dl class="dialog-details"><dt>Время</dt><dd>${s.time}–${endTime(s)} · Минск</dd><dt>Тренер</dt><dd>${trainers[s.trainer].name}</dd><dt>Основание</dt><dd>${s.once?'Разовое занятие':'По расписанию группы'}</dd>${s.change?`<dt>Изменение</dt><dd>${esc(s.change)}</dd>`:''}${s.reason?`<dt>Причина</dt><dd>${esc(s.reason)}</dd>`:''}</dl>${s.status!=='cancelled'?`<button class="button ${s.status==='open'?'primary':''}" data-journal="${id}" style="width:100%">${icon('book')}${s.status==='open'?'Продолжить журнал':'Открыть журнал'}</button>`:''}${s.status==='planned'||s.status==='open'?`<div class="lesson-operations"><button class="button" data-edit="replace" data-id="${id}">${icon('person')}Заменить тренера</button>${s.status==='planned'?`<button class="button" data-edit="move" data-id="${id}">${icon('move')}Перенести занятие</button>`:''}<button class="button danger" data-edit="cancel" data-id="${id}">${icon('close')}Отменить занятие</button></div>`:''}${s.status==='closed'?'<p class="notice">Журнал завершён. Время и тренера этого занятия изменить нельзя.</p>':''}`);
}
function groupOptions(chosen='first') { return Object.entries(groups).map(([key,g])=>`<option value="${key}" ${chosen===key?'selected':''}>${g.name} · ${g.age}</option>`).join(''); }
function trainerOptions(chosen='viktor') { return Object.entries(trainers).map(([key,t])=>`<option value="${key}" ${chosen===key?'selected':''}>${t.name}</option>`).join(''); }
function formFooter(label='Сохранить') { return `<p class="form-error" id="form-error" role="alert" hidden></p><div class="dialog-actions"><button type="button" class="button" data-action="close">Назад</button><button type="submit" class="button primary">${label}</button></div>`; }
function edit(type,id) {
  const s=sessions.find(s=>s.id===id);
  const context=`<p class="dialog-intro">${groups[s.group].name} · ${fmt(s.date,{day:'numeric',month:'long'})}, ${s.time}</p>`;
  let fields='';
  if(type==='replace') fields=`<label class="field">Новый тренер<select name="trainer">${trainerOptions(s.trainer)}</select></label><p class="notice">Изменится только это занятие. Расписание группы останется прежним.</p>`;
  if(type==='move') fields=`<label class="field">Новая дата<input type="date" name="date" value="${s.date}" min="${TODAY}" required></label><div class="two-fields"><label class="field">Начало<input type="time" name="time" value="${s.time}" required></label><label class="field">Длительность, мин<input type="number" name="duration" min="15" max="180" step="15" value="${s.duration}" required></label></div><p class="notice">Перенос касается только этого занятия. Остальные занятия группы не изменятся.</p>`;
  if(type==='cancel') fields=`<p class="notice warning">Занятие останется в истории с пометкой «Отменено». В статистику проведённых оно не войдёт.</p><label class="field">Причина отмены<textarea name="reason" maxlength="1000" placeholder="Например, тренер на соревнованиях" required></textarea></label>${s.status==='open'?'<label class="check-field"><input type="checkbox" name="confirm" required><span>Понимаю, что журнал уже открыт. Существующие отметки сохранятся.</span></label>':''}`;
  openDialog({replace:'Заменить тренера',move:'Перенести занятие',cancel:'Отменить занятие'}[type],`${context}<form data-form="${type}" data-id="${id}">${fields}${formFooter(type==='cancel'?'Отменить занятие':'Сохранить изменение')}</form>`);
}
function add() {
  openDialog('Разовое занятие',`<p class="dialog-intro">Для существующей группы. Не меняет её постоянное расписание.</p><form data-form="add"><label class="field">Группа<select name="group">${groupOptions(filters.group==='all'?'first':filters.group)}</select></label><label class="field">Тренер<select name="trainer">${trainerOptions(filters.trainer==='all'?'viktor':filters.trainer)}</select></label><label class="field">Дата<input type="date" name="date" value="${selected<TODAY?TODAY:selected}" min="${TODAY}" required></label><div class="two-fields"><label class="field">Начало<input type="time" name="time" value="17:30" required></label><label class="field">Длительность, мин<input type="number" name="duration" min="15" max="180" step="15" value="75" required></label></div>${formFooter('Добавить занятие')}</form>`);
}
const sampleNames=['Алексей Демин','Михаил Лосев','София Ветрова','Артём Соколов'];
function journal(id) {
  const s=sessions.find(s=>s.id===id), closed=s.status==='closed';
  const saved=s.marks||['present','present','unknown','absent'];
  openDialog('Журнал занятия',`<p class="dialog-intro">${groups[s.group].name} · ${fmt(s.date,{day:'numeric',month:'long'})}, ${s.time}</p><p class="notice">Демонстрация перехода в журнал. Показаны четыре вымышленных участника. ${closed?'Завершённый журнал открыт для просмотра.':'Отметки сохраняются только в этом прототипе.'}</p><form data-form="journal" data-id="${id}"><div class="journal-demo">${sampleNames.map((name,i)=>`<div class="journal-person"><label for="mark-${i}">${name}</label><select id="mark-${i}" name="mark-${i}" ${closed?'disabled':''}>${[['unknown','Не отмечен'],['present','Присутствует'],['absent','Отсутствует']].map(([key,label])=>`<option value="${key}" ${saved[i]===key?'selected':''}>${label}</option>`).join('')}</select></div>`).join('')}</div>${closed?'<div class="dialog-actions"><button class="button" type="button" data-action="close">Закрыть</button></div>':formFooter('Сохранить отметки')}</form>`);
}
function showDate() {
  openDialog('Перейти к дате',`<form data-form="date"><label class="field">Дата<input type="date" name="date" value="${selected}" required></label><p class="notice">Демонстрационные занятия: 19–24 октября 2026. Другие даты показывают пустое состояние.</p>${formFooter('Перейти')}</form>`);
}
function about() {
  openDialog('Два подхода к занятиям',`<p class="dialog-intro">Те же данные и действия, два способа работы. Визуальная система PRIDE сохранена.</p><h3>Повестка дня</h3><ul class="about-list"><li>Семь дней с количеством занятий — переход в один клик.</li><li>Время, группа, тренер и журнал в одной строке. Текущий журнал выделен.</li><li>Перенос, замена и отмена в действиях занятия — без высокой стопки кнопок.</li></ul><h3>Обзор недели</h3><ul class="about-list"><li>Продолжительность и свободные интервалы видны на временной сетке.</li><li>Нажатие на занятие открывает подробности и те же действия.</li><li>На телефоне — дни недели и список выбранного дня вместо сжатой сетки.</li></ul><p class="notice">Фильтры применяются сразу. Вся работа локальная, демоданные общие для обоих вариантов. Перезагрузка вернёт исходное состояние.</p><div class="dialog-actions"><button class="button" data-action="close">Посмотреть</button></div>`);
}
document.addEventListener('click',e=> {
  const target=e.target.closest('button,a[data-concept]'); if(!target) return;
  if(target.dataset.concept) { e.preventDefault(); concept=target.dataset.concept; history.replaceState(null,'',`?concept=${concept}`); render(); return; }
  if(target.dataset.day) { selected=target.dataset.day; render(); document.querySelector(`[data-day="${selected}"]`).focus(); return; }
  if(target.dataset.openDay) { selected=target.dataset.openDay; concept='day'; history.replaceState(null,'','?concept=day'); render(); return; }
  if(target.dataset.detail) { details(Number(target.dataset.detail)); return; }
  if(target.dataset.journal) { journal(Number(target.dataset.journal)); return; }
  if(target.dataset.edit) { edit(target.dataset.edit,Number(target.dataset.id)); return; }
  const action=target.dataset.action;
  if(action==='close') closeDialog();
  if(action==='about') about();
  if(action==='date') showDate();
  if(action==='add') add();
  if(action==='prev'||action==='next') { const step=concept==='day'?1:7; const offset=action==='prev'?-step:step; selected=addDays(selected,offset); weekStart=monday(selected); render(); }
  if(action==='today') { selected=TODAY; weekStart=monday(TODAY); render(); }
  if(action==='reset') { filters={group:'all',trainer:'all',status:'all'}; Object.keys(filters).forEach(key=>document.querySelector(`#${key}-filter`).value='all'); render(); }
});
Object.keys(filters).forEach(key=>document.querySelector(`#${key}-filter`).addEventListener('change',e=> { filters[key]=e.target.value; render(); }));
document.addEventListener('submit',e=> {
  const form=e.target.closest('form[data-form]'); if(!form) return; e.preventDefault();
  const data=Object.fromEntries(new FormData(form)), type=form.dataset.form;
  const s=sessions.find(s=>s.id===Number(form.dataset.id));
  const fail=message=> { const error=form.querySelector('#form-error'); error.hidden=false; error.textContent=message; };
  if(type==='date') { selected=data.date; weekStart=monday(selected); closeDialog(); render(); return; }
  if(type==='add'||type==='move') {
    if(data.date===TODAY && minutes(data.time)<=980) { fail('Это время уже прошло. Выберите время после 16:20 или другой день.'); return; }
    if(minutes(data.time)+Number(data.duration)>1260 || minutes(data.time)<780) { fail('В демо доступны занятия с 13:00 до 21:00. Выберите время в этом интервале.'); return; }
    const group=type==='add'?data.group:s.group;
    const trainer=type==='add'?data.trainer:s.trainer;
    const overlap=sessions.find(other=>other.id!==s?.id && other.status!=='cancelled' && other.date===data.date && (other.group===group || other.trainer===trainer) && minutes(other.time)<minutes(data.time)+Number(data.duration) && minutes(other.time)+other.duration>minutes(data.time));
    if(overlap) { fail(`В это время у группы или тренера уже есть занятие (${overlap.time}–${endTime(overlap)}). Выберите свободный интервал.`); return; }
    if(type==='add') seed(data.date,data.time,data.group,data.trainer,'planned',{duration:Number(data.duration),once:true});
    else { const before=s.date; Object.assign(s,{date:data.date,time:data.time,duration:Number(data.duration),change:`Перенесено с ${fmt(before,{day:'numeric',month:'long'})}`}); }
    selected=data.date; weekStart=monday(selected);
    filters={group:'all',trainer:'all',status:'all'}; Object.keys(filters).forEach(key=>document.querySelector(`#${key}-filter`).value='all');
  }
  if(type==='replace') {
    const overlap=sessions.find(other=>other.id!==s.id && other.status!=='cancelled' && other.date===s.date && other.trainer===data.trainer && minutes(other.time)<minutes(s.time)+s.duration && minutes(other.time)+other.duration>minutes(s.time));
    if(overlap) {fail(`У тренера уже есть занятие ${overlap.time}–${endTime(overlap)}. Выберите другого тренера.`); return;}
    if(s.trainer!==data.trainer) {s.replacedFrom=s.trainer; s.trainer=data.trainer; s.change='Замена тренера';} }
  if(type==='cancel') { if(!data.reason.trim()) {fail('Укажите причину отмены.'); return;} s.status='cancelled'; s.reason=data.reason.trim(); }
  if(type==='journal') { s.marks=sampleNames.map((_,i)=>data[`mark-${i}`]); s.status='open'; s.marked=s.marks.filter(v=>v!=='unknown').length; }
  closeDialog(); render();
  notify({add:'Разовое занятие добавлено',move:'Занятие перенесено',replace:'Изменение тренера сохранено',cancel:'Занятие отменено. История сохранена',journal:'Демонстрационные отметки сохранены'}[type]);
});
render();

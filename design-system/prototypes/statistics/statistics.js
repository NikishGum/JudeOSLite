/* Synthetic aggregate data only. No API, personal data, or production mutation. */
(() => {
  'use strict';
  const $ = (selector, root = document) => root.querySelector(selector);
  const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];
  const escape = (value) => String(value).replace(/[&<>"']/g, (c) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const number = (n) => new Intl.NumberFormat('ru-RU').format(n);
  const rate = (r) => r.present + r.absent ? r.present / (r.present + r.absent) * 100 : null;
  const percent = (value) => value === null ? '—' : `${value.toFixed(1).replace('.', ',')}%`;
  const plural = (n, forms) => forms[n%100>=11&&n%100<=19?2:n%10===1?0:n%10>=2&&n%10<=4?1:2];
  const sessions = (n) => `${number(n)} ${plural(n,['занятие','занятия','занятий'])}`;
  const arrow = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 6 6 6-6 6"/></svg>';
  const info = '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 7v.1"/></svg>';
  const sortIcon = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m8 14 4 4 4-4M12 18V6"/></svg>';
  const fields = ['closed','planned','open','present','absent','unknown','illness','excused'];
  const zero = () => Object.fromEntries(fields.map((key) => [key, 0]));
  const sum = (rows) => rows.reduce((a, row) => { fields.forEach((key) => { a[key] += row[key]; }); return a; }, zero());
  // Integer allocations preserve every total; daily marks only occur on closed sessions.
  function allocate(total, weights) {
    const denominator = weights.reduce((a,b) => a+b, 0);
    if (!denominator) return weights.map(() => 0);
    const raw = weights.map((w) => total*w/denominator);
    const result = raw.map(Math.floor);
    const order = raw.map((v,i) => ({i,rest:v-result[i]})).sort((a,b) => b.rest-a.rest || a.i-b.i);
    const left = total-result.reduce((a,b) => a+b,0);
    for (let i=0; i<left; i++) result[order[i].i]++;
    return result;
  }
  const branches = [
    {id:'east',name:'Брест · Восток',closed:77,planned:9,open:1,present:1232,absent:308,unknown:80,illness:154,excused:74,weights:[.34,.42,.24],groups:['7–9 лет','10–12 лет','13–16 лет']},
    {id:'center',name:'Брест · Центр',closed:60,planned:8,open:0,present:985,absent:165,unknown:25,illness:100,excused:40,weights:[.56,.44],groups:['7–10 лет','11–14 лет']},
    {id:'south',name:'Минск · Южный',closed:44,planned:7,open:3,present:620,absent:230,unknown:70,illness:100,excused:60,weights:[.56,.44],groups:['7–10 лет','11–14 лет']},
    {id:'pinsk',name:'Пинск · Центр',closed:34,planned:6,open:0,present:546,absent:104,unknown:20,illness:64,excused:20,weights:[.55,.45],groups:['7–10 лет','11–14 лет']},
    {id:'kobrin',name:'Кобрин · Центр',closed:22,planned:6,open:2,present:330,absent:126,unknown:50,illness:44,excused:32,weights:[.58,.42],groups:['7–10 лет','11–14 лет']},
    {id:'new',name:'Брест · Заречье',closed:0,planned:4,open:0,present:0,absent:0,unknown:0,illness:0,excused:0,weights:[1],groups:['7–10 лет']},
  ];
  const groups = [];
  branches.forEach((branch, branchIndex) => {
    const allocated = Object.fromEntries(fields.filter((f) => !['unknown','illness','excused'].includes(f)).map((f) => [f,allocate(branch[f],branch.weights)]));
    const unknown = allocate(branch.unknown, allocated.absent);
    const illness = allocate(branch.illness, allocated.absent.map((n,i) => n-unknown[i]));
    branch.groups.forEach((name,i) => {
      const group = {id:`${branch.id}-${i}`,branchId:branch.id,branchName:branch.name,name, ...Object.fromEntries(Object.entries(allocated).map(([f,values]) => [f,values[i]]))};
      group.unknown = unknown[i]; group.illness = illness[i]; group.excused = group.absent-unknown[i]-illness[i];
      const weights = Array.from({length:20}, (_,day) => 1 + ((day*3+i+branchIndex)%5));
      const closed = allocate(group.closed,weights);
      const present = allocate(group.present, closed.map((c,day) => c*(.85+((day+i)%4)*.1)));
      const absent = allocate(group.absent, closed.map((c,day) => c*(.9+((day+branchIndex)%3)*.1)));
      const u = allocate(group.unknown,absent);
      const ill = allocate(group.illness,absent.map((n,day) => n-u[day]));
      group.days = Array.from({length:20}, (_,day) => ({date:`2026-10-${String(day+1).padStart(2,'0')}`,closed:closed[day],present:present[day],absent:absent[day],unknown:u[day],illness:ill[day],excused:absent[day]-u[day]-ill[day],planned:day===19?group.planned:0,open:day===19?group.open:0}));
      groups.push(group);
    });
  });
  const state = {idea:new URLSearchParams(location.search).get('idea')==='table'?'table':'overview',period:'month',from:'2026-10-01',until:'2026-10-20',branch:'all',group:'all',kind:'branches',issue:'all',search:'',sort:'rate',direction:1,expanded:new Set(),ranking:'rate'};
  let refreshTimer;
  let messageTimer;
  function notify(text) {
    clearTimeout(messageTimer); $('#live-message').textContent=text;
    messageTimer=setTimeout(() => { $('#live-message').textContent=''; },3500);
  }
  function filteredGroups() {
    return groups.filter((g) => (state.branch==='all'||g.branchId===state.branch) && (state.group==='all'||g.id===state.group)).map((g) => ({...g,...sum(g.days.filter((day) => day.date>=state.from&&day.date<=state.until))}));
  }
  function reportRows() {
    const selected = filteredGroups();
    return state.kind==='groups' ? selected : branches.filter((branch) => state.branch==='all'||branch.id===state.branch).map((branch) => ({...branch,...sum(selected.filter((g) => g.branchId===branch.id))}));
  }
  function branchRows() {
    const selected=filteredGroups();
    return branches.filter((branch) => state.branch==='all'||branch.id===state.branch).map((branch) => ({...branch,...sum(selected.filter((g) => g.branchId===branch.id))}));
  }
  function syncFilters() {
    $('#branch').innerHTML='<option value="all">Вся сеть</option>'+branches.map((b) => `<option value="${b.id}">${b.name}</option>`).join('');
    $('#branch').value=state.branch;
    const options=groups.filter((g) => state.branch==='all'||g.branchId===state.branch);
    $('#group').innerHTML='<option value="all">Все группы</option>'+options.map((g) => `<option value="${g.id}">${state.branch==='all'?`${g.branchName} · `:''}${g.name}</option>`).join('');
    $('#group').value=state.group;
    $$('[data-period]').forEach((button) => button.setAttribute('aria-pressed',String(button.dataset.period===state.period)));
    $$('[data-idea]').forEach((a) => { if(a.dataset.idea===state.idea)a.setAttribute('aria-current','page');else a.removeAttribute('aria-current'); });
    $('#concept-description').textContent=state.idea==='overview'?'Посещаемость и полнота данных.':'Сравнение филиалов и групп.';
    $('#from').value=state.from; $('#until').value=state.until;
  }
  function dateLabel() {
    const from=new Date(`${state.from}T12:00:00Z`), until=new Date(`${state.until}T12:00:00Z`);
    const formatter=new Intl.DateTimeFormat('ru-RU',{day:'numeric',month:'long',year:'numeric',timeZone:'Europe/Minsk'});
    if(state.from===state.until)return formatter.format(from).replace(' г.','');
    if(state.from.slice(0,7)===state.until.slice(0,7))return `${from.getUTCDate()}–${formatter.format(until).replace(' г.','')}`;
    return `${formatter.format(from).replace(' г.','')} — ${formatter.format(until).replace(' г.','')}`;
  }
  function context() {
    const branch=branches.find((b) => b.id===state.branch);
    const group=groups.find((g) => g.id===state.group);
    return `${branch?branch.name:'Вся сеть'}${group?` · ${group.name}`:''}`;
  }
  function summary(total) {
    return `<div class="report-context"><h2>${context()}<span class="context-date">${dateLabel()}</span></h2><details class="context-info"><summary aria-label="Контекст отчёта">${info}<span>Контекст отчёта</span></summary><p>Филиал выбран только для этого отчёта. Общий рабочий филиал приложения не меняется.</p></details></div>
    <section class="summary-panel" aria-label="Итоги посещаемости"><dl class="metrics">
    <div class="metric"><dt>Посещаемость</dt><dd class="rate">${percent(rate(total))}<span class="metric-note">${number(total.present+total.absent)} отметок в закрытых журналах</span></dd></div>
    <div class="metric"><dt>Проведено занятий</dt><dd>${number(total.closed)}<span class="metric-note">${number(total.planned)} запланировано</span></dd></div>
    <div class="metric"><dt>Присутствия</dt><dd>${number(total.present)}<span class="metric-note">посещения, не спортсмены</span></dd></div>
    <div class="metric"><dt>Пропуски</dt><dd>${number(total.absent)}<span class="metric-note">${rate(total)===null?'нет отметок':percent(100-rate(total))+' от всех отметок'}</span></dd></div>
    </dl><div class="summary-meta"><span>Только закрытые занятия · отменённые исключены</span>${total.open?`<button type="button" data-show-open>${info}<strong>${total.open}</strong> ${plural(total.open,['журнал не вошёл','журнала не вошли','журналов не вошли'])} в расчёт ${arrow}</button>`:'<span>Открытых журналов нет</span>'}</div></section>`;
  }
  function rowSub(row) {
    return `${sessions(row.closed)}${row.open?` · ${row.open} открыто`:''}${rate(row)===null?' · нет отметок':''}`;
  }
  function reasons(row) {
    return `<section class="panel reason-panel" aria-labelledby="reasons-title"><div class="section-head"><div><h2 id="reasons-title">Почему пропускают</h2><p>${number(row.absent)} пропусков в выбранном контексте</p></div></div><div class="reason-bar" aria-hidden="true">${['illness','excused','unknown'].map((key) => `<span style="width:${row.absent?row[key]/row.absent*100:0}%"></span>`).join('')}</div><dl class="reason-list">${[['illness','Болезнь'],['excused','Уважительная'],['unknown','Неизвестна']].map(([key,label]) => `<div><dt>${label}</dt><dd>${number(row[key])}<span>${row.absent?Math.round(row[key]/row.absent*100)+'%':'—'}</span></dd></div>`).join('')}</dl><p>${row.unknown?'«Неизвестна» также включает неотмеченных при закрытии журнала.':row.absent?'Причина указана у всех пропусков.':'В выбранном контексте пропусков нет.'}</p></section>`;
  }
  function findings(total, rows) {
    const low=rows.filter((r) => rate(r)!==null&&rate(r)<80).sort((a,b) => rate(a)-rate(b));
    const open=rows.filter((r) => r.open).sort((a,b) => b.open-a.open);
    return `<section class="panel findings-panel" aria-labelledby="findings-title"><div class="section-head"><h2 id="findings-title">На что обратить внимание</h2></div>
    ${open.length?`<div class="finding"><div class="finding-head"><span>Неполные данные</span><span class="badge">${total.open} ${plural(total.open,['журнал','журнала','журналов'])}</span></div><p>${open.map((r) => `${r.name} — ${r.open}`).join('; ')}. Эти занятия не входят в процент.</p><button type="button" data-show-open>Показать эти филиалы ${arrow}</button></div>`:''}
    ${low.length?`<div class="finding"><div class="finding-head"><span>Посещаемость ниже 80%</span><span class="badge">${low.length} из ${rows.filter((r) => rate(r)!==null).length}</span></div><p>${low.map((r) => `${r.name} — ${percent(rate(r))}`).join('; ')}.</p><button type="button" data-show-low>Сравнить эти филиалы ${arrow}</button><p class="help">80% — ориентир этого макета, не норматив клуба.</p></div>`:''}
    ${rate(total)===null?'<div class="finding"><div class="finding-head">Пока нечего оценивать</div><p>В этом контексте ещё нет отметок закрытых занятий. Процент появится после заполнения и закрытия журналов.</p></div>':!open.length&&!low.length?`<div class="finding"><div class="finding-head">Отклонений по выбранным признакам нет</div><p>Открытых журналов нет, посещаемость филиалов с отметками не ниже 80%. Это ориентир макета.</p></div>`:''}</section>`;
  }
  function detail(row,isGroup=false) {
    const selected=filteredGroups().filter((g) => isGroup?g.id===row.id:g.branchId===row.id);
    return `<div class="detail-block"><h3>${isGroup?'Показатели группы':'Группы филиала'}</h3><div class="detail-facts"><span>Присутствия <strong>${number(row.present)}</strong></span><span>Пропуски <strong>${number(row.absent)}</strong></span><span>Запланировано <strong>${row.planned}</strong></span><span>Открыто <strong>${row.open}</strong></span></div>${!isGroup?selected.map((g) => `<div class="group-line"><button class="text-button" type="button" data-select-group="${g.id}">${g.name} ${arrow}</button><span class="rate">${percent(rate(g))}</span><span class="muted">${sessions(g.closed)}</span></div>`).join(''):''}<p class="detail-reasons">Пропуски: болезнь — ${row.illness}; уважительная — ${row.excused}; неизвестна — ${row.unknown}.</p>${row.open?'<p class="detail-reasons">Открытые журналы показаны счётчиком. Работа с журналом в этом прототипе не реализована.</p>':''}</div>`;
  }
  function ranking(rows) {
    const sorted=[...rows].sort((a,b) => state.ranking==='open'?b.open-a.open:(rate(a)??Infinity)-(rate(b)??Infinity));
    return `<section class="panel ranking-panel" aria-labelledby="ranking-title"><div class="section-head"><div><h2 id="ranking-title">Посещаемость по филиалам</h2><p>Раскройте строку, чтобы увидеть группы</p></div><label class="sr-label"><span class="visually-hidden">Порядок филиалов</span><select id="ranking-order"><option value="rate" ${state.ranking==='rate'?'selected':''}>Сначала ниже</option><option value="open" ${state.ranking==='open'?'selected':''}>Сначала открытые</option></select></label></div><div class="rank-header" aria-hidden="true"><span>Филиал</span><span>Посещаемость</span><span>Занятия</span><span>Открыто</span><span></span></div>
    ${sorted.map((row) => `<div class="rank-row ${rate(row)!==null&&rate(row)<80?'is-low':''}"><div><span class="branch-name">${row.name}</span><span class="branch-sub">${rowSub(row)}</span></div><div class="rank-rate"><span class="rate">${percent(rate(row))}</span><div class="bar-track" aria-hidden="true"><div class="bar-fill" style="width:${rate(row)??0}%"></div></div></div><span class="count numeric">${row.closed}</span><span class="count numeric ${row.open?'open-count':'muted'}">${row.open||'—'}</span><button class="row-open" type="button" data-expand="${row.id}" aria-label="Группы: ${row.name}" aria-expanded="${state.expanded.has(row.id)}" aria-controls="detail-${row.id}">${arrow}</button></div><div id="detail-${row.id}" ${state.expanded.has(row.id)?'':'hidden'}>${detail(row)}</div>`).join('')}<p class="column-note">«—» в посещаемости: нет отметок. Филиалы без проведённых занятий сохранены.</p></section>`;
  }
  function overview(total) {
    const rows=branchRows();
    return `<div class="overview-grid">${ranking(rows)}<div class="right-column">${findings(total,rows)}${reasons(total)}</div></div>`;
  }
  function visibleRows() {
    return reportRows().filter((r) => (state.issue==='all'||(state.issue==='open'?r.open>0:rate(r)!==null&&rate(r)<80)) && `${r.name} ${r.branchName??''}`.toLocaleLowerCase('ru').includes(state.search.toLocaleLowerCase('ru'))).sort((a,b) => {
      if(state.sort==='name')return state.direction*a.name.localeCompare(b.name,'ru');
      if(state.sort==='rate') {
        if(rate(a)===null)return rate(b)===null?0:1;
        if(rate(b)===null)return -1;
        return state.direction*(rate(a)-rate(b));
      }
      return state.direction*(a[state.sort]-b[state.sort]);
    });
  }
  function tableShell() {
    return `<section class="panel table-panel" aria-label="Сравнение филиалов и групп"><div class="table-toolbar"><div class="table-tabs" role="group" aria-label="Сравнивать"><button type="button" data-kind="branches" aria-pressed="${state.kind==='branches'}">Филиалы <span class="help">${branchRows().length}</span></button><button type="button" data-kind="groups" aria-pressed="${state.kind==='groups'}">Группы <span class="help">${filteredGroups().length}</span></button></div><label class="search-label"><span class="visually-hidden">Поиск по названию</span><svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/></svg><input id="search" type="search" placeholder="Найти филиал или группу" value="${escape(state.search)}"></label></div><div class="table-tools"><div class="table-filters" role="group" aria-label="Отбор по проблеме">${[['all','Все'],['low','Ниже 80%'],['open','Открытые журналы']].map(([id,label]) => `<button type="button" data-issue="${id}" aria-pressed="${state.issue===id}">${label} <span>${reportRows().filter((r) => id==='all'||(id==='open'?r.open>0:rate(r)!==null&&rate(r)<80)).length}</span></button>`).join('')}</div><span class="table-count" id="table-count" role="status"></span></div><div id="table-content"></div><p class="column-note">Присутствия и пропуски — отметки в закрытых журналах. Причины пропусков и группы — внутри строки. 80% — ориентир макета.</p></section>`;
  }
  function emptySearch() {
    return '<div class="empty-state"><h2>Нет подходящих строк</h2><p>Измените запрос или отключите отбор по проблеме. Период и филиал останутся прежними.</p><button type="button" class="text-button" data-clear-search>Сбросить поиск и отбор</button></div>';
  }
  function renderTableContent() {
    const target=$('#table-content'); if(!target)return;
    const rows=visibleRows(), isGroup=state.kind==='groups';
    $('#table-count').textContent=`Показано ${rows.length} из ${reportRows().length}`;
    if(!rows.length) { target.innerHTML=emptySearch(); return; }
    const headers=[['name',isGroup?'Группа / филиал':'Филиал'],['rate','Посещаемость'],['closed','Занятия'],['present','Присутствия'],['absent','Пропуски'],['planned','План'],['open','Открыто']];
    target.innerHTML=`<div class="table-scroll" tabindex="0" role="region" aria-label="Сравнительная таблица"><table><caption class="visually-hidden">${isGroup?'Группы':'Филиалы'} · ${dateLabel()} · ${context()}</caption><thead><tr>${headers.map(([key,label]) => `<th scope="col" ${state.sort===key?`aria-sort="${state.direction===1?'ascending':'descending'}"`:''}><button type="button" data-sort="${key}">${label} ${state.sort===key?sortIcon:''}</button></th>`).join('')}</tr></thead><tbody>${rows.map((row) => `<tr class="data-row ${rate(row)!==null&&rate(row)<80?'is-low':''}"><th scope="row"><div class="table-name"><button class="row-open" type="button" data-expand="${row.id}" aria-label="Детали: ${row.name}${isGroup?' · '+row.branchName:''}" aria-expanded="${state.expanded.has(row.id)}" aria-controls="detail-${row.id}">${arrow}</button><div><span class="branch-name">${row.name}</span>${isGroup?`<small>${row.branchName}</small>`:rate(row)===null?'<small>Нет отметок</small>':''}</div></div></th><td><div class="table-rate"><div class="bar-track" aria-hidden="true"><div class="bar-fill" style="width:${rate(row)??0}%"></div></div><span class="rate">${percent(rate(row))}</span></div></td>${['closed','present','absent','planned','open'].map((key) => `<td class="${key==='open'&&row.open?'open-count':''}">${number(row[key])}</td>`).join('')}</tr><tr class="table-detail" id="detail-${row.id}" ${state.expanded.has(row.id)?'':'hidden'}><td colspan="7">${detail(row,isGroup)}</td></tr>`).join('')}</tbody></table></div>
    <div class="table-mobile"><div class="mobile-sort"><label for="mobile-sort">Сортировка</label><select id="mobile-sort">${headers.map(([key,label]) => `<option value="${key}" ${state.sort===key?'selected':''}>${label}${state.sort===key?(state.direction===1?' · по возрастанию':' · по убыванию'):''}</option>`).join('')}</select><button class="row-open" type="button" data-toggle-direction aria-label="Изменить направление сортировки">${sortIcon}</button></div>${rows.map((row) => `<article class="mobile-row ${rate(row)!==null&&rate(row)<80?'is-low':''}"><div class="mobile-row-head"><div><h3>${row.name}</h3>${isGroup?`<p class="help">${row.branchName}</p>`:''}</div><span class="rate">${percent(rate(row))}</span></div><div class="mobile-row-facts"><span>Занятия <strong>${row.closed}</strong></span><span>Присутствия <strong>${number(row.present)}</strong></span><span>Пропуски <strong>${row.absent}</strong></span><span>План <strong>${row.planned}</strong></span><span class="${row.open?'open-count':''}">Открыто <strong>${row.open}</strong></span></div><button class="text-button" type="button" data-expand="${row.id}" aria-expanded="${state.expanded.has(row.id)}" aria-controls="mobile-detail-${row.id}">Детали ${arrow}</button><div id="mobile-detail-${row.id}" ${state.expanded.has(row.id)?'':'hidden'}>${detail(row,isGroup)}</div></article>`).join('')}</div>`;
  }
  function render() {
    syncFilters();
    const total=sum(filteredGroups());
    const noSessions=!total.closed&&!total.open&&!total.planned;
    $('#report').innerHTML=summary(total)+(noSessions?'<section class="panel empty-state"><h2>В этом периоде занятий нет</h2><p>Попробуйте другой период, филиал или группу. В макете есть данные за 1–20 октября 2026.</p><button class="button secondary" type="button" data-reset>Вернуть обзор октября</button></section>':state.idea==='overview'?overview(total):tableShell());
    if(state.idea==='table'&&!noSessions)renderTableContent();
  }
  function chooseIdea(idea) {
    state.idea=idea; state.expanded.clear();
    const url=new URL(location.href); url.searchParams.set('idea',idea); history.replaceState({},'',url);
    render();
  }
  function showProblem(issue) {
    state.issue=issue;state.search='';state.kind='branches';state.sort=issue==='open'?'open':'rate';state.direction=issue==='open'?-1:1;
    chooseIdea('table');
    const first=$(`[data-issue="${issue}"]`); if(first)first.focus();
    notify(issue==='open'?'Показаны филиалы с открытыми журналами':'Показаны филиалы с посещаемостью ниже 80%');
  }
  document.addEventListener('click',(event) => {
    const button=event.target.closest('button,a'); if(!button)return;
    if(button.matches('[data-idea]')) { event.preventDefault();chooseIdea(button.dataset.idea); }
    else if(button.matches('[data-period]')) {
      const period=button.dataset.period;
      if(period==='custom') { const open=$('#custom-range').hidden; $('#custom-range').hidden=!open;button.setAttribute('aria-expanded',String(open));if(open)$('#from').focus();return; }
      state.period=period;state.from=period==='today'?'2026-10-20':period==='week'?'2026-10-19':'2026-10-01';state.until='2026-10-20';state.expanded.clear();$('#custom-range').hidden=true;$('[data-period="custom"]').setAttribute('aria-expanded','false');$('#date-error').hidden=true;render();
    }
    else if(button.matches('[data-expand]')) {
      const id=button.dataset.expand;
      if(state.expanded.has(id))state.expanded.delete(id);else state.expanded.add(id);
      $$(`[data-expand="${id}"]`).forEach((b) => b.setAttribute('aria-expanded',String(state.expanded.has(id))));
      $$(`#detail-${id},#mobile-detail-${id}`).forEach((detail) => {detail.hidden=!state.expanded.has(id);});
    }
    else if(button.matches('[data-select-group]')) {
      const group=groups.find((g) => g.id===button.dataset.selectGroup);state.branch=group.branchId;state.group=group.id;state.expanded.clear();state.issue='all';state.search='';render();$('#report').focus();notify(`Отчёт: ${group.branchName} · ${group.name}`);
    }
    else if(button.matches('[data-show-open]'))showProblem('open');
    else if(button.matches('[data-show-low]'))showProblem('low');
    else if(button.matches('[data-kind]')) { state.kind=button.dataset.kind;state.expanded.clear();render();$(`[data-kind="${state.kind}"]`).focus(); }
    else if(button.matches('[data-issue]')) { state.issue=button.dataset.issue;render();$(`[data-issue="${state.issue}"]`).focus(); }
    else if(button.matches('[data-sort]')) { const key=button.dataset.sort;state.direction=state.sort===key?-state.direction:1;state.sort=key;renderTableContent();$(`[data-sort="${key}"]`).focus(); }
    else if(button.matches('[data-toggle-direction]')) {state.direction=-state.direction;renderTableContent();$('[data-toggle-direction]').focus();}
    else if(button.matches('[data-clear-search]')) {state.search='';state.issue='all';render();$('#search').focus();}
    else if(button.matches('[data-reset]')) {Object.assign(state,{period:'month',from:'2026-10-01',until:'2026-10-20',branch:'all',group:'all',issue:'all',search:''});render();$('#report').focus();}
    else if(button.matches('[data-demo-error]')) {$('#report').innerHTML='<section class="panel empty-state" role="alert"><h2>Не удалось загрузить статистику</h2><p>Проверьте соединение и повторите попытку. Выбранные период, филиал и группа сохранены. Итоги не подменены нулями.</p><button class="button primary" type="button" data-retry>Повторить загрузку</button></section>';$('#report').focus();}
    else if(button.matches('[data-retry]')) {render();$('#report').focus();notify('Демонстрационный отчёт восстановлен');}
    else if(button.matches('[data-demo-empty]')) {state.period='custom';state.from='2026-11-01';state.until='2026-11-20';render();$('#report').focus();}
  });
  $('#branch').addEventListener('change',(event) => {state.branch=event.target.value;state.group='all';state.expanded.clear();state.search='';state.issue='all';render();notify('Контекст отчёта обновлён');});
  $('#group').addEventListener('change',(event) => {state.group=event.target.value;if(state.group!=='all')state.branch=groups.find((g) => g.id===state.group).branchId;state.expanded.clear();state.search='';state.issue='all';render();notify('Группа отчёта обновлена');});
  document.addEventListener('change',(event) => {
    if(event.target.id==='ranking-order') {state.ranking=event.target.value;render();$('#ranking-order').focus();}
    if(event.target.id==='mobile-sort') {state.sort=event.target.value;state.direction=1;renderTableContent();$('#mobile-sort').focus();}
  });
  document.addEventListener('input',(event) => {if(event.target.id==='search'){state.search=event.target.value;renderTableContent();}});
  $('#custom-range').addEventListener('submit',(event) => {
    event.preventDefault();const from=$('#from').value,until=$('#until').value;
    if(from>until) {$('#date-error').textContent='Дата начала позже даты окончания. Измените одну из дат.';$('#date-error').hidden=false;$('#from').setAttribute('aria-invalid','true');$('#from').setAttribute('aria-describedby','date-error');$('#from').focus();return;}
    $('#from').removeAttribute('aria-invalid');$('#from').removeAttribute('aria-describedby');$('#date-error').hidden=true;Object.assign(state,{period:'custom',from,until});state.expanded.clear();$('#custom-range').hidden=true;$('[data-period="custom"]').setAttribute('aria-expanded','false');render();$('[data-period="custom"]').focus();notify('Период применён');
  });
  $('#refresh').addEventListener('click',() => {
    const button=$('#refresh');button.disabled=true;button.setAttribute('aria-label','Обновление статистики');$('#report').setAttribute('aria-busy','true');
    clearTimeout(refreshTimer);refreshTimer=setTimeout(() => {button.disabled=false;button.setAttribute('aria-label','Обновить статистику');$('#report').removeAttribute('aria-busy');notify('Демонстрационные данные обновлены. Фильтры сохранены.');},500);
  });
  render();
})();

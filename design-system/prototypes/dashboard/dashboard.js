'use strict';

// All people, branches, rosters and report figures below are synthetic.
// This prototype has no API calls, authentication, persistent data or app imports.
const params = new URLSearchParams(location.search);
const role = ['owner', 'admin', 'trainer'].includes(params.get('role')) ? params.get('role') : 'owner';
const roleNames = { owner: 'Владелец сети', admin: 'Администратор', trainer: 'Тренер' };
const personas = { owner: ['Алексей Волков', 'АВ'], admin: ['Мария Соколова', 'МС'], trainer: ['Илья Морозов', 'ИМ'] };
const clubs = [
  { id: 'central', name: 'Центральный', members: 72, trainers: 3 },
  { id: 'south', name: 'Южный', members: 58, trainers: 3 },
  { id: 'north', name: 'Северный', members: 44, trainers: 2 },
];
const allowedClubs = role === 'owner' ? clubs : role === 'admin' ? clubs.slice(0, 2) : clubs.slice(0, 1);
const groups = [
  ['g1', 'central', 'Начальная · 6–8 лет', 18, 90, 12, 7],
  ['g2', 'central', 'Базовая · 9–11 лет', 20, 80, 16, 6],
  ['g3', 'central', 'Старшая · 12–15 лет', 18, 66, 10, 5],
  ['g4', 'central', 'Взрослые', 16, 48, 18, 4],
  ['g5', 'south', 'Начальная · 6–8 лет', 20, 84, 14, 7],
  ['g6', 'south', 'Базовая · 9–11 лет', 20, 70, 12, 6],
  ['g7', 'south', 'Старшая · 12–15 лет', 18, 52, 18, 5],
  ['g8', 'north', 'Начальная · 6–8 лет', 16, 62, 14, 5],
  ['g9', 'north', 'Базовая · 9–11 лет', 16, 54, 16, 5],
  ['g10', 'north', 'Старшая · 12–15 лет', 12, 42, 12, 4],
].map(([id, club, name, size, present, absent, closed]) => ({ id, club, name, size, present, absent, closed }));
const sessions = [
  { id: 's1', group: 'g5', time: '14:30', end: '15:30', trainer: 'Денис Орлов', status: 'closed' },
  { id: 's2', group: 'g1', time: '16:00', end: '17:00', trainer: 'Илья Морозов', status: 'open' },
  { id: 's3', group: 'g2', time: '18:00', end: '19:30', trainer: 'Илья Морозов', status: 'planned' },
  { id: 's4', group: 'g6', time: '18:30', end: '20:00', trainer: 'Денис Орлов', status: 'planned' },
  { id: 's5', group: 'g9', time: '19:00', end: '20:00', trainer: 'Анна Белова', status: 'planned' },
  { id: 's6', group: 'g3', time: '19:30', end: '21:00', trainer: 'Виктор Лебедев', status: 'cancelled' },
];
const names = ['Артём Соколов', 'Максим Ковалёв', 'София Орлова', 'Матвей Волков', 'Анна Белова', 'Егор Новиков', 'Алиса Петрова', 'Михаил Зайцев', 'Дарья Лебедева', 'Иван Морозов', 'Полина Семёнова', 'Тимофей Козлов', 'Варвара Павлова', 'Александр Васильев', 'Ева Фёдорова', 'Даниил Смирнов', 'Ксения Никитина', 'Роман Алексеев'];
const participants = names.map((name, i) => ({ id: `p${i + 1}`, name, group: 'g1', club: 'central', born: `${String(10 + i).padStart(2, '0')}.03.2019`, joined: '01.09.2026', contact: 'Демонстрационный контакт родителя' }));
participants.push({ id: 'p19', name: 'Андрей Громов', group: 'g5', club: 'south', born: '14.06.2018', joined: '05.09.2026', contact: 'Демонстрационный контакт родителя' });
const reasonNames = { unknown: 'Неизвестна', illness: 'Болезнь', excused: 'Уважительная' };
const journalRecords = new Map();
for (const s of sessions) {
  const group = groups.find(g => g.id === s.group);
  journalRecords.set(s.id, Array.from({ length: group.size }, (_, i) => ({
    id: `r-${s.id}-${i}`, name: names[i % names.length], code: `PR-${String(i + 1).padStart(4, '0')}`, born: `${String(10 + i).padStart(2, '0')}.03.${s.group === 'g1' ? '2019' : '2016'}`,
    mark: s.status === 'closed' ? (i < 16 ? 'present' : 'illness') : s.status === 'open' ? (i < 6 ? 'present' : i === 6 ? 'illness' : null) : null,
  })));
}
const state = {
  club: role === 'owner' ? 'all' : allowedClubs[0].id, group: 'all', period: 'week',
  view: 'overview', day: 12, journal: null, record: 0, busy: false, search: '',
  scheduleGroup: 'all',
  scenario: ['normal', 'loading', 'empty', 'error'].includes(params.get('state')) ? params.get('state') : 'normal',
  offline: false, saveError: '',
};
const initialJournals = new Map(sessions.map(s => [s.id, {
  closed: s.status === 'closed',
  present: journalRecords.get(s.id).filter(r => r.mark === 'present').length,
  absent: journalRecords.get(s.id).filter(r => r.mark && r.mark !== 'present').length,
  reasons: Object.fromEntries(['illness', 'excused', 'unknown'].map(key => [key, journalRecords.get(s.id).filter(r => r.mark === key).length])),
}]));
const main = document.querySelector('#main');
const sheet = document.querySelector('#sheet');
let sheetOpener = null;
let toastTimer;
const paths = {
  home: '<path d="m3 10 9-7 9 7v10H3z"/><path d="M9 20v-7h6v7"/>',
  calendar: '<rect x="3" y="5" width="18" height="16" rx="2"/><path d="M7 3v4m10-4v4M3 11h18"/>',
  people: '<circle cx="9" cy="8" r="3"/><path d="M3 21v-3a6 6 0 0 1 12 0v3m2-16a3 3 0 0 1 0 6m2 3a5 5 0 0 1 3 4v3"/>',
  groups: '<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>',
  journal: '<path d="M6 3h13v18H6a3 3 0 0 1-3-3V6a3 3 0 0 1 3-3zM3 17h16M8 7h7m-7 4h5"/>',
  chart: '<path d="M4 3v17h17M8 15v-4m5 4V6m5 9V9"/>',
  history: '<path d="M3 11a9 9 0 1 1 2 7M3 4v7h7m2-5v6l4 2"/>',
  location: '<path d="M19 10c0 6-7 11-7 11S5 16 5 10a7 7 0 0 1 14 0z"/><circle cx="12" cy="10" r="2"/>',
  arrow: '<path d="M5 12h14m-5-5 5 5-5 5"/>',
  chevron: '<path d="m9 5 7 7-7 7"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  close: '<path d="m6 6 12 12M6 18 18 6"/>',
  check: '<path d="m5 12 4 4L19 6"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  more: '<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
  search: '<circle cx="10" cy="10" r="6"/><path d="m15 15 6 6"/>',
  alert: '<path d="m12 3 10 18H2zM12 9v5m0 3v1"/>',
  back: '<path d="M19 12H5m5-5-5 5 5 5"/>',
  wifi: '<path d="M3 8a14 14 0 0 1 18 0M6 12a9 9 0 0 1 12 0m-9 4a4 4 0 0 1 6 0m-3 4h.01"/>',
};
const icon = name => `<svg class="icon" viewBox="0 0 24 24" aria-hidden="true">${paths[name] || paths.groups}</svg>`;
const esc = value => String(value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
const clubName = id => clubs.find(c => c.id === id)?.name ?? 'Все филиалы';
const groupFor = s => groups.find(g => g.id === s.group);
const accessibleGroups = () => groups.filter(g => allowedClubs.some(c => c.id === g.club) && (role !== 'trainer' || ['g1', 'g2'].includes(g.id)) && (state.club === 'all' || g.club === state.club));
const visibleSessions = () => sessions.filter(s => accessibleGroups().some(g => g.id === s.group));
const rate = (present, absent) => present + absent ? `${(present / (present + absent) * 100).toFixed(1).replace('.', ',')}%` : '—';
const number = n => n.toLocaleString('ru-RU');
const plural = (n, one, few, many) => n % 10 === 1 && n % 100 !== 11 ? one : n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 12 || n % 100 > 14) ? few : many;
const status = value => `<span class="status ${value}">${{ planned: 'Запланировано', open: 'Журнал открыт', closed: 'Завершено', cancelled: 'Отменено' }[value]}</span>`;
const link = (view, text) => `<a class="text-link" href="#${view}">${text}${icon('arrow')}</a>`;
const btn = (action, text, type = '', extra = '') => `<button type="button" class="btn ${type}" data-action="${action}" ${extra}>${text}</button>`;
const periodText = () => ({ day: '12 октября 2026', week: '6–12 октября 2026', month: '1–12 октября 2026' }[state.period]);
const scoped = () => accessibleGroups().filter(g => state.group === 'all' || g.id === state.group);
function reportRows() {
  const factor = state.period === 'month' ? 1.7 : 1;
  return scoped().map(g => {
    const row = { ...g, present: state.period === 'day' ? 0 : Math.round(g.present * factor), absent: state.period === 'day' ? 0 : Math.round(g.absent * factor), closed: state.period === 'day' ? 0 : Math.round(g.closed * factor) };
    row.illness = Math.round(row.absent * .55);
    row.excused = Math.round(row.absent * .3);
    row.unknown = row.absent - row.illness - row.excused;
    for (const s of sessions.filter(s => s.group === g.id && s.status === 'closed')) {
      const records = journalRecords.get(s.id), initial = initialJournals.get(s.id);
      const present = records.filter(r => r.mark === 'present').length;
      const absent = records.filter(r => r.mark && r.mark !== 'present').length;
      row.present += present - (state.period !== 'day' && initial.closed ? initial.present : 0);
      row.absent += absent - (state.period !== 'day' && initial.closed ? initial.absent : 0);
      row.closed += state.period === 'day' || !initial.closed ? 1 : 0;
      for (const key of ['illness', 'excused', 'unknown']) row[key] += records.filter(r => r.mark === key).length - (state.period !== 'day' && initial.closed ? initial.reasons[key] : 0);
    }
    return row;
  });
}
function totals() {
  return reportRows().reduce((r, g) => ({ present: r.present + g.present, absent: r.absent + g.absent, closed: r.closed + g.closed }), { present: 0, absent: 0, closed: 0 });
}
function navItems() {
  const items = [['overview', 'home', 'Главная'], ['schedule', 'calendar', 'Занятия']];
  if (role !== 'trainer') items.push(['participants', 'people', 'Участники']);
  items.push(['groups', 'groups', role === 'trainer' ? 'Мои группы' : 'Группы'], ['journals', 'journal', 'Журналы']);
  if (role === 'owner') items.push(['statistics', 'chart', 'Статистика']);
  if (role !== 'trainer') items.push(['staff', 'people', 'Сотрудники']);
  if (role === 'owner') items.push(['clubs', 'location', 'Филиалы']);
  if (role !== 'trainer') items.push(['audit', 'history', 'История изменений']);
  return items;
}
function renderNav() {
  document.querySelector('#desktop-nav').innerHTML = navItems().map(([id, ico, name], i) => `<a class="nav-link ${i > 4 && id === 'staff' ? 'nav-gap' : ''}" href="#${id}" ${state.view === id ? 'aria-current="page"' : ''}>${icon(ico)}${name}</a>`).join('');
  const mobile = role === 'owner' ? ['overview', 'schedule', 'statistics'] : role === 'admin' ? ['overview', 'schedule', 'participants'] : ['overview', 'schedule', 'groups'];
  document.querySelector('#mobile-nav').innerHTML = mobile.map(id => {
    const [, ico, name] = navItems().find(item => item[0] === id);
    return `<a href="#${id}" ${state.view === id ? 'aria-current="page"' : ''}>${icon(ico)}${id === 'overview' ? 'Главная' : name}</a>`;
  }).join('') + `<button data-action="more" ${!mobile.includes(state.view) ? 'aria-current="page"' : ''}>${icon('more')}Ещё</button>`;
}
function heading(title, description, action = '') {
  return `<div class="page-heading"><div><h1>${title}</h1><p>${description}</p></div>${action}</div>`;
}
function reportFilters() {
  return `<div class="filters"><span class="filter-label">Период</span><div class="segmented" aria-label="Период отчёта">${[['day', 'Сегодня'], ['week', 'Неделя'], ['month', 'Месяц']].map(([id, name]) => `<button data-period="${id}" aria-pressed="${state.period === id}">${name}</button>`).join('')}</div><select id="group-filter" aria-label="Группа отчёта"><option value="all">Все группы</option>${accessibleGroups().map(g => `<option value="${g.id}" ${state.group === g.id ? 'selected' : ''}>${g.name}${state.club === 'all' ? ` · ${clubName(g.club)}` : ''}</option>`).join('')}</select><span class="period-label">${periodText()}</span></div>`;
}
function statsStrip(items) {
  return `<dl class="stats-strip">${items.map(([label, value, note]) => `<div class="stat"><dt>${label}</dt><dd>${value}</dd><small>${note}</small></div>`).join('')}</dl>`;
}
function networkPanel() {
  const rows = reportRows();
  const branches = allowedClubs.filter(c => rows.some(g => g.club === c.id)).map(c => ({ ...c, ...rows.filter(g => g.club === c.id).reduce((r, g) => ({ present: r.present + g.present, absent: r.absent + g.absent, closed: r.closed + g.closed, groups: r.groups + 1 }), { present: 0, absent: 0, closed: 0, groups: 0 }) }));
  const label = state.group === 'all' ? 'Посещаемость по филиалам' : 'Посещаемость выбранной группы';
  return `<section class="panel"><div class="panel-header"><div><h2>${label}</h2><p>${periodText()} · только завершённые журналы</p></div></div><div class="network-table table-wrap"><table><thead><tr><th>Филиал</th><th>Занятия</th><th>Присутствия</th><th>Пропуски</th><th>Посещаемость</th></tr></thead><tbody>${branches.map(c => `<tr><td><button class="table-button" data-club="${c.id}">${c.name}</button><small>${c.groups} ${c.groups === 1 ? 'группа' : 'группы'}</small></td><td>${c.closed}</td><td>${c.present}</td><td>${c.absent}</td><td><span class="rate"><span class="rate-track" aria-hidden="true"><span class="rate-fill" style="width:${(c.present + c.absent ? c.present / (c.present + c.absent) * 100 : 0)}%"></span></span><strong>${rate(c.present, c.absent)}</strong></span></td></tr>`).join('')}</tbody></table></div><div class="mobile-network">${branches.map(c => `<div class="network-row"><div><button class="table-button" data-club="${c.id}">${c.name}</button><strong>${rate(c.present, c.absent)}</strong></div><p>${c.closed} занятий · ${c.present} присутствий · ${c.absent} пропусков</p></div>`).join('')}</div><p class="panel-footnote">Доля присутствий среди всех отметок. Отменённые занятия не учитываются.</p></section>`;
}
function splitPanel() {
  const t = totals();
  const { illness, excused, unknown } = reportRows().reduce((r, g) => ({ illness: r.illness + g.illness, excused: r.excused + g.excused, unknown: r.unknown + g.unknown }), { illness: 0, excused: 0, unknown: 0 });
  return `<section class="panel"><div class="panel-header"><div><h2>Присутствия и пропуски</h2><p>Отметки за ${periodText().toLowerCase()}</p></div></div><div class="panel-body"><div class="attendance-split" aria-hidden="true"><span class="split-present" style="flex:${t.present}"></span><span class="split-absent" style="flex:${t.absent}"></span></div><div class="legend"><span><i class="dot present"></i>Присутствия <strong>${number(t.present)}</strong></span><span><i class="dot absent"></i>Пропуски <strong>${number(t.absent)}</strong></span></div><div class="reason-list"><div class="reason-row"><span>Болезнь</span><strong>${illness}</strong></div><div class="reason-row"><span>Уважительная причина</span><strong>${excused}</strong></div><div class="reason-row"><span>Причина неизвестна</span><strong>${unknown}</strong></div></div></div></section>`;
}
function agendaPanel() {
  const list = visibleSessions().filter(s => s.status !== 'cancelled');
  return `<section class="panel"><div class="panel-header"><div><h2>Сегодня</h2><p>12 октября · ${list.length} ${plural(list.length, 'занятие', 'занятия', 'занятий')}</p></div>${link('schedule', 'Все')}</div><ol class="agenda">${list.slice(0, 4).map(s => {
    const g = groupFor(s);
    return `<li><div class="agenda-top"><time>${s.time}</time>${status(s.status)}</div><h3>${g.name}</h3><p>${clubName(g.club)} · ${s.trainer}</p><button class="text-link table-button" data-session="${s.id}">${s.status === 'open' && role === 'trainer' ? 'Продолжить отметки' : 'Открыть занятие'}${icon('arrow')}</button></li>`;
  }).join('')}</ol></section>`;
}
function attentionPanel() {
  const open = visibleSessions().filter(s => s.status === 'open');
  if (!open.length) return '';
  return `<section class="notice">${icon('journal')}<div><strong>${open.length === 1 ? '1 журнал не завершён' : `${open.length} журнала не завершены`}</strong><p>Отметки войдут в статистику после завершения занятия.</p>${link('journals', 'Посмотреть журналы')}</div></section>`;
}
function activityPanel() {
  return `<section class="panel"><div class="panel-header"><h2>Последние изменения</h2>${link('audit', 'История')}</div><ul class="activity-list"><li>${icon('journal')}<div><strong>Завершён журнал занятия</strong><p>Сегодня, 15:35 · ${clubName(state.club === 'all' ? 'south' : state.club)}</p></div><time>15:35</time></li><li>${icon('people')}<div><strong>Участник добавлен в группу</strong><p>Сегодня, 12:20 · ${clubName(state.club === 'all' ? 'central' : state.club)}</p></div><time>12:20</time></li><li>${icon('calendar')}<div><strong>Обновлено расписание группы</strong><p>Сегодня, 10:15 · ${clubName(state.club === 'all' ? 'central' : state.club)}</p></div><time>10:15</time></li></ul></section>`;
}
function overview() {
  if (role === 'owner') {
    const t = totals();
    const open = visibleSessions().filter(s => s.status === 'open').length;
    return heading('Обзор клуба', state.club === 'all' ? 'Вся сеть в одном рабочем пространстве' : `Филиал ${clubName(state.club)} · обзор за выбранный период`, `<a class="btn secondary" href="#statistics">${icon('chart')}Статистика</a>`) + reportFilters() + statsStrip([['Посещаемость', rate(t.present, t.absent), 'по завершённым журналам'], ['Проведено занятий', t.closed, periodText()], ['Присутствия', number(t.present), `${number(t.absent)} ${plural(t.absent, 'пропуск', 'пропуска', 'пропусков')}`], ['Открыто сегодня', open, 'пока вне статистики']]) + `<div class="workspace-grid"><div class="main-column">${networkPanel()}${activityPanel()}</div><div class="side-column">${agendaPanel()}${attentionPanel()}</div></div>`;
  }
  const list = visibleSessions();
  const next = list.find(s => s.status === 'open') || list.find(s => s.status === 'planned');
  const g = next && groupFor(next);
  const marked = next ? journalRecords.get(next.id).filter(r => r.mark).length : 0;
  const lead = next ? `<section class="panel next-session"><div class="next-session-top"><time class="session-time">${next.time}<span class="muted">–${next.end}</span></time>${status(next.status)}</div><h2>${g.name}</h2><p>${clubName(g.club)} · ${role === 'trainer' ? 'Ваше занятие' : next.trainer}</p><div class="session-facts"><span>${icon('people')}${g.size} участников</span><span>${icon('journal')}${marked} из ${g.size} отмечено</span></div>${btn('session', role === 'trainer' ? 'Продолжить отметки' : 'Открыть журнал', '', `data-id="${next.id}"`)}</section>` : empty('На сегодня занятий нет', 'Выберите другой день в расписании.', 'schedule');
  const metrics = role === 'admin' ? statsStrip([['Занятия сегодня', list.filter(s => s.status !== 'cancelled').length, 'в выбранном филиале'], ['Открытые журналы', list.filter(s => s.status === 'open').length, 'требуют завершения'], ['Участники', clubs.find(c => c.id === state.club).members, 'в активном реестре'], ['Группы', accessibleGroups().length, 'в выбранном филиале']]) : '';
  return heading(role === 'trainer' ? 'Ваш тренировочный день' : 'Рабочий день', `Понедельник, 12 октября · ${clubName(state.club)}`, role === 'admin' ? btn('add-participant', `${icon('plus')}Добавить участника`, 'secondary') : '') + `<div class="desktop-only">${metrics}</div><div class="workspace-grid"><div class="main-column">${lead}${role === 'admin' ? `<div class="mobile-only">${metrics}</div>` + dayPanel(list.filter(s => s.id !== next?.id), 'Остальные занятия') : groupsPanel()}</div><div class="side-column">${role === 'trainer' ? agendaPanel() : attentionPanel()}${role === 'admin' ? activityPanel() : `<section class="panel"><div class="panel-header"><h2>Как завершить занятие</h2></div><div class="panel-body"><p class="sheet-copy">Отметьте участников кнопками. Неотмеченные при завершении станут отсутствующими с неизвестной причиной.</p></div></section>`}</div></div>`;
}
function empty(title, copy, destination = 'overview') {
  const label = { overview: 'Перейти на главную', schedule: 'Перейти к занятиям', groups: 'Перейти к группам' }[destination] || 'Открыть раздел';
  return `<section class="empty-state">${icon('calendar')}<h2>${title}</h2><p>${copy}</p><a class="btn secondary" href="#${destination}">${label}</a></section>`;
}
function dayPanel(list, title = 'Занятия') {
  if (!list.length) return empty('На этот день занятий нет', 'Попробуйте выбрать другой день или филиал.', 'schedule');
  return `<section class="panel"><div class="panel-header"><h2>${title}</h2><span class="period-label">${list.length} всего</span></div><ol class="day-list">${list.map(s => { const g = groupFor(s); return `<li><div><time>${s.time}</time><small>до ${s.end}</small></div><div><h3>${g.name}</h3>${status(s.status)}<small>${clubName(g.club)} · ${s.trainer}</small></div>${btn('session', s.status === 'open' && role === 'trainer' ? 'Продолжить' : 'Открыть', 'secondary', `data-id="${s.id}"`)}</li>`; }).join('')}</ol></section>`;
}
function schedule() {
  const days = [['Пн', 12], ['Вт', 13], ['Ср', 14], ['Чт', 15], ['Пт', 16], ['Сб', 17], ['Вс', 18]];
  const filtered = visibleSessions().filter(s => state.scheduleGroup === 'all' || s.group === state.scheduleGroup);
  return heading(role === 'trainer' ? 'Мои занятия' : 'Занятия', '12–18 октября 2026 · Europe/Minsk') + `<div class="filters"><label for="schedule-group">Группа</label><select id="schedule-group"><option value="all">Все группы</option>${accessibleGroups().map(g => `<option value="${g.id}" ${state.scheduleGroup === g.id ? 'selected' : ''}>${g.name}${state.club === 'all' ? ` · ${clubName(g.club)}` : ''}</option>`).join('')}</select>${state.scheduleGroup !== 'all' ? btn('clear-schedule-group', 'Все группы', 'quiet') : ''}</div><div class="date-strip" aria-label="День недели">${days.map(([d, n]) => `<button data-day="${n}" aria-pressed="${state.day === n}"><span>${d}</span><strong>${n}</strong></button>`).join('')}</div>` + dayPanel(state.day === 12 ? filtered : [], state.day === 12 ? 'Понедельник, 12 октября' : `${state.day} октября`);
}
function journals() {
  return heading('Журналы занятий', 'Снимок состава и отметки для каждого занятия') + dayPanel(visibleSessions().filter(s => ['open', 'closed'].includes(s.status)), 'Открытые и завершённые');
}
function groupsPanel() {
  return `<section class="panel"><div class="panel-header"><h2>${role === 'trainer' ? 'Мои группы' : 'Группы филиала'}</h2>${state.view === 'overview' ? link('groups', 'Все') : ''}</div><ul class="participant-list">${accessibleGroups().map(g => `<li><button class="participant-row" data-group="${g.id}"><span><strong>${g.name}</strong><small>${clubName(g.club)} · ${g.size} участников</small></span>${icon('chevron')}</button></li>`).join('')}</ul></section>`;
}
function participantList(query = '') {
  const list = participants.filter(p => allowedClubs.some(c => c.id === p.club) && (state.club === 'all' || p.club === state.club) && p.name.toLowerCase().includes(query.toLowerCase()));
  return list.length ? `<ul class="participant-list">${list.map(p => `<li><button class="participant-row" data-participant="${p.id}"><span class="avatar">${p.name.split(' ').map(x => x[0]).join('')}</span><span><strong>${esc(p.name)}</strong><small>${groups.find(g => g.id === p.group)?.name || 'Группа не назначена'} · ${p.born}</small></span>${icon('chevron')}</button></li>`).join('')}</ul>` : `<div class="empty-state"><h2>Участник не найден</h2><p>Проверьте имя или попробуйте другой филиал.</p></div>`;
}
function registry() {
  return heading('Участники', 'Ручной реестр · показана демонстрационная выборка', btn('add-participant', `${icon('plus')}Добавить участника`)) + `<div class="filters"><label class="search-field">${icon('search')}<input id="participant-search" aria-label="Поиск участника" placeholder="Имя или фамилия" value="${esc(state.search)}"></label></div><section id="participant-results" class="panel" aria-label="Участники">${participantList(state.search)}</section>`;
}
function statistics() {
  const t = totals();
  return heading('Статистика посещаемости', 'Филиал, группа и период · границы дат включительно') + reportFilters() + statsStrip([['Посещаемость', rate(t.present, t.absent), 'округление до 0,1%'], ['Проведено занятий', t.closed, 'только завершённые'], ['Присутствия', t.present, 'отметки участников'], ['Пропуски', t.absent, 'все причины']]) + `<div class="workspace-grid"><div class="main-column">${networkPanel()}<section class="panel"><div class="panel-header"><h2>Как считается отчёт</h2></div><div class="panel-body"><p class="sheet-copy">Присутствия ÷ (присутствия + пропуски) × 100%. Отменённые и открытые занятия исключены. При отсутствии отметок — «—». Переводы и архивирование не меняют исторический состав.</p></div></section></div><div class="side-column">${splitPanel()}${attentionPanel()}</div></div>`;
}
function staffPage() {
  return heading('Сотрудники', 'Тренеры доступных филиалов · демоданные') + `<section class="panel"><ul class="participant-list">${allowedClubs.filter(c => state.club === 'all' || c.id === state.club).map(c => `<li><div class="participant-row"><span class="avatar">${c.id === 'central' ? 'ИМ' : c.id === 'south' ? 'ДО' : 'АБ'}</span><div><strong>${c.id === 'central' ? 'Илья Морозов' : c.id === 'south' ? 'Денис Орлов' : 'Анна Белова'}</strong><small>Тренер · ${c.name}</small></div><span class="status closed">Активен</span></div></li>`).join('')}</ul></section>`;
}
function clubsPage() {
  return heading('Филиалы сети', 'Каждый зал — отдельный филиал') + `<section class="panel"><ul class="participant-list">${clubs.map(c => `<li><button class="participant-row" data-club="${c.id}">${icon('location')}<span><strong>${c.name}</strong><small>${c.members} участников · ${groups.filter(g => g.club === c.id).length} группы · ${c.trainers} тренера</small></span>${icon('chevron')}</button></li>`).join('')}</ul></section>`;
}
function auditPage() {
  return heading('История изменений', 'Факты действий сотрудников · доступные филиалы') + `<div class="filters"><label for="audit-action">Действие</label><select id="audit-action"><option value="all">Все действия</option><option value="journal">Журнал</option><option value="participants">Участники</option><option value="schedule">Расписание</option></select></div><div id="audit-results">${activityPanel()}</div>`;
}
function recordStatus(r) { return r.mark === 'present' ? 'Присутствует' : r.mark ? `Отсутствует · ${reasonNames[r.mark]}` : 'Не отмечено'; }
function journal() {
  const s = sessions.find(s => s.id === state.journal);
  const g = groupFor(s), records = journalRecords.get(s.id);
  const marked = records.filter(r => r.mark).length;
  const writable = role === 'trainer' && s.status === 'open';
  const r = records[state.record];
  const disabled = state.busy ? 'disabled' : '';
  const card = writable && r ? `<section class="panel attendance-card"><div class="avatar">${r.name.split(' ').map(x => x[0]).join('')}</div><h2 id="current-record" tabindex="-1">${r.name}</h2><p>${r.born} · Карточка ${r.code}</p><p style="margin-top:16px">${recordStatus(r)}</p><div class="mark-actions">${btn('mark-present', `${icon('check')}${state.busy ? 'Сохраняем…' : 'Присутствует'}`, 'positive', disabled)}${btn('absence', 'Отсутствует', 'secondary', disabled)}</div>${state.saveError ? `<div class="save-error" role="alert">${state.saveError}</div>` : ''}</section>` : `<section class="panel"><div class="panel-header"><h2>${s.status === 'closed' ? 'Журнал завершён' : 'Сводка посещаемости'}</h2>${status(s.status)}</div><div class="panel-body"><p class="sheet-copy">${s.status === 'closed' ? 'Состав и отметки сохранены. Тренер больше не может их изменять.' : 'Выберите участника в списке, чтобы посмотреть его отметку.'}</p></div></section>`;
  return `<div class="journal-head"><div><h1>${g.name}</h1><p class="journal-info">12 октября · ${s.time}–${s.end} · ${clubName(g.club)}</p></div><a class="btn secondary" href="#journals">${icon('back')}К журналам</a></div><div class="progress-meta"><span>Отмечено ${marked} из ${records.length}</span><span>${status(s.status)}</span></div><progress aria-label="Прогресс отметок" value="${marked}" max="${Math.max(1, records.length)}"></progress><div class="journal-layout" style="margin-top:24px"><div class="main-column">${card}${writable ? btn('close-journal', 'Завершить занятие', 'secondary', disabled) : ''}</div><section class="panel"><div class="panel-header"><h2>Состав занятия</h2></div><div class="panel-body"><ul class="roster-summary">${records.map((r, i) => `<li><div><strong>${r.name}</strong><small>${recordStatus(r)}</small></div>${writable ? btn('select-record', r.mark ? 'Изменить' : 'Открыть', 'quiet', `data-index="${i}" aria-label="${r.mark ? 'Изменить' : 'Открыть'} отметку: ${r.name}" ${disabled}`) : s.status === 'closed' && role !== 'trainer' ? btn('correct-record', 'Исправить', 'quiet', `data-index="${i}" aria-label="Исправить отметку: ${r.name}"`) : ''}</li>`).join('')}</ul></div></section></div>`;
}
function render() {
  renderNav();
  if (state.scenario === 'loading') {
    main.innerHTML = heading('Рабочее пространство', 'Загружаем данные…') + `<div aria-busy="true" aria-label="Загрузка данных"><div class="skeleton" style="margin-bottom:24px"></div><section class="panel loading-panel"><div class="skeleton-line"></div><div class="skeleton-line"></div><div class="skeleton-line"></div></section></div>`;
    return;
  }
  if (state.scenario === 'error') {
    main.innerHTML = heading('Рабочее пространство', clubName(state.club)) + `<section class="empty-state" role="alert">${icon('wifi')}<h2>Не удалось загрузить данные</h2><p>Проверьте интернет и попробуйте снова. Изменения без связи недоступны.</p>${btn('retry', 'Попробовать снова')}</section>`;
    return;
  }
  if (state.scenario === 'empty') {
    main.innerHTML = heading(role === 'owner' ? 'Обзор клуба' : 'Рабочий день', clubName(state.club)) + (role === 'owner' ? statsStrip([['Посещаемость', '—', 'пока нет отметок'], ['Проведено занятий', 0, 'за выбранный период'], ['Присутствия', 0, 'пока нет отметок'], ['Пропуски', 0, 'пока нет отметок']]) : '') + empty('Занятий пока нет', role === 'trainer' ? 'Когда администратор назначит вам занятие, оно появится здесь.' : 'Добавьте участников, создайте группу и задайте расписание. Затем здесь появятся занятия.', role === 'trainer' ? 'schedule' : 'groups');
    return;
  }
  main.innerHTML = ({ overview, schedule, participants: registry, groups: () => heading(role === 'trainer' ? 'Мои группы' : 'Группы', 'Состав на 12 октября 2026') + groupsPanel(), journals, statistics, staff: staffPage, clubs: clubsPage, audit: auditPage, journal }[state.view] || overview)();
}
function navigate(view, focus = false) {
  if (!navItems().some(item => item[0] === view) && view !== 'journal') view = 'overview';
  if (view === 'journal' && !state.journal) view = 'overview';
  state.view = view;
  state.saveError = '';
  render();
  if (focus) { main.focus(); window.scrollTo(0, 0); }
}
function setClub(id) {
  if (id !== 'all' && !allowedClubs.some(c => c.id === id)) return;
  if (id === 'all' && role !== 'owner') return;
  state.club = id; state.group = 'all'; state.scheduleGroup = 'all'; state.journal = null; state.search = '';
  document.querySelector('#club').value = id;
  if (state.view === 'journal') state.view = 'overview';
  render();
}
function showSheet(title, content, actions = '') {
  sheetOpener = document.activeElement;
  document.querySelector('#sheet-content').innerHTML = `<div class="sheet-head"><h2 id="sheet-title">${title}</h2>${btn('dismiss', icon('close'), 'quiet icon-button', 'aria-label="Закрыть панель"')}</div>${content}${actions}`;
  if (!sheet.open) sheet.showModal();
}
function closeSheet() { if (sheet.open) sheet.close(); }
sheet.addEventListener('close', () => { if (sheetOpener?.isConnected) sheetOpener.focus(); });
sheet.addEventListener('click', event => { if (event.target === sheet) { const b = sheet.getBoundingClientRect(); if (event.clientX < b.left || event.clientX > b.right || event.clientY < b.top || event.clientY > b.bottom) closeSheet(); } });
function toast(text) {
  const el = document.querySelector('#toast'); el.textContent = text; el.hidden = false;
  clearTimeout(toastTimer); toastTimer = setTimeout(() => { el.hidden = true; }, 3500);
}
function openSession(id) {
  const s = visibleSessions().find(s => s.id === id); if (!s) return;
  const g = groupFor(s);
  if (s.status === 'cancelled') {
    showSheet('Занятие отменено', `<p class="sheet-copy">${g.name} · 12 октября, ${s.time}. Журнал посещаемости недоступен; занятие исключено из статистики.</p>`, btn('dismiss', 'Понятно', 'secondary')); return;
  }
  if (s.status === 'planned' && role !== 'trainer') {
    showSheet('Запланированное занятие', `<dl class="detail-list"><div><dt>Группа</dt><dd>${g.name}</dd></div><div><dt>Время</dt><dd>12 октября · ${s.time}–${s.end}</dd></div><div><dt>Тренер</dt><dd>${s.trainer}</dd></div><div><dt>Филиал</dt><dd>${clubName(g.club)}</dd></div></dl><p class="sheet-copy" style="margin-top:24px">Журнал появится после открытия занятия тренером.</p>`, `<div class="sheet-actions">${btn('cancel-session', 'Отменить занятие', 'secondary', `data-id="${s.id}"`)}${btn('replace-trainer', 'Заменить тренера', '', `data-id="${s.id}"`)}</div>`); return;
  }
  if (s.status === 'planned') s.status = 'open';
  state.journal = id; state.record = Math.max(0, journalRecords.get(id).findIndex(r => !r.mark));
  state.view = 'journal'; closeSheet(); render(); main.focus(); window.scrollTo(0, 0);
}
async function mark(value) {
  const s = sessions.find(s => s.id === state.journal);
  if (state.busy || role !== 'trainer' || s?.status !== 'open') return;
  const records = journalRecords.get(s.id), current = records[state.record]; if (!current) return;
  state.busy = true; state.saveError = ''; closeSheet(); render();
  await new Promise(resolve => setTimeout(resolve, 350));
  state.busy = false;
  if (state.offline) {
    state.saveError = 'Нет соединения. Отметка не сохранена. Проверьте связь и нажмите ещё раз.'; render(); main.querySelector('.save-error')?.scrollIntoView({ block: 'nearest' }); return;
  }
  current.mark = value;
  let next = records.findIndex((r, i) => i > state.record && !r.mark);
  if (next < 0) next = records.findIndex(r => !r.mark);
  state.record = next;
  render(); document.querySelector('#current-record')?.focus();
  toast('Отметка сохранена в демосценарии');
}
function correction(index) {
  const s = sessions.find(s => s.id === state.journal); if (role === 'trainer' || s?.status !== 'closed') return;
  const r = journalRecords.get(s.id)[index];
  showSheet('Исправить отметку', `<p class="sheet-copy">${r.name} · ${recordStatus(r)}</p><form id="correction-form" data-index="${index}"><label class="field">Новая отметка<select name="mark"><option value="present">Присутствует</option>${Object.entries(reasonNames).map(([v, n]) => `<option value="${v}">Отсутствует · ${n}</option>`).join('')}</select></label><label class="field">Причина исправления<textarea name="reason" rows="3" required minlength="3" maxlength="500" placeholder="Объясните, почему меняется отметка"></textarea></label><div class="sheet-actions">${btn('dismiss', 'Отмена', 'secondary')}<button type="submit" class="btn">Сохранить исправление</button></div><p class="sheet-copy" style="margin-top:20px">Демонстрация поштучного исправления. В приложении изменение должно сохраняться с автором и причиной в истории.</p></form>`);
}
document.addEventListener('click', async event => {
  const el = event.target.closest('button, a'); if (!el) return;
  if (el.dataset.scheduleGroup) { state.scheduleGroup = el.dataset.scheduleGroup; state.day = 12; if (state.view === 'schedule') render(); }
  if (el.dataset.period) { state.period = el.dataset.period; render(); return; }
  if (el.dataset.club) { setClub(el.dataset.club); if (state.view === 'clubs') { location.hash = 'overview'; } return; }
  if (el.dataset.day) { state.day = Number(el.dataset.day); render(); return; }
  if (el.dataset.session) { openSession(el.dataset.session); return; }
  if (el.dataset.participant) {
    const p = participants.find(p => p.id === el.dataset.participant);
    showSheet(p.name, `<dl class="detail-list"><div><dt>Дата рождения</dt><dd>${p.born}</dd></div><div><dt>Филиал</dt><dd>${clubName(p.club)}</dd></div><div><dt>Группа</dt><dd>${groups.find(g => g.id === p.group)?.name || 'Не назначена'}</dd></div><div><dt>Контакт</dt><dd>${p.contact}</dd></div><div><dt>Дата вступления</dt><dd>${p.joined}</dd></div></dl><p class="sheet-copy" style="margin-top:24px">Демонстрационная карточка. Реальных персональных данных здесь нет.</p>`); return;
  }
  if (el.dataset.group) {
    const g = accessibleGroups().find(g => g.id === el.dataset.group); if (!g) return;
    showSheet(g.name, `<dl class="detail-list"><div><dt>Филиал</dt><dd>${clubName(g.club)}</dd></div><div><dt>Состав на 12 октября</dt><dd>${g.size} участников</dd></div><div><dt>История состава</dt><dd>Состав занятий фиксируется при открытии журнала и сохраняется после переводов.</dd></div></dl>`, `<div class="sheet-actions"><a class="btn" href="#schedule" data-action="dismiss" data-schedule-group="${g.id}">К занятиям группы</a></div>`); return;
  }
  const action = el.dataset.action;
  if (action === 'dismiss') closeSheet();
  if (action === 'clear-schedule-group') { state.scheduleGroup = 'all'; render(); }
  if (action === 'more') showSheet('Рабочее пространство', `<nav aria-label="Дополнительная навигация">${navItems().map(([id, ico, name]) => `<a class="nav-link" href="#${id}" data-action="dismiss">${icon(ico)}${name}</a>`).join('')}</nav><p class="sheet-copy" style="margin-top:24px">${roleNames[role]} · Прототип, демоданные</p>`);
  if (action === 'retry') { state.scenario = 'normal'; render(); toast('Показаны демонстрационные данные'); }
  if (action === 'session') openSession(el.dataset.id);
  if (action === 'select-record') { state.record = Number(el.dataset.index); state.saveError = ''; render(); document.querySelector('#current-record')?.focus(); }
  if (action === 'mark-present') await mark('present');
  if (action === 'absence') showSheet('Причина отсутствия', `<p class="sheet-copy">${journalRecords.get(state.journal)[state.record].name}</p><div class="choice-list">${Object.entries(reasonNames).map(([value, name]) => btn('mark-absent', `${name}${icon('chevron')}`, 'secondary', `data-reason="${value}"`)).join('')}</div>`, `<div class="sheet-actions">${btn('dismiss', 'Отмена', 'quiet')}</div>`);
  if (action === 'mark-absent') await mark(el.dataset.reason);
  if (action === 'close-journal') {
    const remaining = journalRecords.get(state.journal).filter(r => !r.mark).length;
    showSheet('Завершить занятие?', `<p class="sheet-copy">После завершения тренер не сможет менять отметки.</p><p class="close-count">${remaining}</p><p>${remaining ? 'Неотмеченные участники станут отсутствующими с неизвестной причиной.' : 'Все участники отмечены. Журнал войдёт в статистику.'}</p>`, `<div class="sheet-actions">${btn('dismiss', 'Вернуться к отметкам', 'secondary')}${btn('confirm-close', 'Завершить занятие')}</div>`);
  }
  if (action === 'confirm-close') {
    if (state.offline) { closeSheet(); state.saveError = 'Нет соединения. Журнал не завершён. Проверьте связь и попробуйте снова.'; render(); return; }
    journalRecords.get(state.journal).forEach(r => { if (!r.mark) r.mark = 'unknown'; });
    sessions.find(s => s.id === state.journal).status = 'closed'; closeSheet(); render(); main.focus(); toast('Журнал завершён в демосценарии');
  }
  if (action === 'correct-record') correction(Number(el.dataset.index));
  if (action === 'cancel-session') showSheet('Отменить занятие?', '<p class="sheet-copy">Занятие останется в календаре со статусом «Отменено» и не войдёт в статистику.</p>', `<div class="sheet-actions">${btn('dismiss', 'Назад', 'secondary')}${btn('confirm-cancel', 'Отменить занятие', '', `data-id="${el.dataset.id}"`)}</div>`);
  if (action === 'confirm-cancel') { sessions.find(s => s.id === el.dataset.id).status = 'cancelled'; closeSheet(); render(); toast('Занятие отменено в демосценарии'); }
  if (action === 'replace-trainer') showSheet('Замена тренера', `<form id="replace-form" data-id="${el.dataset.id}"><label class="field">Тренер<select name="trainer"><option>Виктор Лебедев</option><option>Илья Морозов</option></select></label><p class="sheet-copy">Замена применяется только к этому занятию, расписание группы сохраняется.</p><button class="btn" type="submit">Сохранить замену</button></form>`);
  if (action === 'add-participant' && role !== 'trainer') {
    showSheet('Новый участник', `<form id="add-form"><label class="field">Имя и фамилия<input name="name" required maxlength="100" autocomplete="off"></label><label class="field">Дата рождения<input name="birth" type="date" required max="2026-10-12"></label><label class="field">Группа<select name="group">${accessibleGroups().map(g => `<option value="${g.id}">${g.name} · ${clubName(g.club)}</option>`).join('')}</select></label><p class="sheet-copy">Демонстрация формы. Данные живут только до перезагрузки; используйте вымышленные значения.</p><div class="sheet-actions">${btn('dismiss', 'Отмена', 'secondary')}<button class="btn" type="submit">Добавить участника</button></div></form>`);
  }
});
document.addEventListener('change', event => {
  if (event.target.id === 'club') setClub(event.target.value);
  if (event.target.id === 'group-filter') { state.group = event.target.value; render(); }
  if (event.target.id === 'schedule-group') { state.scheduleGroup = event.target.value; render(); }
  if (event.target.id === 'audit-action') {
    const value = event.target.value;
    document.querySelector('#audit-results').innerHTML = activityPanel();
    if (value !== 'all') document.querySelectorAll('#audit-results .activity-list li').forEach((el, i) => { el.hidden = i !== { journal: 0, participants: 1, schedule: 2 }[value]; });
  }
});
document.addEventListener('input', event => {
  if (event.target.id === 'participant-search') { state.search = event.target.value; document.querySelector('#participant-results').innerHTML = participantList(state.search); }
});
document.addEventListener('submit', event => {
  event.preventDefault(); const form = event.target; if (!form.reportValidity()) return;
  const fields = new FormData(form);
  if (form.id === 'add-form') {
    const g = accessibleGroups().find(g => g.id === fields.get('group'));
    const name = fields.get('name').trim(); if (!name) return;
    participants.push({ id: `p${participants.length + 1}`, name, group: g.id, club: g.club, born: fields.get('birth').split('-').reverse().join('.'), joined: '12.10.2026', contact: 'Демонстрационный контакт' });
    closeSheet(); location.hash = 'participants'; state.view = 'participants'; render(); toast('Карточка добавлена только в прототип');
  }
  if (form.id === 'replace-form') { sessions.find(s => s.id === form.dataset.id).trainer = fields.get('trainer'); closeSheet(); render(); toast('Тренер заменён в демосценарии'); }
  if (form.id === 'correction-form') {
    if (state.offline) { toast('Нет связи. Исправление не сохранено.'); return; }
    const r = journalRecords.get(state.journal)[Number(form.dataset.index)];
    r.mark = fields.get('mark'); closeSheet(); render(); toast('Исправлена одна отметка в прототипе');
  }
});
window.addEventListener('hashchange', () => { closeSheet(); navigate(location.hash.slice(1) || 'overview', true); });
window.addEventListener('message', event => {
  if (event.origin !== location.origin || event.source !== parent || event.data?.type !== 'pride-preview') return;
  state.offline = Boolean(event.data.offline);
  if (!state.offline && state.saveError) { state.saveError = ''; render(); }
});
document.querySelector('#club').innerHTML = (role === 'owner' ? '<option value="all">Все филиалы</option>' : '') + allowedClubs.map(c => `<option value="${c.id}">${c.name}</option>`).join('');
document.querySelector('#club').value = state.club;
document.querySelector('#identity').innerHTML = `<div class="identity"><span class="avatar">${personas[role][1]}</span><div><strong>${personas[role][0]}</strong><small>${roleNames[role]}</small></div></div>`;
document.querySelector('#top-role').textContent = roleNames[role];
navigate(location.hash.slice(1) || 'overview');

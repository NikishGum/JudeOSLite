'use strict';

// Синтетические события. Модель сохраняет публичный envelope текущего аудита:
// дата, автор, филиал, действие, тип и ID записи. Private payload не добавляется.
const actors = ['Ольга Калиновская', 'Алексей Волков', 'Мария Соколова'];
const clubs = ['Брест · Восток', 'Брест · Центр'];
const kinds = {
  corrected: ['Исправлена отметка', 'Посещаемость', 'edit'],
  journal_closed: ['Завершён журнал', 'Журнал', 'check'],
  journal_opened: ['Открыт журнал', 'Журнал', 'open'],
  marked: ['Отмечена посещаемость', 'Отметка', 'check'],
  created: ['Создана запись', 'Спортсмен', 'plus'],
  updated: ['Изменена запись', 'Группа', 'edit'],
  added: ['Спортсмен включён в группу', 'Состав группы', 'plus'],
  archived: ['Запись в архиве', 'Спортсмен', 'archive'],
  other: ['Другое действие', 'Запись', 'other'],
};
const seeds = [
  [20, '18:42:10', 'corrected', 0, 0],
  [20, '18:35:04', 'journal_closed', 1, 0],
  [20, '18:34:51', 'marked', 1, 0],
  [20, '18:34:40', 'marked', 1, 0],
  [20, '18:34:33', 'marked', 1, 0],
  [20, '18:34:20', 'marked', 1, 0],
  [20, '18:00:02', 'journal_opened', 1, 0],
  [20, '16:24:13', 'created', 0, 0],
  [20, '16:22:46', 'added', 0, 0],
  [20, '15:10:07', 'updated', 2, 1],
  [19, '19:12:03', 'journal_closed', 2, 1],
  [19, '19:11:42', 'marked', 2, 1],
  [19, '19:11:19', 'marked', 2, 1],
  [19, '19:11:02', 'marked', 2, 1],
  [19, '17:45:02', 'archived', 0, 0],
  [19, '16:28:10', 'other', 0, 0],
  [18, '18:30:04', 'journal_closed', 1, 0],
  [18, '18:29:51', 'marked', 1, 0],
  [18, '18:29:32', 'marked', 1, 0],
  [18, '18:00:00', 'journal_opened', 1, 0],
  [17, '17:15:00', 'created', 2, 1],
  [17, '17:12:00', 'added', 2, 1],
  [16, '18:30:00', 'journal_closed', 1, 0],
  [16, '16:10:00', 'updated', 0, 0],
  [15, '18:30:00', 'journal_closed', 2, 1],
  [14, '16:10:00', 'created', 0, 0],
  [12, '18:30:00', 'journal_closed', 1, 0],
  [10, '16:10:00', 'created', 2, 1],
];
const events = seeds.map(([day, time, action, actor, club], index) => ({
  id: `d6201020-0000-4000-a000-${String(index + 1).padStart(12, '0')}`,
  entityId: `a${(0xf8c31 + index * 7919).toString(16).padStart(6, '0')}0-1020-4000-a000-${String(1042 + index).padStart(12, '0')}`,
  day, time, date: `2026-10-${String(day).padStart(2, '0')}`,
  createdAt: `2026-10-${String(day).padStart(2, '0')}T${time}+03:00`,
  action, actorName: actors[actor], clubName: clubs[club],
}));
const $ = (id) => document.getElementById(id);
const state = { query: '', limit: 10, loading: false, error: false };
let searchTimer;
let refreshTimer;
const fields = ['club', 'period', 'action', 'actor', 'from', 'until'];
const paths = {
  edit: '<path d="m14 5 5 5M4 20l5-1L20 8a2 2 0 0 0-5-5L4 14z"/>',
  check: '<path d="m5 12 4 4L19 6"/>',
  open: '<path d="M5 4h14v16H5zM9 8h6M9 12h6"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  archive: '<path d="M4 9h16v11H4zM3 4h18v5H3zM9 13h6"/>',
  other: '<circle cx="12" cy="12" r="8"/><path d="M12 7v5l3 2"/>',
};
function icon(name) { return `<svg class="action-icon" aria-hidden="true" viewBox="0 0 24 24">${paths[name]}</svg>`; }
function plural(n) { return `${n} ${n % 10 === 1 && n % 100 !== 11 ? 'событие' : n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 12 || n % 100 > 14) ? 'события' : 'событий'}`; }
function shortId(e) { return e.entityId.slice(0, 6); }
function normalize(text) { return text.toLocaleLowerCase('ru').replaceAll('ё', 'е'); }
function queryWords() { return normalize(state.query).split(/\s+/).filter(Boolean).map(word => word.replace(/^#(?=[a-f0-9])/i, '')); }
function escapeHtml(text) { return String(text).replace(/[&<>"']/g, char => ({'&':'&amp;', '<':'&lt;', '>':'&gt;', '"':'&quot;', "'":'&#39;'}[char])); }
function highlight(text) {
  const normalized = normalize(text);
  const ranges = [];
  for (const word of queryWords()) {
    let start = 0;
    while ((start = normalized.indexOf(word, start)) >= 0) { ranges.push([start, start + word.length]); start += word.length; }
  }
  ranges.sort((a, b) => a[0] - b[0]);
  const merged = [];
  for (const range of ranges) {
    const last = merged[merged.length - 1];
    if (last && range[0] <= last[1]) last[1] = Math.max(last[1], range[1]); else merged.push([...range]);
  }
  let result = '', cursor = 0;
  for (const [start, end] of merged) { result += escapeHtml(text.slice(cursor, start)) + '<mark>' + escapeHtml(text.slice(start, end)) + '</mark>'; cursor = end; }
  return result + escapeHtml(text.slice(cursor));
}
function periodLabel() {
  const period = $('period').value;
  if (period === 'custom') {
    const format = value => value ? value.split('-').reverse().join('.') : '…';
    return `${format($('from').value)} — ${format($('until').value)}`;
  }
  return {week: '14–20 октября', today: '20 октября', yesterday: '19 октября', all: 'За всё время'}[period];
}
function activeFilters() {
  const chips = [];
  if (state.query) chips.push(['search', `Поиск: ${state.query}`]);
  if ($('club').value) chips.push(['club', `Филиал: ${$('club').value}`]);
  if ($('period').value !== 'week' && $('period').value !== 'all') chips.push(['period', `Период: ${periodLabel()}`]);
  if ($('action').value) chips.push(['action', `Действие: ${$('action').selectedOptions[0].textContent}`]);
  if ($('actor').value) chips.push(['actor', `Сотрудник: ${$('actor').value}`]);
  $('active-filters').hidden = !chips.length;
  $('active-filters').innerHTML = chips.map(([field, label]) => `<button type="button" class="filter-chip" data-clear="${field}" aria-label="Убрать условие: ${escapeHtml(label)}"><span>${escapeHtml(label)}</span><svg aria-hidden="true" viewBox="0 0 24 24"><path d="m6 6 12 12M18 6 6 18"/></svg></button>`).join('');
}
function filterEvents() {
  const words = queryWords();
  const period = $('period').value;
  return events.filter(e => {
    if ($('club').value && e.clubName !== $('club').value) return false;
    if ($('action').value && e.action !== $('action').value) return false;
    if ($('actor').value && e.actorName !== $('actor').value) return false;
    if (period === 'today' && e.day !== 20) return false;
    if (period === 'yesterday' && e.day !== 19) return false;
    if (period === 'week' && e.day < 14) return false;
    if (period === 'custom' && (($('from').value && e.date < $('from').value) || ($('until').value && e.date > $('until').value))) return false;
    const actionLabel = $('action').querySelector(`option[value="${e.action}"]`).textContent;
    const haystack = normalize([e.actorName, e.clubName, kinds[e.action][0], kinds[e.action][1], actionLabel, e.action, e.entityId, e.id].join(' '));
    return words.every(word => haystack.includes(word));
  });
}
function details(e) {
  return `<div class="detail-content"><dl><div><dt>Дата и время · Минск</dt><dd>${e.day} октября 2026, ${e.time}</dd></div><div><dt>ID записи</dt><dd><code>${highlight(e.entityId)}</code></dd></div><div><dt>ID события</dt><dd><code>${highlight(e.id)}</code></dd></div></dl></div>`;
}
function journal(items) {
  return `<table class="journal"><caption class="sr-only">История действий, новые события сверху. Подробности раскрываются в строке.</caption><thead><tr><th scope="col">Время</th><th scope="col">Действие</th><th scope="col">Запись</th><th scope="col">Сотрудник / филиал</th><th scope="col"><span class="sr-only">Подробности</span></th></tr></thead><tbody>${items.map(e => `<tr class="event-row"><td><time class="event-time" datetime="${e.createdAt}">${e.time.slice(0, 5)}<small>${e.day} окт.</small></time></td><td><span class="action-text">${icon(kinds[e.action][2])}<span>${highlight(kinds[e.action][0])}</span></span></td><td><span class="entity">${highlight(kinds[e.action][1])}</span><span class="short-id">#${highlight(shortId(e))}</span></td><td><span class="actor">${highlight(e.actorName)}</span><span class="club-name">${highlight(e.clubName)}</span></td><td><button class="expand" type="button" data-detail="${e.id}" aria-label="Подробности события ${e.day} октября в ${e.time}" aria-expanded="false" aria-controls="detail-${e.id}"><svg aria-hidden="true" viewBox="0 0 24 24"><path d="m9 6 6 6-6 6"/></svg></button></td></tr><tr class="detail-row" id="detail-${e.id}" hidden><td colspan="5">${details(e)}</td></tr>`).join('')}</tbody></table>`;
}
function reset() {
  clearTimeout(searchTimer); clearTimeout(refreshTimer);
  $('filters').reset(); state.query = ''; state.limit = 10; state.error = false; state.loading = false;
  $('extra-filters').hidden = true; $('toggle-filters').setAttribute('aria-expanded', 'false');
  render(); $('search').focus();
}
function render() {
  const custom = $('period').value === 'custom';
  $('date-fields').hidden = !custom;
  const invalid = custom && $('from').value && $('until').value && $('from').value > $('until').value;
  $('date-error').hidden = !invalid;
  $('from').setAttribute('aria-invalid', String(Boolean(invalid)));
  $('until').setAttribute('aria-invalid', String(Boolean(invalid)));
  const extraCount = Number(Boolean($('action').value)) + Number(Boolean($('actor').value));
  $('filter-count').hidden = !extraCount;
  $('filter-count').textContent = extraCount;
  $('clear-search').hidden = !$('search').value;
  activeFilters();
  $('reset').hidden = !state.query && !$('club').value && $('period').value === 'week' && !extraCount;
  const filtered = invalid ? [] : filterEvents();
  $('result-count').textContent = state.error ? 'История недоступна' : state.loading ? 'Загружаем историю…' : invalid ? 'Проверьте период' : plural(filtered.length);
  $('results-context').textContent = `${$('club').value || 'Все филиалы'} · ${periodLabel()}`;
  $('results').setAttribute('aria-busy', String(state.loading));
  $('refresh').disabled = state.loading;
  $('more').hidden = state.loading || state.error || filtered.length <= state.limit;
  $('shown-count').textContent = state.loading || state.error || invalid || !filtered.length ? '' : `Показано ${Math.min(filtered.length, state.limit)} из ${filtered.length}`;
  let content;
  if (state.loading) content = `<div class="loading" aria-hidden="true">${Array.from({length: 6}, () => '<div class="skeleton"><span></span><span></span><span></span></div>').join('')}</div>`;
  else if (state.error) content = '<div class="empty-state" role="alert"><h2>Не удалось загрузить историю</h2><p>Попробуйте ещё раз. Выбранные фильтры сохранятся.</p><button class="button" type="button" data-retry>Повторить загрузку</button></div>';
  else if (invalid) content = '<div class="empty-state"><h2>Проверьте даты</h2><p>Дата начала периода должна быть не позже даты окончания.</p></div>';
  else if (!filtered.length) content = `<div class="empty-state"><h2>Событий не найдено</h2><p>Поиск ограничен выбранными фильтрами. ${state.query ? 'Очистите запрос или расширьте период.' : 'Расширьте период или уберите одно из условий.'}</p>${state.query ? '<button class="button" type="button" data-clear-query>Очистить запрос</button>' : '<button class="button" type="button" data-reset>Сбросить фильтры</button>'}</div>`;
  else content = journal(filtered.slice(0, state.limit));
  $('results').innerHTML = content;
  $('announcement').textContent = $('result-count').textContent;
}
function applySearch() {
  clearTimeout(searchTimer); state.query = $('search').value.trim(); state.limit = 10; render();
}
function clearSearch() { $('search').value = ''; applySearch(); $('search').focus(); }
function refresh() {
  clearTimeout(searchTimer); clearTimeout(refreshTimer); state.query = $('search').value.trim();
  state.loading = true; state.error = false; render();
  refreshTimer = setTimeout(() => { state.loading = false; render(); $('announcement').textContent = 'История обновлена. ' + $('result-count').textContent; }, 650);
}
$('filters').addEventListener('submit', e => { e.preventDefault(); applySearch(); });
$('search').maxLength = 160;
$('search').addEventListener('input', () => {
  clearTimeout(searchTimer); $('clear-search').hidden = !$('search').value;
  $('results').setAttribute('aria-busy', 'true'); $('result-count').textContent = 'Ищем…';
  searchTimer = setTimeout(applySearch, 300);
});
$('search').addEventListener('keydown', e => { if (e.key === 'Escape') { e.preventDefault(); clearSearch(); } });
$('clear-search').addEventListener('click', clearSearch);
$('active-filters').addEventListener('click', e => {
  const button = e.target.closest('[data-clear]'); if (!button) return;
  const field = button.dataset.clear;
  if (field === 'search') { clearSearch(); return; }
  $(field).value = field === 'period' ? 'all' : '';
  applySearch();
  if (field === 'action' || field === 'actor') $('toggle-filters').focus(); else $(field).focus();
});
fields.forEach(id => $(id).addEventListener('change', () => {
  if (id === 'period' && $('period').value === 'custom') {
    $('extra-filters').hidden = false; $('toggle-filters').setAttribute('aria-expanded', 'true');
  }
  applySearch();
}));
$('toggle-filters').addEventListener('click', () => {
  const hidden = !$('extra-filters').hidden;
  $('extra-filters').hidden = hidden;
  $('toggle-filters').setAttribute('aria-expanded', String(!hidden));
});
$('reset').addEventListener('click', reset);
$('refresh').addEventListener('click', refresh);
$('more').addEventListener('click', () => { state.limit += 10; render(); });
$('results').addEventListener('click', e => {
  const button = e.target.closest('button'); if (!button) return;
  if (button.hasAttribute('data-reset')) { reset(); $('search').focus(); }
  if (button.hasAttribute('data-retry')) refresh();
  if (button.hasAttribute('data-clear-query')) clearSearch();
  if (button.dataset.detail) {
    const detail = $(`detail-${button.dataset.detail}`);
    detail.hidden = !detail.hidden; button.setAttribute('aria-expanded', String(!detail.hidden));
  }
});
$('demo-empty').addEventListener('click', () => { $('search').value = 'Несуществующее событие'; state.error = false; applySearch(); $('search').focus(); });
$('demo-error').addEventListener('click', () => { state.error = true; render(); });
render();

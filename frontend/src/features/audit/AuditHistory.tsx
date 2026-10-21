import { Fragment, useEffect, useRef, useState } from 'react';
import { ChevronDown, RefreshCw, Search, SlidersHorizontal, X } from 'lucide-react';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { ApiError, apiGet } from '../../shared/api';
import { initialSelection, normalize, periodRange, searchTerms, type Filters, type Page, type Period, type Selection } from './history';
import './audit.css';

const fullTime = new Intl.DateTimeFormat('ru-BY', { timeZone: 'Europe/Minsk', dateStyle: 'medium', timeStyle: 'medium' });
const day = new Intl.DateTimeFormat('ru-BY', { timeZone: 'Europe/Minsk', day: 'numeric', month: 'short', year: 'numeric' });
const clock = new Intl.DateTimeFormat('ru-BY', { timeZone: 'Europe/Minsk', hour: '2-digit', minute: '2-digit' });
const civilDate = (value: string) => day.format(new Date(`${value}T12:00:00+03:00`));
const failure = (cause: unknown) => cause instanceof ApiError ? cause.message : 'Нет соединения. Проверьте интернет и попробуйте снова.';

function Highlight({ text, query }: { text: string; query: string }) {
 const haystack = normalize(text);
 const ranges: [number, number][] = [];
 for (const term of searchTerms(query)) {
  let index = haystack.indexOf(term);
  while (index !== -1) { ranges.push([index, index + term.length]); index = haystack.indexOf(term, index + Math.max(1, term.length)); }
 }
 ranges.sort((a, b) => a[0] - b[0]);
 const merged: [number, number][] = [];
 for (const range of ranges) { const last = merged.at(-1); if (last && range[0] <= last[1]) last[1] = Math.max(last[1], range[1]); else merged.push([...range]); }
 let end = 0;
 return <>{merged.map(([start, until]) => { const prefix = text.slice(end, start); end = until; return <Fragment key={start}>{prefix}<mark>{text.slice(start, until)}</mark></Fragment>; })}{text.slice(end)}</>;
}

export function AuditHistory({ selectedClubId }: { selectedClubId: string }) {
 const [filters, setFilters] = useState<Filters | null>(null);
 const [selection, setSelection] = useState(() => initialSelection(selectedClubId));
 const [query, setQuery] = useState('');
 const [period, setPeriod] = useState<Period>('week');
 const [dates, setDates] = useState(() => periodRange('week'));
 const [open, setOpen] = useState(false);
 const [page, setPage] = useState<Page | null>(null);
 const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
 const [loading, setLoading] = useState(true);
 const [hidden, setHidden] = useState(false);
 const [error, setError] = useState('');
 const [formError, setFormError] = useState('');
 const [request, setRequest] = useState({ sequence: 0, cursor: '', background: false });
 const epoch = useRef(0);
 const active = useRef<AbortController | null>(null);
 const allowedScope = useRef('');
 const data = useRef<Page | null>(null);
 const pages = useRef(1);
 const searchRef = useRef<HTMLInputElement>(null);
 const [composing, setComposing] = useState(false);
 data.current = page;

 useEffect(() => {
  const controller = new AbortController(); active.current?.abort(); active.current = controller;
  const generation = ++epoch.current;
  let previous = data.current;
  setLoading(!request.background || previous === null);
  async function load() {
   try {
    const allowed = await apiGet<Filters>('/api/audit/filters', controller.signal);
    if (controller.signal.aborted || generation !== epoch.current) return;
    const scope = `${allowed.isOwner}:${allowed.clubs.map((club) => club.id).sort().join(',')}`;
    const scopeChanged = Boolean(allowedScope.current && allowedScope.current !== scope);
    allowedScope.current = scope;
    if (scopeChanged) { previous = null; pages.current = 1; setPage(null); setExpanded(new Set()); setLoading(true); }
    setFilters(allowed);
    const cursor = scopeChanged ? '' : request.cursor;
    const read = (cursor: string) => {
     const params = new URLSearchParams(selection); if (cursor) params.set('cursor', cursor);
     return apiGet<Page>(`/api/audit/events?${params}`, controller.signal);
    };
    let result = await read(cursor);
    if (cursor && previous) {
     const seen = new Set(previous.items.map((event) => event.id));
     result = { ...result, items: [...previous.items, ...result.items.filter((event) => !seen.has(event.id))] };
    } else if (request.background && previous) {
     // Refresh the loaded extent, retaining row identity and expanded details.
     for (let i = 1; i < pages.current && result.nextCursor; i++) {
      const next = await read(result.nextCursor);
      const seen = new Set(result.items.map((event) => event.id));
      result = { ...next, items: [...result.items, ...next.items.filter((event) => !seen.has(event.id))] };
     }
    }
    if (!controller.signal.aborted && generation === epoch.current) {
     if (cursor) pages.current += 1;
     setPage(result); setHidden(false); setError('');
    }
   } catch (cause) {
    if (controller.signal.aborted || generation !== epoch.current) return;
    if (cause instanceof ApiError && [401, 403, 404, 410].includes(cause.status)) { setPage(null); setFilters(null); setHidden(true); }
    else setError(failure(cause));
   } finally { if (!controller.signal.aborted && generation === epoch.current) setLoading(false); }
  }
  void load(); return () => controller.abort();
 }, [selection, request]);

 useEffect(() => {
  const refresh = () => { if (document.visibilityState === 'visible') setRequest((current) => ({ sequence: current.sequence + 1, cursor: '', background: true })); };
  window.addEventListener('focus', refresh); window.addEventListener('attendance-changed', refresh); document.addEventListener('visibilitychange', refresh);
  const timer = window.setInterval(refresh, 30_000);
  return () => { window.clearInterval(timer); window.removeEventListener('focus', refresh); window.removeEventListener('attendance-changed', refresh); document.removeEventListener('visibilitychange', refresh); };
 }, []);

 useEffect(() => {
  if (query === selection.q || composing) return;
  const timer = window.setTimeout(() => apply({ ...selection, q: query }), 300);
  return () => window.clearTimeout(timer);
 }, [query, selection, composing]);

 function apply(next: Selection) {
  active.current?.abort(); epoch.current += 1; pages.current = 1;
  setPage(null); setError(''); setLoading(true); setExpanded(new Set()); setSelection(next);
  setRequest((current) => ({ sequence: current.sequence + 1, cursor: '', background: false }));
 }
 function refresh() { apply({ ...selection }); }
 function setPreset(value: Period) {
  setPeriod(value); setFormError('');
  if (value === 'custom') { setOpen(true); return; }
  const range = periodRange(value); setDates(range); apply({ ...selection, ...range });
 }
 function changeDate(field: 'from' | 'until', value: string) {
  const next = { ...dates, [field]: value }; setDates(next);
  if (!next.from || !next.until || !/^\d{4}-\d{2}-\d{2}$/.test(next.from) || !/^\d{4}-\d{2}-\d{2}$/.test(next.until) || next.from > next.until) { setFormError(!next.from || !next.until ? 'Укажите обе даты.' : 'Дата начала должна быть не позже даты окончания.'); return; }
  setFormError(''); apply({ ...selection, ...next });
 }
 function clearSearch() { setQuery(''); apply({ ...selection, q: '' }); searchRef.current?.focus(); }
 function focusField(id: string) { requestAnimationFrame(() => document.getElementById(id)?.focus()); }
 function reset() { const next = initialSelection(); setQuery(''); setPeriod('week'); setDates({ from: next.from, until: next.until }); setFormError(''); apply(next); focusField('audit-search'); }
 function remove(field: 'clubId' | 'action' | 'actorId' | 'period') {
  if (field === 'period') setPreset('week'); else apply({ ...selection, [field]: '' });
  focusField(field === 'clubId' ? 'audit-club' : field === 'period' ? 'audit-period' : open ? `audit-${field}` : 'audit-toggle');
 }
 if (hidden) return null;
 const allClubs = filters?.isOwner ? 'Вся сеть' : 'Все мои филиалы';
 const clubName = selection.clubId === 'network' ? 'Сеть · без филиала' : filters?.clubs.find((club) => club.id === selection.clubId)?.name ?? allClubs;
 const actor = filters?.actors.find((person) => person.id === selection.actorId);
 const actionName = (code: string) => filters?.actionLabels[code] ?? code;
 const filterCount = Number(Boolean(selection.action)) + Number(Boolean(selection.actorId));
 const changed = Boolean(selection.q || selection.clubId || filterCount || period !== 'week');
 const scopeDescription = `${clubName} · ${selection.from ? civilDate(selection.from) : 'С начала истории'} — ${selection.until ? civilDate(selection.until) : 'без ограничения конца'}`;
 const shortActor = (id: string, name: string) => filters!.actors.filter((person) => person.name === name).length > 1 ? ` · #${id.slice(0, 8)}` : '';
 return <section className="connection audit-history" aria-labelledby="audit-title">
  <h2 id="audit-title" className="sr-only">История изменений</h2>
  <div className="audit-filters" role="group" aria-label="Фильтры истории">
   <div className="audit-toolbar">
    <div className="audit-search-control"><label htmlFor="audit-search">Поиск по истории</label><div className="audit-search-field"><Search aria-hidden="true"/><Input id="audit-search" ref={searchRef} type="search" maxLength={320} value={query} placeholder="Сотрудник, действие или ID" autoComplete="off" aria-describedby="audit-search-help" onChange={(event) => setQuery(Array.from(event.target.value).slice(0, 160).join(''))} onCompositionStart={() => { setComposing(true); }} onCompositionEnd={(event) => { setComposing(false); setQuery(Array.from(event.currentTarget.value).slice(0, 160).join('')); }} onKeyDown={(event) => {
     if (event.nativeEvent.isComposing) return;
     if (event.key === 'Enter') { event.preventDefault(); apply({ ...selection, q: query }); }
     if (event.key === 'Escape') { event.preventDefault(); clearSearch(); }
    }}/>{query && <Button className="audit-clear" type="button" variant="ghost" aria-label="Очистить поиск" onClick={clearSearch}><X aria-hidden="true"/></Button>}</div></div>
    <label className="audit-select-field" htmlFor="audit-club"><span id="audit-club-label">Филиал</span><select aria-labelledby="audit-club-label" id="audit-club" value={selection.clubId} disabled={!filters} onChange={(event) => apply({ ...selection, clubId: event.target.value })}>
     <option value="">{allClubs}</option>{filters?.isOwner && <option value="network">Сеть · без филиала</option>}{filters?.clubs.map((club) => <option key={club.id} value={club.id}>{club.name}{club.archived ? ' · в архиве' : ''}</option>)}
    </select></label>
    <label className="audit-select-field" htmlFor="audit-period"><span id="audit-period-label">Период</span><select aria-labelledby="audit-period-label" id="audit-period" value={period} onChange={(event) => setPreset(event.target.value as Period)}><option value="week">За 7 дней</option><option value="today">Сегодня</option><option value="yesterday">Вчера</option><option value="all">За всё время</option><option value="custom">Выбрать даты…</option></select></label>
    <Button id="audit-toggle" className="audit-filter-toggle" type="button" variant="outline" aria-expanded={open} aria-controls="audit-extra-filters" onClick={() => setOpen(!open)}><SlidersHorizontal aria-hidden="true"/>Фильтры{filterCount > 0 && <span className="audit-filter-count">{filterCount}</span>}</Button>
   </div>
   <p className="audit-search-help" id="audit-search-help">Все слова запроса, в любом порядке. Учитываются все выбранные фильтры.</p>
   <div className="audit-extra-filters" id="audit-extra-filters" hidden={!open}>
    <label><span id="audit-action-label">Действие</span><select aria-labelledby="audit-action-label" id="audit-action" value={selection.action} onChange={(event) => apply({ ...selection, action: event.target.value })}><option value="">Все действия</option>{filters?.actions.map((action) => <option key={action} value={action}>{actionName(action)}</option>)}</select></label>
    <label><span id="audit-actor-label">Сотрудник</span><select aria-labelledby="audit-actor-label" id="audit-actorId" value={selection.actorId} onChange={(event) => apply({ ...selection, actorId: event.target.value })}><option value="">Все сотрудники</option>{filters?.actors.map((person) => <option key={person.id} value={person.id}>{person.name}{shortActor(person.id, person.name)}{person.disabled ? ' · аккаунт отключён' : ''}</option>)}</select></label>
    {period === 'custom' && <div className="audit-date-fields"><label>С даты<Input type="date" value={dates.from} min="0001-01-01" max="9999-12-31" aria-invalid={Boolean(formError)} aria-describedby={formError ? 'audit-date-error' : undefined} onChange={(event) => changeDate('from', event.target.value)}/></label><label>По дату<Input type="date" value={dates.until} min="0001-01-01" max="9999-12-31" aria-invalid={Boolean(formError)} aria-describedby={formError ? 'audit-date-error' : undefined} onChange={(event) => changeDate('until', event.target.value)}/></label></div>}
    {formError && <p id="audit-date-error" className="error" role="alert">{formError}</p>}
   </div>
   {changed && <div className="audit-active-filters" role="group" aria-label="Применённые фильтры">
    {selection.clubId && <Button type="button" variant="outline" onClick={() => remove('clubId')} aria-label={`Снять фильтр филиала: ${clubName}`}><span>{clubName}</span><X aria-hidden="true"/></Button>}
    {period !== 'week' && <Button type="button" variant="outline" onClick={() => remove('period')} aria-label="Снять фильтр периода"><span>{period === 'custom' ? 'Выбранные даты' : period === 'all' ? 'За всё время' : period === 'today' ? 'Сегодня' : 'Вчера'}</span><X aria-hidden="true"/></Button>}
    {selection.action && <Button type="button" variant="outline" onClick={() => remove('action')} aria-label="Снять фильтр действия"><span>{actionName(selection.action)}</span><X aria-hidden="true"/></Button>}
    {selection.actorId && <Button type="button" variant="outline" onClick={() => remove('actorId')} aria-label="Снять фильтр сотрудника"><span>{actor?.name ?? selection.actorId}{actor && shortActor(actor.id, actor.name)}</span><X aria-hidden="true"/></Button>}
    {selection.q && <Button type="button" variant="outline" onClick={clearSearch} aria-label="Снять поисковый запрос"><span>Поиск: {selection.q}</span><X aria-hidden="true"/></Button>}
   </div>}
  </div>
  <div className="audit-results-heading"><div className="audit-result-summary"><strong role="status">{page ? `Найдено событий: ${page.total}` : loading ? 'Загружаем историю…' : 'История недоступна'}</strong><span>{scopeDescription}</span></div>{changed && <Button type="button" variant="link" onClick={reset}>Сбросить всё</Button>}<div className="audit-results-actions"><span>Новые сверху · Минск</span><Button type="button" variant="outline" disabled={loading} onClick={refresh}><RefreshCw aria-hidden="true"/>Обновить</Button></div></div>
  {error && <div className="error audit-error" role="alert"><p>{error}</p><Button type="button" variant="outline" onClick={() => setRequest((current) => ({ ...current, sequence: current.sequence + 1 }))}>Повторить загрузку истории</Button></div>}
  <div className="audit-results" aria-busy={loading}>
   {loading && page && <p className="audit-updating" role="status">{request.cursor ? 'Загружаем ещё…' : 'Обновляем историю…'}</p>}
   {page && page.items.length === 0 && <div className="audit-empty"><h3>Изменений не найдено</h3><p>Попробуйте другой запрос или измените ограничения.</p><div>{query && <Button type="button" variant="outline" onClick={clearSearch}>Очистить поиск</Button>}<Button type="button" variant="outline" onClick={() => { setOpen(true); focusField('audit-action'); }}>Изменить фильтры</Button></div></div>}
   {page && page.items.length > 0 && <table className="audit-events" aria-label="События истории"><thead><tr><th scope="col">Время</th><th scope="col">Действие</th><th scope="col">Запись</th><th scope="col">Сотрудник / филиал</th><th scope="col"><span className="sr-only">Подробности</span></th></tr></thead>{page.items.map((event) => <tbody key={event.id} className="audit-event">
    <tr className="audit-event-row"><td className="audit-time"><time dateTime={event.createdAt}><span>{day.format(new Date(event.createdAt))}</span><strong>{clock.format(new Date(event.createdAt))}</strong></time></td>
     <td className="audit-action"><strong><Highlight text={event.actionLabel ?? actionName(event.action)} query={selection.q}/></strong></td>
     <td className="audit-record"><span><Highlight text={event.entityLabel ?? filters?.entityLabels[event.entityType] ?? event.entityType} query={selection.q}/></span><span className="audit-short-id">#<Highlight text={event.entityId.slice(0, 8)} query={selection.q}/></span></td>
     <td className="audit-person"><span><Highlight text={event.actorName} query={selection.q}/>{event.actorDisabled && <span className="audit-meta"> · аккаунт отключён</span>}</span><span className="audit-meta"><Highlight text={event.clubName ?? 'Сеть · без филиала'} query={selection.q}/>{event.clubArchived && ' · в архиве'}</span></td>
     <td className="audit-details-toggle"><Button type="button" variant="ghost" aria-label={`Подробнее о событии #${event.id}`} aria-expanded={expanded.has(event.id)} aria-controls={`audit-detail-${event.id}`} onClick={() => setExpanded((current) => { const next = new Set(current); if (next.has(event.id)) next.delete(event.id); else next.add(event.id); return next; })}><ChevronDown aria-hidden="true"/></Button></td>
    </tr>
    {expanded.has(event.id) && <tr className="audit-detail-row" id={`audit-detail-${event.id}`}><td colSpan={5}><dl><div><dt>Время с секундами</dt><dd>{fullTime.format(new Date(event.createdAt))}</dd></div><div><dt>ID события</dt><dd><Highlight text={event.id} query={selection.q}/></dd></div><div><dt>ID записи</dt><dd><Highlight text={event.entityId} query={selection.q}/></dd></div><div><dt>Действие / тип</dt><dd><Highlight text={`${event.action} · ${event.entityType}`} query={selection.q}/></dd></div><div><dt>ID сотрудника</dt><dd><Highlight text={event.actorId} query={selection.q}/></dd></div></dl></td></tr>}
   </tbody>)}</table>}
  </div>
  {page && page.items.length > 0 && <div className="audit-footer"><span>Загружено {page.items.length} из {page.total}</span>{page.nextCursor && <Button type="button" variant="outline" disabled={loading} onClick={() => setRequest((current) => ({ sequence: current.sequence + 1, cursor: page.nextCursor, background: false }))}>Показать ещё</Button>}</div>}
  <p className="audit-footnote">Имена сотрудников и названия филиалов показаны на сегодня. Количество обновляется при чтении.</p>
 </section>;
}

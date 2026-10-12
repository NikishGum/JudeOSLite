import { useEffect, useRef, useState, type FormEvent } from 'react';
import { ApiError, apiGet } from '../../shared/api';
import './audit.css';
type Club = { id: string; name: string; archived: boolean };
type Filters = { isOwner: boolean; clubs: Club[]; actions: string[] };
type Event = { id: string; createdAt: string; actorName: string; actorDisabled: boolean; clubName: string | null; clubArchived: boolean; entityType: string; entityId: string; action: string };
type Page = { items: Event[]; nextCursor: string };
type Selection = { clubId: string; from: string; until: string; action: string };
const empty: Selection = { clubId: '', from: '', until: '', action: '' };
const actions: Record<string, string> = { corrected: 'Исправление закрытой отметки', journal_opened: 'Открытие журнала', journal_closed: 'Завершение журнала', marked: 'Отметка посещаемости', created: 'Создание', updated: 'Изменение', archived: 'Архивирование', restored: 'Возврат из архива', administrator_created: 'Создание администратора', administration_updated: 'Изменение администратора', trainer_created: 'Создание тренера', trainer_account_updated: 'Изменение аккаунта тренера', trainer_password_changed: 'Смена пароля тренера', trainer_assignment_created: 'Назначение тренера', trainer_assignment_revoked: 'Отзыв назначения тренера', added: 'Включение в группу', excluded: 'Исключение из группы', transferred: 'Перевод', ended: 'Завершение периода', superseded: 'Замена планового занятия', cancelled: 'Отмена занятия', cancel: 'Отмена занятия', 'replace-trainer': 'Замена тренера', reschedule: 'Перенос занятия', trainer_replaced: 'Замена тренера' };
const entities: Record<string, string> = { attendance_record: 'Отметка посещаемости', club: 'Филиал', staff_user: 'Сотрудник', club_membership: 'Карточка участника', training_group: 'Группа', group_membership: 'Период в группе', schedule_rule: 'Правило расписания', training_session: 'Занятие' };
const actionName = (name: string) => actions[name] ?? 'Другое действие';
const timestamp = new Intl.DateTimeFormat('ru-BY', { timeZone: 'Europe/Minsk', dateStyle: 'medium', timeStyle: 'medium' });
const failure = (cause: unknown) => cause instanceof ApiError ? cause.message : 'Нет соединения. Проверьте интернет и попробуйте снова.';
export function AuditHistory({ selectedClubId }: { selectedClubId: string }) {
 const [filters, setFilters] = useState<Filters | null>(null);
 const [draft, setDraft] = useState<Selection>({ ...empty, clubId: selectedClubId });
 const [selection, setSelection] = useState<Selection>({ ...empty, clubId: selectedClubId });
 const [page, setPage] = useState<Page | null>(null);
 const [cursors, setCursors] = useState(['']);
 const [loading, setLoading] = useState(true);
 const [hidden, setHidden] = useState(false);
 const [error, setError] = useState('');
 const [attempt, setAttempt] = useState(0);
 const [formError, setFormError] = useState('');
 const epoch = useRef(0);
 const active = useRef<AbortController | null>(null);
 const cursor = cursors[cursors.length - 1];
 useEffect(() => {
  const controller = new AbortController(); active.current?.abort(); active.current = controller;
  const generation = ++epoch.current;
  setPage(null); setLoading(true); setError('');
  async function load() {
   try {
    const allowed = await apiGet<Filters>('/api/audit/filters', controller.signal);
    if (controller.signal.aborted || generation !== epoch.current) return;
    setFilters(allowed);
    const params = new URLSearchParams(selection); if (cursor) params.set('cursor', cursor);
    const result = await apiGet<Page>(`/api/audit/events?${params}`, controller.signal);
    if (!controller.signal.aborted && generation === epoch.current) { setFilters(allowed); setPage(result); setHidden(false); }
   } catch (cause) {
    if (controller.signal.aborted || generation !== epoch.current) return;
    setPage(null);
    if (cause instanceof ApiError && (cause.status === 401 || cause.status === 403)) { setFilters(null); setHidden(true); }
    else setError(failure(cause));
   } finally { if (!controller.signal.aborted && generation === epoch.current) setLoading(false); }
  }
  void load(); return () => controller.abort();
 }, [selection, cursor, attempt]);
 useEffect(() => {
  const refresh = () => { if (document.visibilityState === 'visible') setAttempt((value) => value + 1); };
  window.addEventListener('focus', refresh); document.addEventListener('visibilitychange', refresh);
  const timer = window.setInterval(refresh, 30_000);
  return () => { window.clearInterval(timer); window.removeEventListener('focus', refresh); document.removeEventListener('visibilitychange', refresh); };
 }, []);
 function clearPending() { active.current?.abort(); epoch.current += 1; setPage(null); setError(''); setLoading(true); }
 function apply(next: Selection) { clearPending(); setFormError(''); setDraft(next); setSelection({ ...next }); setCursors(['']); }
 function submit(event: FormEvent<HTMLFormElement>) {
  event.preventDefault();
  if (draft.from && draft.until && draft.from > draft.until) { setFormError('Дата начала должна быть не позже даты окончания.'); return; }
  apply(draft);
 }
 function move(next: string[]) { clearPending(); setCursors(next); }
 if (hidden) return null;
 return <section className="connection audit-history" aria-labelledby="audit-title" aria-busy={loading}>
  <div className="audit-heading"><h2 id="audit-title">История изменений</h2><button type="button" className="audit-secondary" disabled={loading} onClick={() => { clearPending(); setCursors(['']); setAttempt((value) => value + 1); }}>Обновить историю</button></div>
  <p className="help">Кто и когда изменил записи. Время и границы дат — по Минску. Имена сотрудников и филиалов показаны на сегодня.</p>
  {filters && <form className="audit-filters" onSubmit={submit}>
   <label className="field"><span id="audit-club-label">Филиал истории</span><select aria-labelledby="audit-club-label" value={draft.clubId} onChange={(event) => setDraft({ ...draft, clubId: event.target.value })}>
    <option value="">{filters.isOwner ? 'Вся сеть' : 'Все мои филиалы'}</option>
    {filters.isOwner && <option value="network">Сеть · без филиала</option>}
    {filters.clubs.map((club) => <option key={club.id} value={club.id}>{club.name}{club.archived ? ' · в архиве' : ''}</option>)}
   </select></label>
   <label className="field">С даты<input type="date" value={draft.from} onChange={(event) => setDraft({ ...draft, from: event.target.value })} /></label>
   <label className="field">По дату<input type="date" value={draft.until} onChange={(event) => setDraft({ ...draft, until: event.target.value })} /></label>
   <label className="field"><span id="audit-action-label">Действие</span><select aria-labelledby="audit-action-label" value={draft.action} onChange={(event) => setDraft({ ...draft, action: event.target.value })}><option value="">Все действия</option>{filters.actions.map((action) => <option key={action} value={action}>{actionName(action)}{actions[action] ? '' : ` (${action})`}</option>)}</select></label>
   <div className="audit-buttons"><button type="submit">Показать историю</button><button type="button" className="audit-secondary" onClick={() => apply({ ...empty })}>Сбросить фильтры</button></div>
   {formError && <p className="error" role="alert">{formError}</p>}
  </form>}
  {loading && <p role="status">Загружаем историю…</p>}
  {error && <div className="error" role="alert"><p>{error}</p><button type="button" onClick={() => setAttempt((value) => value + 1)}>Повторить загрузку истории</button></div>}
  {page && <>
   <p role="status">{page.items.length ? `Страница ${cursors.length} · событий: ${page.items.length}` : 'По выбранным фильтрам изменений нет.'}</p>
   <ol className="audit-events" aria-label="События истории">{page.items.map((event) => <li key={event.id} className="audit-event">
    <div className="audit-event-heading"><strong>{actionName(event.action)}</strong><time dateTime={event.createdAt}>{timestamp.format(new Date(event.createdAt))}</time></div>
    <dl><div><dt>Сотрудник</dt><dd>{event.actorName}{event.actorDisabled && ' · аккаунт отключён'}</dd></div>
     <div><dt>Филиал</dt><dd>{event.clubName ?? 'Сеть · без филиала'}{event.clubArchived && ' · в архиве'}</dd></div>
     <div><dt>Запись</dt><dd>{entities[event.entityType] ?? 'Запись'} <span className="audit-record-id">№ {event.entityId}</span></dd></div></dl>
   </li>)}</ol>
   {(cursors.length > 1 || page.nextCursor) && <nav className="audit-buttons" aria-label="Страницы истории">
    <button type="button" className="audit-secondary" disabled={cursors.length === 1} onClick={() => move(cursors.slice(0, -1))}>Предыдущая страница</button>
    <button type="button" disabled={!page.nextCursor} onClick={() => move([...cursors, page.nextCursor])}>Следующая страница</button>
   </nav>}
  </>}
 </section>;
}

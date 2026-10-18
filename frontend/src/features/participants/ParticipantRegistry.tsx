import { useEffect, useRef, useState, type FormEvent } from 'react';
import { ApiError, apiGet, apiPost, apiPut } from '../../shared/api';
import { ParticipantEditor, type Card, type Draft } from './ParticipantEditor';
import { Dialog } from '../../shared/ui/Dialog';
import { StatusDot } from '../../app/RuntimeShell';
import './participants.css';
import { MembershipHistory } from '../groups/MembershipHistory';
import type { Group } from '../groups/types';
import '../groups/groups.css';

type List = { items: Card[]; hasMore: boolean };
const message = (cause: unknown) => cause instanceof ApiError ? cause.message : 'Нет соединения. Проверьте интернет и повторите действие.';
const displayDate = (value: string) => value.split('-').reverse().join('.');

export function ParticipantRegistry({ clubId, clubName, timezone, onSessionRefresh, detailId = '', onDetailChange, beforeNavigate }: { beforeNavigate?: (action: () => void) => void; detailId?: string; onDetailChange?: (id: string) => void; clubId: string; clubName: string; timezone: string; onSessionRefresh: () => Promise<void> }) {
  const [list, setList] = useState<List>({ items: [], hasMore: false });
  const [queryDraft, setQueryDraft] = useState('');
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState('active');
  const [page, setPage] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [attempt, setAttempt] = useState(0);
  const [selected, setSelected] = useState<string | null>(detailId || null);
  const [card, setCard] = useState<Card | null>(null);
  const [cardLoading, setCardLoading] = useState(false);
  const [cardError, setCardError] = useState('');
  const [cardAttempt, setCardAttempt] = useState(0);
  const [editing, setEditing] = useState(false);
  const [confirmArchive, setConfirmArchive] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [conflict, setConflict] = useState(false);
  const [busy, setBusy] = useState(false);
  const [historyBusy, setHistoryBusy] = useState(false);
  const [groups, setGroups] = useState<Group[]>([]);
  const [groupsAttempt, setGroupsAttempt] = useState(0);
  const [groupsLoading, setGroupsLoading] = useState(true);
  const [groupsError, setGroupsError] = useState('');
  const pending = useRef(false);
  const epoch = useRef(0);
  const heading = useRef<HTMLHeadingElement>(null);
  const base: `/api/${string}` = `/api/clubs/${clubId}/participants`;
  // Calendar date in this club, without converting birth dates through UTC.
  const parts = new Intl.DateTimeFormat('en', { timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(new Date());
  const today = ['year', 'month', 'day'].map((name) => parts.find((part) => part.type === name)?.value).join('-');

  useEffect(() => {
    const controller = new AbortController(); setGroupsLoading(true); setGroupsError('');
    apiGet<Group[]>(`/api/clubs/${clubId}/groups?state=all`, controller.signal)
      .then((data) => { if (!controller.signal.aborted) setGroups(data); })
      .catch((cause: unknown) => { if (!controller.signal.aborted) setGroupsError(message(cause)); })
      .finally(() => { if (!controller.signal.aborted) setGroupsLoading(false); });
    return () => controller.abort();
  }, [clubId, groupsAttempt]);

  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setLoadError(''); setList({ items: [], hasMore: false });
    const params = new URLSearchParams({ q: query, state: filter, page: String(page) });
    apiGet<List>(`${base}?${params}`, controller.signal)
      .then((result) => { if (!controller.signal.aborted) setList(result); })
      .catch((cause: unknown) => { if (!controller.signal.aborted) setLoadError(message(cause)); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [base, query, filter, page, attempt]);
  useEffect(() => {
    const controller = new AbortController(); setCard(null); setCardError('');
    if (!selected || selected === 'new') { setCardLoading(false); return () => controller.abort(); }
    setCardLoading(true);
    apiGet<Card>(`${base}/${selected}`, controller.signal)
      .then((result) => { if (!controller.signal.aborted) setCard(result); })
      .catch((cause: unknown) => { if (!controller.signal.aborted) setCardError(message(cause)); })
      .finally(() => { if (!controller.signal.aborted) setCardLoading(false); });
    return () => controller.abort();
  }, [base, selected, cardAttempt]);
  useEffect(() => () => { epoch.current += 1; }, []);
  useEffect(() => { if (selected) heading.current?.focus(); }, [selected, cardLoading, editing, confirmArchive]);

  useEffect(() => { setSelected(detailId || null); setEditing(detailId === 'new'); setConfirmArchive(false); setError(''); setConflict(false); }, [detailId]);
  function openNow(id: string) { if (id !== 'new') onDetailChange?.(id); setSelected(id); setEditing(id === 'new'); setConfirmArchive(false); setError(''); setNotice(''); setConflict(false); setCardAttempt((value) => value + 1); }
  function closeNow() { onDetailChange?.(''); setSelected(null); setCard(null); setEditing(false); setConfirmArchive(false); setError(''); setConflict(false); }
  function open(id: string) { if (beforeNavigate) beforeNavigate(() => openNow(id)); else openNow(id); }
  function close() { if (beforeNavigate) beforeNavigate(closeNow); else closeNow(); }
  function refreshCard() { setEditing(false); setConfirmArchive(false); setError(''); setConflict(false); setCardAttempt((value) => value + 1); setAttempt((value) => value + 1); }
  async function mutate(action: () => Promise<Card>, success: string) {
    if (pending.current) return;
    pending.current = true; setBusy(true); setError(''); setNotice(''); setConflict(false);
    const generation = epoch.current;
    try {
      const result = await action(); if (generation !== epoch.current) return;
      setCard(result); setSelected(result.id); onDetailChange?.(result.id); setEditing(false); setConfirmArchive(false); setNotice(success); setAttempt((value) => value + 1);
    } catch (cause) {
      if (generation !== epoch.current) return;
      setError(message(cause));
      if (cause instanceof ApiError && (cause.code === 'version_conflict' || cause.code === 'identity_owner_required')) { setConflict(true); setAttempt((value) => value + 1); }
      if (cause instanceof ApiError && (cause.code === 'csrf_failed' || cause.code === 'club_archived' || cause.status === 404)) await onSessionRefresh();
    } finally { if (generation === epoch.current) { pending.current = false; setBusy(false); } }
  }
  function save(draft: Draft) {
    const local = { joinedOn: draft.joinedOn, contactKind: draft.contactKind, contactValue: draft.contactValue, contactName: draft.contactName };
    if (selected === 'new') {
      void mutate(() => apiPost<Card>(base, { ...local, identity: { fullName: draft.fullName, birthDate: draft.birthDate, version: 0 }, version: 0, ...(draft.groupId ? { initialGroup: { groupId: draft.groupId, from: draft.groupFrom } } : {}) }), 'Участник добавлен.');
    } else if (card) {
      const identity = card.canEditIdentity && (draft.fullName !== card.fullName || draft.birthDate !== card.birthDate)
        ? { fullName: draft.fullName, birthDate: draft.birthDate, version: card.participantVersion } : undefined;
      void mutate(() => apiPut<Card>(`${base}/${card.id}`, { ...local, version: card.version, identity }), 'Карточка сохранена.');
    }
  }
  function search(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setQuery(queryDraft.trim()); setPage(0); setAttempt((value) => value + 1); }

  return <section className="connection registry" aria-labelledby="registry-title">
    <div className="section-heading"><div><h2 id="registry-title">Участники филиала</h2><p className="help context-name">{clubName}</p></div>
      <button disabled={busy} onClick={() => open('new')}>Добавить участника</button></div>
    {notice && <p role="status">{notice}</p>}
      {editing && (selected === 'new' || card) && <Dialog className="subject-panel" title={selected === 'new' ? 'Новый участник' : 'Редактирование участника'} busy={busy} cancelLabel="Закрыть форму" onCancel={() => { if (beforeNavigate) beforeNavigate(() => { if (selected === 'new') closeNow(); else setEditing(false); }); else if (selected === 'new') closeNow(); else setEditing(false); }}>
        <p className="help context-name">{clubName}</p>
        {error && <p className="error" role="alert">{error}</p>}
        {conflict && <><p className="help">Ваши правки остались в форме. Загрузка актуальной карточки сбросит их.</p><button className="secondary" disabled={busy} onClick={refreshCard}>Загрузить актуальную карточку</button></>}
        <ParticipantEditor key={selected} card={selected === 'new' ? null : card} busy={busy} today={today} groups={groups} groupsLoading={groupsLoading} groupsError={groupsError} onRetryGroups={() => setGroupsAttempt((value) => value + 1)} onSave={save} onCancel={() => { if (selected === 'new') closeNow(); else { setEditing(false); setError(''); setConflict(false); } }} />
      </Dialog>}
    {selected && !editing && <Dialog className="registry-card participant-detail subject-panel" title={card?.fullName || 'Карточка участника'} busy={busy || historyBusy} onCancel={close} cancelLabel="Закрыть карточку">

      {!editing && error && <p className="error" role="alert">{error}</p>}
      {!editing && conflict && <><p className="help">Ваши правки остались в форме. Загрузка актуальной карточки сбросит их.</p><button className="secondary" disabled={busy} onClick={refreshCard}>Загрузить актуальную карточку</button></>}
      {cardLoading && <p role="status">Загружаем карточку…</p>}
      {cardError && <div role="alert"><p>{cardError}</p><button className="secondary" onClick={refreshCard}>Попробовать снова</button></div>}

      {card && !editing && <>
        <dl><dt>Дата рождения</dt><dd>{displayDate(card.birthDate)}</dd><dt>Дата вступления</dt><dd>{displayDate(card.joinedOn)}</dd>
          <dt>{card.contactKind === 'phone' ? 'Телефон' : 'Электронная почта'}</dt><dd>{card.contactValue}</dd><dt>Контактное лицо</dt><dd>{card.contactName || 'Не указано'}</dd><dt>Состояние карточки</dt><dd>{card.archived ? 'В архиве' : 'Действующая'}</dd></dl>
        {!card.canEditIdentity && <p className="help">Общие ФИО и дату рождения меняет владелец сети.</p>}
        {confirmArchive ? <div role="region" aria-label="Подтверждение архива"><p>Перенести карточку в архив этого филиала? История занятий и отметки сохранятся. Карточку можно вернуть.</p>
          <div className="actions"><button disabled={busy} onClick={() => void mutate(() => apiPost<Card>(`${base}/${card.id}/archive`, { version: card.version }), 'Карточка в архиве. История сохранена.')}>Подтвердить архивирование</button><button className="secondary" disabled={busy} onClick={() => setConfirmArchive(false)}>Отмена</button></div></div> : <div className="actions">
          <button disabled={busy} onClick={() => { setEditing(true); setError(''); setConflict(false); }}>Редактировать участника</button>
          {card.archived ? <button className="secondary" disabled={busy} onClick={() => void mutate(() => apiPost<Card>(`${base}/${card.id}/restore`, { version: card.version }), 'Карточка возвращена из архива.')}>Вернуть из архива</button> : <button className="secondary" disabled={busy} onClick={() => setConfirmArchive(true)}>В архив</button>}
        </div>}
        <MembershipHistory onBusyChange={setHistoryBusy} onChanged={() => setAttempt((value) => value + 1)} key={`${card.id}:${card.version}`} clubId={clubId} membershipId={card.id} archived={card.archived} joinedOn={card.joinedOn} groups={groups} />
      </>}
    </Dialog>}
    <form className="registry-search" onSubmit={search}><label className="field">Поиск по ФИО или контакту<input type="search" maxLength={200} value={queryDraft} onChange={(event) => setQueryDraft(event.target.value)} /></label><button type="submit" disabled={busy}>Найти</button></form>
    <div className="section-heading"><label className="field">Состояние<select disabled={busy} value={filter} onChange={(event) => { setFilter(event.target.value); setPage(0); }}><option value="active">Действующие</option><option value="archived">В архиве</option><option value="all">Все карточки</option></select></label>
      <button className="secondary" disabled={busy || loading} onClick={() => setAttempt((value) => value + 1)}>Обновить участников</button></div>
    {loading && <p role="status">Загружаем участников…</p>}
    {loadError && <div role="alert"><p>{loadError}</p><button className="secondary" onClick={() => setAttempt((value) => value + 1)}>Попробовать снова</button></div>}
    {!loading && !loadError && <>
      {list.items.length === 0 && <p>{query ? 'По вашему запросу участников нет. Уточните ФИО или контакт.' : filter === 'archived' ? 'В архиве пока нет участников.' : 'В этом списке пока нет участников. Добавьте первую карточку с датой рождения и контактом.'}</p>}
      <div className="registry-columns" aria-hidden="true"><span>Участник</span><span>Дата рождения</span><span>Контакт</span></div>
      <ul className="registry-list participant-rows">{list.items.map((item) => <li key={item.id}><div className="participant-name-cell"><div className="participant-name"><StatusDot active={!item.archived} label={item.archived ? 'В архиве' : 'Действующая карточка'} /><button type="button" className="name-link" disabled={busy} onClick={() => open(item.id)} aria-label={`Открыть карточку ${item.fullName}, ${displayDate(item.birthDate)}`}>{item.fullName}</button></div><p className="row-meta">{clubName}</p></div><div className="row-birth"><span className="mobile-label">Дата рождения</span>{displayDate(item.birthDate)}</div><div className="row-contact"><span className="mobile-label">Контакт</span><span>{item.contactValue}</span><p className="row-meta">{item.contactName || 'Контакт родителя или взрослого'}</p></div></li>)}</ul>
      <div className="actions"><button className="secondary" disabled={busy || page === 0} onClick={() => setPage((value) => value - 1)}>Предыдущие</button><span>Страница {page + 1}</span><button className="secondary" disabled={busy || !list.hasMore} onClick={() => setPage((value) => value + 1)}>Следующие</button></div>
    </>}
  </section>;
}

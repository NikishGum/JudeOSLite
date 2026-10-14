import { useEffect, useId, useRef, useState, type FormEvent } from 'react';
import { ApiError, apiGet, apiPost, apiPut } from '../../shared/api';
import { MembershipHistory } from './MembershipHistory';
import { displayDate, type Group, type Interval } from './types';
import type { Card } from '../participants/ParticipantEditor';
import { Dialog } from '../../shared/ui/Dialog';
import '../participants/participants.css';
import './groups.css';
const message = (cause: unknown) => cause instanceof ApiError ? cause.message : 'Нет соединения. Проверьте интернет и повторите действие.';

export function GroupManagement({ clubId, clubName, timezone, detailId = '', beforeNavigate, onDetailChange, onOpenSessions }: { clubId: string; clubName: string; timezone: string; detailId?: string; beforeNavigate: (action: () => void) => void; onDetailChange: (id: string) => void; onOpenSessions: (id: string) => void }) {
  const stateSelectId = useId();
  const base: `/api/${string}` = `/api/clubs/${clubId}/groups`;
  const [groups, setGroups] = useState<Group[]>([]);
  const [filter, setFilter] = useState('active');
  const [attempt, setAttempt] = useState(0);
  const [rosterAttempt, setRosterAttempt] = useState(0);
  const [cardsAttempt, setCardsAttempt] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [selected, setSelected] = useState(detailId);
  const [editor, setEditor] = useState<Group | 'new' | null>(null);
  const [name, setName] = useState('');
  const [archive, setArchive] = useState<Group | null>(null);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [conflict, setConflict] = useState(false);
  const [busy, setBusy] = useState(false);
  const [date, setDate] = useState(() => {
    const parts = new Intl.DateTimeFormat('en', { timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(new Date());
    return ['year', 'month', 'day'].map((part) => parts.find((p) => p.type === part)?.value).join('-');
  });
  const [roster, setRoster] = useState<Interval[]>([]);
  const [rosterLoading, setRosterLoading] = useState(false);
  const [rosterError, setRosterError] = useState('');
  const [search, setSearch] = useState('');
  const [query, setQuery] = useState('');
  const [cards, setCards] = useState<{ items: Card[]; hasMore: boolean }>({ items: [], hasMore: false });
  const [page, setPage] = useState(0);
  const [cardsLoading, setCardsLoading] = useState(true);
  const [cardsError, setCardsError] = useState('');
  const [member, setMember] = useState<Card | null>(null);
  const pending = useRef(false);
  const mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setLoadError('');
    apiGet<Group[]>(`${base}?state=all`, controller.signal).then((v) => { if (!controller.signal.aborted) setGroups(v); })
      .catch((e: unknown) => { if (!controller.signal.aborted) setLoadError(message(e)); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [base, attempt]);
  useEffect(() => {
    const controller = new AbortController(); setRoster([]); setRosterError('');
    if (!selected || !date) { setRosterLoading(false); return () => controller.abort(); }
    setRosterLoading(true);
    apiGet<Interval[]>(`${base}/${selected}/roster?date=${encodeURIComponent(date)}`, controller.signal).then((v) => { if (!controller.signal.aborted) setRoster(v); })
      .catch((e: unknown) => { if (!controller.signal.aborted) setRosterError(message(e)); })
      .finally(() => { if (!controller.signal.aborted) setRosterLoading(false); });
    return () => controller.abort();
  }, [base, selected, date, rosterAttempt]);
  useEffect(() => {
    const controller = new AbortController(); setCardsLoading(true); setCardsError(''); setCards({ items: [], hasMore: false });
    const params = new URLSearchParams({ q: query, page: String(page), state: 'all' });
    apiGet<{ items: Card[]; hasMore: boolean }>(`/api/clubs/${clubId}/participants?${params}`, controller.signal).then((v) => { if (!controller.signal.aborted) setCards(v); })
      .catch((e: unknown) => { if (!controller.signal.aborted) setCardsError(message(e)); })
      .finally(() => { if (!controller.signal.aborted) setCardsLoading(false); });
    return () => controller.abort();
  }, [clubId, query, page, cardsAttempt]);
  useEffect(() => { setSelected(detailId); setMember(null); }, [detailId]);
  function openGroup(id: string) { beforeNavigate(() => { setSelected(id); setMember(null); onDetailChange(id); }); }
  function closeEditor() { beforeNavigate(() => { setEditor(null); setError(''); setConflict(false); }); }
  async function mutate(action: () => Promise<Group>, text: string) {
    if (pending.current) return; pending.current = true; setBusy(true); setError(''); setNotice(''); setConflict(false);
    try { await action(); if (!mounted.current) return; setEditor(null); setArchive(null); setNotice(text); setAttempt((v) => v + 1); }
    catch (cause) { if (mounted.current) { setError(message(cause)); setConflict(cause instanceof ApiError && cause.code === 'version_conflict'); } }
    finally { if (mounted.current) { pending.current = false; setBusy(false); } }
  }
  function save(event: FormEvent<HTMLFormElement>) { event.preventDefault(); if (!editor) return; void mutate(() => editor === 'new' ? apiPost<Group>(base, { name, version: 0 }) : apiPut<Group>(`${base}/${editor.id}`, { name, version: editor.version }), 'Группа сохранена.'); }
  function refresh() { setEditor(null); setArchive(null); setError(''); setConflict(false); setAttempt((v) => v + 1); }
  return <section className="connection registry groups" aria-labelledby="groups-title">
    <div className="section-heading"><div><h2 id="groups-title">Группы филиала</h2><p className="help context-name">{clubName}</p></div><button disabled={busy} onClick={() => beforeNavigate(() => { setEditor('new'); setName(''); setArchive(null); setError(''); setConflict(false); })}>Создать группу</button></div>
    {notice && <p role="status">{notice}</p>}{!editor && error && <p className="error" role="alert">{error}</p>}{!editor && conflict && <p className="help">Форма сохранена. Обновление групп сбросит правки и загрузит актуальные версии.</p>}
    {editor && <Dialog className="subject-panel" title={editor === 'new' ? 'Новая группа' : 'Изменить группу'} busy={busy} onCancel={closeEditor} cancelLabel="Закрыть форму"><p className="help context-name">{clubName}</p>{error && <p className="error" role="alert">{error}</p>}{conflict && <><p className="help">Правки остались в форме. Перечитывание сбросит их.</p><button className="secondary" disabled={busy} onClick={refresh}>Загрузить актуальные группы</button></>}<form onSubmit={save} aria-busy={busy}><fieldset disabled={busy}><label className="field">Название группы<input required maxLength={200} value={name} onChange={(e) => setName(e.target.value)} /></label><div className="actions"><button type="submit">Сохранить группу</button><button type="button" className="secondary" onClick={() => setEditor(null)}>Отменить правку</button></div></fieldset></form></Dialog>}
    {archive && <div role="region" aria-label="Подтверждение архива группы"><p>Перенести «{archive.name}» в архив? Новое включение будет запрещено. История состава и снимки занятий сохранятся. Участников можно перевести в действующую группу.</p><div className="actions"><button disabled={busy} onClick={() => void mutate(() => apiPost<Group>(`${base}/${archive.id}/archive`, { version: archive.version }), 'Группа в архиве. История сохранена.')}>Подтвердить архив группы</button><button className="secondary" disabled={busy} onClick={() => setArchive(null)}>Отмена архива группы</button></div></div>}
    <div className="section-heading"><div className="field"><label htmlFor={stateSelectId}>Состояние групп</label><select id={stateSelectId} value={filter} onChange={(e) => setFilter(e.target.value)}><option value="active">Действующие</option><option value="archived">В архиве</option><option value="all">Все группы</option></select></div><button className="secondary" disabled={busy || loading} onClick={refresh}>Обновить группы</button></div>
    {loading && <p role="status">Загружаем группы…</p>}{loadError && <div role="alert"><p>{loadError}</p><button onClick={refresh}>Повторить загрузку групп</button></div>}
    {!loading && !loadError && <ul className="registry-list">{groups.filter((g) => filter === 'all' || g.archived === (filter === 'archived')).map((g) => <li key={g.id}><div><h4>{g.name}</h4><p>{g.archived ? 'В архиве' : 'Действующая группа'}</p></div><div className="actions"><button className="secondary" disabled={busy} onClick={() => openGroup(g.id)}>Показать состав {g.name}</button>{!g.archived && <><button className="secondary" disabled={busy} onClick={() => beforeNavigate(() => { setEditor(g); setName(g.name); setError(''); setConflict(false); })}>Изменить {g.name}</button><button className="secondary" disabled={busy} onClick={() => beforeNavigate(() => { setArchive(g); setEditor(null); })}>В архив {g.name}</button></>}</div></li>)}</ul>}
    {!loading && !loadError && groups.filter((g) => filter === 'all' || g.archived === (filter === 'archived')).length === 0 && <p>{groups.length === 0 ? 'Групп пока нет. Создайте первую группу.' : 'Групп с выбранным состоянием нет.'}</p>}
    {selected && <div className="registry-card group-detail"><div className="section-heading"><h3>Состав: {groups.find((g) => g.id === selected)?.name || 'Группа'}</h3><div className="actions"><button className="secondary" onClick={() => onOpenSessions(selected)}>К занятиям группы</button><button className="secondary" onClick={() => openGroup('')}>Закрыть состав</button></div></div><label className="field">Состав на дату<input type="date" required min="0001-01-01" value={date} onChange={(e) => setDate(e.target.value)} /></label><p className="help">Начало периода включено, конец исключён. Показан состав именно на выбранную дату, включая архивные карточки. Снимки открытых и закрытых занятий хранятся отдельно.</p>{rosterLoading && <p role="status">Загружаем состав…</p>}{rosterError && <div role="alert"><p>{rosterError}</p><button onClick={() => setRosterAttempt((v) => v + 1)}>Повторить загрузку состава</button></div>}{!rosterLoading && !rosterError && <><p>На {date ? displayDate(date) : 'выбранную дату'}: {roster.length} участников.</p><ul className="registry-list">{roster.map((i) => <li key={i.id}><div><strong>{i.fullName}</strong><p>{displayDate(i.birthDate)}{i.cardArchived ? ' · карточка в архиве' : ''}</p></div></li>)}</ul></>}</div>}
    <div className="registry-card"><h3>Изменить состав и посмотреть историю участника</h3><form className="registry-search" onSubmit={(e) => { e.preventDefault(); setQuery(search.trim()); setPage(0); }}><label className="field">Найти участника для состава<input type="search" maxLength={200} value={search} onChange={(e) => setSearch(e.target.value)} /></label><button type="submit">Найти в реестре</button></form>{cardsLoading && <p role="status">Загружаем реестр…</p>}{cardsError && <div role="alert"><p>{cardsError}</p><button onClick={() => setCardsAttempt((v) => v + 1)}>Повторить загрузку реестра</button></div>}{!cardsLoading && !cardsError && <><ul className="registry-list">{cards.items.map((c) => <li key={c.id}><div><strong>{c.fullName}</strong><p>{displayDate(c.birthDate)} · {c.archived ? 'В архиве' : 'Действующая карточка'}</p></div><button className="secondary" onClick={() => beforeNavigate(() => setMember(c))}>Группы участника {c.fullName}</button></li>)}</ul><div className="actions"><button className="secondary" disabled={page === 0} onClick={() => setPage((v) => v - 1)}>Назад по реестру</button><span>Страница {page + 1}</span><button className="secondary" disabled={!cards.hasMore} onClick={() => setPage((v) => v + 1)}>Далее по реестру</button></div></>}
      {member && <div className="member-detail"><h4>{member.fullName}</h4><MembershipHistory key={member.id} clubId={clubId} membershipId={member.id} archived={member.archived} joinedOn={member.joinedOn} groups={groups} onChanged={() => setRosterAttempt((v) => v + 1)} /></div>}
    </div>
  </section>;
}

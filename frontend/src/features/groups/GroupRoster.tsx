import { useEffect, useRef, useState, type FormEvent } from 'react';
import { ApiError, apiGet, apiPost } from '../../shared/api';
import { beginMutation, endMutation } from '../../shared/app-lifecycle';
import type { Card } from '../participants/ParticipantEditor';
import { message } from '../schedule/types';
import { trainingChanged } from '../sessions/navigation';
import { GroupMemberDialog } from './GroupMemberDialog';
import { displayDate, type Group, type Interval } from './types';

type Result = { card: Card; state: 'saved' | 'failed' | 'unknown'; detail: string };
export function GroupRoster({ clubId, group, groups, today, date, onDate, beforeNavigate }: {
  clubId: string; group: Group; groups: Group[]; today: string; date: string; onDate: (date: string) => void; beforeNavigate: (action: () => void) => void;
}) {
  const [roster, setRoster] = useState<Interval[]>([]), [attempt, setAttempt] = useState(0);
  const [loading, setLoading] = useState(true), [error, setError] = useState('');
  const [adding, setAdding] = useState(false), [search, setSearch] = useState(''), [query, setQuery] = useState(''), [page, setPage] = useState(0);
  const [cards, setCards] = useState<{ items: Card[]; hasMore: boolean }>({ items: [], hasMore: false });
  const [cardsLoading, setCardsLoading] = useState(false), [cardsError, setCardsError] = useState('');
  const [selected, setSelected] = useState<Card[]>([]), [from, setFrom] = useState(today);
  const [formKey, setFormKey] = useState(0);
  const [results, setResults] = useState<Result[]>([]), [busy, setBusy] = useState(false);
  const [member, setMember] = useState<{ interval: Interval; action: 'transfer' | 'exclude' | 'history' } | null>(null);
  const [notice, setNotice] = useState('');
  const pending = useRef(false), mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  useEffect(() => {
    const c = new AbortController(); setLoading(true); setError(''); setRoster([]);
    if (!date) { setLoading(false); setError('Выберите дату состава.'); return () => c.abort(); }
    apiGet<Interval[]>(`/api/clubs/${clubId}/groups/${group.id}/roster?date=${date}`, c.signal)
      .then((v) => { if (!c.signal.aborted) setRoster(v); }).catch((e: unknown) => { if (!c.signal.aborted) setError(message(e)); })
      .finally(() => { if (!c.signal.aborted) setLoading(false); });
    return () => c.abort();
  }, [clubId, group.id, date, attempt]);
  useEffect(() => {
    if (!adding) return;
    const c = new AbortController(); setCardsLoading(true); setCardsError(''); setCards({ items: [], hasMore: false });
    const params = new URLSearchParams({ q: query, page: String(page), state: 'active' });
    apiGet<{ items: Card[]; hasMore: boolean }>(`/api/clubs/${clubId}/participants?${params}`, c.signal)
      .then((v) => { if (!c.signal.aborted) setCards(v); }).catch((e: unknown) => { if (!c.signal.aborted) setCardsError(message(e)); })
      .finally(() => { if (!c.signal.aborted) setCardsLoading(false); });
    return () => c.abort();
  }, [clubId, adding, query, page, attempt]);
  function changed() { setMember(null); setAttempt((v) => v + 1); setNotice('Состав сохранён. История и снимки занятий сохранены.'); trainingChanged(); }
  async function verify(card: Card) {
    if (pending.current) return;
    pending.current = true; setBusy(true);
    try {
      const history = await apiGet<Interval[]>(`/api/clubs/${clubId}/participants/${card.id}/groups`);
      if (!mounted.current) return;
      const saved = history.some((i) => i.groupId === group.id && i.from === from && i.until === null);
      setResults((old) => old.map((r) => r.card.id === card.id ? { card, state: saved ? 'saved' : 'failed', detail: saved ? 'Добавление подтверждено по истории' : 'Изменение не найдено в истории. Можно повторить.' } : r));
      if (saved) { setSelected((old) => old.filter((c) => c.id !== card.id)); setAttempt((v) => v + 1); trainingChanged(); }
    } catch { /* Keep the explicit unknown result until the server can be read. */ }
    finally { if (mounted.current) { pending.current = false; setBusy(false); } }
  }
  async function add(event: FormEvent) {
    event.preventDefault(); if (pending.current || !selected.length) return;
    // Independent, dated commands: never present this as an atomic batch.
    try { beginMutation(); } catch (cause) { setCardsError(message(cause)); return; }
    pending.current = true; setBusy(true); setResults([]); setNotice('');
    const outcomes: Result[] = [];
    try {
      for (const card of selected) {
        if (!mounted.current) break;
        const base: `/api/${string}` = `/api/clubs/${clubId}/participants/${card.id}/groups`;
        try { await apiPost(base, { groupId: group.id, from, until: null }); outcomes.push({ card, state: 'saved', detail: 'Добавлен' }); }
        catch (cause) {
          if (!mounted.current) break;
          // A lost response may follow a commit. Reconcile before offering a retry.
          if (!(cause instanceof ApiError) || cause.status === 0 || cause.status >= 500) {
            try {
              const history = await apiGet<Interval[]>(base);
              const found = history.some((i) => i.groupId === group.id && i.from === from && i.until === null);
              outcomes.push({ card, state: found ? 'saved' : 'failed', detail: found ? 'Добавление подтверждено по истории' : 'Изменение не найдено в истории. Можно повторить.' });
            } catch { outcomes.push({ card, state: 'unknown', detail: 'Результат неизвестен. Проверьте историю участника перед повтором.' }); }
          } else outcomes.push({ card, state: 'failed', detail: message(cause) });
        }
        if (mounted.current) setResults([...outcomes]);
      }
      if (!mounted.current) return;
      setSelected((old) => old.filter((card) => !outcomes.some((r) => r.card.id === card.id && r.state === 'saved')));
      if (outcomes.length === selected.length && outcomes.every((r) => r.state === 'saved')) setFormKey((v) => v + 1);
      if (outcomes.some((r) => r.state === 'saved')) { setAttempt((v) => v + 1); trainingChanged(); }
    } finally { endMutation(); if (mounted.current) { pending.current = false; setBusy(false); } }
  }
  return <div className="group-roster">
    <div className="section-heading"><h4>Участники группы</h4>{!group.archived && <button disabled={busy} onClick={() => beforeNavigate(() => { setAdding(true); setNotice(''); })}>Добавить участников</button>}</div>
    {notice && <p role="status">{notice}</p>}
    <div className="group-roster-date"><label className="field">Состав на дату<input type="date" min="0001-01-01" required value={date} onChange={(e) => onDate(e.target.value)} /></label><button className="secondary" disabled={loading || busy} onClick={() => setAttempt((v) => v + 1)}>Обновить состав</button></div>
    <p className="help">Просмотр истории на выбранную дату. Дата добавления или перевода задаётся отдельно.</p>
    {loading && <p role="status">Загружаем состав…</p>}{error && <div role="alert"><p>{error}</p><button onClick={() => setAttempt((v) => v + 1)}>Повторить загрузку состава</button></div>}
    {!loading && !error && <><p>На {displayDate(date)}: {roster.length} участников.</p>{roster.length === 0 && <p>В составе на эту дату никого нет.{!group.archived && ' Добавьте участников из реестра.'}</p>}<ul className="group-people">{roster.map((i) => <li key={i.id}><div><strong>{i.fullName}</strong><p className="help">{displayDate(i.birthDate)}{i.cardArchived ? ' · карточка в архиве' : ''}</p></div><details><summary>Действия с участником<span className="sr-only"> {i.fullName}</span></summary><div className="actions">
      {!i.cardArchived && <button className="secondary" onClick={() => beforeNavigate(() => setMember({ interval: i, action: 'transfer' }))}>Перевести</button>}<button className="secondary" onClick={() => beforeNavigate(() => setMember({ interval: i, action: 'exclude' }))}>Завершить участие</button><button className="secondary" onClick={() => beforeNavigate(() => setMember({ interval: i, action: 'history' }))}>История участия</button>
    </div></details></li>)}</ul></>}
    {adding && <div className="group-add"><div className="section-heading"><h4>Добавить в «{group.name}»</h4><button className="secondary" disabled={busy} onClick={() => beforeNavigate(() => { setAdding(false); setSelected([]); setResults([]); })}>Закрыть добавление</button></div>
      <form className="registry-search" onSubmit={(e) => { e.preventDefault(); setQuery(search.trim()); setPage(0); }}><label className="field">Найти участника в реестре<input type="search" maxLength={200} value={search} onChange={(e) => setSearch(e.target.value)} /></label><button type="submit" disabled={busy}>Найти</button></form>
      {cardsLoading && <p role="status">Загружаем реестр…</p>}{cardsError && <div role="alert"><p>{cardsError}</p><button disabled={busy} onClick={() => setAttempt((v) => v + 1)}>Повторить загрузку реестра</button></div>}
      <form key={formKey} onSubmit={(e) => void add(e)} aria-busy={busy}><fieldset disabled={busy || results.some((r) => r.state === 'unknown')}>
        <ul className="group-pick-list">{cards.items.map((card) => <li key={card.id}><label><input type="checkbox" checked={selected.some((c) => c.id === card.id)} onChange={(e) => { setSelected((old) => e.target.checked ? [...old, card] : old.filter((c) => c.id !== card.id)); setResults([]); }} /><span><strong>{card.fullName}</strong><small>{displayDate(card.birthDate)}</small></span></label></li>)}</ul>
        {!cardsLoading && !cardsError && cards.items.length === 0 && <p>Участников по запросу не найдено.</p>}
        <div className="actions"><button type="button" className="secondary" disabled={page === 0} onClick={() => setPage((v) => v - 1)}>Назад по реестру</button><span>Страница {page + 1}</span><button type="button" className="secondary" disabled={!cards.hasMore} onClick={() => setPage((v) => v + 1)}>Далее по реестру</button></div>
        <p>Выбрано: {selected.length}. {selected.map((card) => card.fullName).join(', ')}</p>
        <label className="field">Дата зачисления<input type="date" required min={selected.reduce((max, card) => card.joinedOn > max ? card.joinedOn : max, '0001-01-01')} value={from} onChange={(e) => { setFrom(e.target.value); setResults([]); }} /></label>
        <p className="help">С {displayDate(from)} в группе «{group.name}». Каждый участник сохраняется отдельно. При частичной ошибке успешные добавления остаются, повтор отправляется только для оставшихся. Для уже состоящего в другой группе используйте перевод.</p>
        <button type="submit" disabled={!selected.length || results.some((r) => r.state === 'unknown')}>{busy ? 'Добавляем участников…' : `Добавить выбранных (${selected.length})`}</button>
      </fieldset></form>
      {results.length > 0 && <div role="status"><h4>Результат добавления</h4><ul>{results.map((r) => <li key={r.card.id}><strong>{r.card.fullName}</strong> — {r.detail}{r.state === 'unknown' && <button className="secondary" disabled={busy} onClick={() => void verify(r.card)}>Проверить результат</button>}</li>)}</ul></div>}
    </div>}
    {member && <GroupMemberDialog clubId={clubId} interval={member.interval} action={member.action} groups={groups} onClose={() => beforeNavigate(() => setMember(null))} onChanged={changed} />}
  </div>;
}

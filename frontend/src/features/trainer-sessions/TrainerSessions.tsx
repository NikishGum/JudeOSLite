import { useEffect, useRef, useState } from 'react';
import { ApiError, apiGet } from '../../shared/api';
import type { Session } from '../sessions/types';
import { sessionHref } from '../sessions/navigation';
import '../sessions/sessions.css';
import { SessionJournal } from './SessionJournal';
import { stateNames } from './types';
import './trainer-sessions.css';

type Props = { clubId: string; timezone: string };
export function TrainerSessions({ clubId, timezone }: Props) {
  const today = () => new Intl.DateTimeFormat('en-CA', { timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date());
  const [date, setDate] = useState(today);
  const [items, setItems] = useState<Session[]>([]);
  const [selected, setSelected] = useState<Session | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [attempt, setAttempt] = useState(0);
  const mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setError(''); setItems([]); setSelected(null);
    apiGet<Session[]>(`/api/clubs/${clubId}/trainer-sessions?date=${encodeURIComponent(date)}`, controller.signal)
      .then((result) => { if (!controller.signal.aborted) setItems(result); })
      .catch((cause: unknown) => { if (!controller.signal.aborted) setError(cause instanceof ApiError ? cause.message : 'Нет соединения. Повторите загрузку.'); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [clubId, date, attempt]);
  return <section className="connection trainer-sessions" aria-labelledby="trainer-sessions-title">
    <h2 id="trainer-sessions-title">Мои занятия и журнал</h2>
    <div className="journal-toolbar"><label className="field" htmlFor="trainer-date">Дата занятия<input id="trainer-date" type="date" required value={date} onChange={(event) => { if (event.target.value) setDate(event.target.value); }} /></label>
      <button onClick={() => setDate(today())}>Сегодня</button><button onClick={() => setAttempt((value) => value + 1)}>Обновить занятия</button></div>
    {loading && <p role="status">Загружаем занятия…</p>}
    {error && <div role="alert"><p>{error}</p><button onClick={() => setAttempt((value) => value + 1)}>Попробовать снова</button></div>}
    {!loading && !error && items.length === 0 && <p>На эту дату вам не назначены занятия.</p>}
    <ul className="trainer-session-list">{items.map((item) => <li key={item.id}>
      <div><strong>{item.groupName}</strong><p>{new Intl.DateTimeFormat('ru', { timeZone: item.timezone, hour: '2-digit', minute: '2-digit' }).format(new Date(item.startsAt))} · {stateNames[item.status]}</p>
        {item.status === 'cancelled' && <p>{item.cancellationReason}</p>}</div>
      <a className="session-link" href={sessionHref(clubId, { sessionId: item.id, journal: 'trainer', from: date, through: date, view: 'day' })}>Страница тренировки</a>
      {item.status !== 'cancelled' && <button disabled={selected?.id === item.id || (item.status === 'planned' && item.date > today())} onClick={() => setSelected(item)}>{item.status === 'planned' ? 'Открыть журнал' : 'Посмотреть журнал'}</button>}
    </li>)}</ul>
    {selected && <SessionJournal key={selected.id} clubId={clubId} initial={selected} onChanged={(item) => { if (mounted.current) setItems((values) => values.map((v) => v.id === item.id ? item : v)); }} />}
  </section>;
}

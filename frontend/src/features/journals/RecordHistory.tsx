import { useEffect, useRef, useState } from 'react';
import { apiGet, ApiError } from '../../shared/api';
import { type Event, markLabel, originLabel } from './types';
export function RecordHistory({ path, timezone, onUnavailable }: { path: `/api/${string}`; timezone: string; onUnavailable: () => void }) {
  const [events, setEvents] = useState<Event[]>([]), [loading, setLoading] = useState(true), [error, setError] = useState(''), [attempt, setAttempt] = useState(0);
  const unavailable = useRef(onUnavailable); unavailable.current = onUnavailable;
  useEffect(() => {
    const c = new AbortController(); setLoading(true); setError(''); setEvents([]);
    apiGet<Event[]>(path, c.signal).then((v) => { if (!c.signal.aborted) setEvents(v); })
      .catch((e: unknown) => { if (!c.signal.aborted) { setError(e instanceof ApiError ? e.message : 'Нет соединения. Повторите загрузку истории.'); if (e instanceof ApiError && (e.status === 401 || e.status === 404)) unavailable.current(); } })
      .finally(() => { if (!c.signal.aborted) setLoading(false); });
    return () => c.abort();
  }, [path, attempt]);
  return <div className="record-history">
    <h4>История отметки</h4>
    {loading && <p role="status">Загружаем историю…</p>}
    {error && <div role="alert"><p>{error}</p><button onClick={() => setAttempt((n) => n + 1)}>Повторить загрузку истории</button></div>}
    {!loading && !error && events.length === 0 && <p>Отметка ещё не менялась.</p>}
    <ol>{events.map((v) => <li key={v.id}>
      <p><strong>{v.actorName}</strong> · {new Intl.DateTimeFormat('ru', { timeZone: timezone, dateStyle: 'short', timeStyle: 'medium' }).format(new Date(v.createdAt))}</p>
      <p>{markLabel(v.before)} → {markLabel(v.after)}</p><p>{originLabel(v.after.origin)} · Версия {v.before.version} → {v.after.version}</p>
      {v.changeReason && <p className="change-reason">Причина исправления: {v.changeReason}</p>}
    </li>)}</ol>
  </div>;
}

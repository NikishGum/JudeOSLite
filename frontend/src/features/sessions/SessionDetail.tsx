import { useEffect, useState } from 'react';
import { apiGet } from '../../shared/api';
import { message, dateLabel, timeLabel } from '../schedule/types';
import { SessionJournal } from '../trainer-sessions/SessionJournal';
import { AdminJournal } from '../journals/AdminJournal';
import { sessionHref, type SessionLocation } from './navigation';
import type { Session } from './types';
import '../trainer-sessions/trainer-sessions.css';
import '../journals/journals.css';

export function SessionDetail({ clubId, location, canManage }: { clubId: string; location: SessionLocation; canManage: boolean }) {
  const [session, setSession] = useState<Session | null>(null), [error, setError] = useState(''), [attempt, setAttempt] = useState(0);
  const trainer = !canManage || location.journal === 'trainer';
  useEffect(() => {
    const controller = new AbortController(); setSession(null); setError('');
    const path = trainer ? `/api/clubs/${clubId}/trainer-sessions/${location.sessionId}/journal` : `/api/clubs/${clubId}/journals/${location.sessionId}`;
    apiGet<{ session: Session }>(path as `/api/${string}`, controller.signal).then((value) => {
      if (controller.signal.aborted) return;
      if (location.groupId && value.session.groupId !== location.groupId) { setError('Занятие не относится к выбранной группе. Вернитесь к списку занятий.'); return; }
      setSession(value.session);
    }).catch((cause: unknown) => { if (!controller.signal.aborted) setError(message(cause)); });
    return () => controller.abort();
  }, [clubId, location.sessionId, location.groupId, trainer, attempt]);
  return <section className="connection session-detail" aria-label="Выбранное занятие">
    <a className="session-link" href={sessionHref(clubId, { ...location, sessionId: '' })}>К списку занятий</a>
    {!session && !error && <p role="status">Загружаем выбранное занятие…</p>}
    {error && <div role="alert"><p>{error}</p><button onClick={() => setAttempt((n) => n + 1)}>Повторить загрузку занятия</button></div>}
    {session && <><p className="session-detail-time">{dateLabel(session.date)} · {timeLabel(session.startsAt, session.timezone)}–{timeLabel(session.endsAt, session.timezone)} · {session.timezone}</p>
      {trainer ? <SessionJournal key={`${session.id}:${attempt}`} clubId={clubId} initial={session} onChanged={setSession} />
        : <AdminJournal key={`${session.id}:${attempt}`} clubId={clubId} initial={session} onUnavailable={() => { setSession(null); setError('Журнал недоступен. Проверьте назначение и обновите страницу.'); }} />}</>}
  </section>;
}

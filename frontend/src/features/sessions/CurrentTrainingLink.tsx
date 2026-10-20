import { useEffect, useState } from 'react';
import { apiGet } from '../../shared/api';
import { localDate, timeLabel } from '../schedule/types';
import { sessionHref } from './navigation';
import { isHappening, nextDate, type Session } from './types';

export type TrainingContext = { userId: string; clubs: { id: string; timezone: string }[] };

export function CurrentTrainingLink({ context, sessionId, now }: { context: TrainingContext; sessionId: string; now: number }) {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [failed, setFailed] = useState(false), [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let controller: AbortController | undefined;
    const refresh = () => {
      controller?.abort();
      const request = new AbortController(); controller = request;
      // Include yesterday for sessions that continue across local midnight.
      void Promise.allSettled(context.clubs.map((club) => {
        const today = localDate(club.timezone);
        return apiGet<Session[]>(`/api/clubs/${club.id}/sessions?from=${nextDate(today, -1)}&until=${nextDate(today)}`, request.signal);
      })).then((results) => {
        if (request.signal.aborted) return;
        setFailed(results.some((result) => result.status === 'rejected'));
        setSessions(results.flatMap((result) => result.status === 'fulfilled' ? result.value : []).filter((item) => item.trainerId === context.userId));
      });
    };
    const visibleRefresh = () => { if (document.visibilityState === 'visible') refresh(); };
    refresh();
    const timer = window.setInterval(visibleRefresh, 30_000);
    window.addEventListener('focus', visibleRefresh);
    window.addEventListener('attendance-changed', visibleRefresh);
    document.addEventListener('visibilitychange', visibleRefresh);
    return () => { controller?.abort(); window.clearInterval(timer); window.removeEventListener('focus', visibleRefresh); window.removeEventListener('attendance-changed', visibleRefresh); document.removeEventListener('visibilitychange', visibleRefresh); };
  }, [context, attempt]);
  const current = sessions.filter((item) => item.id !== sessionId && isHappening(item, now));
  return <div className="session-current-links">
    {current.map((item) => <a key={item.id} className="session-link session-primary" href={sessionHref(item.clubId, { sessionId: item.id, journal: 'trainer' })}>К текущей тренировке{current.length > 1 ? ` · ${timeLabel(item.startsAt, item.timezone)} · ${item.groupName}` : ''}</a>)}
    {failed && <button type="button" className="secondary" onClick={() => setAttempt((value) => value + 1)}>Повторить поиск текущей тренировки</button>}
  </div>;
}

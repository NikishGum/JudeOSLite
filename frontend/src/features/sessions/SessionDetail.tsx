import { useEffect, useState } from 'react';
import { apiGet } from '../../shared/api';
import { message, dateLabel, statusNames, timeLabel, localDate } from '../schedule/types';
import { SessionJournal } from '../trainer-sessions/SessionJournal';
import { AdminJournal } from '../journals/AdminJournal';
import type { Journal } from '../journals/types';
import { sessionHref, weekStart, type SessionLocation } from './navigation';
import { durationLabel, isHappening, localDateOf, nextDate } from './types';
import { SessionRoster } from './SessionRoster';
import '../trainer-sessions/trainer-sessions.css';
import '../journals/journals.css';

export function SessionDetail({ clubId, timezone, location, canManage }: { clubId: string; timezone: string; location: SessionLocation; canManage: boolean }) {
  const [journal, setJournal] = useState<Journal | null>(null), [error, setError] = useState(''), [attempt, setAttempt] = useState(0);
  const [canTrain, setCanTrain] = useState(false), [now, setNow] = useState(Date.now);
  const trainer = !canManage || location.journal === 'trainer';
  useEffect(() => { const timer = window.setInterval(() => setNow(Date.now()), 30_000); return () => clearInterval(timer); }, []);
  useEffect(() => {
    const controller = new AbortController(); setJournal(null); setError(''); setCanTrain(false);
    const path = trainer ? `/api/clubs/${clubId}/trainer-sessions/${location.sessionId}/journal` : `/api/clubs/${clubId}/journals/${location.sessionId}`;
    Promise.all([apiGet<Journal>(path as `/api/${string}`, controller.signal), apiGet<{ user: { id: string }; clubs: { id: string; name: string; roles: string[] }[] }>('/api/auth/me', controller.signal)]).then(([value, me]) => {
      if (controller.signal.aborted) return;
      if (location.groupId && value.session.groupId !== location.groupId) { setError('Занятие не относится к выбранной группе. Вернитесь к расписанию.'); return; }
      setJournal(value); setCanTrain(me.user.id === value.session.trainerId && me.clubs.some((c) => c.id === clubId && c.roles.includes('trainer')));
      setClubName(me.clubs.find((c) => c.id === clubId)?.name ?? 'Филиал');
      if (new URLSearchParams(window.location.search).has('session')) window.history.replaceState(window.history.state, '', sessionHref(clubId, { ...location, from: location.hasCalendar ? location.from : undefined, through: location.hasCalendar ? location.through : undefined, view: location.hasCalendar ? location.view : undefined }));
    }).catch((cause: unknown) => { if (!controller.signal.aborted) setError(message(cause)); });
    return () => controller.abort();
  }, [clubId, location.sessionId, location.groupId, trainer, attempt]);
  const [clubName, setClubName] = useState('Филиал');
  const session = journal?.session;
  const back = location.hasCalendar || !session ? { ...location, sessionId: '' } : { sessionId: '', from: weekStart(session.date), through: nextDate(weekStart(session.date), 6), view: 'week' as const };
  return <section className="connection session-detail" aria-label="Выбранное занятие">
    <a className="session-link" href={sessionHref(clubId, back)}>К расписанию</a>
    {!journal && !error && <div className="session-loading" role="status">Загружаем выбранное занятие…</div>}
    {error && <div role="alert"><p>{error}</p><button onClick={() => setAttempt((n) => n + 1)}>Повторить загрузку занятия</button></div>}
    {journal && session && <>
      <header className="session-detail-heading"><div><h2>{session.groupName}</h2><p className="session-detail-time">{dateLabel(session.date)} · {timeLabel(session.startsAt, session.timezone)}–{timeLabel(session.endsAt, session.timezone)} · {durationLabel(session)}</p>
        {localDateOf(session.endsAt, session.timezone) !== session.date && <p>Окончание {dateLabel(localDateOf(session.endsAt, session.timezone))}</p>}
        <p className="help">{clubName} · {session.timezone}{session.timezone !== timezone && ` · текущая зона филиала ${timezone}`}</p>
        {canManage && !trainer && <p>Тренер: {session.trainerName}</p>}
        <p><span className={`training-status status-${session.status}`}>{statusNames[session.status]}</span>{isHappening(session, now) && <span className="session-now">Сейчас</span>}</p>
        {!session.ruleId && <p className="help">Разовое занятие</p>}{session.date !== session.sourceDate && <p>Перенесено с {dateLabel(session.sourceDate)}</p>}
        {session.previousTrainerName && <p>Замена тренера{canManage && !trainer ? ` · вместо ${session.previousTrainerName}` : ''}</p>}{session.cancellationReason && <p>Причина отмены: {session.cancellationReason}</p>}
      </div><div className="session-detail-actions">
        {trainer && session.status === 'planned' && session.date > localDate(session.timezone) && <p className="help">Открыть журнал можно с {dateLabel(session.date)}. Пока доступен предварительный состав.</p>}
        {trainer && (session.status === 'planned' || session.status === 'open') && <a className="session-link session-primary" href="#session-workspace">{session.status === 'planned' ? session.date > localDate(session.timezone) ? 'Посмотреть журнал' : 'Открыть журнал' : 'Продолжить отметку'}</a>}
        {!trainer && canTrain && <a className="session-link session-primary" href={sessionHref(clubId, { ...location, journal: 'trainer' })}>{session.status === 'open' ? 'Продолжить отметку' : 'Мой журнал тренера'}</a>}
        {!trainer && !canTrain && <p className="help">{session.status === 'closed' ? 'Коррекция одной отметки с причиной и историей доступна в журнале ниже.' : 'Отмечает фактический тренер. Коррекции доступны после завершения.'}</p>}
      </div></header>
      {journal.roster.summary ? <p className="session-attendance-summary" role="status">Присутствуют: {journal.roster.summary.present} · Отсутствуют: {journal.roster.summary.absent} · Не отмечено: {journal.roster.summary.unmarked}</p> : <p className="session-attendance-summary">Журнал ещё не открыт · фактической сводки нет</p>}
      <div className={`session-detail-layout ${trainer || journal.roster.source === 'preliminary' ? '' : 'session-detail-admin'}`}>
        <div id="session-workspace" className="session-workspace" tabIndex={-1}>
          {trainer ? <SessionJournal key={`${session.id}:${attempt}`} clubId={clubId} initial={session} initialJournal={journal} embedded onRead={setJournal} onChanged={() => {}} onUnavailable={() => { setJournal(null); setError('Доступ отозван или тренер заменён. Обновите страницу.'); }} />
            : <AdminJournal key={`${session.id}:${attempt}`} clubId={clubId} initial={session} initialJournal={journal} onRead={setJournal} onUnavailable={() => { setJournal(null); setError('Журнал недоступен. Проверьте назначение и обновите страницу.'); }} />}
        </div>
        {(trainer || journal.roster.source === 'preliminary') && <SessionRoster roster={journal.roster} cancelled={session.status === 'cancelled'} />}
      </div>
    </>}
  </section>;
}

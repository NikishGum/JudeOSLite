import { useEffect, useState } from 'react';
import { apiGet } from '../../shared/api';
import type { Group } from '../groups/types';
import { dateLabel, localDate, message, statusNames, timeLabel, type TrainerOption } from '../schedule/types';
import { SessionForm } from './SessionForm';
import { SessionDetail } from './SessionDetail';
import { sessionHref, sessionLocation, trainingChanged, validSessionIds, weekStart, type SessionLocation } from './navigation';
import { actionNames, durationLabel, isHappening, localDateOf, nextDate, type Action, type Session } from './types';
import './sessions.css';
import { DateNavigation } from '../../shared/ui/DateNavigation';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

export function SessionCalendar(props: { clubId: string; timezone: string; canManage: boolean }) {
  const location = sessionLocation(props.timezone);
  if (location.sessionId) return validSessionIds(location)
    ? <SessionDetail {...props} location={location} />
    : <section className="connection" role="alert"><p>Ссылка содержит неверный ID или фильтр.</p><a className="session-link" href={sessionHref(props.clubId)}>К расписанию</a></section>;
  return <CalendarView {...props} />;
}
function CalendarView({ clubId, timezone, canManage }: { clubId: string; timezone: string; canManage: boolean }) {
  const [jumpToday, setJumpToday] = useState(false);
  const [now, setNow] = useState(Date.now);
  const today = localDate(timezone);
  const [location, setLocation] = useState(() => sessionLocation(timezone));
  const { from, through, groupId, view, trainerId, status } = location;
  const [sessions, setSessions] = useState<Session[]>([]), [groups, setGroups] = useState<Group[]>([]), [trainers, setTrainers] = useState<TrainerOption[]>([]);
  const [attempt, setAttempt] = useState(0), [loading, setLoading] = useState(true), [optionsLoading, setOptionsLoading] = useState(true);
  const [error, setError] = useState(''), [optionsError, setOptionsError] = useState(''), [notice, setNotice] = useState('');
  const [editor, setEditor] = useState<'new' | { session: Session; action: Action } | null>(null);
  useEffect(() => {
    const changed = () => setAttempt((n) => n + 1);
    const timer = window.setInterval(() => setNow(Date.now()), 30_000);
    window.addEventListener('attendance-changed', changed);
    return () => { window.removeEventListener('attendance-changed', changed); window.clearInterval(timer); };
  }, []);
  useEffect(() => {
    const c = new AbortController(); setOptionsLoading(true); setOptionsError(''); setGroups([]); setTrainers([]);
    Promise.all([apiGet<Group[]>(`/api/clubs/${clubId}/sessions/groups`, c.signal), canManage ? apiGet<TrainerOption[]>(`/api/clubs/${clubId}/trainers?available=true`, c.signal) : Promise.resolve([])])
      .then(([g, t]) => { if (!c.signal.aborted) { setGroups(g); setTrainers(t); } })
      .catch((cause: unknown) => { if (!c.signal.aborted) setOptionsError(message(cause)); })
      .finally(() => { if (!c.signal.aborted) setOptionsLoading(false); });
    return () => c.abort();
  }, [clubId, canManage, attempt]);
  useEffect(() => {
    const c = new AbortController(); setLoading(true); setError(''); setSessions([]);
    if (!validSessionIds(location)) { setError('Ссылка содержит неверный ID группы, тренера или статус. Сбросьте фильтры.'); setLoading(false); return () => c.abort(); }
    if (!from || !through || through < from || (Date.parse(through) - Date.parse(from)) / 86400000 > 365) { setError('Выберите период до 366 дней: дата конца не раньше начала.'); setLoading(false); return () => c.abort(); }
    apiGet<Session[]>(`/api/clubs/${clubId}/sessions?from=${from}&until=${nextDate(through)}${groupId ? `&groupId=${encodeURIComponent(groupId)}` : ''}`, c.signal)
      .then((v) => { if (!c.signal.aborted) setSessions(v); }).catch((cause: unknown) => { if (!c.signal.aborted) setError(message(cause)); })
      .finally(() => { if (!c.signal.aborted) setLoading(false); });
    return () => c.abort();
  }, [clubId, from, through, groupId, attempt]);
  useEffect(() => {
    if (!loading && !optionsLoading && location.scrollY) {
      const frame = requestAnimationFrame(() => window.scrollTo(0, location.scrollY));
      return () => cancelAnimationFrame(frame);
    }
  }, [loading, optionsLoading]);
  function filters(patch: Partial<SessionLocation>) {
    const next = { ...location, ...patch, sessionId: '', scrollY: 0 }; setLocation(next); setNotice('');
    if (!validSessionIds(location)) setAttempt((n) => n + 1);
    window.history.replaceState(window.history.state, '', sessionHref(clubId, next));
  }
  function period(mode: SessionLocation['view'], date = from || today) {
    const start = mode === 'week' ? weekStart(date) : date;
    filters({ view: mode, from: start, through: mode === 'week' ? nextDate(start, 6) : mode === 'day' ? start : through < start ? start : through });
  }
  function refresh() { setEditor(null); setAttempt((n) => n + 1); }
  function changed() { setEditor(null); setNotice('Занятие сохранено.'); trainingChanged(); }
  const ready = !loading && !optionsLoading && !error && !optionsError;
  useEffect(() => {
    if (!jumpToday || !ready) return;
    const frame = requestAnimationFrame(() => {
      const days = document.querySelector('.session-days');
      if (days && getComputedStyle(days).gridTemplateColumns.split(' ').length === 1) document.getElementById(`day-${today}`)?.scrollIntoView({ block: 'start' });
      setJumpToday(false);
    });
    return () => cancelAnimationFrame(frame);
  }, [jumpToday, ready, today]);
  const selectedGroup = groups.find((g) => g.id === groupId);
  const actualTrainers = [...new Map(sessions.map((s) => [s.trainerId, s.trainerName])).entries()];
  const visible = sessions.filter((s) => (!trainerId || s.trainerId === trainerId) && (!status || s.status === status));
  const days = view === 'week' ? Array.from({ length: 7 }, (_, i) => nextDate(from, i)) : view === 'day' ? [from] : [...new Set(visible.map((s) => s.date))].sort();
  return <section className="connection registry session-calendar" aria-labelledby="sessions-title">
    <div className="section-heading"><div><h2 id="sessions-title">Календарь занятий</h2><p className="help">{canManage ? 'Занятия филиала' : 'Мои назначенные занятия'} · {timezone}</p></div>
      {canManage && <button disabled={!ready || !!editor} onClick={() => { setNotice(''); setEditor('new'); }}>Добавить разовое занятие</button>}
    </div>
    <div className="session-period-toolbar">
      <div className="session-week-navigation"><DateNavigation date={from} timezone={timezone} disabled={!!editor} step={view === 'week' ? 7 : 1} onChange={(date) => period(view === 'period' ? 'day' : view, date)} />
        <button className="secondary" disabled={!!editor} onClick={() => { period(view === 'period' ? 'day' : view, today); setJumpToday(true); }}>Сегодня</button></div>
      <ToggleGroup type="single" className="session-view" aria-label="Вид календаря" value={view} disabled={!!editor} onValueChange={(value) => { if (value === 'day' || value === 'week' || value === 'period') period(value); }}>{([['day', 'День'], ['week', 'Неделя'], ['period', 'Период']] as const).map(([mode, label]) => <ToggleGroupItem key={mode} value={mode}>{label}</ToggleGroupItem>)}</ToggleGroup>
    </div>
    <p className="session-range">{dateLabel(from)} — {dateLabel(through)}{view === 'period' && ' · даты включены'}</p>
    <div className="session-filters">
      {view === 'period' && <><label className="field">С даты<input type="date" value={from} disabled={!!editor} onChange={(e) => filters({ from: e.target.value })} /></label><label className="field">По дату включительно<input type="date" value={through} disabled={!!editor} onChange={(e) => filters({ through: e.target.value })} /></label></>}
      <div className="field"><label htmlFor="session-group-filter">Группа календаря</label><select id="session-group-filter" value={groupId} disabled={!!editor || optionsLoading} onChange={(e) => filters({ groupId: e.target.value })}><option value="">Все доступные группы</option>{groupId && !selectedGroup && <option value={groupId}>Группа из ссылки недоступна</option>}{groups.map((g) => <option key={g.id} value={g.id}>{g.name}{groups.filter((other) => other.name === g.name).length > 1 ? ` · ${g.id.slice(-8)}` : ''}{g.archived ? ' · в архиве' : ''}</option>)}</select></div>
      {canManage && <div className="field"><label htmlFor="session-trainer-filter">Фактический тренер</label><select id="session-trainer-filter" value={trainerId} disabled={!!editor || loading} onChange={(e) => filters({ trainerId: e.target.value })}><option value="">Все тренеры</option>{trainerId && !actualTrainers.some(([id]) => id === trainerId) && <option value={trainerId}>Нет занятий выбранного тренера</option>}{actualTrainers.map(([id, name]) => <option key={id} value={id}>{name}</option>)}</select></div>}
      <div className="field"><label htmlFor="session-status-filter">Статус занятия</label><select id="session-status-filter" value={status} disabled={!!editor} onChange={(e) => filters({ status: e.target.value })}><option value="">Все статусы</option>{Object.entries(statusNames).map(([value, name]) => <option key={value} value={value}>{name}</option>)}</select></div>
    </div>
    <div className="session-list-heading"><div>{(groupId || trainerId || status) && <button className="secondary" disabled={!!editor} onClick={() => filters({ groupId: '', trainerId: '', status: '' })}>Сбросить фильтры · Все группы</button>}{ready && <p role="status">Занятий за период: {visible.length}</p>}</div><button className="secondary" disabled={!!editor} onClick={refresh}>Обновить календарь</button></div>
    {notice && <p role="status">{notice}</p>}{(loading || optionsLoading) && <div className="session-loading" role="status">Загружаем занятия…</div>}
    {(error || optionsError) && <div role="alert"><p className="error">{error || optionsError}</p><button onClick={refresh}>Повторить загрузку занятий</button><a className="session-link" href={sessionHref(clubId)}>Сбросить параметры</a></div>}
    {editor && <SessionForm key={editor === 'new' ? 'new' : `${editor.session.id}:${editor.action}`} clubId={clubId} timezone={timezone} groups={groups} trainers={trainers} selectedGroup={groupId} session={editor === 'new' ? null : editor.session} action={editor === 'new' ? undefined : editor.action} onSaved={changed} onCancel={() => setEditor(null)} onRefresh={refresh} />}
    {ready && <><div className={`session-days ${view === 'week' ? 'session-week' : ''}`}>{days.map((day) => {
      const items = visible.filter((s) => s.date === day).sort((a, b) => a.startsAt.localeCompare(b.startsAt) || a.id.localeCompare(b.id));
      return <section className={`session-day ${day === today ? 'session-today' : ''}`} id={`day-${day}`} key={day} aria-label={`Занятия ${dateLabel(day)}`}>
        <h3 className="session-day-heading"><span>{new Intl.DateTimeFormat('ru', { weekday: 'short', timeZone: 'UTC' }).format(new Date(`${day}T12:00:00Z`))}</span><strong>{new Intl.DateTimeFormat('ru', { day: 'numeric', month: 'short', timeZone: 'UTC' }).format(new Date(`${day}T12:00:00Z`))}</strong><small>{day === today ? 'Сегодня · ' : ''}Занятий: {items.length}</small></h3>
        {items.length === 0 && <p className="session-empty-day">Нет занятий{(groupId || trainerId || status) ? ' по фильтрам' : ''}</p>}
        <ul className="registry-list session-list">{items.map((v) => <li key={v.id} data-session-id={v.id}>
          <a className="session-card-link" href={sessionHref(clubId, { ...location, sessionId: v.id, journal: canManage ? 'admin' : 'trainer' })} onClick={(event) => { event.currentTarget.href = sessionHref(clubId, { ...location, sessionId: v.id, journal: canManage ? 'admin' : 'trainer', scrollY: window.scrollY }); }}>
            <span className="session-clock"><strong>{timeLabel(v.startsAt, v.timezone)}–{timeLabel(v.endsAt, v.timezone)}</strong><span>{durationLabel(v)}</span></span><h4>{v.groupName}</h4><span className="session-card-cta">Открыть тренировку</span>
          </a>
          <div className="session-information">{canManage && <p>Тренер: {v.trainerName}</p>}<p><span className={`training-status status-${v.status}`}>{statusNames[v.status]}</span>{isHappening(v, now) && <span className="session-now">Сейчас</span>}</p>
            {!v.ruleId && <p className="help">Разовое занятие</p>}{v.previousTrainerName && <p className="help">Замена тренера{canManage ? ` · вместо ${v.previousTrainerName}` : ''}</p>}
            {v.timezone !== timezone && <p className="help">Время занятия: {v.timezone}</p>}{localDateOf(v.endsAt, v.timezone) !== v.date && <p className="help">Окончание {dateLabel(localDateOf(v.endsAt, v.timezone))}</p>}
            {v.date !== v.sourceDate && <p className="help">Перенесено с {dateLabel(v.sourceDate)}</p>}{v.cancellationReason && <p>Причина отмены: {v.cancellationReason}</p>}
          </div>
          {canManage && (v.status === 'planned' || v.status === 'open') && <div className="session-row-actions"><details><summary>Изменить занятие</summary><div className="actions">{(['replace-trainer', 'reschedule', 'cancel'] as Action[]).filter((a) => a !== 'reschedule' || (v.status === 'planned' && !v.rosterFrozen && Date.parse(v.startsAt) > now)).map((action) => <button key={action} className="secondary" disabled={!!editor} onClick={() => { setNotice(''); setEditor({ session: v, action }); window.scrollTo({ top: 0 }); }}>{actionNames[action]}</button>)}</div></details></div>}
        </li>)}</ul>
      </section>;
    })}</div>{visible.length === 0 && <div className="session-empty"><h3>{sessions.length ? 'Нет занятий по выбранным фильтрам' : 'Занятий пока нет'}</h3><p>{canManage ? 'Выберите другой период, сбросьте фильтры либо добавьте разовое занятие.' : 'В выбранном периоде вам не назначены занятия. Попробуйте другую дату.'}</p></div>}</>}
  </section>;
}

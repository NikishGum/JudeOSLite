import { useEffect, useState } from 'react';
import { apiGet } from '../../shared/api';
import type { Group } from '../groups/types';
import { dateLabel, localDate, message, statusNames, timeLabel, type TrainerOption } from '../schedule/types';
import { SessionForm } from './SessionForm';
import { SessionDetail } from './SessionDetail';
import { sessionHref, sessionLocation, trainingChanged, validSessionIds, weekStart, type SessionLocation } from './navigation';
import { actionNames, nextDate, type Action, type Session } from './types';
import './sessions.css';
import { DateNavigation } from '../../shared/ui/DateNavigation';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

type Viewer = { user: { id: string }; clubs: { id: string; roles: string[] }[] };
export function SessionCalendar({ clubId, timezone, canManage }: { clubId: string; timezone: string; canManage: boolean }) {
  const today = localDate(timezone);
  const [location, setLocation] = useState(() => sessionLocation(timezone));
  const { from, through, groupId, view } = location;
  const [sessions, setSessions] = useState<Session[]>([]), [groups, setGroups] = useState<Group[]>([]), [trainers, setTrainers] = useState<TrainerOption[]>([]);
  const [viewer, setViewer] = useState<Viewer | null>(null);
  const [attempt, setAttempt] = useState(0), [loading, setLoading] = useState(true), [optionsLoading, setOptionsLoading] = useState(true);
  const [error, setError] = useState(''), [optionsError, setOptionsError] = useState(''), [notice, setNotice] = useState('');
  const [editor, setEditor] = useState<'new' | { session: Session; action: Action } | null>(null);
  useEffect(() => {
    const changed = () => setAttempt((n) => n + 1);
    window.addEventListener('attendance-changed', changed);
    return () => window.removeEventListener('attendance-changed', changed);
  }, []);
  useEffect(() => {
    const c = new AbortController(); setOptionsLoading(true); setOptionsError(''); setGroups([]); setTrainers([]); setViewer(null);
    Promise.all([apiGet<Group[]>(`/api/clubs/${clubId}/sessions/groups`, c.signal), canManage ? apiGet<TrainerOption[]>(`/api/clubs/${clubId}/trainers?available=true`, c.signal) : Promise.resolve([]), apiGet<Viewer>('/api/auth/me', c.signal)])
      .then(([g, t, me]) => { if (!c.signal.aborted) { setGroups(g); setTrainers(t); setViewer(me); } })
      .catch((cause: unknown) => { if (!c.signal.aborted) setOptionsError(message(cause)); })
      .finally(() => { if (!c.signal.aborted) setOptionsLoading(false); });
    return () => c.abort();
  }, [clubId, canManage, attempt]);
  useEffect(() => {
    const c = new AbortController(); setLoading(true); setError(''); setSessions([]);
    if (!validSessionIds(location)) { setError('Ссылка содержит неверный ID группы или занятия. Сбросьте фильтр или вернитесь к занятиям.'); setLoading(false); return () => c.abort(); }
    if (!from || !through || through < from || (Date.parse(through) - Date.parse(from)) / 86400000 > 365) { setError('Выберите период до 366 дней: дата конца не раньше начала.'); setLoading(false); return () => c.abort(); }
    apiGet<Session[]>(`/api/clubs/${clubId}/sessions?from=${from}&until=${nextDate(through)}${groupId ? `&groupId=${encodeURIComponent(groupId)}` : ''}`, c.signal)
      .then((v) => { if (!c.signal.aborted) setSessions(v); }).catch((cause: unknown) => { if (!c.signal.aborted) setError(message(cause)); })
      .finally(() => { if (!c.signal.aborted) setLoading(false); });
    return () => c.abort();
  }, [clubId, from, through, groupId, location.sessionId, attempt]);
  function filters(patch: Partial<SessionLocation>) {
    const next = { ...location, ...patch, sessionId: '' }; setLocation(next); setNotice('');
    // Filter edits replace this entry. Opening a journal uses a real link, so browser
    // back/reload and the shell's beforeunload guard retain their normal semantics.
    window.history.replaceState(window.history.state, '', sessionHref(clubId, next));
  }
  function period(mode: SessionLocation['view'], date = from || today) {
    const start = mode === 'week' ? weekStart(date) : date;
    filters({ view: mode, from: start, through: mode === 'week' ? nextDate(start, 6) : mode === 'day' ? start : through });
  }
  function refresh() { setEditor(null); setAttempt((n) => n + 1); }
  function changed() { setEditor(null); setNotice('Занятие сохранено.'); trainingChanged(); }
  const ready = !loading && !optionsLoading && !error && !optionsError;
  const selectedGroup = groups.find((g) => g.id === groupId);
  const days = [...new Set(sessions.map((s) => s.date))].sort();
  if (location.sessionId && validSessionIds(location)) return <SessionDetail clubId={clubId} location={location} canManage={canManage} />;
  return <section className="connection registry session-calendar" aria-labelledby="sessions-title">
    <div className="section-heading"><div><h2 id="sessions-title">Календарь занятий</h2><p className="help">{canManage ? 'Занятия филиала' : 'Мои назначенные занятия'} · {timezone}</p></div>
      <DateNavigation date={from} timezone={timezone} disabled={!!editor} step={view === 'week' ? 7 : 1} onChange={(date) => period(view === 'period' ? 'day' : view, date)} />
      {canManage && <button disabled={!ready || !!editor} onClick={() => { setNotice(''); setEditor('new'); }}>Добавить разовое занятие</button>}
    </div>
    <div className="session-period-toolbar">
      <ToggleGroup type="single" className="session-view" aria-label="Вид календаря" value={view} disabled={!!editor} onValueChange={(value) => { if (value === 'day' || value === 'week' || value === 'period') period(value); }}>{([['day', 'День'], ['week', 'Неделя'], ['period', 'Период']] as const).map(([mode, label]) => <ToggleGroupItem key={mode} value={mode}>{label}</ToggleGroupItem>)}</ToggleGroup>
      <button className="secondary" disabled={!!editor} onClick={() => period(view === 'period' ? 'day' : view, today)}>Сегодня</button>

    </div>
    <div className="session-filters"><label className="field">С даты<input type="date" value={from} disabled={!!editor} onChange={(e) => filters({ from: e.target.value, view: 'period' })} /></label><label className="field">По дату включительно<input type="date" value={through} disabled={!!editor} onChange={(e) => filters({ through: e.target.value, view: 'period' })} /></label>
      <div className="field"><label htmlFor="session-group-filter">Группа календаря</label><select id="session-group-filter" value={groupId} disabled={!!editor || optionsLoading} onChange={(e) => filters({ groupId: e.target.value })}><option value="">Все доступные группы</option>{groupId && !selectedGroup && <option value={groupId}>Группа из ссылки недоступна</option>}{groups.map((g) => <option key={g.id} value={g.id}>{g.name}{groups.filter((other) => other.name === g.name).length > 1 ? ` · ${g.id.slice(-8)}` : ''}{g.archived ? ' · в архиве' : ''}</option>)}</select></div>
    </div>
    <div className="session-list-heading"><div>{groupId && <button className="secondary" disabled={!!editor} onClick={() => filters({ groupId: '' })}>Все группы · сбросить фильтр</button>}<p className="help">{dateLabel(from)} — {dateLabel(through)} · даты включены</p></div><button className="secondary" disabled={!!editor} onClick={refresh}>Обновить календарь</button></div>
    {notice && <p role="status">{notice}</p>}{(loading || optionsLoading) && <p role="status">Загружаем занятия…</p>}
    {(error || optionsError) && <div role="alert"><p className="error">{error || optionsError}</p><button onClick={refresh}>Повторить загрузку занятий</button></div>}
    {editor && <SessionForm key={editor === 'new' ? 'new' : `${editor.session.id}:${editor.action}`} clubId={clubId} timezone={timezone} groups={groups} trainers={trainers} selectedGroup={groupId} session={editor === 'new' ? null : editor.session} action={editor === 'new' ? undefined : editor.action} onSaved={changed} onCancel={() => setEditor(null)} onRefresh={refresh} />}
    {ready && <><p role="status">Занятий за период: {sessions.length}</p>{days.map((day) => <section className="session-day" key={day} aria-label={`Занятия ${dateLabel(day)}`}><h3 className="session-day-heading">{new Intl.DateTimeFormat('ru', { weekday: 'long', day: 'numeric', month: 'long', timeZone: 'UTC' }).format(new Date(`${day}T12:00:00Z`))}{day === today && ' · Сегодня'}</h3><ul className="registry-list session-list">{sessions.filter((s) => s.date === day).map((v) => <li key={v.id} data-session-id={v.id}>
      <div className="session-clock"><strong>{timeLabel(v.startsAt, v.timezone)}</strong><span>{timeLabel(v.endsAt, v.timezone)}</span></div>
      <div className="session-information"><h3>{v.groupName}</h3><p>Тренер: {v.trainerName}</p><p><span className={`training-status status-${v.status}`}>{statusNames[v.status]}</span>{!v.ruleId && ' · разовое'}{v.manualOverride && ' · ручное исключение'}</p>
        {v.timezone !== timezone && <p className="help">Время занятия: {v.timezone}</p>}
        {localDateOf(v.endsAt, v.timezone) !== v.date && <p className="help">Окончание {dateLabel(localDateOf(v.endsAt, v.timezone))}</p>}
        {v.date !== v.sourceDate && <p className="help">Перенесено с {dateLabel(v.sourceDate)}</p>}
        {v.cancellationReason && <p>Причина отмены: {v.cancellationReason}</p>}
      </div>
      <div className="session-row-actions">
        {(!canManage || (viewer?.user.id === v.trainerId && viewer.clubs.some((c) => c.id === clubId && c.roles.includes('trainer')))) && <a className="session-link session-primary" href={sessionHref(clubId, { ...location, sessionId: v.id, journal: 'trainer' })}>{v.status === 'open' ? 'Продолжить отметки' : v.status === 'planned' ? 'Открыть журнал' : 'Посмотреть журнал'}</a>}
        {canManage && <a className="session-link" href={sessionHref(clubId, { ...location, sessionId: v.id, journal: 'admin' })}>Журнал филиала</a>}
        {canManage && (v.status === 'planned' || v.status === 'open') && <details><summary>Изменить занятие</summary><div className="actions">{(['replace-trainer', 'reschedule', 'cancel'] as Action[]).filter((a) => a !== 'reschedule' || (v.status === 'planned' && !v.rosterFrozen && Date.parse(v.startsAt) > Date.now())).map((action) => <button key={action} className="secondary" disabled={!!editor} onClick={() => { setNotice(''); setEditor({ session: v, action }); }}>{actionNames[action]}</button>)}</div></details>}
      </div>
    </li>)}</ul></section>)}{sessions.length === 0 && <div className="session-empty"><h3>Занятий пока нет</h3><p>{canManage ? 'Выберите другой день или период, проверьте группу либо добавьте разовое занятие.' : 'В выбранном периоде вам не назначены занятия. Попробуйте другую дату.'}</p></div>}</>}
  </section>;
}
function localDateOf(value: string, timezone: string) { return new Intl.DateTimeFormat('en-CA', { timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date(value)); }

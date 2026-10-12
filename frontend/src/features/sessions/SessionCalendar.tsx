import { useEffect, useState } from 'react';
import { apiGet } from '../../shared/api';
import type { Group } from '../groups/types';
import { dateLabel, localDate, message, statusNames, timeLabel, type TrainerOption } from '../schedule/types';
import { SessionForm } from './SessionForm';
import { actionNames, nextDate, type Action, type Session } from './types';
import './sessions.css';

export function SessionCalendar({ clubId, timezone, canManage }: { clubId: string; timezone: string; canManage: boolean }) {
  const today = localDate(timezone);
  const [from, setFrom] = useState(today), [through, setThrough] = useState(nextDate(today, 30)), [groupId, setGroupId] = useState('');
  const [sessions, setSessions] = useState<Session[]>([]), [groups, setGroups] = useState<Group[]>([]), [trainers, setTrainers] = useState<TrainerOption[]>([]);
  const [attempt, setAttempt] = useState(0), [loading, setLoading] = useState(true), [optionsLoading, setOptionsLoading] = useState(true);
  const [error, setError] = useState(''), [optionsError, setOptionsError] = useState(''), [notice, setNotice] = useState('');
  const [editor, setEditor] = useState<'new' | { session: Session; action: Action } | null>(null);
  useEffect(() => {
    const c = new AbortController(); setOptionsLoading(true); setOptionsError('');
    Promise.all([apiGet<Group[]>(`/api/clubs/${clubId}/sessions/groups`, c.signal), canManage ? apiGet<TrainerOption[]>(`/api/clubs/${clubId}/trainers?available=true`, c.signal) : Promise.resolve([])])
      .then(([g, t]) => { if (!c.signal.aborted) { setGroups(g); setTrainers(t); } })
      .catch((cause: unknown) => { if (!c.signal.aborted) setOptionsError(message(cause)); })
      .finally(() => { if (!c.signal.aborted) setOptionsLoading(false); });
    return () => c.abort();
  }, [clubId, canManage, attempt]);
  useEffect(() => {
    const c = new AbortController(); setLoading(true); setError(''); setSessions([]);
    if (!from || !through || through < from) { setError('Выберите период: дата конца не раньше начала.'); setLoading(false); return () => c.abort(); }
    const until = nextDate(through);
    apiGet<Session[]>(`/api/clubs/${clubId}/sessions?from=${from}&until=${until}${groupId ? `&groupId=${groupId}` : ''}`, c.signal)
      .then((v) => { if (!c.signal.aborted) setSessions(v); }).catch((cause: unknown) => { if (!c.signal.aborted) setError(message(cause)); })
      .finally(() => { if (!c.signal.aborted) setLoading(false); });
    return () => c.abort();
  }, [clubId, from, through, groupId, attempt]);
  function refresh() { setEditor(null); setAttempt((n) => n + 1); }
  function changed() { setEditor(null); setNotice('Занятие сохранено. Обновите другой открытый экран, чтобы увидеть изменения.'); refresh(); }
  const ready = !loading && !optionsLoading && !error && !optionsError;
  return <section className="connection registry session-calendar" aria-labelledby="sessions-title">
    <div className="section-heading"><div><p className="eyebrow">{canManage ? 'Занятия филиала' : 'Мои занятия'}</p><h2 id="sessions-title">Календарь занятий</h2></div>
      {canManage && <button disabled={!ready || !!editor} onClick={() => { setNotice(''); setEditor('new'); }}>Добавить разовое занятие</button>}
    </div>
    <p>{canManage ? 'Плановые и разовые занятия, отмены и замены тренера.' : 'Занятия, где вы назначены фактическим тренером.'} Даты периода включены, максимум 366 дней.</p>
    <div className="session-filters"><label className="field">С даты<input type="date" value={from} disabled={!!editor} onChange={(e) => { setFrom(e.target.value); setNotice(''); }} /></label><label className="field">По дату включительно<input type="date" value={through} disabled={!!editor} onChange={(e) => { setThrough(e.target.value); setNotice(''); }} /></label>
      <div className="field"><label htmlFor="session-group-filter">Группа календаря</label><select id="session-group-filter" value={groupId} disabled={!!editor || optionsLoading} onChange={(e) => { setGroupId(e.target.value); setNotice(''); }}><option value="">Все доступные группы</option>{groups.map((g) => <option key={g.id} value={g.id}>{g.name}{g.archived ? ' · в архиве' : ''}</option>)}</select></div>
    </div>
    <button className="secondary" disabled={!!editor} onClick={refresh}>Обновить календарь</button>
    {notice && <p role="status">{notice}</p>}{(loading || optionsLoading) && <p role="status">Загружаем занятия…</p>}
    {(error || optionsError) && <div role="alert"><p className="error">{error || optionsError}</p><button onClick={refresh}>Повторить загрузку занятий</button></div>}
    {editor && <SessionForm key={editor === 'new' ? 'new' : `${editor.session.id}:${editor.action}`} clubId={clubId} timezone={timezone} groups={groups} trainers={trainers} selectedGroup={groupId} session={editor === 'new' ? null : editor.session} action={editor === 'new' ? undefined : editor.action} onSaved={changed} onCancel={() => setEditor(null)} onRefresh={refresh} />}
    {ready && <><p role="status">Занятий за период: {sessions.length}</p><ul className="registry-list session-list">{sessions.map((v) => <li key={v.id} data-session-id={v.id}>
      <div><h3>{dateLabel(v.date)} · {timeLabel(v.startsAt, v.timezone)}–{timeLabel(v.endsAt, v.timezone)}</h3>
        <p>{v.groupName} · {v.timezone}{new Date(v.endsAt).toLocaleDateString('en-CA', { timeZone: v.timezone }) !== new Date(v.startsAt).toLocaleDateString('en-CA', { timeZone: v.timezone }) ? ` · окончание ${new Intl.DateTimeFormat('ru', { timeZone: v.timezone, day: '2-digit', month: '2-digit', year: 'numeric' }).format(new Date(v.endsAt))}` : ''}</p>
        <p>Тренер: {v.trainerName}</p><p><strong>{statusNames[v.status]}</strong>{!v.ruleId && ' · разовое'}{v.manualOverride && ' · ручное исключение'}</p>
        {v.date !== v.sourceDate && <p className="help">Перенесено с {dateLabel(v.sourceDate)}</p>}
        {v.cancellationReason && <p>Причина отмены: {v.cancellationReason}</p>}
        {(v.status === 'closed' || v.status === 'cancelled') && <p className="help">Запись закрыта для изменения тренера, времени и отмены.</p>}
      </div>
      {canManage && (v.status === 'planned' || v.status === 'open') && <div className="actions">{(['cancel', 'replace-trainer', 'reschedule'] as Action[]).filter((a) => a !== 'reschedule' || (v.status === 'planned' && !v.rosterFrozen && Date.parse(v.startsAt) > Date.now())).map((action) => <button key={action} className="secondary" disabled={!!editor} onClick={() => { setNotice(''); setEditor({ session: v, action }); }}>{actionNames[action]}</button>)}</div>}
    </li>)}</ul>{sessions.length === 0 && <p>{canManage ? 'В выбранном периоде занятий нет. Выберите другой период или добавьте разовое занятие.' : 'В этом периоде вам не назначены занятия.'}</p>}</>}
  </section>;
}

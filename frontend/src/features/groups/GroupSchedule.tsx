import { useEffect, useRef, useState } from 'react';
import { apiGet } from '../../shared/api';
import { dateLabel, message, statusNames, timeLabel, type Rule, type TrainerOption } from '../schedule/types';
import { SessionForm } from '../sessions/SessionForm';
import { trainingChanged } from '../sessions/navigation';
import { actionNames, nextDate, type Action, type Session } from '../sessions/types';
import { GroupScheduleEditor } from './GroupScheduleEditor';
import { currentRules, futureRules, ruleSummary } from './scheduleSummary';
import type { Group } from './types';

export function GroupSchedule({ clubId, timezone, today, group, rules, trainers, loading, error, onRefresh, beforeNavigate, from, onFrom }: {
  clubId: string; timezone: string; today: string; group: Group; rules: Rule[]; trainers: TrainerOption[]; loading: boolean; error: string;
  onRefresh: () => void; beforeNavigate: (action: () => void) => void; from: string; onFrom: (date: string) => void;
}) {
  const [editor, setEditor] = useState<Rule | 'new' | null>(null), [history, setHistory] = useState(false);
  const [sessions, setSessions] = useState<Session[]>([]), [attempt, setAttempt] = useState(0), [sessionsLoading, setSessionsLoading] = useState(true), [sessionsError, setSessionsError] = useState('');
  const [sessionEditor, setSessionEditor] = useState<'new' | { session: Session; action: Action } | null>(null), [notice, setNotice] = useState('');
  const editedDate = useRef('');
  useEffect(() => {
    const c = new AbortController(); setSessions([]); setSessionsLoading(true); setSessionsError('');
    if (!from) { setSessionsLoading(false); setSessionsError('Выберите начало периода.'); return () => c.abort(); }
    apiGet<Session[]>(`/api/clubs/${clubId}/sessions?from=${from}&until=${nextDate(from, 28)}&groupId=${group.id}`, c.signal)
      .then((v) => { if (!c.signal.aborted) setSessions(v); }).catch((cause: unknown) => { if (!c.signal.aborted) setSessionsError(message(cause)); })
      .finally(() => { if (!c.signal.aborted) setSessionsLoading(false); });
    return () => c.abort();
  }, [clubId, group.id, from, attempt, rules]);
  function saved() { if (editedDate.current && (!from || editedDate.current < from || editedDate.current >= nextDate(from, 28))) onFrom(editedDate.current); editedDate.current = ''; setEditor(null); setSessionEditor(null); setNotice('Изменения сохранены на сервере.'); setAttempt((v) => v + 1); onRefresh(); trainingChanged(); }
  function reset() { setEditor(null); setSessionEditor(null); setAttempt((v) => v + 1); onRefresh(); }
  const current = currentRules(rules, group.id, today), future = futureRules(rules, group.id, today);
  const groupRules = rules.filter((r) => r.groupId === group.id);
  const latest = (rule: Rule) => !groupRules.some((r) => r.seriesId === rule.seriesId && r.from > rule.from);
  const visible = history ? groupRules : [...current, ...future];
  const editing = !!editor || !!sessionEditor;
  return <div className="group-schedule">
    {notice && <p role="status">{notice}</p>}
    <div className="section-heading"><h4>Регулярное расписание</h4>{!group.archived && <button disabled={loading || !!error || editing} onClick={() => beforeNavigate(() => setEditor('new'))}>{current.length || future.length ? 'Добавить интервал' : 'Настроить расписание'}</button>}</div>
    <p className="help">Обычные дни и время группы. Перенос и отмена одной даты — ниже, в ближайших занятиях.</p>
    {loading && <p role="status">Загружаем правила…</p>}{error && <div role="alert"><p>{error}</p><button onClick={onRefresh}>Повторить загрузку расписания</button></div>}
    {!loading && !error && <>{visible.length === 0 && <p>Регулярное расписание не задано.</p>}<ul className="group-rules">{visible.map((rule) => <li key={rule.id}>
      <div><strong>{rule.from > today ? `С ${dateLabel(rule.from)} · ` : ''}{ruleSummary(rule)}</strong><p>{trainers.find((t) => t.id === rule.trainerId)?.displayName ?? sessions.find((s) => s.trainerId === rule.trainerId)?.trainerName ?? 'Тренер недоступен для новых назначений'}</p><p className="help">С {dateLabel(rule.from)}{rule.until ? ` до ${dateLabel(rule.until)} (не включено)` : ' · без окончания'} · {rule.timezone}{!latest(rule) && ' · версия до следующего изменения'}</p></div>
      {latest(rule) && !group.archived && (!rule.until || rule.until > today) && <button className="secondary" disabled={editing} onClick={() => beforeNavigate(() => setEditor(rule))}>Изменить расписание</button>}
    </li>)}</ul><label className="group-check"><input type="checkbox" checked={history} onChange={(e) => setHistory(e.target.checked)} />История версий расписания</label></>}
    {editor && <GroupScheduleEditor key={editor === 'new' ? 'new' : editor.id} clubId={clubId} timezone={timezone} group={group} rule={editor === 'new' ? null : editor} trainers={trainers} onCancel={() => beforeNavigate(() => setEditor(null))} onRefresh={() => beforeNavigate(reset)} onSaved={saved} />}
    <div className="group-exceptions"><div className="section-heading"><h4>Ближайшие занятия и исключения</h4>{!group.archived && <button className="secondary" disabled={editing || loading || !!error} onClick={() => beforeNavigate(() => { editedDate.current = ''; setSessionEditor('new'); })}>Добавить разовое занятие</button>}</div>
      <p className="help">Конкретные даты выбранной группы. Перенос одного занятия сохраняет остальную серию.</p>
      <div className="group-period"><button className="secondary" disabled={editing || !from} onClick={() => onFrom(nextDate(from, -28))}>Предыдущие даты</button><label className="field">Период с даты<input type="date" value={from} disabled={editing} onChange={(e) => onFrom(e.target.value)} /></label><button className="secondary" disabled={editing || !from} onClick={() => onFrom(nextDate(from, 28))}>Следующие даты</button></div>
      {from && <p className="help">{dateLabel(from)} — {dateLabel(nextDate(from, 27))} · 28 дней · {timezone}</p>}
      {sessionsLoading && <p role="status">Загружаем занятия…</p>}{sessionsError && <div role="alert"><p>{sessionsError}</p><button onClick={() => setAttempt((v) => v + 1)}>Повторить загрузку занятий</button></div>}
      {sessionEditor && <div className="group-session-editor" onChangeCapture={(e) => { const field = e.target as HTMLInputElement; if (field.type === 'date') editedDate.current = field.value; }}><p><strong>{group.name}</strong>{sessionEditor !== 'new' && <> · {dateLabel(sessionEditor.session.date)} · {timeLabel(sessionEditor.session.startsAt, sessionEditor.session.timezone)}–{timeLabel(sessionEditor.session.endsAt, sessionEditor.session.timezone)} · {sessionEditor.session.trainerName}</>}</p>
        <SessionForm key={sessionEditor === 'new' ? 'once' : `${sessionEditor.session.id}:${sessionEditor.action}`} clubId={clubId} timezone={timezone} groups={[group]} trainers={trainers} selectedGroup={group.id} session={sessionEditor === 'new' ? null : sessionEditor.session} action={sessionEditor === 'new' ? undefined : sessionEditor.action} onSaved={saved} onCancel={() => beforeNavigate(() => setSessionEditor(null))} onRefresh={() => beforeNavigate(reset)} />
      </div>}
      {!sessionsLoading && !sessionsError && <>{sessions.length === 0 && <p>В этом периоде занятий нет. Выберите другие даты; регулярные правила показаны выше.</p>}<ul className="group-sessions">{[...sessions].sort((a, b) => a.startsAt.localeCompare(b.startsAt)).map((session) => <li key={session.id} data-session-id={session.id}>
        <div><strong>{dateLabel(session.date)} · {timeLabel(session.startsAt, session.timezone)}–{timeLabel(session.endsAt, session.timezone)}</strong><p>{session.trainerName} · {statusNames[session.status]}</p>{!session.ruleId && <p className="help">Разовое занятие</p>}{session.sourceDate !== session.date && <p>Перенесено с {dateLabel(session.sourceDate)}</p>}{session.manualOverride && session.sourceDate === session.date && <p className="help">Есть изменение отдельного занятия</p>}{session.previousTrainerName && <p className="help">Замена тренера · вместо {session.previousTrainerName}</p>}{session.cancellationReason && <p>Причина отмены: {session.cancellationReason}</p>}<p className="help">{session.timezone}</p></div>
        {(session.status === 'planned' || session.status === 'open') && <details><summary>Действия с занятием<span className="sr-only"> {dateLabel(session.date)} {timeLabel(session.startsAt, session.timezone)}</span></summary><div className="actions">{(['reschedule', 'cancel', 'replace-trainer'] as Action[]).filter((action) => action === 'cancel' || (!group.archived && (action !== 'reschedule' || (session.status === 'planned' && !session.rosterFrozen && Date.parse(session.startsAt) > Date.now())))).map((action) => <button className="secondary" key={action} disabled={editing} onClick={() => beforeNavigate(() => { editedDate.current = ''; setSessionEditor({ session, action }); })}>{actionNames[action]}</button>)}</div></details>}
      </li>)}</ul></>}
    </div>
  </div>;
}

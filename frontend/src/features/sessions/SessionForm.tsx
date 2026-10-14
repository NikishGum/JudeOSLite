import { useEffect, useId, useRef, useState, type FormEvent } from 'react';
import { ApiError, apiPost } from '../../shared/api';
import type { Group } from '../groups/types';
import { localDate, message, timeLabel, type TrainerOption } from '../schedule/types';
import { actionNames, type Action, type Session } from './types';

export function SessionForm({ clubId, timezone, groups, trainers, selectedGroup, session, action, onSaved, onCancel, onRefresh }: {
  clubId: string; timezone: string; groups: Group[]; trainers: TrainerOption[]; selectedGroup: string;
  session: Session | null; action?: Action; onSaved: () => void; onCancel: () => void; onRefresh: () => void;
}) {
  const prefix = useId(), zone = session?.timezone ?? timezone;
  const [groupId, setGroupId] = useState(selectedGroup), [trainerId, setTrainerId] = useState('');
  const [date, setDate] = useState(session?.date ?? localDate(zone));
  const [clock, setClock] = useState(session ? timeLabel(session.startsAt, zone) : '18:00');
  const [duration, setDuration] = useState(session ? (Date.parse(session.endsAt) - Date.parse(session.startsAt)) / 60_000 : 60);
  const [reason, setReason] = useState(''), [confirmed, setConfirmed] = useState(false);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [stale, setStale] = useState(false);
  const pending = useRef(false), mounted = useRef(true), heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => { mounted.current = true; heading.current?.focus(); heading.current?.scrollIntoView({ block: 'start' }); return () => { mounted.current = false; }; }, []);
  async function submit(event: FormEvent) {
    event.preventDefault(); if (pending.current || stale) return;
    pending.current = true; setBusy(true); setError('');
    const base: `/api/${string}` = `/api/clubs/${clubId}/sessions`;
    const fields = { date, startsTime: clock, durationMinutes: duration };
    const body = session ? { version: session.version, ...(action === 'cancel' ? { reason, confirmOpen: confirmed } : action === 'replace-trainer' ? { trainerId } : fields) } : { groupId, trainerId, ...fields };
    try { await apiPost(session ? `${base}/${session.id}/${action}` : base, body); if (mounted.current) onSaved(); }
    catch (cause) { if (mounted.current) { setError(message(cause)); setStale(cause instanceof ApiError && ['version_conflict', 'session_state_conflict', 'open_confirmation_required'].includes(cause.code)); } }
    finally { if (mounted.current) { pending.current = false; setBusy(false); } }
  }
  return <div className="registry-card session-form"><h3 ref={heading} tabIndex={-1}>{action ? actionNames[action] : 'Новое разовое занятие'}</h3>
    {error && <p role="alert" className="error">{error}</p>}
    {stale && <p>Ввод сохранён. <button className="secondary" disabled={busy} onClick={onRefresh}>Загрузить актуальные занятия и сбросить форму</button></p>}
    <form onSubmit={(event) => void submit(event)} aria-busy={busy}><fieldset disabled={busy}>
      {!session && <div className="field"><label htmlFor={`${prefix}-group`}>Группа разового занятия</label><select id={`${prefix}-group`} value={groupId} required onChange={(e) => setGroupId(e.target.value)}><option value="">Выберите группу</option>{groups.filter((g) => !g.archived).map((g) => <option key={g.id} value={g.id}>{g.name}</option>)}</select></div>}
      {(!session || action === 'replace-trainer') && <div className="field"><label htmlFor={`${prefix}-trainer`}>Фактический тренер</label><select id={`${prefix}-trainer`} value={trainerId} required onChange={(e) => setTrainerId(e.target.value)}><option value="">Выберите тренера</option>{trainers.map((t) => <option key={t.id} value={t.id}>{t.displayName}</option>)}</select></div>}
      {(!session || action === 'reschedule') && <><p className="help">Дата и время: {zone}. Разовое занятие не создаёт регулярное правило.</p>
        <label className="field">Дата занятия<input type="date" required min={localDate(zone)} value={date} onChange={(e) => setDate(e.target.value)} /></label>
        <div className="session-time"><label className="field">Время занятия<input type="time" required value={clock} onChange={(e) => setClock(e.target.value)} /></label><label className="field">Длительность занятия, минут<input type="number" required min={1} max={1440} value={duration} onChange={(e) => setDuration(Number(e.target.value))} /></label></div>
      </>}
      {action === 'cancel' && <><p>Отменённое занятие исключается из статистики. Автоматические пропуски не создаются.</p>
        {session?.status === 'open' && <div className="session-warning"><p>Журнал открыт. Отмена закроет запись в журнале; снимок состава и уже сохранённые отметки останутся в истории.</p><label><input type="checkbox" required checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} />Подтверждаю отмену открытого журнала</label></div>}
        <div className="field"><label htmlFor={`${prefix}-reason`}>Причина отмены</label><textarea id={`${prefix}-reason`} required maxLength={1000} value={reason} onChange={(e) => setReason(e.target.value)} /></div>
      </>}
      {action === 'replace-trainer' && <p className="help">Прежний тренер потеряет доступ к этому занятию после обновления. Авторство сохранённых отметок и снимок состава сохраняются.</p>}
      {action === 'reschedule' && <p className="help">Перенос действует только на это занятие. Исключение сохраняется при генерации расписания.</p>}
      <div className="actions"><button type="submit" disabled={stale}>{busy ? 'Сохраняем…' : action ? 'Подтвердить изменение занятия' : 'Создать разовое занятие'}</button><button type="button" className="secondary" onClick={onCancel}>Закрыть форму занятия</button></div>
    </fieldset></form>
  </div>;
}

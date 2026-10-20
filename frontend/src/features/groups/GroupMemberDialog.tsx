import { useEffect, useId, useRef, useState, type FormEvent } from 'react';
import { ApiError, apiGet, apiPost } from '../../shared/api';
import { Dialog } from '../../shared/ui/Dialog';
import { message } from '../schedule/types';
import { nextDate } from '../sessions/types';
import { displayDate, type Group, type Interval } from './types';

export function GroupMemberDialog({ clubId, interval, groups, action, onClose, onChanged }: {
  clubId: string; interval: Interval; groups: Group[]; action: 'transfer' | 'exclude' | 'history'; onClose: () => void; onChanged: () => void;
}) {
  const groupSelectId = useId();
  const [date, setDate] = useState(''), [groupId, setGroupId] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [stale, setStale] = useState(false);
  const [history, setHistory] = useState<Interval[] | null>(null);
  const [attempt, setAttempt] = useState(0);
  const pending = useRef(false), mounted = useRef(true);
  const base: `/api/${string}` = `/api/clubs/${clubId}/participants/${interval.membershipId}/groups`;
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  useEffect(() => {
    if (action !== 'history') return;
    const c = new AbortController(); setError(''); setHistory(null);
    apiGet<Interval[]>(base, c.signal).then((v) => { if (!c.signal.aborted) setHistory(v); })
      .catch((e: unknown) => { if (!c.signal.aborted) setError(message(e)); });
    return () => c.abort();
  }, [base, action, attempt]);
  async function save(event: FormEvent) {
    event.preventDefault(); if (pending.current || stale) return;
    pending.current = true; setBusy(true); setError('');
    try {
      await apiPost(`${base}/${interval.id}/${action}`, { date, version: interval.version, ...(action === 'transfer' ? { groupId } : {}) });
      if (mounted.current) onChanged();
    } catch (cause) {
      if (mounted.current) { setError(message(cause)); setStale(cause instanceof ApiError && cause.code === 'version_conflict'); }
    } finally { if (mounted.current) { pending.current = false; setBusy(false); } }
  }
  return <Dialog title={action === 'history' ? 'История участия' : action === 'transfer' ? 'Перевести участника' : 'Завершить участие'} busy={busy} onCancel={onClose} cancelLabel="Закрыть участника" className="group-member-dialog">
    <p><strong>{interval.fullName}</strong><br />{interval.groupName}</p>
    {error && <p role="alert" className="error">{error}</p>}
    {action === 'history' ? <>{!history && !error && <p role="status">Загружаем историю…</p>}{error && <button onClick={() => setAttempt((v) => v + 1)}>Повторить загрузку истории</button>}<ol>{history?.map((item) => <li key={item.id}><strong>{item.groupName}</strong> · с {displayDate(item.from)}{item.until ? ` до ${displayDate(item.until)} (не включено)` : ' без окончания'}{item.groupArchived ? ' · группа в архиве' : ''}</li>)}</ol></> : <form onSubmit={(e) => void save(e)} aria-busy={busy}><fieldset disabled={busy || stale}>
      {action === 'transfer' && <div className="field"><label htmlFor={groupSelectId}>Новая группа</label><select id={groupSelectId} required value={groupId} onChange={(e) => setGroupId(e.target.value)}><option value="">Выберите группу</option>{groups.filter((g) => !g.archived && g.id !== interval.groupId).map((g) => <option key={g.id} value={g.id}>{g.name}</option>)}</select></div>}
      <label className="field">Дата действия<input type="date" required min={nextDate(interval.from)} max={interval.until ? nextDate(interval.until, -1) : undefined} value={date} onChange={(e) => setDate(e.target.value)} /></label>
      <p className="help">{date ? `Последний день в прежней группе — ${displayDate(nextDate(date, -1))}. ${action === 'transfer' ? `С ${displayDate(date)} участник в новой группе.` : `С ${displayDate(date)} участие завершено.`}` : 'Выберите дату вступления изменения в силу.'} История состава и снимки занятий сохраняются.</p>
      <button type="submit">{busy ? 'Сохраняем…' : action === 'transfer' ? 'Сохранить перевод' : 'Завершить участие с даты'}</button>
    </fieldset>{stale && <p>Период уже изменён. Ввод сохранён. Закройте форму и перечитайте состав перед повтором.</p>}</form>}
  </Dialog>;
}

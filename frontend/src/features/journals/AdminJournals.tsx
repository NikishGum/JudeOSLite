import { useEffect, useState } from 'react';
import { apiGet, ApiError } from '../../shared/api';
import type { Group } from '../groups/types';
import type { Session } from '../sessions/types';
import { localDate, timeLabel } from '../schedule/types';
import { stateNames } from '../trainer-sessions/types';
import { AdminJournal } from './AdminJournal';
import './journals.css';

export function AdminJournals({ clubId, timezone }: { clubId: string; timezone: string }) {
  const [date, setDate] = useState(() => localDate(timezone)), [groupId, setGroupId] = useState('');
  const [items, setItems] = useState<Session[]>([]), [groups, setGroups] = useState<Group[]>([]), [selected, setSelected] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true), [error, setError] = useState(''), [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const c = new AbortController(); setLoading(true); setError(''); setItems([]); setGroups([]); setSelected(null);
    if (!date) { setError('Выберите дату занятия.'); setLoading(false); return () => c.abort(); }
    Promise.all([apiGet<Session[]>(`/api/clubs/${clubId}/journals?date=${encodeURIComponent(date)}&groupId=${encodeURIComponent(groupId)}`, c.signal), apiGet<Group[]>(`/api/clubs/${clubId}/sessions/groups`, c.signal)])
      .then(([s, g]) => { if (!c.signal.aborted) { setItems(s); setGroups(g); } })
      .catch((e: unknown) => { if (!c.signal.aborted) setError(e instanceof ApiError ? e.message : 'Нет соединения. Повторите загрузку журналов.'); })
      .finally(() => { if (!c.signal.aborted) setLoading(false); });
    return () => c.abort();
  }, [clubId, date, groupId, attempt]);
  return <section className="connection admin-journals" aria-labelledby="admin-journals-title">
    <h2 id="admin-journals-title">Журналы филиала</h2>
    <div className="admin-journal-filters"><label className="field">Дата журнала<input type="date" value={date} onChange={(e) => { setSelected(null); setDate(e.target.value); }} /></label>
      <div className="field"><label htmlFor="admin-journal-group">Группа журнала</label><select id="admin-journal-group" value={groupId} disabled={loading} onChange={(e) => { setSelected(null); setGroupId(e.target.value); }}><option value="">Все группы</option>{groups.map((g) => <option key={g.id} value={g.id}>{g.name}{g.archived ? ' · в архиве' : ''}</option>)}</select></div>
      <button onClick={() => setAttempt((n) => n + 1)}>Обновить журналы</button>
    </div>
    {loading && <p role="status">Загружаем журналы…</p>}{error && <div role="alert"><p className="error">{error}</p><button onClick={() => setAttempt((n) => n + 1)}>Повторить загрузку журналов</button></div>}
    {!loading && !error && <><p role="status">Занятий на дату: {items.length}</p><ul className="admin-session-list">{items.map((v) => <li key={v.id} data-session-id={v.id}>
      <div><h3>{v.groupName} · {timeLabel(v.startsAt, v.timezone)}</h3><p>{stateNames[v.status]} · Тренер: {v.trainerName}</p></div>
      <button disabled={selected?.id === v.id} onClick={() => setSelected(v)}>Посмотреть журнал филиала</button>
    </li>)}</ul>{items.length === 0 && <p>На выбранную дату занятий нет.</p>}</>}
    {selected && <AdminJournal key={`${selected.id}:${date}:${groupId}`} clubId={clubId} initial={selected} onUnavailable={() => setAttempt((n) => n + 1)} />}
  </section>;
}

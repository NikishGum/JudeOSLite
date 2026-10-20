import { useEffect, useRef, useState } from 'react';
import { apiGet, apiPut, ApiError } from '../../shared/api';
import type { Session } from '../sessions/types';
import { stateNames } from '../trainer-sessions/types';
import { type Journal, type Record, markLabel, originLabel } from './types';
import { trainingChanged } from '../sessions/navigation';
import { RecordHistory } from './RecordHistory';

export function AdminJournal({ clubId, initial, onUnavailable, initialJournal, onRead }: { clubId: string; initial: Session; onUnavailable: () => void; initialJournal?: Journal; onRead?: (value: Journal) => void }) {
  const base = `/api/clubs/${clubId}/journals/${initial.id}` as const;
  const [journal, setJournal] = useState<Journal | null>(initialJournal ?? null), [loading, setLoading] = useState(!initialJournal), [busy, setBusy] = useState(false);
  const [error, setError] = useState(''), [conflict, setConflict] = useState(false), [attempt, setAttempt] = useState(0), [notice, setNotice] = useState('');
  const [draft, setDraft] = useState<{ record: Record; value: string; reason: string } | null>(null), [historyId, setHistoryId] = useState('');
  const [search, setSearch] = useState('');
  const firstRead = useRef(!!initialJournal), read = useRef(onRead); read.current = onRead;
  const mounted = useRef(true), pending = useRef(false), unavailable = useRef(onUnavailable); unavailable.current = onUnavailable;
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  useEffect(() => {
    if (attempt === 0 && firstRead.current) return;
    const c = new AbortController(); setLoading(true); setError(''); setJournal(null); setDraft(null); setHistoryId(''); setConflict(false); setNotice('');
    apiGet<Journal>(base, c.signal).then((v) => { if (!c.signal.aborted) { setJournal(v); read.current?.(v); } })
      .catch((e: unknown) => { if (!c.signal.aborted) {
        setError(e instanceof ApiError ? e.message : 'Нет соединения. Повторите загрузку журнала.');
        if (e instanceof ApiError && (e.status === 401 || e.status === 404)) unavailable.current();
      } }).finally(() => { if (!c.signal.aborted) setLoading(false); });
    return () => c.abort();
  }, [base, attempt]);
  async function save() {
    if (!draft || pending.current || conflict || !draft.reason.trim()) return;
    pending.current = true; setBusy(true); setError(''); setNotice('');
    try {
      const result = await apiPut<Journal>(`${base}/records/${draft.record.id}`, { version: draft.record.version, state: draft.value === 'present' ? 'present' : 'absent', absenceReason: draft.value === 'present' ? null : draft.value, changeReason: draft.reason });
      if (!mounted.current) return;
      setJournal(result); read.current?.(result); setDraft(null); setHistoryId(draft.record.id); setNotice('Отметка подтверждена сервером.');
      trainingChanged();
    } catch (e) {
      if (!mounted.current) return;
      setError(e instanceof ApiError ? e.message : 'Нет соединения. Исправление не подтверждено. Ввод сохранён для повтора.');
      if (e instanceof ApiError && e.status === 409) setConflict(true);
      if (e instanceof ApiError && (e.status === 401 || e.status === 404)) { setJournal(null); setDraft(null); setHistoryId(''); unavailable.current(); }
    } finally { pending.current = false; if (mounted.current) setBusy(false); }
  }
  function edit(record: Record) { setDraft({ record, value: record.state === 'present' ? 'present' : record.absenceReason ?? 'unknown', reason: '' }); setError(''); setNotice(''); }
  return <div className="admin-journal" aria-busy={busy || loading}>
    <h3>Журнал · {initial.groupName} · {initial.date}</h3>
    <button className="secondary" disabled={busy} onClick={() => setAttempt((n) => n + 1)}>{conflict ? 'Перечитать журнал и сбросить ввод' : 'Обновить журнал'}</button>
    {error && <p className="error" role="alert">{error}</p>}{notice && <p role="status">{notice}</p>}{loading && <p role="status">Загружаем снимок состава…</p>}
    {journal && <>
      <p><strong>{stateNames[journal.session.status]}</strong>{journal.session.rosterFrozen ? ` · Участников в снимке: ${journal.records.length}` : " · Журнал ещё не открыт"}</p>
      {journal.session.rosterFrozen && <p>Состав сохранён на дату занятия. Архив и переводы участников не меняют снимок.</p>}
      {journal.session.status === 'closed' ? <p>Исправляйте по одной отметке с объяснением причины. Давность занятия не ограничена.</p> : <p>Здесь журнал доступен для чтения. Исправление администратора доступно после закрытия.</p>}
      {journal.records.length === 0 && <p>{journal.session.rosterFrozen ? 'Снимок состава пуст.' : 'Снимок состава ещё не создан.'}</p>}
      {journal.session.rosterFrozen && <label className="field">Поиск по составу<input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Имя участника" /></label>}
      {search && <p role="status">Показано: {journal.records.filter((r) => r.fullName.toLocaleLowerCase('ru').includes(search.trim().toLocaleLowerCase('ru'))).length} из {journal.records.length}</p>}
      <ul className="admin-records">{journal.records.filter((r) => !search || r.fullName.toLocaleLowerCase('ru').includes(search.trim().toLocaleLowerCase('ru')) || draft?.record.id === r.id).map((record) => <li key={record.id} data-record-id={record.id}>
        <h4>{record.fullName}{record.archived && ' · Карточка в архиве'}</h4>
        <p><span className={`training-status status-${record.state ?? 'unmarked'}`}>{markLabel(record)}</span></p>{record.origin && <p>{originLabel(record.origin)} · Версия {record.version}</p>}
        {record.changeReason && <p className="change-reason">Причина последнего исправления: {record.changeReason}</p>}
        <div className="actions">
          {journal.session.status === 'closed' && <button disabled={busy || conflict || draft !== null} onClick={() => edit(record)}>Исправить отметку</button>}
          <button className="secondary" disabled={busy} aria-expanded={historyId === record.id} onClick={() => setHistoryId(historyId === record.id ? '' : record.id)}>{historyId === record.id ? 'Скрыть историю' : 'История отметки'}</button>
        </div>
        {draft?.record.id === record.id && <form className="correction-form" onSubmit={(e) => { e.preventDefault(); void save(); }}>
          <div className="field"><label htmlFor={`correction-${record.id}`}>Новая отметка</label><select id={`correction-${record.id}`} value={draft.value} disabled={busy || conflict} onChange={(e) => setDraft({ ...draft, value: e.target.value })}>
            <option value="present">Присутствует</option><option value="unknown">Отсутствует — причина неизвестна</option><option value="illness">Отсутствует — болезнь</option><option value="excused">Отсутствует — уважительная причина</option>
          </select></div>
          <div className="field"><label htmlFor={`correction-reason-${record.id}`}>Причина исправления</label><textarea id={`correction-reason-${record.id}`} required maxLength={1000} rows={3} disabled={busy || conflict} value={draft.reason} onChange={(e) => setDraft({ ...draft, reason: e.target.value })} /></div>
          <p className="help">Объясните изменение отметки. Это отдельное поле от причины отсутствия.</p>
          <div className="actions"><button type="submit" disabled={busy || conflict || !draft.reason.trim()}>{busy ? 'Сохраняем…' : 'Сохранить исправление'}</button><button type="button" className="secondary" disabled={busy} onClick={() => setDraft(null)}>Отмена исправления</button></div>
        </form>}
        {historyId === record.id && <RecordHistory key={`${record.id}:${record.version}`} path={`${base}/records/${record.id}/history`} timezone={journal.session.timezone} onUnavailable={() => { setJournal(null); setDraft(null); setHistoryId(''); unavailable.current(); }} />}
      </li>)}</ul>
    </>}
  </div>;
}

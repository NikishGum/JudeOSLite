import { useEffect, useRef, useState } from 'react';
import { ApiError, apiGet, apiPost, apiPut } from '../../shared/api';
import type { Session } from '../sessions/types';
import { type AttendanceRecord, type Journal, reasonNames, stateNames } from './types';

type Props = { clubId: string; initial: Session; onChanged: (value: Session) => void };
export function SessionJournal({ clubId, initial, onChanged }: Props) {
  const base = `/api/clubs/${clubId}/trainer-sessions/${initial.id}/journal` as const;
  const [journal, setJournal] = useState<Journal | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [conflict, setConflict] = useState(false);
  const [unavailable, setUnavailable] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [confirm, setConfirm] = useState(false);
  const [draft, setDraft] = useState<{ id: string; value: string } | null>(null);
  const mounted = useRef(true);
  const pending = useRef(false);
  const changed = useRef(onChanged); changed.current = onChanged;
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setError(''); setUnavailable(false); setConflict(false); setConfirm(false); setDraft(null); setJournal(null);
    apiGet<Journal>(base, controller.signal).then((result) => {
      if (controller.signal.aborted) return;
      setJournal(result); changed.current(result.session);
    }).catch((cause: unknown) => { if (!controller.signal.aborted) setError(cause instanceof ApiError ? cause.message : 'Нет соединения. Повторите загрузку.'); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [base, attempt]);
  async function command(run: () => Promise<Journal>) {
    if (pending.current || unavailable || conflict) return;
    pending.current = true; setBusy(true); setError('');
    try {
      const result = await run();
      if (!mounted.current) return;
      setJournal(result); setDraft(null); setConfirm(false); changed.current(result.session);
    } catch (cause) {
      if (!mounted.current) return;
      setError(cause instanceof ApiError ? cause.message : 'Нет соединения. Отметка не подтверждена. Повторите запрос или обновите журнал.');
      if (cause instanceof ApiError && cause.status === 409) { setConflict(true); setConfirm(false); }
      if (cause instanceof ApiError && (cause.status === 404 || cause.status === 401)) { setUnavailable(true); setJournal(null); setDraft(null); }
    } finally { pending.current = false; if (mounted.current) setBusy(false); }
  }
  function mark(record: AttendanceRecord, value: string) {
    if (!journal) return;
    void command(() => apiPut<Journal>(`${base}/records/${record.id}`, {
      version: record.version, sessionVersion: journal.session.version,
      state: value === 'present' ? 'present' : 'absent', absenceReason: value === 'present' ? null : value,
    }));
  }
  const unmarked = journal?.records.filter((item) => item.state === null).length ?? 0;
  const canWrite = journal?.session.status === 'open' && !conflict && !unavailable;
  return <div className="session-journal" aria-busy={busy || loading}>
    <h3>Журнал · {initial.groupName}</h3>
    <button disabled={busy} onClick={() => setAttempt((value) => value + 1)}>{conflict ? 'Загрузить актуальный журнал и сбросить ввод' : 'Обновить журнал'}</button>
    {error && <p className="error" role="alert">{error}</p>}
    {loading && <p role="status">Загружаем журнал…</p>}
    {journal && <>
      <p role="status">{stateNames[journal.session.status]} · Не отмечено: {unmarked} из {journal.records.length}</p>
      {journal.session.status === 'planned' && <><p>При открытии сохраняется состав группы на {journal.session.date}.</p><button disabled={busy || conflict || unavailable} onClick={() => void command(() => apiPost<Journal>(`${base}/open`, { version: journal.session.version }))}>Подтвердить открытие журнала</button></>}
      {journal.session.status === 'cancelled' && <p>Занятие отменено. Отметки сохранены, автоматических отсутствий нет.</p>}
      {journal.session.status === 'closed' && <p>Журнал завершён. Для исправления обратитесь к администратору.</p>}
      {journal.session.status !== 'planned' && journal.records.length === 0 && <p>В снимке состава нет участников.</p>}
      <ul className="journal-records">{journal.records.map((record) => <li key={record.id}>
        <strong>{record.fullName}</strong>{record.archived && <span> · Карточка в архиве</span>}
        <p>{record.state === null ? 'Не отмечено' : record.state === 'present' ? 'Присутствует' : `Отсутствует · ${reasonNames[record.absenceReason!]}`}{record.origin === 'close_default' && ' · При завершении журнала'}</p>
        {canWrite && <form className="journal-mark" onSubmit={(event) => { event.preventDefault(); mark(record, draft?.id === record.id ? draft.value : record.state === 'present' ? 'present' : record.absenceReason ?? 'present'); }}>
          <label htmlFor={`mark-${record.id}`}>Отметка для {record.fullName}</label>
          <select id={`mark-${record.id}`} disabled={busy} value={draft?.id === record.id ? draft.value : record.state === 'present' ? 'present' : record.absenceReason ?? 'present'} onChange={(event) => setDraft({ id: record.id, value: event.target.value })}>
            <option value="present">Присутствует</option><option value="unknown">Отсутствует — причина неизвестна</option><option value="illness">Отсутствует — болезнь</option><option value="excused">Отсутствует — уважительная причина</option>
          </select><button type="submit" disabled={busy}>{busy ? 'Сохраняем…' : 'Сохранить отметку'}</button>
        </form>}
      </li>)}</ul>
      {canWrite && <div className="journal-completion">
        {!confirm ? <button disabled={busy} onClick={() => setConfirm(true)}>Завершить журнал</button> : <>
          <p>Не отмечено: {unmarked}. При завершении эти участники получат «Отсутствует — причина неизвестна». После завершения вы не сможете менять отметки.</p>
          <button disabled={busy} onClick={() => void command(() => apiPost<Journal>(`${base}/close`, { version: journal.session.version }))}>Подтвердить завершение</button>
          <button disabled={busy} onClick={() => setConfirm(false)}>Продолжить отметки</button>
        </>}
      </div>}
    </>}
  </div>;
}

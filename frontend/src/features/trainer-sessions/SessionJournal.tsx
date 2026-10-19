import { useEffect, useRef, useState } from 'react';
import { ApiError, apiGet, apiPost, apiPut } from '../../shared/api';
import type { Session } from '../sessions/types';
import { localDate } from '../schedule/types';
import { trainingChanged } from '../sessions/navigation';
import { AttendanceCards } from '../attendance/AttendanceCards';
import { ActionSheet } from '../attendance/ActionSheet';
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
  const mounted = useRef(true);
  const heading = useRef<HTMLHeadingElement>(null);
  const pending = useRef(false);
  const changed = useRef(onChanged); changed.current = onChanged;
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  useEffect(() => {
    if (loading || journal?.session.status !== 'planned') return;
    heading.current?.focus();
    heading.current?.scrollIntoView({ block: 'start' });
  }, [loading]);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true); setError(''); setUnavailable(false); setConflict(false); setConfirm(false); setJournal(null);
    apiGet<Journal>(base, controller.signal).then((result) => {
      if (controller.signal.aborted) return;
      setJournal(result); changed.current(result.session);
    }).catch((cause: unknown) => {
      if (controller.signal.aborted) return;
      setError(cause instanceof ApiError ? cause.message : 'Нет соединения. Повторите загрузку.');
      if (cause instanceof ApiError && (cause.status === 404 || cause.status === 401)) setUnavailable(true);
    }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [base, attempt]);
  async function command(run: () => Promise<Journal>): Promise<Journal | null> {
    if (pending.current || unavailable || conflict) return null;
    pending.current = true; setBusy(true); setError('');
    try {
      const result = await run();
      if (!mounted.current) return null;
      setJournal(result); setConfirm(false); changed.current(result.session); trainingChanged();
      return result;
    } catch (cause) {
      if (!mounted.current) return null;
      setError(cause instanceof ApiError ? cause.message : 'Нет соединения. Изменение не подтверждено. Повторите запрос или обновите журнал.');
      if (cause instanceof ApiError && cause.status === 409) { setConflict(true); setConfirm(false); }
      if (cause instanceof ApiError && (cause.status === 404 || cause.status === 401)) { setUnavailable(true); setJournal(null); setConfirm(false); }
      return null;
    } finally { pending.current = false; if (mounted.current) setBusy(false); }
  }
  function mark(record: AttendanceRecord, value: 'present' | keyof typeof reasonNames): Promise<Journal | null> {
    if (!journal) return Promise.resolve(null);
    return command(() => apiPut<Journal>(`${base}/records/${record.id}`, {
      version: record.version, sessionVersion: journal.session.version,
      state: value === 'present' ? 'present' : 'absent', absenceReason: value === 'present' ? null : value,
    }));
  }
  const unmarked = journal?.records.filter((item) => item.state === null).length ?? 0;
  const canWrite = journal?.session.status === 'open' && !conflict && !unavailable;
  return <div className="session-journal" aria-busy={busy || loading}>
    <h3 ref={heading} tabIndex={-1}>Журнал · {initial.groupName}</h3>
    <button className="secondary" disabled={busy} onClick={() => setAttempt((value) => value + 1)}>{conflict ? 'Загрузить актуальный журнал и сбросить ввод' : 'Обновить журнал'}</button>
    {error && <p className="error" role="alert">{error}</p>}
    {unavailable && <p>Доступ к журналу отозван или занятие больше вам не назначено. Обновите список занятий.</p>}
    {loading && <p role="status">Загружаем журнал…</p>}
    {journal && <>
      <p role="status">{stateNames[journal.session.status]} · Не отмечено: {unmarked} из {journal.records.length}</p>
      {journal.session.status === 'planned' ? <>
        <p>При открытии сохраняется состав группы на {journal.session.date}.</p>
        {journal.session.date > localDate(journal.session.timezone) && <p>Открыть журнал можно в день занятия.</p>}
        <button disabled={busy || conflict || unavailable || journal.session.date > localDate(journal.session.timezone)} onClick={() => void command(() => apiPost<Journal>(`${base}/open`, { version: journal.session.version }))}>Подтвердить открытие журнала</button>
      </> : <>
        {journal.session.status === 'cancelled' && <p>Занятие отменено. Отметки сохранены, автоматических отсутствий нет.</p>}
        {journal.session.status === 'closed' && <p>Журнал завершён. Для исправления обратитесь к администратору.</p>}
        <AttendanceCards key={attempt} journal={journal} busy={busy} blocked={!canWrite} error={error} onMark={mark} />
      </>}
      {canWrite && <div className="journal-completion">
        <button disabled={busy} onClick={() => setConfirm(true)}>Завершить журнал</button>
        {confirm && <ActionSheet title="Завершить журнал?" busy={busy} onCancel={() => setConfirm(false)} cancelLabel="Продолжить отметки">
          <p>{journal.records.length === 0 ? 'В журнале нет участников. Закрыть пустой журнал?' : unmarked === 0 ? 'Все участники отмечены. Вы уверены, что хотите закрыть журнал?' : `Осталось отметить: ${unmarked}. При закрытии они будут отмечены как отсутствующие с неизвестной причиной.`}</p>
          <p className="help">После закрытия исправить отметки сможет администратор или владелец.</p>
          {error && <p className="error" role="alert">{error}</p>}
          <button disabled={busy} onClick={() => void command(() => apiPost<Journal>(`${base}/close`, { version: journal.session.version }))}>{busy ? 'Завершаем…' : 'Подтвердить завершение'}</button>
        </ActionSheet>}
      </div>}
    </>}
  </div>;
}

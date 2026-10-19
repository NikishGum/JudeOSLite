import { useEffect, useId, useRef, useState } from 'react';
import { type AttendanceRecord, type Journal, reasonNames } from '../trainer-sessions/types';
import { ActionSheet } from './ActionSheet';
import './attendance.css';
import { Button } from '@/components/ui/button';

type Reason = keyof typeof reasonNames;
type Props = {
  journal: Journal; busy: boolean; blocked: boolean; error: string;
  onMark: (record: AttendanceRecord, value: 'present' | Reason) => Promise<Journal | null>;
};
function recordLabel(record: AttendanceRecord) {
  return record.state === null ? 'Не отмечено' : record.state === 'present' ? 'Присутствует' : `Отсутствует · ${reasonNames[record.absenceReason!]}`;
}
export function AttendanceCards({ journal, busy, blocked, error, onMark }: Props) {
  const { records } = journal;
  const [activeId, setActiveId] = useState<string | null>(() => records.find((record) => record.state === null)?.id ?? null);
  const [sheet, setSheet] = useState(false);
  const [retryReason, setRetryReason] = useState<Reason | null>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const mounted = useRef(true);
  const pending = useRef(false);
  const [retryPresent, setRetryPresent] = useState(false);
  const titleId = useId();
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  useEffect(() => { heading.current?.focus(); heading.current?.scrollIntoView({ block: 'start' }); }, [activeId]);
  const index = records.findIndex((record) => record.id === activeId);
  const current = records[index];
  const writable = journal.session.status === 'open' && !blocked;
  const unmarked = records.filter((record) => record.state === null).length;

  async function save(value: 'present' | Reason) {
    if (!current || busy || pending.current || !writable) return;
    pending.current = true;
    setRetryPresent(value === 'present');
    if (value !== 'present') setRetryReason(value);
    const saved = await onMark(current, value);
    pending.current = false;
    if (!saved || !mounted.current) return;
    setSheet(false); setRetryReason(null); setRetryPresent(false);
    // Advance only after the server confirms; corrections return to any remaining cards.
    const confirmedIndex = saved.records.findIndex((record) => record.id === current.id);
    const next = saved.records.slice(confirmedIndex + 1).find((record) => record.state === null)
      ?? saved.records.find((record) => record.id !== current.id && record.state === null);
    setActiveId(next?.id ?? null);
  }
  function select(id: string | null) { setActiveId(id); setSheet(false); setRetryReason(null); setRetryPresent(false); }
  return <div className="attendance-cards">
    <p className="attendance-progress-label" role="status">Отмечено: {records.length - unmarked} из {records.length}</p>
    <progress max={Math.max(1, records.length)} value={records.length - unmarked} aria-label="Прогресс отметок" />
    <div className="attendance-workspace">{current && writable ? <section className="attendance-card" aria-labelledby={titleId}>
      <p className="help">Участник {index + 1} из {records.length} · {journal.session.groupName}</p>
      <h4 id={titleId} ref={heading} tabIndex={-1}>{current.fullName}</h4>
      <p className="help">Карточка {current.membershipId.slice(-8)}{current.archived && ' · В архиве'}</p>
      <p>{recordLabel(current)}</p>
      <div className="attendance-actions">
        <Button type="button" className="attendance-present" disabled={busy} aria-busy={busy && !sheet} onClick={() => void save('present')}>{busy && !sheet ? 'Сохраняем…' : 'Присутствует'}</Button>
        <Button type="button" variant="outline" className="attendance-secondary" disabled={busy} onClick={() => { setRetryReason(null); setSheet(true); }}>Отсутствует</Button>
      </div>
      {error && retryPresent && !busy && <button onClick={() => void save('present')}>Повторить сохранение присутствия</button>}
      <button className="attendance-secondary" disabled={busy} onClick={() => select(null)}>К сводке и предыдущим отметкам</button>
      {index > 0 && <button className="attendance-secondary" disabled={busy} onClick={() => select(records[index - 1].id)}>Предыдущий участник</button>}
    </section> : <>
      <h4 ref={heading} tabIndex={-1}>Сводка посещаемости</h4>
      {records.length === 0 ? <p>В снимке состава нет участников.</p> : <ul className="attendance-summary">{records.map((record, position) => <li key={record.id}>
        <div><strong>{position + 1}. {record.fullName}</strong><p className="help">Карточка {record.membershipId.slice(-8)}{record.archived && ' · В архиве'}</p>
          <p>{recordLabel(record)}{record.origin === 'close_default' && ' · При завершении журнала'}</p></div>
        {writable && <button className="attendance-secondary" disabled={busy} onClick={() => select(record.id)} aria-label={`Открыть карточку ${position + 1}: ${record.fullName}`}>{record.state === null ? 'Отметить' : 'Исправить'}</button>}
      </li>)}</ul>}
    </>}
    {current && writable && <aside className="attendance-roster" aria-label="Состав и отметки"><h4>Состав группы</h4><ol>{records.map((record, position) => <li key={record.id}><button className="attendance-roster-item" aria-current={record.id === activeId ? 'true' : undefined} disabled={busy} onClick={() => select(record.id)}><span>{position + 1}. {record.fullName}</span><small>{recordLabel(record)}</small></button></li>)}</ol></aside>}
    </div>
    {sheet && current && writable && <ActionSheet title={`Причина отсутствия · ${current.fullName}`} busy={busy}
      onCancel={() => { setSheet(false); setRetryReason(null); }}>
      <p>Выберите причину. Отметка сохранится после ответа сервера.</p>
      <div className="attendance-reasons">{(Object.keys(reasonNames) as Reason[]).map((reason) => <button key={reason} disabled={busy} onClick={() => void save(reason)}>{reasonNames[reason]}</button>)}</div>
      {busy && <p role="status">Сохраняем отметку…</p>}
      {error && <p className="error" role="alert">{error}</p>}
      {retryReason && error && !busy && <button onClick={() => void save(retryReason)}>Повторить сохранение: {reasonNames[retryReason]}</button>}
    </ActionSheet>}
  </div>;
}

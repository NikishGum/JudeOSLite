import { useEffect, useId, useRef, useState, type FormEvent } from 'react';
import { ApiError, apiGet, apiPost } from '../../shared/api';
import { displayDate, type Group, type Interval } from './types';
const message = (cause: unknown) => cause instanceof ApiError ? cause.message : 'Нет соединения. Проверьте интернет и повторите действие.';

export function MembershipHistory({ clubId, membershipId, archived, joinedOn, groups, onChanged, onBusyChange }: {
  clubId: string; membershipId: string; archived: boolean; joinedOn: string; groups: Group[]; onChanged?: () => void; onBusyChange?: (busy: boolean) => void;
}) {
  const groupSelectId = useId();
  const base: `/api/${string}` = `/api/clubs/${clubId}/participants/${membershipId}/groups`;
  const [items, setItems] = useState<Interval[]>([]);
  const [attempt, setAttempt] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const [mode, setMode] = useState<'add' | 'exclude' | 'transfer'>('add');
  const [chosen, setChosen] = useState<Interval | null>(null);
  const [groupId, setGroupId] = useState('');
  const [date, setDate] = useState('');
  const [until, setUntil] = useState('');
  const [conflict, setConflict] = useState(false);
  const pending = useRef(false);
  const mounted = useRef(true);
  const formHeading = useRef<HTMLHeadingElement>(null);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setLoadError('');
    apiGet<Interval[]>(base, controller.signal).then((data) => { if (!controller.signal.aborted) setItems(data); })
      .catch((cause: unknown) => { if (!controller.signal.aborted) setLoadError(message(cause)); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [base, attempt]);
  function reset() { setMode('add'); setChosen(null); setDate(''); setUntil(''); setGroupId(''); setError(''); setConflict(false); }
  function choose(item: Interval, next: 'exclude' | 'transfer') { setChosen(item); setMode(next); setDate(''); setUntil(''); setGroupId(''); setError(''); setConflict(false); requestAnimationFrame(() => formHeading.current?.focus()); }
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (pending.current) return; pending.current = true; setBusy(true); onBusyChange?.(true); setError(''); setNotice('');
    try {
      if (mode === 'add') await apiPost(base, { groupId, from: date, until: until || null });
      else if (chosen) await apiPost(`${base}/${chosen.id}/${mode}`, { date, version: chosen.version, ...(mode === 'transfer' ? { groupId } : {}) });
      if (!mounted.current) return;
      reset(); setNotice('Состав сохранён. Предыдущие периоды и снимки занятий сохранены.'); setAttempt((v) => v + 1); onChanged?.();
    } catch (cause) { if (mounted.current) { setError(message(cause)); setConflict(cause instanceof ApiError && cause.code === 'version_conflict'); } }
    finally { if (mounted.current) { pending.current = false; setBusy(false); onBusyChange?.(false); } }
  }
  return <section className="membership-history" aria-label="История групп участника">
    <div className="section-heading"><h4>История групп</h4><button className="secondary" disabled={busy || loading} onClick={() => { reset(); setAttempt((v) => v + 1); }}>Обновить историю и сбросить форму</button></div>
    {loading && <p role="status">Загружаем историю…</p>}
    {loadError && <div role="alert"><p>{loadError}</p><button className="secondary" onClick={() => setAttempt((v) => v + 1)}>Повторить загрузку истории</button></div>}
    {!loading && !loadError && <>{items.length === 0 && <p>Периодов в группах пока нет.</p>}<ol className="registry-list">{items.map((item) => <li key={item.id}><div><strong>{item.groupName}{item.groupArchived ? ' · группа в архиве' : ''}</strong><p>С {displayDate(item.from)} {item.until ? `до ${displayDate(item.until)} (дата исключена)` : 'без даты окончания'}</p></div><div className="actions">
      <button className="secondary" disabled={busy} onClick={() => choose(item, 'exclude')}>Исключить с даты</button>
      {!archived && <button className="secondary" disabled={busy} onClick={() => choose(item, 'transfer')}>Перевести с даты</button>}
    </div></li>)}</ol></>}
    {notice && <p role="status">{notice}</p>}
    {error && <p className="error" role="alert">{error}</p>}
    {conflict && <p className="help">Правки сохранены в форме. Обновление истории сбросит форму и загрузит актуальные периоды.</p>}
    {(mode !== 'add' || !archived) && <form onSubmit={(event) => void submit(event)} aria-busy={busy}>
      <h4 ref={formHeading} tabIndex={-1}>{mode === 'add' ? 'Добавить в группу' : mode === 'exclude' ? `Исключить из «${chosen?.groupName}»` : `Перевести из «${chosen?.groupName}»`}</h4>
      <fieldset disabled={busy || loading || !!loadError}>
        {mode !== 'exclude' && <div className="field"><label htmlFor={groupSelectId}>{mode === 'transfer' ? 'Новая группа' : 'Группа'}</label><select id={groupSelectId} required value={groupId} onChange={(event) => setGroupId(event.target.value)}><option value="">Выберите группу</option>{groups.filter((g) => !g.archived && g.id !== chosen?.groupId).map((g) => <option key={g.id} value={g.id}>{g.name}</option>)}</select></div>}
        <label className="field">Дата действия<input type="date" required min={joinedOn} value={date} onChange={(event) => setDate(event.target.value)} /></label>
        {mode === 'add' && <label className="field">Конец периода (необязательно, дата исключена)<input type="date" min={date || joinedOn} value={until} onChange={(event) => setUntil(event.target.value)} /></label>}
        <p className="help">{mode === 'add' ? 'Начало включено, конец исключён. Пересекающиеся периоды недопустимы.' : 'Дата должна быть позже начала и раньше конца выбранного периода. В день перевода участник уже в новой группе.'}</p>
        <div className="actions"><button type="submit">{busy ? 'Сохраняем…' : mode === 'add' ? 'Добавить в группу' : mode === 'exclude' ? 'Сохранить исключение' : 'Сохранить перевод'}</button>{mode !== 'add' && <button type="button" className="secondary" onClick={reset}>Отменить действие</button>}</div>
      </fieldset>
    </form>}
  </section>;
}

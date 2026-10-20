import { useEffect, useId, useRef, useState, type FormEvent } from 'react';
import { ApiError, apiGet, apiPost, apiPut } from '../../shared/api';
import { Dialog } from '../../shared/ui/Dialog';
import { localDate, message, type Rule, type TrainerOption } from '../schedule/types';
import { trainingChanged } from '../sessions/navigation';
import { GroupRoster } from './GroupRoster';
import { GroupSchedule } from './GroupSchedule';
import { currentRules, futureRules, ruleSummary } from './scheduleSummary';
import { displayDate, type Group } from './types';
import '../participants/participants.css';
import './groups.css';

type View = { tab: 'roster' | 'schedule'; date: string; from: string };
export function GroupManagement({ clubId, clubName, timezone, detailId = '', beforeNavigate, onDetailChange, onOpenSessions }: {
  clubId: string; clubName: string; timezone: string; detailId?: string; beforeNavigate: (action: () => void) => void; onDetailChange: (id: string) => void; onOpenSessions: (id: string) => void;
}) {
  const prefix = useId(), today = localDate(timezone);
  const base: `/api/${string}` = `/api/clubs/${clubId}/groups`;
  const [groups, setGroups] = useState<Group[]>([]), [rules, setRules] = useState<Rule[]>([]), [trainers, setTrainers] = useState<TrainerOption[]>([]);
  const [filter, setFilter] = useState('active'), [selected, setSelected] = useState(detailId), [views, setViews] = useState<Record<string, View>>({});
  const [attempt, setAttempt] = useState(0), [scheduleAttempt, setScheduleAttempt] = useState(0);
  const [loading, setLoading] = useState(true), [loadError, setLoadError] = useState(''), [scheduleLoading, setScheduleLoading] = useState(true), [scheduleError, setScheduleError] = useState('');
  const [editor, setEditor] = useState<Group | 'new' | null>(null), [archive, setArchive] = useState<Group | null>(null);
  const [name, setName] = useState(''), [error, setError] = useState(''), [notice, setNotice] = useState(''), [conflict, setConflict] = useState(false), [busy, setBusy] = useState(false);
  const pending = useRef(false), mounted = useRef(true), heading = useRef<HTMLHeadingElement>(null);
  const view = views[selected] ?? { tab: 'roster', date: today, from: today };
  function patchView(patch: Partial<View>) { setViews((old) => ({ ...old, [selected]: { ...(old[selected] ?? { tab: 'roster', date: today, from: today }), ...patch } })); }
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  useEffect(() => { setSelected(detailId); }, [detailId]);
  useEffect(() => { if (selected) heading.current?.focus(); }, [selected, loading]);
  useEffect(() => {
    const c = new AbortController(); setLoading(true); setLoadError('');
    apiGet<Group[]>(`${base}?state=all`, c.signal).then((v) => { if (!c.signal.aborted) setGroups(v); })
      .catch((e: unknown) => { if (!c.signal.aborted) { setGroups([]); setLoadError(message(e)); } }).finally(() => { if (!c.signal.aborted) setLoading(false); });
    return () => c.abort();
  }, [base, attempt]);
  useEffect(() => {
    const c = new AbortController(); setScheduleLoading(true); setScheduleError(''); setRules([]); setTrainers([]);
    Promise.all([apiGet<Rule[]>(`/api/clubs/${clubId}/schedule`, c.signal), apiGet<TrainerOption[]>(`/api/clubs/${clubId}/trainers?available=true`, c.signal)])
      .then(([r, t]) => { if (!c.signal.aborted) { setRules(r); setTrainers(t); } }).catch((e: unknown) => { if (!c.signal.aborted) setScheduleError(message(e)); }).finally(() => { if (!c.signal.aborted) setScheduleLoading(false); });
    return () => c.abort();
  }, [clubId, scheduleAttempt]);
  function openGroup(id: string) { beforeNavigate(() => { setSelected(id); setEditor(null); setArchive(null); onDetailChange(id); }); }
  function edit(group: Group | 'new') { beforeNavigate(() => { setEditor(group); setName(group === 'new' ? '' : group.name); setError(''); setConflict(false); }); }
  async function mutate(action: () => Promise<Group>, text: string) {
    if (pending.current) return;
    pending.current = true; setBusy(true); setError(''); setNotice(''); setConflict(false);
    try {
      const result = await action(); if (!mounted.current) return;
      if (editor === 'new') { setSelected(result.id); onDetailChange(result.id); }
      setEditor(null); setArchive(null); setNotice(text); setAttempt((v) => v + 1); setScheduleAttempt((v) => v + 1); trainingChanged();
    } catch (cause) { if (mounted.current) { setError(message(cause)); setConflict(cause instanceof ApiError && cause.code === 'version_conflict'); } }
    finally { if (mounted.current) { pending.current = false; setBusy(false); } }
  }
  function save(event: FormEvent) { event.preventDefault(); if (editor) void mutate(() => editor === 'new' ? apiPost<Group>(base, { name, version: 0 }) : apiPut<Group>(`${base}/${editor.id}`, { name, version: editor.version }), 'Группа сохранена. Теперь можно добавить участников и настроить расписание.'); }
  const group = groups.find((g) => g.id === selected), visible = groups.filter((g) => filter === 'all' || g.archived === (filter === 'archived'));
  function summary(g: Group, compact = false) {
    if (scheduleLoading) return <p className="help">Загружаем расписание…</p>;
    if (scheduleError) return <p className="help">Не удалось загрузить расписание</p>;
    const current = currentRules(rules, g.id, today), future = futureRules(rules, g.id, today);
    const trainer = (r: Rule) => trainers.find((t) => t.id === r.trainerId)?.displayName ?? 'Тренер недоступен для новых назначений';
    return <>{current.length === 0 && <p className="help">Регулярное расписание не задано</p>}{(compact ? current.slice(0, 1) : current).map((r) => <p key={r.id} className="group-rule-summary">{ruleSummary(r)}{!compact && <span className="help"> · {trainer(r)}</span>}</p>)}{compact && current.length > 1 && <p className="help">Ещё интервалов: {current.length - 1} · все в группе</p>}{future.map((r) => <p key={r.id} className="help">С {displayDate(r.from)} · {ruleSummary(r)}{!compact && ` · ${trainer(r)}`}</p>)}</>;
  }
  return <section className={`connection registry groups group-workspace ${selected ? 'has-selection' : ''}`} aria-labelledby="groups-title">
    <div className="section-heading"><div><h2 id="groups-title">Группы филиала</h2><p className="help context-name">{clubName}</p></div><button disabled={busy} onClick={() => edit('new')}>Создать группу</button></div>
    {notice && <p role="status">{notice}</p>}
    {editor && <Dialog title={editor === 'new' ? 'Новая группа' : 'Переименовать группу'} busy={busy} onCancel={() => beforeNavigate(() => setEditor(null))} cancelLabel="Закрыть форму группы"><p className="context-name">{clubName}</p>{error && <p role="alert" className="error">{error}</p>}{conflict && <p>Ввод сохранён. Закройте форму и обновите группы перед повтором.</p>}<form onSubmit={save} aria-busy={busy}><fieldset disabled={busy || conflict}><label className="field">Название группы<input required maxLength={200} value={name} onChange={(e) => setName(e.target.value)} /></label><button type="submit">Сохранить группу</button></fieldset></form></Dialog>}
    {archive && <Dialog title="В архив группы" busy={busy} onCancel={() => beforeNavigate(() => setArchive(null))} cancelLabel="Отмена архива группы"><p>Перенести «{archive.name}» в архив? Новое включение будет запрещено. История состава и снимки занятий сохранятся. Участников можно перевести в действующую группу.</p>{error && <p role="alert" className="error">{error}</p>}<button disabled={busy || conflict} onClick={() => void mutate(() => apiPost<Group>(`${base}/${archive.id}/archive`, { version: archive.version }), 'Группа в архиве. История сохранена.')}>Подтвердить архив группы</button>{conflict && <p>Группа уже изменена. Закройте подтверждение и обновите группы.</p>}</Dialog>}
    {loading && <p role="status">Загружаем группы…</p>}{loadError && <div role="alert"><p>{loadError}</p><button onClick={() => setAttempt((v) => v + 1)}>Повторить загрузку групп</button></div>}
    <div className="group-columns"><aside className="group-index" aria-label="Выбор группы"><div className="field"><label htmlFor={`${prefix}-state`}>Состояние групп</label><select id={`${prefix}-state`} value={filter} onChange={(e) => beforeNavigate(() => setFilter(e.target.value))}><option value="active">Действующие</option><option value="archived">В архиве</option><option value="all">Все группы</option></select></div>
      {!loading && !loadError && <><ul>{visible.map((g) => <li key={g.id}><button className="group-choice" aria-current={selected === g.id ? 'true' : undefined} onClick={() => openGroup(g.id)}><strong>{g.name}</strong>{g.archived && <span>В архиве</span>}{summary(g, true)}</button></li>)}</ul>{visible.length === 0 && <p>Групп с выбранным состоянием нет.</p>}</>}
      <button className="secondary" disabled={busy || loading} onClick={() => beforeNavigate(() => { setAttempt((v) => v + 1); setScheduleAttempt((v) => v + 1); })}>Обновить группы</button>
    </aside><div className="group-workarea">
      {group && !loading && !loadError ? <><button className="secondary group-back" onClick={() => openGroup('')}>К списку групп</button><header className="group-header"><div className="section-heading"><div><h3 ref={heading} tabIndex={-1}>{group.name}</h3><p className="help">{clubName}{group.archived ? ' · группа в архиве' : ''}</p></div>{!group.archived && <details><summary>Настройки группы</summary><div className="actions"><button className="secondary" onClick={() => edit(group)}>Переименовать</button><button className="secondary" onClick={() => beforeNavigate(() => { setArchive(group); setError(''); setConflict(false); })}>В архив</button></div></details>}</div>
        <div aria-label="Сводка расписания группы">{summary(group)}</div>{scheduleError && <button className="secondary" onClick={() => setScheduleAttempt((v) => v + 1)}>Повторить загрузку расписания</button>}
        {view.tab === 'roster' && !group.archived && !scheduleLoading && !scheduleError && !currentRules(rules, group.id, today).length && !futureRules(rules, group.id, today).length && <p><button className="secondary" onClick={() => beforeNavigate(() => patchView({ tab: 'schedule' }))}>Настроить расписание</button><span className="help"> Можно отложить, как и добавление участников.</span></p>}
      </header><div className="group-tabs" role="tablist" aria-label="Рабочая область группы">{(['roster', 'schedule'] as const).map((tab) => <button id={`${prefix}-${tab}`} key={tab} type="button" role="tab" aria-selected={view.tab === tab} aria-controls={`${prefix}-${tab}-panel`} tabIndex={view.tab === tab ? 0 : -1} onKeyDown={(e) => { if (['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key)) { e.preventDefault(); const next = e.key === 'Home' ? 'roster' : e.key === 'End' ? 'schedule' : tab === 'roster' ? 'schedule' : 'roster'; beforeNavigate(() => { patchView({ tab: next }); document.getElementById(`${prefix}-${next}`)?.focus(); }); } }} onClick={() => beforeNavigate(() => patchView({ tab }))}>{tab === 'roster' ? 'Состав' : 'Расписание'}</button>)}</div>
      <div role="tabpanel" id={`${prefix}-roster-panel`} aria-labelledby={`${prefix}-roster`} hidden={view.tab !== 'roster'}>{view.tab === 'roster' && <GroupRoster key={group.id} clubId={clubId} group={group} groups={groups} today={today} date={view.date} onDate={(date) => patchView({ date })} beforeNavigate={beforeNavigate} />}</div>
      <div role="tabpanel" id={`${prefix}-schedule-panel`} aria-labelledby={`${prefix}-schedule`} hidden={view.tab !== 'schedule'}>{view.tab === 'schedule' && <GroupSchedule key={group.id} clubId={clubId} group={group} timezone={timezone} today={today} rules={rules} trainers={trainers} loading={scheduleLoading} error={scheduleError} onRefresh={() => setScheduleAttempt((v) => v + 1)} beforeNavigate={beforeNavigate} from={view.from} onFrom={(from) => patchView({ from })} />}</div>
      <div className="group-calendar-link"><button className="secondary" onClick={() => onOpenSessions(group.id)}>К занятиям группы</button></div></> : !loading && !loadError && <div className="group-empty"><h3>{selected ? 'Группа недоступна' : groups.length ? 'Выберите группу' : 'Создайте первую группу'}</h3><p>{selected ? 'Проверьте филиал или выберите другую группу.' : 'Внутри группы можно добавить участников, настроить расписание и изменить отдельное занятие.'}</p></div>}
    </div></div>
  </section>;
}

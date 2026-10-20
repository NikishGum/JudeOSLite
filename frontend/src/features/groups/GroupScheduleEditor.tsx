import { useEffect, useId, useRef, useState, type FormEvent } from 'react';
import { ApiError, apiPost, apiPut } from '../../shared/api';
import type { Group } from '../groups/types';
import { dateLabel, localDate, modeNames, statusNames, timeLabel, type Preview, type Rule, type ScheduleInput, type TrainerOption } from '../schedule/types';

export function GroupScheduleEditor({ clubId, timezone, rule, group, trainers, onSaved, onCancel, onRefresh }: {
  clubId: string; timezone: string; rule: Rule | null; group: Group; trainers: TrainerOption[];
  onSaved: () => void; onCancel: () => void; onRefresh: () => void;
}) {
  const prefix = useId();
  const today = localDate(timezone);
  const [input, setInput] = useState<ScheduleInput>(() => {
    let from = today;
    if (rule && from <= rule.from) { const next = new Date(`${rule.from}T00:00:00Z`); next.setUTCDate(next.getUTCDate() + 1); from = next.toISOString().slice(0, 10); }
    return rule ? { ...rule, from, selectedDates: rule.selectedDates?.filter((date) => date >= from), previewToken: undefined } : {
      groupId: group.id, trainerId: '', mode: 'weekly', weekdays: [1, 3, 5], startsTime: '18:00', durationMinutes: 60, from, until: null, version: 0,
    };
  });
  const [picked, setPicked] = useState(today);
  const [preview, setPreview] = useState<Preview | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [stale, setStale] = useState(false);
  const pending = useRef(false), mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  function change(patch: Partial<ScheduleInput>) { setInput((old) => ({ ...old, ...patch })); setPreview(null); setError(''); }
  function body(): ScheduleInput {
    return { groupId: input.groupId, trainerId: input.trainerId, mode: input.mode, startsTime: input.startsTime,
      durationMinutes: input.durationMinutes, from: input.mode === 'once' ? (input.selectedDates?.[0] ?? '') : input.from,
      until: input.mode === 'weekly' || input.mode === 'day_parity' ? input.until : null, version: rule?.version ?? 0,
      ...(input.mode === 'weekly' ? { weekdays: input.weekdays } : input.mode === 'day_parity' ? { dayParity: input.dayParity } : { selectedDates: input.selectedDates }),
    };
  }
  async function submit(event?: FormEvent, save = false) {
    event?.preventDefault(); if (pending.current || (save && !preview)) return;
    pending.current = true; setBusy(true); setError(''); setStale(false);
    const base: `/api/${string}` = `/api/clubs/${clubId}/schedule${rule ? `/${rule.id}` : ''}`;
    try {
      if (save) { const payload = { ...body(), previewToken: preview!.token }; await (rule ? apiPut(base, payload) : apiPost(base, payload)); if (mounted.current) onSaved(); }
      else { const result = await apiPost<Preview>(`${base}/preview`, body()); if (mounted.current) setPreview(result); }
    } catch (cause) {
      if (mounted.current) { setError(cause instanceof ApiError ? cause.message : 'Нет соединения. Проверьте интернет и повторите.'); setStale(cause instanceof ApiError && cause.code === 'version_conflict'); setPreview(null); }
    } finally { if (mounted.current) { pending.current = false; setBusy(false); } }
  }
  function chooseDate() {
    if (!picked) return;
    const dates = input.mode === 'once' ? [picked] : [...new Set([...(input.selectedDates ?? []), picked])].sort();
    change({ selectedDates: dates, ...(!rule ? { from: dates[0] } : {}) });
  }
  return <div className="group-rule-editor"><h3>{rule ? 'Изменить расписание с даты' : 'Настроить расписание'}</h3>
    <p><strong>{group.name}</strong></p><p className="help">Даты и время филиала: {timezone}. Длительность в минутах. Конец регулярного периода не включается.</p>
    {error && <p className="error" role="alert">{error}</p>}{stale && <p>Ввод сохранён. <button type="button" className="secondary" onClick={onRefresh} disabled={busy}>Загрузить актуальные правила и сбросить форму</button></p>}
    <form onSubmit={(event) => void submit(event)} aria-busy={busy}><fieldset disabled={busy}>

      <div className="field"><label htmlFor={`${prefix}-trainer`}>Тренер занятия</label><select id={`${prefix}-trainer`} required value={input.trainerId} onChange={(e) => change({ trainerId: e.target.value })}><option value="">Выберите тренера</option>{trainers.map((t) => <option key={t.id} value={t.id}>{t.displayName}</option>)}</select></div>
      <div className="field"><label htmlFor={`${prefix}-mode`}>Режим расписания</label><select id={`${prefix}-mode`} value={input.mode} onChange={(e) => change({ mode: e.target.value as ScheduleInput['mode'], weekdays: [1, 3, 5], dayParity: 'even', selectedDates: [], until: null })}>{Object.entries(modeNames).filter(([mode]) => mode !== 'once').map(([mode, name]) => <option key={mode} value={mode}>{name}</option>)}</select></div>
      {input.mode === 'weekly' && <fieldset className="schedule-weekdays"><legend>Дни недели</legend>{['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'].map((day, i) => <label key={day}><input type="checkbox" checked={input.weekdays?.includes(i + 1) ?? false} onChange={(e) => change({ weekdays: e.target.checked ? [...(input.weekdays ?? []), i + 1].sort() : input.weekdays?.filter((v) => v !== i + 1) })} />{day}</label>)}</fieldset>}
      {input.mode === 'day_parity' && <><div className="field"><label htmlFor={`${prefix}-parity`}>Чётность числа месяца</label><select id={`${prefix}-parity`} value={input.dayParity ?? 'even'} onChange={(e) => change({ dayParity: e.target.value })}><option value="even">Чётные: 2, 4, 6…</option><option value="odd">Нечётные: 1, 3, 5…</option></select></div><p className="help">Чётность определяется числом дня месяца. Каждый месяц начинается заново.</p></>}
      {(input.mode === 'date_list' || input.mode === 'once') && <div><label className="field">Дата в календаре<input type="date" required min={rule ? input.from : today} value={picked} onChange={(e) => { setPicked(e.target.value); setPreview(null); }} /></label><button type="button" className="secondary" onClick={chooseDate}>Выбрать дату</button><ul className="schedule-dates">{input.selectedDates?.map((date) => <li key={date}>{dateLabel(date)} <button type="button" className="secondary" aria-label={`Убрать дату ${dateLabel(date)}`} onClick={() => change({ selectedDates: input.selectedDates?.filter((d) => d !== date) })}>Убрать</button></li>)}</ul><p className="help">{input.mode === 'once' ? 'Одно занятие без регулярного правила.' : 'Будут созданы все выбранные даты, включая даты за пределами следующего месяца.'}</p></div>}
      {(rule || input.mode === 'weekly' || input.mode === 'day_parity') && <label className="field">{rule ? 'Применить изменения с даты' : 'Начало действия'}<input type="date" required min={rule ? today : '0001-01-01'} value={input.from} onChange={(e) => change({ from: e.target.value })} /></label>}
      {(input.mode === 'weekly' || input.mode === 'day_parity') && <label className="field">Конец действия (не включён, необязательно)<input type="date" min={input.from} value={input.until ?? ''} onChange={(e) => change({ until: e.target.value || null })} /></label>}
      <div className="schedule-time"><label className="field">Время начала<input type="time" required value={input.startsTime} onChange={(e) => change({ startsTime: e.target.value })} /></label><label className="field">Длительность, минут<input type="number" required min={1} max={1440} value={input.durationMinutes} onChange={(e) => change({ durationMinutes: Number(e.target.value) })} /></label></div>
      <div className="actions"><button type="submit">Показать предпросмотр</button><button type="button" className="secondary" onClick={onCancel}>Отменить форму расписания</button></div>
    </fieldset></form>
    {preview && <div className="schedule-preview" aria-label="Предпросмотр расписания"><h4>Проверьте перед сохранением</h4><p>Новых занятий: {preview.dates.length}. {input.mode === 'weekly' || input.mode === 'day_parity' ? 'Показаны будущие даты текущего и следующего месяца; далее расписание продлевается автоматически.' : 'Показаны все выбранные даты.'}</p>
      <ul className="schedule-dates">{preview.dates.map((d) => <li key={d.startsAt}>{dateLabel(d.date)} · {timeLabel(d.startsAt, preview.timezone)}–{timeLabel(d.endsAt, preview.timezone)}</li>)}</ul>
      {rule && <><p>Заменяемые будущие плановые занятия: {preview.affected.length}.</p><ul className="schedule-dates">{preview.affected.map((s) => <li key={s.id}>{dateLabel(s.date)} · {timeLabel(s.startsAt, s.timezone)}</li>)}</ul><p>Сохраняемые исключения и журналы: {preview.preserved.length}. Отмены, ручные замены, открытые/закрытые и уже начавшиеся занятия сохраняются.</p><ul className="schedule-dates">{preview.preserved.map((s) => <li key={s.id}>{dateLabel(s.date)} · {statusNames[s.status] ?? 'Сохранено'}{s.manualOverride ? ' · ручная замена' : ''}</li>)}</ul></>}
      <button disabled={busy} onClick={() => void submit(undefined, true)}>Подтвердить расписание</button>
    </div>}
  </div>;
}

import { useEffect, useState } from 'react';
import { apiGet } from '../../shared/api';
import type { Group } from '../groups/types';
import { trainingChanged } from '../sessions/navigation';
import { ScheduleEditor } from './ScheduleEditor';
import { dateLabel, message, modeNames, type Rule, type TrainerOption } from './types';
import './schedule.css';

export function ScheduleManagement({ clubId, timezone }: { clubId: string; timezone: string }) {
  const [rules, setRules] = useState<Rule[]>([]), [groups, setGroups] = useState<Group[]>([]), [trainers, setTrainers] = useState<TrainerOption[]>([]);
  const [attempt, setAttempt] = useState(0), [loading, setLoading] = useState(true), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [editor, setEditor] = useState<Rule | 'new' | null>(null), [history, setHistory] = useState(false);
  useEffect(() => {
    const c = new AbortController(); setLoading(true); setError(''); setRules([]); setGroups([]); setTrainers([]);
    Promise.all([apiGet<Rule[]>(`/api/clubs/${clubId}/schedule`, c.signal), apiGet<Group[]>(`/api/clubs/${clubId}/groups?state=all`, c.signal), apiGet<TrainerOption[]>(`/api/clubs/${clubId}/trainers?available=true`, c.signal)])
      .then(([r, g, t]) => { if (!c.signal.aborted) { setRules(r); setGroups(g); setTrainers(t); } })
      .catch((cause: unknown) => { if (!c.signal.aborted) setError(message(cause)); }).finally(() => { if (!c.signal.aborted) setLoading(false); });
    return () => c.abort();
  }, [clubId, attempt]);
  function refresh() { setEditor(null); setAttempt((v) => v + 1); }
  const latest = new Map<string, string>(); for (const r of rules) if (!latest.has(r.seriesId)) latest.set(r.seriesId, r.id);
  return <section className="connection registry schedule" aria-labelledby="schedule-title">
    <div className="section-heading"><div><h2 id="schedule-title">Расписание филиала</h2></div><button disabled={loading || !!error || !!editor} onClick={() => { setEditor('new'); setNotice(''); }}>Создать расписание</button></div>
    <p>Группа, тренер, время и длительность. Дни недели, чётность числа месяца, выбранные даты или одно занятие.</p>
    {notice && <p role="status">{notice}</p>}{loading && <p role="status">Загружаем расписание…</p>}
    {error && <div role="alert"><p className="error">{error}</p><button onClick={refresh}>Повторить загрузку расписания</button></div>}
    {!loading && !error && <><div className="actions"><label className="schedule-toggle"><input type="checkbox" checked={history} onChange={(e) => setHistory(e.target.checked)} />Показать историю версий</label><button className="secondary" disabled={!!editor} onClick={refresh}>Обновить расписание</button></div>
      {editor && <ScheduleEditor key={editor === 'new' ? 'new' : editor.id} clubId={clubId} timezone={timezone} rule={editor === 'new' ? null : editor} groups={groups.filter((g) => !g.archived || (editor !== 'new' && g.id === editor.groupId))} trainers={trainers} onCancel={() => setEditor(null)} onRefresh={refresh} onSaved={() => { setNotice('Расписание сохранено. Даты созданы, история и исключения сохранены.'); refresh(); trainingChanged(); }} />}
      {!editor && (groups.every((g) => g.archived) || trainers.length === 0) && <p className="help">Для нового расписания создайте действующую группу и тренера. Если они появились в другом разделе, обновите расписание.</p>}
      <ul className="registry-list schedule-rules">{rules.filter((r) => history || latest.get(r.seriesId) === r.id).map((r) => <li key={r.id}><div><h4>{groups.find((g) => g.id === r.groupId)?.name ?? 'Группа'}</h4><p>{modeNames[r.mode]} · {r.startsTime} · {r.durationMinutes} мин</p><p>{trainers.find((t) => t.id === r.trainerId)?.displayName ?? 'Тренер недоступен для новых назначений'}</p>
        {r.mode === 'weekly' && <p>Дни: {r.weekdays?.map((d) => ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'][d - 1]).join(', ')}</p>}{r.mode === 'day_parity' && <p>{r.dayParity === 'even' ? 'Чётные' : 'Нечётные'} числа месяца</p>}{r.mode === 'date_list' && <p>Даты: {r.selectedDates?.map(dateLabel).join(', ')}</p>}
        <p>{r.mode === 'date_list' ? `Конечный список дат с ${dateLabel(r.from)}${r.until ? ` до ${dateLabel(r.until)} (не включено)` : ''}` : `С ${dateLabel(r.from)}${r.until ? ` до ${dateLabel(r.until)} (не включено)` : ' · без окончания'}`} · {r.timezone}</p>{latest.get(r.seriesId) !== r.id && <p>Историческая версия</p>}</div>
        {latest.get(r.seriesId) === r.id && !groups.find((g) => g.id === r.groupId)?.archived && <button className="secondary" disabled={!!editor} onClick={() => setEditor(r)}>Изменить правило с даты</button>}</li>)}</ul>{rules.length === 0 && <p>Регулярных правил пока нет. Разовое занятие создаётся без правила и будет видно в календаре занятий.</p>}
    </>}
  </section>;
}

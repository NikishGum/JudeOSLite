import { useState } from 'react';
import { sessionHref } from '../sessions/navigation';
import { AttendanceReport } from './AttendanceReport';
import { reportPeriod, type Period } from './periods';
import { useAttendanceReport } from './useAttendanceReport';
import type { ReportSelection } from './types';
import './owner-dashboard.css';

export function OwnerDashboard() {
  const [selection, setSelection] = useState<ReportSelection>(() => ({ ...reportPeriod('month'), clubId: '', groupId: '' }));
  const [period, setPeriod] = useState<Period>('month');
  const [manual, setManual] = useState(() => reportPeriod('month'));
  const { filters, report, loading, error, refresh } = useAttendanceReport(selection);
  const { from, until, clubId, groupId } = selection;
  const clubs = new Map(filters?.clubs.map((club) => [club.id, club]));
  const selectedClub = clubs.get(clubId);
  const selectedGroup = filters?.groups.find((group) => group.id === groupId);
  const context = selectedClub ? `${selectedClub.name}${selectedClub.archived ? ' · в архиве' : ''}${selectedGroup ? ` / ${selectedGroup.name}${selectedGroup.archived ? ' · в архиве' : ''}` : ' / Все группы'}` : 'Вся сеть / Все группы';
  function choosePeriod(next: Period) {
    setPeriod(next); setSelection({ ...selection, ...(next === 'custom' ? manual : reportPeriod(next)) });
  }
  function editDate(field: 'from' | 'until', value: string) {
    const range = { from, until, [field]: value };
    setManual(range); setPeriod('custom'); setSelection({ ...selection, ...range });
  }
  return <section className="connection owner-dashboard" aria-labelledby="owner-statistics-title" aria-busy={loading}>
    <div className="report-heading"><h2 id="owner-statistics-title">Статистика владельца</h2><button type="button" className="secondary" disabled={loading} onClick={refresh}>Обновить статистику</button></div>
    <p className="help report-scope">Филиал отчёта выбирается отдельно от рабочего филиала в шапке.</p>
    <div className="report-periods" role="group" aria-label="Быстрый период отчёта">
      {([['today', 'Сегодня'], ['week', 'Неделя'], ['month', 'Месяц'], ['custom', 'Свой период']] as const).map(([value, label]) => <button key={value} type="button" className="secondary" aria-pressed={period === value} onClick={() => choosePeriod(value)}>{label}</button>)}
    </div>
    <div className="report-filters">
      <div className="field"><label htmlFor="report-club">Филиал отчёта</label><select id="report-club" value={clubId} onChange={(event) => setSelection({ ...selection, clubId: event.target.value, groupId: '' })}><option value="">Вся сеть</option>{filters?.clubs.map((club) => <option key={club.id} value={club.id}>{club.name}{club.archived ? ' · в архиве' : ''}</option>)}</select></div>
      <div className="field"><label htmlFor="report-group">Группа отчёта</label><select id="report-group" value={groupId} onChange={(event) => { const group = filters?.groups.find((item) => item.id === event.target.value); setSelection({ ...selection, groupId: event.target.value, clubId: group?.clubId ?? clubId }); }}><option value="">Все группы</option>{filters?.groups.filter((group) => !clubId || group.clubId === clubId).map((group) => <option key={group.id} value={group.id}>{group.name} · {clubs.get(group.clubId)?.name}{group.archived ? ' · в архиве' : ''}</option>)}</select></div>
      <label className="field">Период с<input type="date" min="0001-01-01" max="9999-12-31" value={from} onChange={(event) => editDate('from', event.target.value)} /></label>
      <label className="field">Период по<input type="date" min="0001-01-01" max="9999-12-31" value={until} onChange={(event) => editDate('until', event.target.value)} /></label>
    </div>
    <p className="help">Даты включены. «Сегодня» — по Минску; занятия — по своему сохранённому часовому поясу.</p>
    <div className="report-context"><h3>{context}</h3>{report && selectedClub && !selectedClub.archived && <a className="report-session-link" href={sessionHref(clubId, { groupId, from, through: until, view: 'period' })}>Занятия выбранного контекста</a>}{selectedClub?.archived && <p className="help">Исторический отчёт. Филиал в архиве; рабочие действия недоступны.</p>}</div>
    {loading && <p role="status">Считаем посещаемость…</p>}
    {error && <div role="alert"><p className="error">{error}</p><button type="button" disabled={loading} onClick={refresh}>Повторить загрузку статистики</button></div>}
    {report && <AttendanceReport report={report} onSelect={(context) => setSelection({ ...selection, ...context })} />}
  </section>;
}

import { useState } from 'react';
import { routePath } from '../../app/navigation';
import { AttendanceSummary, ReportComparison } from '../owner-dashboard/AttendanceReport';
import { useAttendanceReport } from '../owner-dashboard/useAttendanceReport';
import { reportPeriod, type Period } from '../owner-dashboard/periods';
import { sessionHref } from '../sessions/navigation';

export function OwnerOverview({ clubId }: { clubId: string }) {
  const [period, setPeriod] = useState<Period>('month');
  const [dates, setDates] = useState(() => reportPeriod('month'));
  const [custom, setCustom] = useState(dates);
  const [datesOpen, setDatesOpen] = useState(false);
  const [context, setContext] = useState({ clubId, groupId: '' });
  const block = useAttendanceReport({ ...dates, ...context });
  const selectedClub = block.filters?.clubs.find((club) => club.id === context.clubId);
  const selectedGroup = block.filters?.groups.find((group) => group.id === context.groupId);
  const groups = block.filters?.groups.filter((group) => !context.clubId || group.clubId === context.clubId) ?? [];
  function choosePeriod(value: Period) { setPeriod(value); setDates(value === 'custom' ? custom : reportPeriod(value)); setDatesOpen(value === 'custom'); }
  function changeDate(key: 'from' | 'until', value: string) { const next = { ...dates, [key]: value }; setPeriod('custom'); setDates(next); setCustom(next); }
  return <section className="dashboard-panel dashboard-report" aria-labelledby="home-report-title" aria-busy={block.loading}>
    <div className="dashboard-section-heading"><h2 id="home-report-title">Обзор посещаемости</h2><a href={routePath({ clubId, section: 'statistics' })}>Полный отчёт</a></div>
    <div className="dashboard-report-filters">
      <label className="field">Период главной<select aria-label="Период главной" value={period} onChange={(event) => choosePeriod(event.target.value as Period)}>{([['today', 'Сегодня'], ['week', 'Неделя'], ['month', 'Месяц'], ['custom', 'Свой период']] as const).map(([value, name]) => <option key={value} value={value}>{name}</option>)}</select></label>
      <label className="field">Группа отчёта<select aria-label="Группа отчёта" value={context.groupId} onChange={(event) => {
        const group = block.filters?.groups.find((item) => item.id === event.target.value);
        setContext({ clubId: group?.clubId ?? context.clubId, groupId: group?.id ?? '' });
      }}><option value="">Все группы</option>{groups.map((group) => <option key={group.id} value={group.id}>{group.name} · {block.filters?.clubs.find((club) => club.id === group.clubId)?.name}{group.archived ? ' · в архиве' : ''}</option>)}</select></label>
    </div>
    <details className="dashboard-date-editor" open={datesOpen} onToggle={(event) => setDatesOpen(event.currentTarget.open)}><summary>Даты периода · {dates.from} — {dates.until}</summary><div>
      <label className="field">С даты<input type="date" value={dates.from} onChange={(event) => changeDate('from', event.target.value)} /></label>
      <label className="field">По дату<input type="date" value={dates.until} onChange={(event) => changeDate('until', event.target.value)} /></label>
    </div></details>
    <p className="dashboard-report-scope">{selectedClub?.name ?? 'Вся сеть'}{selectedClub?.archived && ' · в архиве'}{selectedGroup && ` · ${selectedGroup.name}`}</p>
    {block.loading && <p role="status">Загружаем обзор посещаемости…</p>}
    {block.error && <div role="alert" className="error"><p>{block.error}</p><button type="button" onClick={block.refresh}>Повторить загрузку обзора</button></div>}
    {block.report && <><AttendanceSummary report={block.report} heading={false} /><ReportComparison rows={block.report.clubs} compact onSelect={(selection) => setContext(selection)} />
      <details className="dashboard-report-explanation"><summary>Как читать обзор</summary><p className="help">Быстрые периоды — по Минску, обе даты включены. День каждого занятия — в его сохранённом timezone. Процент учитывает только закрытые журналы. «—» означает отсутствие отметок; отменённые исключены. Все показатели филиалов и групп доступны в полном отчёте.</p></details>
      {context.clubId && !selectedClub?.archived && <div className="dashboard-links"><a href={sessionHref(context.clubId, { groupId: context.groupId, from: dates.from, through: dates.until, view: 'period' })}>Занятия этого периода</a><a href={routePath({ clubId: context.clubId, section: 'home' })}>Рабочий день филиала</a></div>}
      {context.clubId !== clubId && <button type="button" className="secondary" onClick={() => setContext({ clubId, groupId: '' })}>Вернуться к контексту главной</button>}
    </>}
  </section>;
}

import { percentage, type Report, type Row, type Totals } from './types';
import './owner-dashboard.css';

type Selection = { clubId: string; groupId: string };
const fields: { key: keyof Omit<Totals, 'percentage'>; label: string }[] = [
  { key: 'closed', label: 'Проведено' }, { key: 'planned', label: 'Запланировано' }, { key: 'open', label: 'Открыто' },
  { key: 'present', label: 'Присутствия' }, { key: 'absent', label: 'Пропуски' },
  { key: 'unknown', label: 'Неизвестна' }, { key: 'illness', label: 'Болезнь' }, { key: 'excused', label: 'Уважительная' },
];
/** Presentation only. RD-05 reuses the same authorized report and filters. */
export function ReportComparison({ rows, groups = false, compact = false, onSelect }: { rows: Row[]; groups?: boolean; compact?: boolean; onSelect?: (selection: Selection) => void }) {
  const title = groups ? 'Сравнение групп' : 'Сравнение филиалов';
  const identity = (row: Row) => <>
    {groups && <strong>{row.groupName}{row.groupArchived && <span className="report-archive">Группа в архиве</span>}</strong>}
    <span>{row.clubName}{row.clubArchived && <span className="report-archive">Филиал в архиве</span>}</span>
  </>;
  const name = (row: Row) => onSelect ? <button type="button" className="report-drilldown" onClick={() => onSelect({ clubId: row.clubId, groupId: groups ? row.groupId ?? '' : '' })}>{identity(row)}</button> : <div className="report-identity">{identity(row)}</div>;
  return <section className="report-comparison" aria-label={title}>
    <h3>{title}</h3>
    {rows.length === 0 ? <p className="help">Нет {groups ? 'групп' : 'филиалов'} для выбранных фильтров.</p> : <>
      <div className="report-table-scroll" role="region" aria-label={title} tabIndex={0}>
        <table className="report-table"><caption>{title}{!compact && ' · все показатели'}</caption><thead><tr>
          <th scope="col">{groups ? 'Группа · Филиал' : 'Филиал'}</th><th scope="col">Посещаемость</th><th scope="col">Проведено</th>{!compact && <><th scope="col">Незавершённые</th><th scope="col">Отметки</th><th scope="col">Причины пропусков</th></>}
        </tr></thead><tbody>{rows.map((row) => <tr key={groups ? row.groupId : row.clubId} data-club-id={row.clubId} data-group-id={row.groupId ?? undefined}>
          <th scope="row">{name(row)}</th><td className="report-rate">{percentage(row.percentage)}</td><td>{row.closed}</td>
          {!compact && ([['planned', 'open'], ['present', 'absent'], ['unknown', 'illness', 'excused']] as const).map((keys) => <td key={keys[0]}><dl className="report-cell-facts">{keys.map((key) => <div key={key}><dt>{fields.find((field) => field.key === key)?.label}</dt><dd>{row[key]}</dd></div>)}</dl></td>)}
        </tr>)}</tbody></table>
      </div>
      <ul className="report-mobile-list" aria-label={title}>{rows.map((row) => <li key={groups ? row.groupId : row.clubId} data-club-id={row.clubId} data-group-id={row.groupId ?? undefined}>
        <div className="report-mobile-heading">{name(row)}<span className="report-rate"><span className="help">Посещаемость</span>{percentage(row.percentage)}</span></div>
        <dl>{fields.filter((field) => !compact || ['closed', 'present', 'absent'].includes(field.key)).map(({ key, label }) => <div key={key}><dt>{label}</dt><dd>{row[key]}</dd></div>)}</dl>
      </li>)}</ul>
    </>}
  </section>;
}

export function AttendanceSummary({ report, heading = true }: { report: Report; heading?: boolean }) {
  return <>
    {heading && <h3>Итог выбранного периода · {report.from} — {report.until}</h3>}
    <dl className="report-totals">
      <div><dt>Проведённые занятия</dt><dd data-metric="closed">{report.total.closed}</dd></div>
      <div><dt>Присутствия</dt><dd data-metric="present">{report.total.present}</dd></div>
      <div><dt>Пропуски</dt><dd data-metric="absent">{report.total.absent}</dd></div>
      <div><dt>Посещаемость</dt><dd data-metric="percentage">{percentage(report.total.percentage)}</dd></div>
    </dl>
    <dl className="report-pending"><div><dt>Запланировано</dt><dd data-metric="planned">{report.total.planned}</dd></div><div><dt>Открыто, не завершено</dt><dd data-metric="open">{report.total.open}</dd></div></dl>
    <p className="report-reasons" data-metric="reasons">Причины пропусков: неизвестна — {report.total.unknown}; болезнь — {report.total.illness}; уважительная — {report.total.excused}.</p>
    {report.total.closed + report.total.planned + report.total.open === 0 && <p role="status">В выбранном периоде занятий нет. Выберите другой период или филиал.</p>}
  </>;
}

export function AttendanceReport({ report, onSelect }: { report: Report; onSelect?: (selection: Selection) => void }) {
  return <div className="attendance-report">
    <AttendanceSummary report={report} />
    <ReportComparison rows={report.clubs} onSelect={onSelect} /><ReportComparison rows={report.groups} groups onSelect={onSelect} />
    <details className="report-explanation"><summary>Как считается посещаемость</summary><p className="help">Посещаемость = присутствия / (присутствия + пропуски) × 100 по закрытым занятиям, округлена до 0,1%. «—» означает отсутствие отметок. Пустое закрытое занятие считается проведённым. Отменённые исключены; запланированные и открытые не входят в процент.</p><p className="help">Текущие исправления учитываются при обновлении: после изменения посещаемости, каждые 30 секунд и при возврате в окно.</p></details>
  </div>;
}

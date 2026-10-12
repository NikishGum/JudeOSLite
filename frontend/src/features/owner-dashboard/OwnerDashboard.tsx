import { useEffect, useState } from 'react';
import { apiGet, ApiError } from '../../shared/api';
import { localDate } from '../schedule/types';
import { percentage, type Filters, type Report, type Row } from './types';
import './owner-dashboard.css';

function Comparison({ rows, groups }: { rows: Row[]; groups: boolean }) {
  const caption = groups ? 'Сравнение групп' : 'Сравнение филиалов';
  return <div className="report-table-scroll" role="region" aria-label={caption} tabIndex={0}>
    <table className="report-table"><caption>{caption}</caption><thead><tr>
      <th scope="col">{groups ? 'Группа · Филиал' : 'Филиал'}</th><th scope="col">Проведено</th><th scope="col">Запланировано</th><th scope="col">Открыто</th>
      <th scope="col">Присутствия</th><th scope="col">Пропуски</th><th scope="col">Неизвестна</th><th scope="col">Болезнь</th><th scope="col">Уважительная</th><th scope="col">Посещаемость</th>
    </tr></thead><tbody>{rows.map((row) => <tr key={groups ? row.groupId : row.clubId} data-club-id={row.clubId} data-group-id={row.groupId ?? undefined}>
      <th scope="row">{groups ? <>{row.groupName}{row.groupArchived ? ' · группа в архиве' : ''}<br /></> : null}{row.clubName}{row.clubArchived ? ' · филиал в архиве' : ''}</th>
      <td>{row.closed}</td><td>{row.planned}</td><td>{row.open}</td><td>{row.present}</td><td>{row.absent}</td><td>{row.unknown}</td><td>{row.illness}</td><td>{row.excused}</td><td>{percentage(row.percentage)}</td>
    </tr>)}</tbody></table>{rows.length === 0 && <p>Нет {groups ? 'групп' : 'филиалов'} для выбранных фильтров.</p>}
  </div>;
}

export function OwnerDashboard() {
  const [from, setFrom] = useState(() => `${localDate('Europe/Minsk').slice(0, 7)}-01`);
  const [until, setUntil] = useState(() => localDate('Europe/Minsk'));
  const [clubId, setClubId] = useState(''), [groupId, setGroupId] = useState('');
  const [filters, setFilters] = useState<Filters | null>(null), [report, setReport] = useState<Report | null>(null);
  const [loading, setLoading] = useState(true), [error, setError] = useState(''), [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const refresh = () => { if (document.visibilityState === 'visible') setAttempt((n) => n + 1); };
    window.addEventListener('focus', refresh);
    window.addEventListener('attendance-changed', refresh);
    const interval = window.setInterval(refresh, 30_000);
    return () => { window.clearInterval(interval); window.removeEventListener('focus', refresh); window.removeEventListener('attendance-changed', refresh); };
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true); setError(''); setReport(null);
    if (!from || !until || from > until) {
      setLoading(false); setError('Укажите обе даты; начало периода должно быть не позже конца.');
      return () => controller.abort();
    }
    const query = new URLSearchParams({ from, until, clubId, groupId });
    Promise.all([apiGet<Filters>('/api/reports/filters', controller.signal), apiGet<Report>(`/api/reports/attendance?${query}`, controller.signal)])
      .then(([f, r]) => { if (!controller.signal.aborted) { setFilters(f); setReport(r); } })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return;
        setError(cause instanceof ApiError ? cause.message : 'Нет соединения. Повторите загрузку статистики.');
        if (cause instanceof ApiError && (cause.status === 401 || cause.status === 403)) setFilters(null);
      })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [from, until, clubId, groupId, attempt]);
  const clubs = new Map(filters?.clubs.map((c) => [c.id, c.name]));
  return <section className="connection owner-dashboard" aria-labelledby="owner-statistics-title" aria-busy={loading}>
    <h2 id="owner-statistics-title">Статистика владельца</h2>
    <p className="help">По локальной дате начала занятия в его сохранённом часовом поясе. Обе границы периода включены.</p>
    <div className="report-filters">
      <div className="field"><label htmlFor="report-club">Филиал отчёта</label><select id="report-club" value={clubId} onChange={(e) => { setClubId(e.target.value); setGroupId(''); }}><option value="">Вся сеть</option>{filters?.clubs.map((c) => <option key={c.id} value={c.id}>{c.name}{c.archived ? ' · в архиве' : ''}</option>)}</select></div>
      <div className="field"><label htmlFor="report-group">Группа отчёта</label><select id="report-group" value={groupId} onChange={(e) => setGroupId(e.target.value)}><option value="">Все группы</option>{filters?.groups.filter((g) => !clubId || g.clubId === clubId).map((g) => <option key={g.id} value={g.id}>{g.name} · {clubs.get(g.clubId)}{g.archived ? ' · в архиве' : ''}</option>)}</select></div>
      <label className="field">Период с<input type="date" min="0001-01-01" max="9999-12-31" value={from} onChange={(e) => setFrom(e.target.value)} /></label>
      <label className="field">Период по<input type="date" min="0001-01-01" max="9999-12-31" value={until} onChange={(e) => setUntil(e.target.value)} /></label>
    </div>
    <button disabled={loading} onClick={() => setAttempt((n) => n + 1)}>Обновить статистику</button>
    {loading && <p role="status">Считаем посещаемость…</p>}
    {error && <div role="alert"><p className="error">{error}</p><button disabled={loading} onClick={() => setAttempt((n) => n + 1)}>Повторить загрузку статистики</button></div>}
    {report && <>
      <h3>Итог выбранного периода · {report.from} — {report.until}</h3>
      <dl className="report-totals">
        <div><dt>Проведённые занятия</dt><dd data-metric="closed">{report.total.closed}</dd></div>
        <div><dt>Присутствия</dt><dd data-metric="present">{report.total.present}</dd></div>
        <div><dt>Пропуски</dt><dd data-metric="absent">{report.total.absent}</dd></div>
        <div><dt>Посещаемость</dt><dd data-metric="percentage">{percentage(report.total.percentage)}</dd></div>
        <div><dt>Запланировано</dt><dd data-metric="planned">{report.total.planned}</dd></div>
        <div><dt>Открыто, не завершено</dt><dd data-metric="open">{report.total.open}</dd></div>
      </dl>
      <p data-metric="reasons">Причины пропусков: неизвестна — {report.total.unknown}; болезнь — {report.total.illness}; уважительная — {report.total.excused}.</p>
      <p className="help">Посещаемость = присутствия / (присутствия + пропуски) × 100 по закрытым занятиям, округлена до 0,1%. «—» означает отсутствие отметок. Пустое закрытое занятие считается проведённым. Отменённые исключены; запланированные и открытые не входят в процент.</p>
      <p className="help">Текущие исправления учитываются при обновлении. После исправления в этом окне отчёт обновляется сразу; также каждые 30 секунд и при возврате в окно.</p>
      <Comparison rows={report.clubs} groups={false} /><Comparison rows={report.groups} groups />
    </>}
  </section>;
}

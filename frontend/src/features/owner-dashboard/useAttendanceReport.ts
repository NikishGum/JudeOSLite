import { useEffect, useState } from 'react';
import { apiGet, ApiError } from '../../shared/api';
import type { Filters, Report, ReportSelection } from './types';

/** Read-only API contract for statistics and RD-05. Totals are calculated only by SQL. */
export function useAttendanceReport({ from, until, clubId, groupId }: ReportSelection) {
  const [attempt, setAttempt] = useState(0);
  const key = JSON.stringify([from, until, clubId, groupId, attempt]);
  const [state, setState] = useState<{ key: string; report: Report | null; loading: boolean; error: string }>({ key: '', report: null, loading: true, error: '' });
  const [filters, setFilters] = useState<Filters | null>(null);
  const refresh = () => setAttempt((value) => value + 1);
  useEffect(() => {
    const refreshVisible = () => { if (document.visibilityState === 'visible') setAttempt((value) => value + 1); };
    window.addEventListener('focus', refreshVisible);
    window.addEventListener('attendance-changed', refreshVisible);
    document.addEventListener('visibilitychange', refreshVisible);
    const timer = window.setInterval(refreshVisible, 30_000);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener('focus', refreshVisible);
      window.removeEventListener('attendance-changed', refreshVisible);
      document.removeEventListener('visibilitychange', refreshVisible);
    };
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    if (!from || !until || from > until) {
      setState({ key, report: null, loading: false, error: 'Укажите обе даты; начало периода должно быть не позже конца.' });
      return () => controller.abort();
    }
    setState({ key, report: null, loading: true, error: '' });
    const query = new URLSearchParams({ from, until, clubId, groupId });
    Promise.all([apiGet<Filters>('/api/reports/filters', controller.signal), apiGet<Report>(`/api/reports/attendance?${query}`, controller.signal)])
      .then(([allowed, report]) => { if (!controller.signal.aborted) { setFilters(allowed); setState({ key, report, loading: false, error: '' }); } })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return;
        if (cause instanceof ApiError && (cause.status === 401 || cause.status === 403)) setFilters(null);
        setState({ key, report: null, loading: false, error: cause instanceof ApiError ? cause.message : 'Нет соединения. Повторите загрузку статистики.' });
      });
    return () => controller.abort();
  }, [from, until, clubId, groupId, key]);
  // Never render the previous selection, even before effect cleanup.
  return { filters, refresh, ...(state.key === key ? state : { report: null, loading: true, error: '' }) };
}

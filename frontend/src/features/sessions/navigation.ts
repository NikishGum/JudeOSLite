import { localDate } from '../schedule/types';
import { nextDate } from './types';

export type SessionLocation = { groupId: string; sessionId: string; from: string; through: string; view: 'day' | 'week' | 'period'; journal: 'trainer' | 'admin'; trainerId: string; status: string; scrollY: number; hasCalendar: boolean };
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
function validDate(value: string | null): value is string {
  return !!value && /^\d{4}-\d{2}-\d{2}$/.test(value) && !Number.isNaN(Date.parse(value)) && new Date(`${value}T00:00:00Z`).toISOString().slice(0, 10) === value;
}
export function sessionLocation(timezone: string): SessionLocation {
  const query = new URLSearchParams(window.location.search);
  const from = query.get('from'), through = query.get('through');
  const view = query.get('view') === 'day' ? 'day' : query.get('view') === 'period' ? 'period' : 'week';
  const selected = validDate(from) ? from : localDate(timezone);
  const start = view === 'week' ? weekStart(selected) : selected;
  const pathId = window.location.pathname.split('/').filter(Boolean)[3];
  return { groupId: query.get('group') ?? '', sessionId: pathId ?? query.get('session') ?? '', from: start,
    through: view === 'week' ? nextDate(start, 6) : view === 'day' ? start : validDate(through) ? through : start, view,
    journal: query.get('journal') === 'trainer' ? 'trainer' : 'admin', trainerId: query.get('trainer') ?? '',
    status: query.get('status') ?? '', scrollY: Math.max(0, Math.min(1_000_000, Number(query.get('scroll')) || 0)),
    hasCalendar: ['from', 'through', 'view', 'group', 'trainer', 'status'].some((key) => query.has(key)) };
}
export function validSessionIds(location: SessionLocation) {
  return (!location.groupId || uuid.test(location.groupId)) && (!location.sessionId || uuid.test(location.sessionId)) && (!location.trainerId || uuid.test(location.trainerId)) && (!location.status || ['planned', 'open', 'closed', 'cancelled'].includes(location.status));
}
/** Canonical session links carry only whitelisted calendar context, never a return URL. */
export function sessionHref(clubId: string, location: Partial<SessionLocation> = {}) {
  const query = new URLSearchParams();
  if (location.groupId) query.set('group', location.groupId);
  if (location.from) query.set('from', location.from);
  if (location.through) query.set('through', location.through);
  if (location.view) query.set('view', location.view);
  if (location.journal) query.set('journal', location.journal);
  if (location.trainerId) query.set('trainer', location.trainerId);
  if (location.status) query.set('status', location.status);
  if (location.scrollY) query.set('scroll', String(location.scrollY));
  return `/clubs/${encodeURIComponent(clubId)}/sessions${location.sessionId ? `/${encodeURIComponent(location.sessionId)}` : ''}${query.size ? `?${query}` : ''}`;
}
export function weekStart(date: string) {
  return nextDate(date, -((new Date(`${date}T12:00:00Z`).getUTCDay() + 6) % 7));
}
export function trainingChanged() { window.dispatchEvent(new Event('attendance-changed')); }

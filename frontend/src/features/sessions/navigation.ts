import { localDate } from '../schedule/types';
import { nextDate } from './types';

export type SessionLocation = { groupId: string; sessionId: string; from: string; through: string; view: 'day' | 'week' | 'period'; journal: 'trainer' | 'admin' };
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
function validDate(value: string | null): value is string {
  return !!value && /^\d{4}-\d{2}-\d{2}$/.test(value) && !Number.isNaN(Date.parse(value)) && new Date(`${value}T00:00:00Z`).toISOString().slice(0, 10) === value;
}
export function sessionLocation(timezone: string): SessionLocation {
  const query = new URLSearchParams(window.location.search);
  const from = query.get('from'), through = query.get('through');
  const start = validDate(from) ? from : localDate(timezone);
  return { groupId: query.get('group') ?? '', sessionId: query.get('session') ?? '', from: start,
    through: validDate(through) ? through : start, view: query.get('view') === 'week' ? 'week' : query.get('view') === 'period' ? 'period' : 'day',
    journal: query.get('journal') === 'trainer' ? 'trainer' : 'admin' };
}
export function validSessionIds(location: SessionLocation) {
  return (!location.groupId || uuid.test(location.groupId)) && (!location.sessionId || uuid.test(location.sessionId));
}
/** Public RD-05 adapter. IDs and date filters never grant access. */
export function sessionHref(clubId: string, location: Partial<SessionLocation> = {}) {
  const query = new URLSearchParams();
  if (location.groupId) query.set('group', location.groupId);
  if (location.sessionId) query.set('session', location.sessionId);
  if (location.from) query.set('from', location.from);
  if (location.through) query.set('through', location.through);
  if (location.view) query.set('view', location.view);
  if (location.journal) query.set('journal', location.journal);
  return `/clubs/${encodeURIComponent(clubId)}/sessions${query.size ? `?${query}` : ''}`;
}
export function weekStart(date: string) {
  return nextDate(date, -((new Date(`${date}T12:00:00Z`).getUTCDay() + 6) % 7));
}
/** Invalidates read-only consumers after a server-confirmed operation. No personal data. */
export function trainingChanged() { window.dispatchEvent(new Event('attendance-changed')); }

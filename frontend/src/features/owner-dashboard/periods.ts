import { localDate } from '../schedule/types';

export type Period = 'today' | 'week' | 'month' | 'custom';
/** Minsk calendar, inclusive. Week starts Monday; month starts on the first. */
export function reportPeriod(period: Exclude<Period, 'custom'>, today = localDate('Europe/Minsk')) {
  const start = new Date(`${today}T12:00:00Z`);
  if (period === 'week') start.setUTCDate(start.getUTCDate() - (start.getUTCDay() + 6) % 7);
  const from = period === 'month' ? `${today.slice(0, 7)}-01` : period === 'week' ? start.toISOString().slice(0, 10) : today;
  return { from, until: today };
}

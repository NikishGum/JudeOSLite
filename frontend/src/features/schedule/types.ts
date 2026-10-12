import { ApiError } from '../../shared/api';

export type ScheduleInput = {
  groupId: string; trainerId: string; mode: 'weekly' | 'day_parity' | 'date_list' | 'once';
  weekdays?: number[]; dayParity?: string; selectedDates?: string[];
  startsTime: string; durationMinutes: number; from: string; until: string | null; version: number; previewToken?: string;
};
export type Rule = ScheduleInput & { id: string; clubId: string; seriesId: string; timezone: string };
export type Session = { id: string; date: string; timezone: string; startsAt: string; endsAt: string; status: string; manualOverride: boolean; rosterFrozen: boolean; trainerId: string };
export type Preview = { token: string; timezone: string; dates: { date: string; startsAt: string; endsAt: string }[]; affected: Session[]; preserved: Session[] };
export type TrainerOption = { id: string; displayName: string };
export const modeNames = { weekly: 'Дни недели', day_parity: 'Чётные / нечётные числа месяца', date_list: 'Выбранные даты календаря', once: 'Разовое занятие' };
export const statusNames: Record<string, string> = { planned: 'Плановое', open: 'Журнал открыт', closed: 'Завершено', cancelled: 'Отменено' };
export const message = (cause: unknown) => cause instanceof ApiError ? cause.message : 'Нет соединения. Проверьте интернет и повторите.';
export function localDate(zone: string) {
  const parts = new Intl.DateTimeFormat('en', { timeZone: zone, year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(new Date());
  return ['year', 'month', 'day'].map((part) => parts.find((p) => p.type === part)?.value).join('-');
}
export function dateLabel(date: string) { return date.split('-').reverse().join('.'); }
export function timeLabel(value: string, zone: string) { return new Intl.DateTimeFormat('ru', { timeZone: zone, hour: '2-digit', minute: '2-digit' }).format(new Date(value)); }

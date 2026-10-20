export type Session = {
  id: string; clubId: string; groupId: string; groupName: string; ruleId: string | null;
  trainerId: string; trainerName: string; previousTrainerName: string | null; date: string; sourceDate: string; timezone: string;
  startsAt: string; endsAt: string; status: 'planned' | 'open' | 'closed' | 'cancelled';
  manualOverride: boolean; rosterFrozen: boolean; closedAt: string | null;
  cancelledAt: string | null; cancellationReason: string | null; supersededAt: string | null; version: number;
};
export type Action = 'cancel' | 'replace-trainer' | 'reschedule';
export const actionNames = { cancel: 'Отменить занятие', 'replace-trainer': 'Заменить тренера', reschedule: 'Перенести занятие' };
export function nextDate(date: string, days = 1) {
  const value = new Date(`${date}T00:00:00Z`); value.setUTCDate(value.getUTCDate() + days);
  return value.toISOString().slice(0, 10);
}

export function durationLabel(session: Session) { return `${Math.round((Date.parse(session.endsAt) - Date.parse(session.startsAt)) / 60000)} мин`; }
export function isHappening(session: Session, now: number) { return session.status !== 'cancelled' && session.status !== 'closed' && Date.parse(session.startsAt) <= now && now < Date.parse(session.endsAt); }
export function localDateOf(value: string, timezone: string) { return new Intl.DateTimeFormat('en-CA', { timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date(value)); }
export type SessionRoster = { source: 'preliminary' | 'snapshot'; date: string; members: { membershipId: string; fullName: string; archived: boolean }[]; summary: { present: number; absent: number; unmarked: number } | null };

export type Session = {
  id: string; clubId: string; groupId: string; groupName: string; ruleId: string | null;
  trainerId: string; trainerName: string; date: string; sourceDate: string; timezone: string;
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

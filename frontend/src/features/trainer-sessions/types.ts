import type { Session, SessionRoster } from '../sessions/types';
export type AttendanceRecord = {
  id: string; membershipId: string; sourceGroupMembershipId: string; fullName: string; archived: boolean;
  state: 'present' | 'absent' | null; absenceReason: 'unknown' | 'illness' | 'excused' | null;
  markedBy: string | null; markedAt: string | null; origin: 'manual' | 'close_default' | 'correction' | null; version: number;
};
export type Journal = { session: Session; roster: SessionRoster; records: AttendanceRecord[] };
export const stateNames = { planned: 'Запланировано', open: 'Журнал открыт', closed: 'Завершено', cancelled: 'Отменено' };
export const reasonNames = { unknown: 'Неизвестна', illness: 'Болезнь', excused: 'Уважительная' };

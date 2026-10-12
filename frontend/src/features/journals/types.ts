import type { Session } from '../sessions/types';
import type { AttendanceRecord } from '../trainer-sessions/types';
export type Record = Omit<AttendanceRecord, 'origin'> & { origin: 'manual' | 'close_default' | 'correction' | null; changeReason: string | null };
export type Journal = { session: Session; records: Record[] };
export type Value = Pick<Record, 'state' | 'absenceReason' | 'markedBy' | 'markedAt' | 'origin' | 'version' | 'changeReason'>;
export type Event = { id: string; actorId: string; actorName: string; createdAt: string; action: string; before: Value; after: Value; changeReason: string | null };
const reasons = { unknown: 'Причина неизвестна', illness: 'Болезнь', excused: 'Уважительная причина' };
export const markLabel = (value: Pick<Value, 'state' | 'absenceReason'>) => value.state === null ? 'Не отмечено' : value.state === 'present' ? 'Присутствует' : `Отсутствует · ${value.absenceReason ? reasons[value.absenceReason] : 'Причина неизвестна'}`;
export const originLabel = (origin: Value['origin']) => origin === 'close_default' ? 'При завершении журнала' : origin === 'correction' ? 'Исправление администратора' : 'Ручная отметка';

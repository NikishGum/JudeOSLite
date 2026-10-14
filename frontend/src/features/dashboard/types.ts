export type Mode = 'owner' | 'administrator' | 'trainer';
export type Club = { id: string; name: string; timezone: string; today: string; canManage: boolean };
export type Session = {
  id: string; clubId: string; groupId: string; groupName: string; trainerName: string;
  timezone: string; date: string; branchDate: string; startsAt: string; endsAt: string;
  status: 'planned' | 'open' | 'closed' | 'cancelled'; canMark: boolean; roster: number; marked: number;
};
export type Panel = {
  mode: Mode; readAt: string; clubs: Club[];
  counters: { participants: number | null; groups: number; today: number; open: number };
  next: Session | null; today: Session[]; unfinished: Session[];
  groups: { id: string; clubId: string; name: string; members: number }[];
};
export const calendarDate = (value: string) => new Intl.DateTimeFormat('ru-BY', { day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' }).format(new Date(`${value}T12:00:00Z`));
export const sessionTime = (session: Session, value: string) => new Intl.DateTimeFormat('ru-BY', { hour: '2-digit', minute: '2-digit', timeZone: session.timezone }).format(new Date(value));

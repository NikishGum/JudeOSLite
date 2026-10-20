export type StaffRole = 'trainer' | 'administrator';
export type StaffAssignment = { clubId: string; clubName: string; role: StaffRole; archived: boolean; canRevoke: boolean };
export type StaffCard = { id: string; login: string; displayName: string; disabled: boolean; version: number; canManageAccount: boolean; assignments: StaffAssignment[] };
export type StaffDirectory = { staff: StaffCard[]; clubs: { id: string; name: string; archived: boolean }[]; canAssignExisting: boolean };
export const staffRoles = { trainer: 'Тренер', administrator: 'Администратор' };
export const assignmentLabel = (assignment: StaffAssignment) => `${staffRoles[assignment.role]} · ${assignment.clubName}${assignment.archived ? ' (архив)' : ''}`;
export const accountState = (person: StaffCard) => person.disabled ? 'Аккаунт отключён' : person.assignments.length === 0 ? 'Включён · без назначений' : person.assignments.every((assignment) => assignment.archived) ? 'Включён · только архивные филиалы' : 'Аккаунт включён';

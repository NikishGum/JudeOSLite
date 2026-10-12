export type Group = { id: string; clubId: string; name: string; archived: boolean; version: number };
export type Interval = { id: string; clubId: string; groupId: string; membershipId: string; groupName: string; fullName: string; birthDate: string; cardArchived: boolean; groupArchived: boolean; from: string; until: string | null; version: number };
export const displayDate = (value: string) => value.split('-').reverse().join('.');

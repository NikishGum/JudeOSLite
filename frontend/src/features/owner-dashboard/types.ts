export type Totals = {
  closed: number;
  planned: number;
  open: number;
  present: number;
  absent: number;
  unknown: number;
  illness: number;
  excused: number;
  percentage: number | null;
};
export type Row = Totals & {
  clubId: string;
  clubName: string;
  clubArchived: boolean;
  groupId: string | null;
  groupName: string | null;
  groupArchived: boolean;
};
export type OpenSession = {
  id: string;
  clubId: string;
  groupId: string;
  date: string;
  startsAt: string;
  timezone: string;
};
export type Report = {
  from: string;
  until: string;
  total: Totals;
  clubs: Row[];
  groups: Row[];
  openSessions?: OpenSession[];
};
export type ReportSelection = {
  from: string;
  until: string;
  clubId: string;
  groupId: string;
};
export type Filters = {
  clubs: { id: string; name: string; archived: boolean }[];
  groups: { id: string; clubId: string; name: string; archived: boolean }[];
};
export const percentage = (value: number | null) =>
  value === null
    ? "—"
    : `${value.toLocaleString("ru-RU", { minimumFractionDigits: 1, maximumFractionDigits: 1 })}%`;

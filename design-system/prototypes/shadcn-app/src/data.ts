import type {
  Panel,
  Session,
} from "../../../../frontend/src/features/dashboard/types";
import type { Card as Participant } from "../../../../frontend/src/features/participants/ParticipantEditor";
import type { Group } from "../../../../frontend/src/features/groups/types";
export {
  allowedSections,
  sections,
  type Section,
} from "../../../../frontend/src/app/navigation";
export type Role = Panel["mode"];
export const roleNames = {
  owner: "Владелец",
  administrator: "Администратор",
  trainer: "Тренер",
};
export const demoDate = "2026-10-16";
const uuid = (n: number) =>
  `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
export const clubs: Panel["clubs"] = [
  "PRIDE · Центральный",
  "PRIDE · Восток",
  "PRIDE · Уручье",
].map((name, i) => ({
  id: uuid(i + 1),
  name,
  timezone: "Europe/Minsk",
  today: demoDate,
  canManage: true,
}));
export const groups: Group[] = clubs.flatMap((club, i) =>
  ["Младшая группа", "Старшая группа"].map((name, j) => ({
    id: uuid(10 + i * 2 + j),
    clubId: club.id,
    name,
    archived: false,
    version: 1,
  })),
);
const names = ["Алексей Демо", "Мария Пример", "Иван Образец", "София Макет"];
export const participants: Participant[] = clubs.flatMap((club, i) =>
  names.map((fullName, j) => ({
    id: uuid(100 + i * 4 + j),
    participantId: uuid(200 + i * 4 + j),
    clubId: club.id,
    fullName,
    birthDate: `201${j + 2}-0${j + 1}-15`,
    participantVersion: 1,
    joinedOn: "2026-09-01",
    contactKind: "email",
    contactValue: `demo-${i}-${j}@example.invalid`,
    contactName: "Демонстрационный контакт",
    archived: j === 3,
    version: 1,
    canEditIdentity: true,
  })),
);
export const memberGroup = (p: Participant) =>
  groups.filter((g) => g.clubId === p.clubId)[
    participants
      .filter((x) => x.clubId === p.clubId)
      .findIndex((x) => x.id === p.id) % 2
  ];
export const sessions: Session[] = groups.map((group, i) => ({
  id: uuid(300 + i),
  clubId: group.clubId,
  groupId: group.id,
  groupName: group.name,
  trainerName: i % 2 ? "Александр Демо" : "Елена Пример",
  timezone: "Europe/Minsk",
  date: demoDate,
  branchDate: demoDate,
  startsAt: `${demoDate}T${String(12 + i).padStart(2, "0")}:00:00Z`,
  endsAt: `${demoDate}T${String(13 + i).padStart(2, "0")}:00:00Z`,
  status: (
    ["closed", "closed", "open", "planned", "planned", "cancelled"] as const
  )[i],
  canMark: true,
  roster: 4,
  marked: i < 2 ? 4 : i === 2 ? 2 : 0,
}));
export const statusNames = {
  planned: "Запланировано",
  open: "Открыт журнал",
  closed: "Завершено",
  cancelled: "Отменено",
};
export const time = (s: Session) =>
  new Intl.DateTimeFormat("ru-BY", {
    hour: "2-digit",
    minute: "2-digit",
    timeZone: s.timezone,
  }).format(new Date(s.startsAt));
export const clubName = (id: string) =>
  clubs.find((c) => c.id === id)?.name ?? "";
export const registries = [
  {
    name: "@shadcn",
    title: "Основа приложения",
    text: "Sidebar, Sheet, Table, Select, Alert и состояния. Используются в этом шаблоне.",
    url: "https://ui.shadcn.com/docs/components/sidebar",
    fit: "Используется",
  },
  {
    name: "@reui",
    title: "Таблицы и расписание",
    text: "Data Grid, Event Calendar, Filters, Timeline. Для расширенного реестра и календаря занятий.",
    url: "https://reui.io/docs/components/radix/data-grid",
    fit: "Приоритет",
  },
  {
    name: "@tablecn",
    title: "Реестр с фильтрами",
    text: "Таблица с поиском, сортировкой и пагинацией. Параметры фильтров можно хранить в URL.",
    url: "https://table-cn.vercel.app",
    fit: "Реестр",
  },
  {
    name: "@kibo-ui",
    title: "Календарные представления",
    text: "Calendar и Combobox: сетка месяца, поиск группы или тренера. Серверные правила расписания остаются в Go.",
    url: "https://www.kibo-ui.com/components/calendar",
    fit: "Расписание",
  },
];

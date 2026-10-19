export type List<T> = { items: T[]; total: number; hasMore: boolean };
export type MemberSummary = {
  id: string;
  clubId: string;
  fullName: string;
  birthDate: string;
  groupName: string;
  trainerName: string;
  archived: boolean;
};
export type Family = {
  id: string;
  clubId: string;
  name: string;
  contactName: string;
  phone: string;
  email: string;
  version: number;
  children: MemberSummary[];
  accounts: { id: string; login: string; displayName: string }[];
};
export type Visit = {
  id: string;
  sessionId: string;
  date: string;
  time: string;
  groupName: string;
  state: "present" | "absent" | null;
  absenceReason: "unknown" | "illness" | "excused" | null;
};
export type MemberProfileData = MemberSummary & {
  joinedOn: string;
  contactName: string;
  contactValue: string;
  familyId: string | null;
  familyName: string | null;
  groups: {
    id: string;
    name: string;
    trainerName: string;
    from: string;
    until: string | null;
  }[];
  attendance: {
    present: number;
    absent: number;
    rate: number | null;
    history: Visit[];
  };
  upcoming: {
    id: string;
    date: string;
    time: string;
    groupName: string;
    trainerName: string;
  }[];
};
export type Method = "cash" | "card" | "transfer" | "erip";
export type Plan = {
  id: string;
  clubId: string;
  name: string;
  amountMinor: number;
  visits: number;
  days: number;
  archived: boolean;
  version: number;
};
export type Payment = {
  id: string;
  clubId: string;
  membershipId: string;
  fullName: string;
  description: string;
  amountMinor: number;
  currency: "BYN";
  status: "pending" | "paid" | "void";
  method: Method | null;
  paidOn: string | null;
  createdAt: string;
  version: number;
  voidReason: string | null;
};
export type Subscription = {
  id: string;
  membershipId: string;
  paymentId: string;
  planName: string;
  visits: number;
  used: number;
  validFrom: string;
  validUntil: string;
  status: "pending" | "active" | "expired" | "exhausted" | "cancelled";
};
export type MemberDocument = {
  id: string;
  membershipId: string;
  name: string;
  kind: "medical" | "contract" | "consent" | "other";
  filename: string;
  mediaType: string;
  size: number;
  expiresOn: string | null;
  createdAt: string;
  version: number;
};
export type Notification = {
  id: string;
  kind: "family" | "payment" | "member_document" | "training_session";
  title: string;
  body: string;
  href: string;
  read: boolean;
  createdAt: string;
};
export type BillingSummary = {
  paidMinor: number;
  pendingMinor: number;
  paidCount: number;
  pendingCount: number;
  currency: "BYN";
  byMonth: { month: string; paidMinor: number }[];
};
export const methodNames: Record<Method, string> = {
  cash: "Наличные",
  card: "Карта",
  transfer: "Перевод",
  erip: "ЕРИП",
};
export const paymentNames: Record<Payment["status"], string> = {
  paid: "Оплачен",
  pending: "Ожидает оплаты",
  void: "Аннулирован",
};
export const subscriptionNames: Record<Subscription["status"], string> = {
  active: "Активен",
  pending: "Ожидает оплаты",
  expired: "Срок истёк",
  exhausted: "Занятия использованы",
  cancelled: "Аннулирован",
};
export const documentNames: Record<MemberDocument["kind"], string> = {
  medical: "Медицинская справка",
  contract: "Договор",
  consent: "Согласие",
  other: "Другой документ",
};
export const money = (minor: number) =>
  new Intl.NumberFormat("ru-BY", { style: "currency", currency: "BYN" }).format(
    minor / 100,
  );
export const day = (date: string) =>
  /^\d{4}-\d{2}-\d{2}$/.test(date) ? date.split("-").reverse().join(".") : "—";
export const today = () =>
  new Intl.DateTimeFormat("en-CA", {
    timeZone: "Europe/Minsk",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date());
export function amountMinor(value: string): number | null {
  const normalized = value.trim().replace(",", ".");
  if (!/^\d{1,7}(\.\d{1,2})?$/.test(normalized)) return null;
  const [whole, fraction = ""] = normalized.split(".");
  const amount = Number(whole) * 100 + Number(fraction.padEnd(2, "0"));
  return amount >= 1 && amount <= 999999999 ? amount : null;
}

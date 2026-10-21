export type Club = { id: string; name: string; archived: boolean };
export type Actor = { id: string; name: string; disabled: boolean };
export type Filters = { isOwner: boolean; clubs: Club[]; actions: string[]; actors: Actor[]; actionLabels: Record<string, string>; entityLabels: Record<string, string> };
export type AuditEvent = { id: string; createdAt: string; actorId: string; actorName: string; actorDisabled: boolean; clubName: string | null; clubArchived: boolean; entityType: string; entityId: string; action: string; actionLabel: string; entityLabel: string };
export type Page = { items: AuditEvent[]; nextCursor: string; total: number };
export type Selection = { clubId: string; from: string; until: string; action: string; actorId: string; q: string };
export type Period = 'week' | 'today' | 'yesterday' | 'all' | 'custom';
export function periodRange(period: Exclude<Period, 'custom'>, now = new Date()) {
 if (period === 'all') return { from: '', until: '' };
 const parts = new Intl.DateTimeFormat('en-CA', { timeZone: 'Europe/Minsk', year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(now);
 const part = (name: string) => parts.find((p) => p.type === name)!.value;
 const today = new Date(`${part('year')}-${part('month')}-${part('day')}T12:00:00Z`);
 const date = (offset: number) => new Date(today.getTime() + offset * 86_400_000).toISOString().slice(0, 10);
 return { from: date(period === 'week' ? -6 : period === 'yesterday' ? -1 : 0), until: date(period === 'yesterday' ? -1 : 0) };
}
export const initialSelection = (clubId = ''): Selection => ({ clubId, ...periodRange('week'), action: '', actorId: '', q: '' });
export const normalize = (value: string) => value.toLowerCase().replaceAll('ё', 'е');
export const searchTerms = (value: string) => normalize(value).split(/\s+/u).map((term) => term.replace(/^#/, '')).filter(Boolean);

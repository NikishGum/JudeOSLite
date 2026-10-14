export const sections = {
  home: 'Главная', sessions: 'Занятия', participants: 'Участники', groups: 'Группы',
  journals: 'Журналы', statistics: 'Статистика', staff: 'Сотрудники', clubs: 'Филиалы', history: 'История',
};
export type Section = keyof typeof sections;
export type Route = { clubId: string; section: Section; detail: string; invalid: boolean };
export function readRoute(): Route {
  const parts = window.location.pathname.split('/').filter(Boolean);
  const clubId = parts[0] === 'clubs' && parts.length > 1 ? parts[1] : '';
  const section = clubId ? parts[2] ?? 'home' : parts[0] ?? 'home';
  const known = Object.hasOwn(sections, section);
  const detail = new URLSearchParams(window.location.search).get('detail') ?? '';
  return { clubId, section: known ? section as Section : 'home', detail,
    invalid: (!known && section !== 'login') || parts.length > (clubId ? 3 : 1) || (!!detail && !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(detail)) };
}
export function routePath(route: Pick<Route, 'clubId' | 'section'> & { detail?: string }) {
  const base = route.clubId ? `/clubs/${encodeURIComponent(route.clubId)}/${route.section}` : route.section === 'home' ? '/' : `/${route.section}`;
  return base + (route.detail ? `?detail=${encodeURIComponent(route.detail)}` : '');
}
export function allowedSections(owner: boolean, roles: string[], hasAdministration: boolean): Section[] {
  const managing = owner || roles.includes('administrator');
  return (Object.keys(sections) as Section[]).filter((s) => {
    if (s === 'statistics' || s === 'clubs') return owner;
    if (s === 'history') return owner || hasAdministration;
    if (s === 'participants' || s === 'groups' || s === 'staff') return managing;
    return true;
  });
}

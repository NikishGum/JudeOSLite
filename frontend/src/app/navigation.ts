export const sections = {
  home: 'Главная', families: 'Семьи и родители', participants: 'Участники', groups: 'Группы', sessions: 'Занятия',
  journals: 'Журналы', payments: 'Платежи', staff: 'Сотрудники', statistics: 'Статистика',
  notifications: 'Уведомления', clubs: 'Филиалы', history: 'История', profile: 'Профиль', settings: 'Настройки',
};
export type Section = keyof typeof sections;
export type Route = { clubId: string; section: Section; detail: string; group?: string; invalid: boolean };
export function readRoute(): Route {
  const parts = window.location.pathname.split('/').filter(Boolean);
  const clubId = parts[0] === 'clubs' && parts.length > 1 ? parts[1] : '';
  const section = clubId ? parts[2] ?? 'home' : parts[0] ?? 'home';
  const known = Object.hasOwn(sections, section);
  const detail = new URLSearchParams(window.location.search).get('detail') ?? '';
  const group = new URLSearchParams(window.location.search).get('group') ?? '';
  const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
  return { clubId, group, section: known ? section as Section : 'home', detail,
    invalid: (!known && section !== 'login') || parts.length > (clubId ? 3 : 1) || (!!detail && !uuid.test(detail) && !(detail === 'new' && section === 'participants')) || (!!group && (!uuid.test(group) || section !== 'sessions')) };
}
export function routePath(route: Pick<Route, 'clubId' | 'section'> & { detail?: string; group?: string }) {
  const base = route.clubId ? `/clubs/${encodeURIComponent(route.clubId)}/${route.section}` : route.section === 'home' ? '/' : `/${route.section}`;
  const params = new URLSearchParams();
  if (route.detail) params.set('detail', route.detail);
  if (route.group && route.section === 'sessions') params.set('group', route.group);
  return base + (params.size ? `?${params}` : '');
}
export function allowedSections(owner: boolean, roles: string[], hasAdministration: boolean, parent = false): Section[] {
  const managing = owner || roles.includes('administrator');
  const parentOnly = parent && !owner && !hasAdministration && !roles.includes('trainer');
  return (Object.keys(sections) as Section[]).filter((s) => {
    if (parentOnly) return ['home', 'families', 'payments', 'notifications', 'profile', 'settings'].includes(s);
    if (s === 'statistics' || s === 'clubs') return owner;
    if (s === 'history') return owner || hasAdministration;
    if (s === 'families' || s === 'payments') return managing || parent;
    if (s === 'participants' || s === 'groups' || s === 'staff') return managing;
    return true;
  });
}

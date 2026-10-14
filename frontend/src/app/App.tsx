import { useEffect, useRef, useState, type FormEvent } from 'react';
import { flushSync } from 'react-dom';
import { pendingMutations, notifySessionChanged } from '../shared/app-lifecycle';
import { PwaStatus } from './PwaStatus';
import { Dialog } from '../shared/ui/Dialog';
import { NavigationIcon } from '../shared/ui/NavigationIcon';
import { NavigationBoundary } from '../shared/ui/NavigationBoundary';
import { allowedSections, readRoute, routePath, sections, type Route, type Section } from './navigation';
import logo from '../../../design-system/assets/logo-pride.png';
import { ApiError, apiGet, apiPost, setCsrfToken } from '../shared/api';
import { OwnerManagement } from '../features/clubs/OwnerManagement';
import { ParticipantRegistry } from '../features/participants/ParticipantRegistry';
import { TrainerManagement } from '../features/staff/TrainerManagement';
import { GroupManagement } from '../features/groups/GroupManagement';
import { ScheduleManagement } from '../features/schedule/ScheduleManagement';
import { AuditHistory } from '../features/audit/AuditHistory';
import { TrainerSessions } from '../features/trainer-sessions/TrainerSessions';
import { AdminJournals } from '../features/journals/AdminJournals';
import { SessionCalendar } from '../features/sessions/SessionCalendar';
import { OwnerDashboard } from '../features/owner-dashboard/OwnerDashboard';

type Club = { id: string; name: string; timezone: string; archived: boolean; roles: string[] };
type Session = { user: { id: string; displayName: string; isOwner: boolean }; clubs: Club[]; csrfToken: string };
const roleNames: Record<string, string> = { owner: 'Владелец', administrator: 'Администратор', trainer: 'Тренер' };
const message = (error: unknown) => error instanceof ApiError ? error.message : 'Нет соединения. Проверьте интернет и попробуйте снова.';
export function App() {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [login, setLogin] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [route, setRoute] = useState(readRoute);
  const selectedId = route.clubId;
  const [more, setMore] = useState(false);
  const [leave, setLeave] = useState<(() => void) | null>(null);
  const [navigationNotice, setNavigationNotice] = useState('');
  const dirty = useRef<() => boolean>(() => false);
  const routeRef = useRef(route); routeRef.current = route;
  const historyIndex = useRef<number>(window.history.state?.prideIndex ?? 0);
  const restoring = useRef(false);
  const accepting = useRef(false);
  const requestLeaveRef = useRef<(action: () => void) => void>(() => {});
  function requestLeave(action: () => void) {
    if (pendingMutations()) { setNavigationNotice('Дождитесь окончания сохранения, затем повторите переход.'); return; }
    if (dirty.current()) setLeave(() => action);
    else action();
  }
  requestLeaveRef.current = requestLeave;
  function navigate(next: Route, replace = false) {
    const path = routePath(next);
    if (path === window.location.pathname + window.location.search && !next.invalid) { setMore(false); return; }
    if (replace) window.history.replaceState({ prideIndex: historyIndex.current }, '', path);
    else window.history.pushState({ prideIndex: ++historyIndex.current }, '', path);
    routeRef.current = next; setRoute(next); setMore(false); setNavigationNotice('');
  }
  function openSection(section: Section) { if (section === route.section && !route.detail && !route.invalid) { setMore(false); return; } requestLeave(() => navigate({ ...route, section, detail: '', invalid: false })); }
  const [club, setClub] = useState<Club | null>(null);
  const [clubError, setClubError] = useState('');
  const [clubAttempt, setClubAttempt] = useState(0);
  const epoch = useRef(0);
  const pending = useRef(false);
  const authenticated = useRef(false);
  const pageTitle = useRef<HTMLHeadingElement>(null);

  function forgetSession() {
    epoch.current += 1;
    authenticated.current = false;
    setCsrfToken(null); setSession(null); setClub(null); setMore(false); setLeave(null);
    setLogin(''); setPassword(''); setClubError(''); setLoading(false);
  }
  async function loadSession(signal?: AbortSignal) {
    const generation = ++epoch.current;
    try {
      const result = await apiGet<Session>('/api/auth/me', signal);
      if (signal?.aborted || generation !== epoch.current) return;
      authenticated.current = true;
      setCsrfToken(result.csrfToken); setSession(result); setError('');
    } catch (cause) {
      if (signal?.aborted || generation !== epoch.current) return;
      if (cause instanceof ApiError && cause.status === 401) forgetSession();
      else { setError(message(cause)); setSession(null); setClub(null); setCsrfToken(null); }
    } finally { if (!signal?.aborted && generation === epoch.current) setLoading(false); }
  }
  useEffect(() => {
    const controller = new AbortController();
    const ended = () => { const wasAuthenticated = authenticated.current; forgetSession(); if (wasAuthenticated) setError('Сессия завершена. Войдите снова.'); };
    const changed = () => { flushSync(() => { forgetSession(); setLoading(true); setError(''); }); void loadSession(controller.signal); };
    // Remove personal DOM before a document can enter the back/forward cache.
    const hide = () => { flushSync(() => { epoch.current += 1; setCsrfToken(null); setSession(null); setClub(null); setPassword(''); setMore(false); setLeave(null); setLoading(true); }); };
    const show = (event: PageTransitionEvent) => { if (event.persisted) void loadSession(controller.signal); };
    window.addEventListener('session-ended', ended);
    window.addEventListener('session-changed', changed);
    window.addEventListener('pagehide', hide); window.addEventListener('pageshow', show);
    void loadSession(controller.signal);
    const refresh = () => { if (authenticated.current && document.visibilityState === 'visible' && !pending.current) void loadSession(controller.signal); };
    window.history.replaceState({ prideIndex: historyIndex.current }, '', window.location.href);
    const popstate = () => {
      if (restoring.current) { restoring.current = false; return; }
      const nextIndex = window.history.state?.prideIndex ?? 0;
      const delta = nextIndex - historyIndex.current;
      if (!accepting.current && delta && (dirty.current() || pendingMutations())) {
        restoring.current = true; window.history.go(-delta);
        requestLeaveRef.current(() => { accepting.current = true; window.history.go(delta); }); return;
      }
      accepting.current = false; historyIndex.current = nextIndex;
      const next = readRoute(); routeRef.current = next; setRoute(next); setMore(false);
      refresh();
    };
    window.addEventListener('focus', refresh);
    window.addEventListener('popstate', popstate);
    document.addEventListener('visibilitychange', refresh);
    const interval = window.setInterval(refresh, 30_000);
    return () => {
      controller.abort(); window.clearInterval(interval);
      window.removeEventListener('session-ended', ended);
      window.removeEventListener('session-changed', changed);
      window.removeEventListener('pagehide', hide); window.removeEventListener('pageshow', show);
      window.removeEventListener('focus', refresh); window.removeEventListener('popstate', popstate);
      document.removeEventListener('visibilitychange', refresh);
    };
  }, []);

  const userId = session?.user.id;
  useEffect(() => { if (!loading) pageTitle.current?.focus(); }, [userId, loading, route.section]);
  const available = session?.clubs.find((item) => item.id === selectedId);
  const roles = available?.roles.join(',');
  const clubName = available?.name;
  useEffect(() => {
    setClub(null); setClubError('');
    if (!userId || !selectedId || roles === undefined) return;
    const controller = new AbortController();
    apiGet<Club>(`/api/auth/clubs/${selectedId}`, controller.signal)
      .then((result) => { if (!controller.signal.aborted) setClub(result); })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return;
        setClubError(message(cause));
        if (cause instanceof ApiError && cause.status === 404) void loadSession();
      });
    return () => controller.abort();
  }, [userId, selectedId, roles, clubName, clubAttempt]);

  async function signIn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (pending.current) return;
    pending.current = true; setBusy(true); setError(''); epoch.current += 1;
    try { await apiPost('/api/auth/login', { login, password }); notifySessionChanged(); setPassword(''); await loadSession(); }
    catch (cause) { setPassword(''); setError(message(cause)); }
    finally { pending.current = false; setBusy(false); }
  }
  async function signOut() {
    if (pending.current) return;
    pending.current = true; setBusy(true); setError(''); epoch.current += 1;
    try { await apiPost('/api/auth/logout'); forgetSession(); navigate({ clubId: '', section: 'home', detail: '', invalid: false }, true); notifySessionChanged(); }
    catch (cause) {
      if (!(cause instanceof ApiError && cause.status === 401)) setError(message(cause));
      if (cause instanceof ApiError && cause.code === 'csrf_failed') await loadSession();
    } finally { pending.current = false; setBusy(false); }
  }
  const currentClub = club && available && club.id === available.id && club.name === available.name && club.roles.join(',') === roles ? club : null;
  const canManage = !!session && (session.user.isOwner || !!currentClub?.roles.includes('administrator'));
  const canTrain = !!currentClub?.roles.includes('trainer');
  const permitted = session ? allowedSections(session.user.isOwner, available?.roles ?? [...new Set(session.clubs.flatMap((c) => c.roles))], session.clubs.some((c) => c.roles.includes('administrator'))) : [];
  const invalid = route.invalid || (!!selectedId && !available) || !permitted.includes(route.section) || (!!route.detail && route.section !== 'participants');
  const items = permitted.map((section) => ({ section, label: sections[section] }));
  const hasAdministration = !!session?.clubs.some((c) => c.roles.includes('administrator'));
  const primary = session?.user.isOwner ? ['home', 'statistics', 'sessions'] : (canManage || (!selectedId && hasAdministration)) ? ['home', 'sessions', 'participants'] : ['home', 'sessions', 'journals'];
  const links = (mobile = false) => items.filter((item) => !mobile || primary.includes(item.section)).map(({ section, label }) => <a key={section} href={routePath({ clubId: selectedId, section })} aria-current={route.section === section ? 'page' : undefined} onClick={(event) => {
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault(); openSection(section);
  }}><NavigationIcon section={section} /><span>{label}</span></a>);
  return <div className={session ? 'app-shell' : 'public-shell'}>
    <a className="skip-link" href="#workspace">К основному содержимому</a>
    <header className="header"><div className="header-inner">
      <a className="brand" href="/" aria-label="PRIDE — главная" onClick={(event) => { if (session && !event.ctrlKey && !event.metaKey) { event.preventDefault(); openSection('home'); } }}><img src={logo} width="56" height="56" alt="" /><span>Клуб дзюдо <strong>PRIDE</strong></span></a>
      {session ? <><label className="club-context">Филиал<select aria-label="Филиал" value={available ? selectedId : ''} onChange={(event) => {
        const id = event.target.value; requestLeave(() => navigate({ clubId: id, section: route.section, detail: '', invalid: false }));
      }}><option value="">{session.user.isOwner ? 'Вся сеть' : 'Выберите филиал'}</option>{session.clubs.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</select></label>
        <div className="account"><div><strong>{session.user.displayName}</strong><span>{session.user.isOwner ? 'Владелец сети' : available?.roles.map((r) => roleNames[r] ?? r).join(' · ') || 'Сотрудник клуба'}</span></div><button type="button" className="secondary" disabled={busy} onClick={() => requestLeave(() => void signOut())}>{busy ? 'Выходим…' : 'Выйти'}</button></div>
      </> : <span className="header-label">Для сотрудников клуба</span>}
    </div></header>
    {session && <><nav className="sidebar workspace-navigation" aria-label="Разделы филиала">{links()}<p className="sidebar-caption">PRIDE · Рабочая панель</p></nav>
      <nav className="bottom-navigation" aria-label="Основная навигация">{links(true)}<button type="button" className="secondary" aria-current={!primary.includes(route.section) ? 'page' : undefined} aria-haspopup="dialog" aria-expanded={more} onClick={() => setMore(true)}><NavigationIcon section="more" /><span>Ещё</span></button></nav></>}
    <main id="workspace" className="workspace" tabIndex={-1}>
      <PwaStatus beforeUpdate={requestLeave} />
      <h1 ref={pageTitle} tabIndex={-1}>{session ? sections[route.section] : 'Вход для сотрудников'}</h1>
      {error && <div className="error" role="alert"><p>{error}</p>{!session && !loading && <button type="button" onClick={() => { setLoading(true); void loadSession(); }}>Проверить подключение и доступ</button>}</div>}
      {navigationNotice && <p className="error" role="status">{navigationNotice}</p>}
      {loading ? <p role="status">Проверяем доступ…</p> : session ? <>
        {invalid ? <section className="connection" role="alert"><h2>Раздел или филиал недоступен</h2><p>Проверьте ссылку и свои назначения. Доступ к данным проверяет сервер.</p><button type="button" onClick={() => requestLeave(() => navigate({ clubId: '', section: 'home', detail: '', invalid: false }))}>На главную</button></section> : <>
          {clubError && <div role="alert"><p>{clubError}</p><button type="button" onClick={() => setClubAttempt((v) => v + 1)}>Попробовать снова</button></div>}
          {selectedId && !currentClub && !clubError && <p role="status">Проверяем доступ к филиалу…</p>}
          <NavigationBoundary key={`${session.user.id}:${selectedId}:${roles}:${route.section}`} dirty={dirty}>
            {route.section === 'home' && <section className="connection home-context"><p className="eyebrow">Рабочая панель PRIDE</p><h2>{available?.name ?? (session.user.isOwner ? 'Обзор сети' : 'Ваши назначения')}</h2><p>{session.user.isOwner ? 'Управляйте филиалами и сотрудниками, следите за статистикой сети.' : (canManage || (!selectedId && hasAdministration)) ? 'Откройте занятия, реестр или журналы выбранного филиала.' : 'Откройте назначенные занятия и журнал для отметок.'}</p>{!session.clubs.length && <p>{session.user.isOwner ? 'Филиалы пока не добавлены.' : 'Нет доступных филиалов. Обратитесь к владельцу клуба.'}</p>}<div className="home-links">{links()}</div></section>}
            {route.section === 'statistics' && session.user.isOwner && <OwnerDashboard />}
            {route.section === 'clubs' && session.user.isOwner && <OwnerManagement mode="clubs" onChanged={() => loadSession()} />}
            {route.section === 'history' && <AuditHistory selectedClubId={canManage ? selectedId : ''} />}
            {['sessions', 'participants', 'groups', 'journals', 'staff'].includes(route.section) && !selectedId && <section className="connection"><h2>Выберите филиал</h2><p>Используйте выбор филиала в верхней панели.</p></section>}
            {currentClub && <>
              {route.section === 'participants' && canManage && <ParticipantRegistry clubId={currentClub.id} timezone={currentClub.timezone} onSessionRefresh={() => loadSession()} detailId={route.detail} beforeNavigate={requestLeave} onDetailChange={(detail) => navigate({ ...routeRef.current, detail })} />}
              {route.section === 'groups' && canManage && <GroupManagement clubId={currentClub.id} timezone={currentClub.timezone} />}
              {route.section === 'sessions' && <><SessionCalendar clubId={currentClub.id} timezone={currentClub.timezone} canManage={canManage} />{canManage && <ScheduleManagement clubId={currentClub.id} timezone={currentClub.timezone} />}</>}
              {route.section === 'journals' && <>{canTrain && <TrainerSessions clubId={currentClub.id} timezone={currentClub.timezone} />}{canManage && <AdminJournals clubId={currentClub.id} timezone={currentClub.timezone} />}</>}
              {route.section === 'staff' && canManage && <TrainerManagement clubId={currentClub.id} isOwner={session.user.isOwner} onSessionRefresh={() => loadSession()} />}
            </>}
            {route.section === 'staff' && session.user.isOwner && <OwnerManagement mode="staff" onChanged={() => loadSession()} />}
          </NavigationBoundary>
        </>}
      </> : <section className="connection login-panel" aria-labelledby="login-title"><p className="eyebrow">Каждая тренировка имеет значение</p><h2 id="login-title">Ваш аккаунт PRIDE</h2>
        <form onSubmit={(event) => void signIn(event)} aria-busy={busy}>
          <label className="field">Логин<input name="username" autoComplete="username" autoCapitalize="none" spellCheck={false} required maxLength={100} value={login} onChange={(event) => setLogin(event.target.value)} disabled={busy} /></label>
          <label className="field">Пароль<input name="password" type="password" autoComplete="current-password" required maxLength={128} value={password} onChange={(event) => setPassword(event.target.value)} disabled={busy} /></label>
          <button type="submit" disabled={busy}>{busy ? 'Входим…' : 'Войти'}</button>
        </form><p className="help">Для получения доступа обратитесь к владельцу клуба.</p>
      </section>}
    </main>
    {more && session && <Dialog title="Разделы PRIDE" onCancel={() => setMore(false)} cancelLabel="Закрыть меню"><nav className="more-navigation" aria-label="Все разделы">{links()}</nav></Dialog>}
    {leave && session && <Dialog title="Есть несохранённые изменения" onCancel={() => setLeave(null)} cancelLabel="Продолжить редактирование"><p>При переходе введённые данные будут потеряны.</p><button type="button" onClick={() => { const action = leave; setLeave(null); if (!pendingMutations()) action(); else setNavigationNotice('Дождитесь окончания сохранения.'); }}>Перейти без сохранения</button></Dialog>}
    {!session && <footer>Клуб дзюдо PRIDE</footer>}
  </div>;
}

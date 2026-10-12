import { useEffect, useRef, useState, type FormEvent } from 'react';
import logo from '../../../design-system/assets/logo-pride.png';
import { ApiError, apiGet, apiPost, setCsrfToken } from '../shared/api';
import { OwnerManagement } from '../features/clubs/OwnerManagement';
import { ParticipantRegistry } from '../features/participants/ParticipantRegistry';
import { TrainerManagement } from '../features/staff/TrainerManagement';

type Club = { id: string; name: string; timezone: string; archived: boolean; roles: string[] };
type Session = { user: { id: string; displayName: string; isOwner: boolean }; clubs: Club[]; csrfToken: string };
const roleNames: Record<string, string> = { owner: 'Владелец', administrator: 'Администратор', trainer: 'Тренер' };
const message = (error: unknown) => error instanceof ApiError ? error.message : 'Нет соединения. Проверьте интернет и попробуйте снова.';
const routeClub = () => /^\/clubs\/([^/]+)$/.exec(window.location.pathname)?.[1] ?? '';
const navigate = (path: string) => window.history.replaceState(null, '', path);

export function App() {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [login, setLogin] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [selectedId, setSelectedId] = useState(routeClub);
  const [club, setClub] = useState<Club | null>(null);
  const [clubError, setClubError] = useState('');
  const [clubAttempt, setClubAttempt] = useState(0);
  const epoch = useRef(0);
  const pending = useRef(false);
  const authenticated = useRef(false);
  const selectedRef = useRef(selectedId);
  selectedRef.current = selectedId;

  function forgetSession() {
    epoch.current += 1;
    authenticated.current = false;
    setCsrfToken(null); setSession(null); setClub(null); setSelectedId('');
    setPassword(''); setLoading(false); navigate('/login');
  }
  async function loadSession(signal?: AbortSignal) {
    const generation = ++epoch.current;
    try {
      const result = await apiGet<Session>('/api/auth/me', signal);
      if (signal?.aborted || generation !== epoch.current) return;
      authenticated.current = true;
      setCsrfToken(result.csrfToken); setSession(result); setError('');
      const id = result.clubs.some((item) => item.id === selectedRef.current) ? selectedRef.current : '';
      setSelectedId(id); navigate(id ? `/clubs/${id}` : '/');
    } catch (cause) {
      if (signal?.aborted || generation !== epoch.current) return;
      if (cause instanceof ApiError && cause.status === 401) forgetSession();
      else { setError(message(cause)); setSession(null); setClub(null); setCsrfToken(null); }
    } finally { if (!signal?.aborted && generation === epoch.current) setLoading(false); }
  }
  useEffect(() => {
    const controller = new AbortController();
    const ended = () => { const wasAuthenticated = authenticated.current; forgetSession(); if (wasAuthenticated) setError('Сессия завершена. Войдите снова.'); };
    window.addEventListener('session-ended', ended);
    void loadSession(controller.signal);
    const refresh = () => { if (authenticated.current && document.visibilityState === 'visible' && !pending.current) void loadSession(controller.signal); };
    const popstate = () => { selectedRef.current = routeClub(); setSelectedId(selectedRef.current); refresh(); };
    window.addEventListener('focus', refresh);
    window.addEventListener('popstate', popstate);
    document.addEventListener('visibilitychange', refresh);
    const interval = window.setInterval(refresh, 30_000);
    return () => {
      controller.abort(); window.clearInterval(interval);
      window.removeEventListener('session-ended', ended);
      window.removeEventListener('focus', refresh); window.removeEventListener('popstate', popstate);
      document.removeEventListener('visibilitychange', refresh);
    };
  }, []);

  const userId = session?.user.id;
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
    try { await apiPost('/api/auth/login', { login, password }); setPassword(''); await loadSession(); }
    catch (cause) { setPassword(''); setError(message(cause)); }
    finally { pending.current = false; setBusy(false); }
  }
  async function signOut() {
    if (pending.current) return;
    pending.current = true; setBusy(true); setError(''); epoch.current += 1;
    try { await apiPost('/api/auth/logout'); forgetSession(); }
    catch (cause) {
      if (!(cause instanceof ApiError && cause.status === 401)) setError(message(cause));
      if (cause instanceof ApiError && cause.code === 'csrf_failed') await loadSession();
    } finally { pending.current = false; setBusy(false); }
  }
  return <>
    <a className="skip-link" href="#workspace">К основному содержимому</a>
    <header className="header"><div className="header-inner">
      <a className="brand" href="/" aria-label="PRIDE — главная"><img src={logo} width="56" height="56" alt="" /><span>Клуб дзюдо <strong>PRIDE</strong></span></a>
      <span className="header-label">Для сотрудников клуба</span>
    </div></header>
    <main id="workspace" className="workspace" tabIndex={-1}>
      <p className="eyebrow">PRIDE · Вместе к результату</p>
      <h1>{session ? 'Рабочее пространство клуба' : 'Вход для сотрудников'}</h1>
      <p className="intro">Каждая тренировка имеет значение.</p>
      {error && <p className="error" role="alert">{error}</p>}
      {loading ? <p role="status">Проверяем доступ…</p> : session ? <>
        <section className="connection account" aria-labelledby="account-title">
          <div><h2 id="account-title">{session.user.displayName}</h2><p>{session.user.isOwner ? 'Владелец сети' : 'Сотрудник клуба'}</p></div>
          <button onClick={() => void signOut()} disabled={busy}>{busy ? 'Выходим…' : 'Выйти'}</button>
        </section>
        <section className="connection" aria-labelledby="clubs-title"><h2 id="clubs-title">Доступные филиалы</h2>
          {session.clubs.length === 0 ? <p>{session.user.isOwner ? 'Филиалы пока не добавлены.' : 'Нет доступных филиалов. Обратитесь к владельцу клуба.'}</p> : <>
            <div className="field"><label htmlFor="club-select">Филиал</label><select id="club-select" value={selectedId} onChange={(event) => { const id = event.target.value; setSelectedId(id); navigate(id ? `/clubs/${id}` : '/'); }}>
              <option value="">Выберите филиал</option>{session.clubs.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
            </select></div>
            {clubError && <div role="alert"><p>{clubError}</p><button onClick={() => setClubAttempt((value) => value + 1)}>Попробовать снова</button></div>}
            {selectedId && !club && !clubError && <p role="status">Проверяем доступ к филиалу…</p>}
            {club && <div className="club-summary"><h3>{club.name}</h3><p>{club.roles.map((role) => roleNames[role] ?? role).join(' · ')}</p></div>}
          </>}
        </section>
        {club && (session.user.isOwner || club.roles.includes('administrator')) && <ParticipantRegistry key={`${session.user.id}:${club.id}:${roles}`} clubId={club.id} timezone={club.timezone} onSessionRefresh={() => loadSession()} />}
        {club && (session.user.isOwner || club.roles.includes('administrator')) && <TrainerManagement key={`${session.user.id}:${club.id}:${roles}`} clubId={club.id} isOwner={session.user.isOwner} onSessionRefresh={() => loadSession()} />}
        {session.user.isOwner && <OwnerManagement key={session.user.id} onChanged={() => loadSession()} />}
      </> : <section className="connection login-panel" aria-labelledby="login-title"><h2 id="login-title">Ваш аккаунт PRIDE</h2>
        <form onSubmit={(event) => void signIn(event)} aria-busy={busy}>
          <label className="field">Логин<input name="username" autoComplete="username" autoCapitalize="none" spellCheck={false} required maxLength={100} value={login} onChange={(event) => setLogin(event.target.value)} disabled={busy} /></label>
          <label className="field">Пароль<input name="password" type="password" autoComplete="current-password" required maxLength={128} value={password} onChange={(event) => setPassword(event.target.value)} disabled={busy} /></label>
          <button type="submit" disabled={busy}>{busy ? 'Входим…' : 'Войти'}</button>
        </form><p className="help">Для получения доступа обратитесь к владельцу клуба.</p>
      </section>}
    </main><footer>Клуб дзюдо PRIDE</footer>
  </>;
}

import { useEffect, useRef, useState, type FormEvent } from "react";
import { flushSync } from "react-dom";
import { SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar";
import { useIsMobile } from "@/hooks/use-mobile";
import { cn } from "@/lib/utils";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { RuntimeProfile, RuntimeSidebar } from "./RuntimeShell";
import { Separator } from "@/components/ui/separator";
import { TooltipProvider } from "@/components/ui/tooltip";
import {
  pendingMutations,
  notifySessionChanged,
} from "../shared/app-lifecycle";
import { PwaStatus } from "./PwaStatus";
import { Dialog } from "../shared/ui/Dialog";
import { NavigationIcon } from "../shared/ui/NavigationIcon";
import { NavigationBoundary } from "../shared/ui/NavigationBoundary";
import {
  allowedSections,
  readRoute,
  routePath,
  sections,
  type Route,
  type Section,
} from "./navigation";
import logo from "../../../design-system/assets/logo-pride.png";
import { ApiError, apiGet, apiPost, setCsrfToken } from "../shared/api";
import { OwnerManagement } from "../features/clubs/OwnerManagement";
import { ParticipantRegistry } from "../features/participants/ParticipantRegistry";
import { TrainerManagement } from "../features/staff/TrainerManagement";
import { GroupManagement } from "../features/groups/GroupManagement";
import { ScheduleManagement } from "../features/schedule/ScheduleManagement";
import { AuditHistory } from "../features/audit/AuditHistory";
import { TrainerSessions } from "../features/trainer-sessions/TrainerSessions";
import { AdminJournals } from "../features/journals/AdminJournals";
import { SessionCalendar } from "../features/sessions/SessionCalendar";
import { Dashboard } from "../features/dashboard/Dashboard";
import { Families } from "../features/club-workspace/Families";
import { Payments } from "../features/club-workspace/Payments";
import {
  Notifications,
  NotificationBell,
} from "../features/club-workspace/Notifications";
import { ApplicationSettings } from "../features/club-workspace/Settings";
import { Profile } from "../features/club-workspace/Profile";
import { Reports } from "../features/club-workspace/Reports";
import "../features/club-workspace/workspace.css";

type Club = {
  id: string;
  name: string;
  timezone: string;
  archived: boolean;
  roles: string[];
};
type Session = {
  user: {
    id: string;
    displayName: string;
    isOwner: boolean;
    isParent?: boolean;
  };
  clubs: Club[];
  csrfToken: string;
};
const roleNames: Record<string, string> = {
  owner: "Владелец",
  administrator: "Администратор",
  trainer: "Тренер",
  parent: "Родитель",
};
const message = (error: unknown) =>
  error instanceof ApiError
    ? error.message
    : "Нет соединения. Проверьте интернет и попробуйте снова.";
export function App() {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [login, setLogin] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [route, setRoute] = useState(readRoute);
  const selectedId = route.clubId;
  const [more, setMore] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [installContainer, setInstallContainer] =
    useState<HTMLDivElement | null>(null);
  const [staffView, setStaffView] = useState<"network" | "trainers">("network");
  const mobile = useIsMobile();
  const [leave, setLeave] = useState<(() => void) | null>(null);
  const [navigationNotice, setNavigationNotice] = useState("");
  const dirty = useRef<() => boolean>(() => false);
  const routeRef = useRef(route);
  routeRef.current = route;
  const historyIndex = useRef<number>(window.history.state?.prideIndex ?? 0);
  const restoring = useRef(false);
  const accepting = useRef(false);
  const requestLeaveRef = useRef<(action: () => void) => void>(() => {});
  function requestLeave(action: () => void) {
    if (pendingMutations()) {
      setNavigationNotice(
        "Дождитесь окончания сохранения, затем повторите переход.",
      );
      return;
    }
    if (dirty.current()) setLeave(() => action);
    else action();
  }
  requestLeaveRef.current = requestLeave;
  function navigate(next: Route, replace = false) {
    const path = routePath(next);
    if (
      path === window.location.pathname + window.location.search &&
      !next.invalid
    ) {
      setMore(false);
      return;
    }
    if (replace)
      window.history.replaceState(
        { prideIndex: historyIndex.current },
        "",
        path,
      );
    else
      window.history.pushState(
        { prideIndex: ++historyIndex.current },
        "",
        path,
      );
    routeRef.current = next;
    setRoute(next);
    setMore(false);
    setNavigationNotice("");
  }
  function openSection(section: Section) {
    if (section === route.section && !route.detail && !route.invalid) {
      setMore(false);
      return;
    }
    requestLeave(() =>
      navigate({ ...route, section, detail: "", group: "", invalid: false }),
    );
  }
  const [club, setClub] = useState<Club | null>(null);
  const [clubError, setClubError] = useState("");
  const [clubAttempt, setClubAttempt] = useState(0);
  const epoch = useRef(0);
  const pending = useRef(false);
  const authenticated = useRef(false);
  const pageTitle = useRef<HTMLHeadingElement>(null);

  function forgetSession() {
    epoch.current += 1;
    authenticated.current = false;
    setCsrfToken(null);
    setSession(null);
    setClub(null);
    setMore(false);
    setLeave(null);
    setSidebarOpen(true);
    setLogin("");
    setPassword("");
    setClubError("");
    setLoading(false);
  }
  async function loadSession(signal?: AbortSignal) {
    const generation = ++epoch.current;
    try {
      const result = await apiGet<Session>("/api/auth/me", signal);
      if (signal?.aborted || generation !== epoch.current) return;
      authenticated.current = true;
      setCsrfToken(result.csrfToken);
      setSession(result);
      setError("");
    } catch (cause) {
      if (signal?.aborted || generation !== epoch.current) return;
      if (cause instanceof ApiError && cause.status === 401) forgetSession();
      else {
        setError(message(cause));
        setSession(null);
        setClub(null);
        setCsrfToken(null);
      }
    } finally {
      if (!signal?.aborted && generation === epoch.current) setLoading(false);
    }
  }
  useEffect(() => {
    const controller = new AbortController();
    const ended = () => {
      const wasAuthenticated = authenticated.current;
      forgetSession();
      if (wasAuthenticated) setError("Сессия завершена. Войдите снова.");
    };
    const changed = () => {
      flushSync(() => {
        forgetSession();
        setLoading(true);
        setError("");
      });
      void loadSession(controller.signal);
    };
    // Remove personal DOM before a document can enter the back/forward cache.
    const hide = () => {
      flushSync(() => {
        epoch.current += 1;
        setCsrfToken(null);
        setSession(null);
        setClub(null);
        setPassword("");
        setMore(false);
        setLeave(null);
        setLoading(true);
      });
    };
    const show = (event: PageTransitionEvent) => {
      if (event.persisted) void loadSession(controller.signal);
    };
    window.addEventListener("session-ended", ended);
    window.addEventListener("session-changed", changed);
    window.addEventListener("pagehide", hide);
    window.addEventListener("pageshow", show);
    void loadSession(controller.signal);
    const refresh = () => {
      if (
        authenticated.current &&
        document.visibilityState === "visible" &&
        !pending.current
      )
        void loadSession(controller.signal);
    };
    window.history.replaceState(
      { prideIndex: historyIndex.current },
      "",
      window.location.href,
    );
    const popstate = () => {
      if (restoring.current) {
        restoring.current = false;
        return;
      }
      const nextIndex = window.history.state?.prideIndex ?? 0;
      const delta = nextIndex - historyIndex.current;
      if (
        !accepting.current &&
        delta &&
        (dirty.current() || pendingMutations())
      ) {
        restoring.current = true;
        window.history.go(-delta);
        requestLeaveRef.current(() => {
          accepting.current = true;
          window.history.go(delta);
        });
        return;
      }
      accepting.current = false;
      historyIndex.current = nextIndex;
      const next = readRoute();
      routeRef.current = next;
      setRoute(next);
      setMore(false);
      refresh();
    };
    window.addEventListener("focus", refresh);
    window.addEventListener("popstate", popstate);
    document.addEventListener("visibilitychange", refresh);
    const interval = window.setInterval(refresh, 30_000);
    return () => {
      controller.abort();
      window.clearInterval(interval);
      window.removeEventListener("session-ended", ended);
      window.removeEventListener("session-changed", changed);
      window.removeEventListener("pagehide", hide);
      window.removeEventListener("pageshow", show);
      window.removeEventListener("focus", refresh);
      window.removeEventListener("popstate", popstate);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, []);

  const userId = session?.user.id;
  useEffect(() => {
    if (!loading) pageTitle.current?.focus();
  }, [userId, loading, route.section]);
  const available = session?.clubs.find((item) => item.id === selectedId);
  const roles = available?.roles.join(",");
  const clubName = available?.name;
  const clubTimezone = available?.timezone;
  const trainerOnly =
    !!session &&
    !session.user.isOwner &&
    session.clubs.some((c) => c.roles.includes("trainer")) &&
    !session.clubs.some((c) => c.roles.includes("administrator"));
  const parentOnly =
    !!session?.user.isParent &&
    !session.user.isOwner &&
    !session.clubs.some((c) =>
      c.roles.some((role) => role === "trainer" || role === "administrator"),
    );
  useEffect(() => {
    if (
      trainerOnly &&
      session?.clubs.length === 1 &&
      !route.clubId &&
      !route.invalid
    )
      navigate({ ...route, clubId: session.clubs[0].id }, true);
  }, [
    trainerOnly,
    userId,
    session?.clubs.length,
    route.clubId,
    route.section,
    route.invalid,
  ]);
  useEffect(() => {
    setClub(null);
    setClubError("");
    if (!userId || !selectedId || roles === undefined) return;
    const controller = new AbortController();
    apiGet<Club>(`/api/auth/clubs/${selectedId}`, controller.signal)
      .then((result) => {
        if (!controller.signal.aborted) setClub(result);
      })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return;
        setClubError(message(cause));
        if (cause instanceof ApiError && cause.status === 404)
          void loadSession();
      });
    return () => controller.abort();
  }, [userId, selectedId, roles, clubName, clubTimezone, clubAttempt]);

  async function signIn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending.current) return;
    pending.current = true;
    setBusy(true);
    setError("");
    epoch.current += 1;
    try {
      await apiPost("/api/auth/login", { login, password });
      notifySessionChanged();
      setPassword("");
      await loadSession();
    } catch (cause) {
      setPassword("");
      setError(message(cause));
    } finally {
      pending.current = false;
      setBusy(false);
    }
  }
  async function signOut() {
    if (pending.current) return;
    pending.current = true;
    setBusy(true);
    setError("");
    epoch.current += 1;
    try {
      await apiPost("/api/auth/logout");
      forgetSession();
      navigate(
        { clubId: "", section: "home", detail: "", invalid: false },
        true,
      );
      notifySessionChanged();
    } catch (cause) {
      if (!(cause instanceof ApiError && cause.status === 401))
        setError(message(cause));
      if (cause instanceof ApiError && cause.code === "csrf_failed")
        await loadSession();
    } finally {
      pending.current = false;
      setBusy(false);
    }
  }
  const currentClub =
    club &&
    available &&
    club.id === available.id &&
    club.name === available.name &&
    club.timezone === available.timezone &&
    club.roles.join(",") === roles
      ? club
      : null;
  const canManage =
    !!session &&
    (session.user.isOwner || !!currentClub?.roles.includes("administrator"));
  const canTrain = !!currentClub?.roles.includes("trainer");
  const permitted = session
    ? allowedSections(
        session.user.isOwner,
        available?.roles ?? [...new Set(session.clubs.flatMap((c) => c.roles))],
        session.clubs.some((c) => c.roles.includes("administrator")),
        !!session.user.isParent,
      )
    : [];
  const invalid =
    route.invalid ||
    (!!selectedId && !available) ||
    !permitted.includes(route.section) ||
    (!!route.detail &&
      !["participants", "groups", "families"].includes(route.section));
  const items = permitted.map((section) => ({
    section,
    label: sections[section],
  }));
  const assignmentKey = session?.clubs
    .map((c) => `${c.id}:${c.roles.join(",")}`)
    .sort()
    .join(";");
  const hasAdministration = !!session?.clubs.some((c) =>
    c.roles.includes("administrator"),
  );
  const primary = parentOnly
    ? ["home", "families", "payments"]
    : session?.user.isOwner
      ? ["home", "statistics", "sessions"]
      : canManage || (!selectedId && hasAdministration)
        ? ["home", "sessions", "participants"]
        : ["home", "sessions", "journals"];
  const links = (mobile = false, secondary = false) =>
    items
      .filter(
        (item) =>
          (!mobile || primary.includes(item.section)) &&
          (!secondary || !primary.includes(item.section)),
      )
      .map(({ section, label }) => (
        <a
          key={section}
          aria-label={label}
          href={routePath({ clubId: selectedId, section })}
          aria-current={route.section === section ? "page" : undefined}
          onClick={(event) => {
            if (
              event.button !== 0 ||
              event.metaKey ||
              event.ctrlKey ||
              event.shiftKey ||
              event.altKey
            )
              return;
            event.preventDefault();
            openSection(section);
          }}
        >
          <NavigationIcon section={section} />
          <span>{label}</span>
        </a>
      ));
  const roleLabel = session?.user.isOwner
    ? "Владелец сети"
    : parentOnly
      ? "Родитель"
      : available?.roles.map((r) => roleNames[r] ?? r).join(" · ") ||
        (trainerOnly ? "Тренер" : "Сотрудник клуба");
  const profile = session && (
    <RuntimeProfile
      key={`${session.user.id}:${assignmentKey}`}
      name={session.user.displayName}
      role={roleLabel}
      clubs={session.clubs}
      compact={mobile}
      busy={busy}
      collapsed={!sidebarOpen}
      onCollapsed={(collapsed) => setSidebarOpen(!collapsed)}
      onSignOut={() => requestLeave(() => void signOut())}
    />
  );
  return (
    <TooltipProvider>
      <SidebarProvider
        style={
          {
            "--sidebar-width": "248px",
            "--sidebar-width-icon": "76px",
          } as React.CSSProperties
        }
        open={sidebarOpen}
        onOpenChange={setSidebarOpen}
      >
        <div
          className={cn(session ? "app-shell" : "public-shell", {
            "sidebar-collapsed": !sidebarOpen,
          })}
        >
          <a className="skip-link" href="#workspace">
            К основному содержимому
          </a>
          <header className="header">
            <div className="header-inner">
              <a
                className="brand"
                href="/"
                aria-label="PRIDE — главная"
                onClick={(event) => {
                  if (session && !event.ctrlKey && !event.metaKey) {
                    event.preventDefault();
                    openSection("home");
                  }
                }}
              >
                <img src={logo} width="56" height="56" alt="" />
                <span>
                  Клуб дзюдо <strong>PRIDE</strong>
                </span>
              </a>
              {session ? (
                <>
                  {!mobile && (
                    <>
                      <SidebarTrigger aria-label="Переключить боковое меню" />
                      <Separator orientation="vertical" />
                    </>
                  )}
                  {trainerOnly || parentOnly ? (
                    <div className="club-context club-context-static">
                      <span>
                        {parentOnly
                          ? "Семейный кабинет"
                          : (available?.name ??
                            (session.clubs.length === 1
                              ? session.clubs[0].name
                              : "Мои назначенные занятия"))}
                      </span>
                      <small>
                        {parentOnly ? "PRIDE CLUB OS" : "Назначение тренера"}
                      </small>
                    </div>
                  ) : (
                    <label className="club-context">
                      <span className="sr-only">Филиал</span>
                      <select
                        aria-label="Филиал"
                        value={available ? selectedId : ""}
                        onChange={(event) => {
                          const id = event.target.value;
                          requestLeave(() =>
                            navigate({
                              clubId: id,
                              section: route.section,
                              detail: "",
                              group: "",
                              invalid: false,
                            }),
                          );
                        }}
                      >
                        <option value="">
                          {session.user.isOwner
                            ? "Вся сеть"
                            : "Выберите филиал"}
                        </option>
                        {session.clubs.map((c) => (
                          <option key={c.id} value={c.id}>
                            {c.name}
                          </option>
                        ))}
                      </select>
                    </label>
                  )}
                  <NotificationBell
                    key={session.user.id}
                    onOpen={() => openSection("notifications")}
                  />
                  {profile}
                </>
              ) : (
                <span className="header-label">Для сотрудников клуба</span>
              )}
            </div>
          </header>
          {session && (
            <>
              {!mobile && (
                <RuntimeSidebar
                  links={links()}
                  onHome={(event) => {
                    if (!event.ctrlKey && !event.metaKey) {
                      event.preventDefault();
                      openSection("home");
                    }
                  }}
                />
              )}
              <nav
                className="bottom-navigation"
                aria-label="Основная навигация"
              >
                {links(true)}
                <button
                  type="button"
                  className="secondary"
                  aria-current={
                    !primary.includes(route.section) ? "page" : undefined
                  }
                  aria-haspopup="dialog"
                  aria-expanded={more}
                  onClick={() => setMore(true)}
                >
                  <NavigationIcon section="more" />
                  <span>Ещё</span>
                </button>
              </nav>
            </>
          )}
          <main id="workspace" className="workspace" tabIndex={-1}>
            <div id="app-pwa">
              <PwaStatus
                beforeUpdate={requestLeave}
                installContainer={installContainer}
              />
            </div>
            <div className="page-heading">
              <div>
                <h1 ref={pageTitle} tabIndex={-1}>
                  {session ? sections[route.section] : "Вход в аккаунт"}
                </h1>
                {session && (
                  <p>
                    {
                      (
                        {
                          home: "Ваш клуб сегодня: люди, занятия и важные события.",
                          families: parentOnly
                            ? "Дети, занятия и документы вашей семьи."
                            : "Семьи, дети и контакты — всё рядом.",
                          participants: "Участники клуба и их карточки.",
                          groups: "Состав групп и история изменений.",
                          sessions:
                            "Расписание тренировок и назначенные занятия.",
                          journals: "Отметки участников и завершение журналов.",
                          payments: "Платежи, абонементы и тарифы клуба.",
                          staff: "Команда клуба и назначения.",
                          statistics: "Посещаемость и результаты работы клуба.",
                          notifications:
                            "События, которые требуют вашего внимания.",
                          clubs: "Филиалы и залы сети PRIDE.",
                          history:
                            "История действий в вашем рабочем пространстве.",
                          profile: "Ваш профиль и назначения в клубе.",
                          settings: "Профиль, доступ и удобство работы.",
                        } as Record<Section, string>
                      )[route.section]
                    }
                  </p>
                )}
              </div>
            </div>
            {error && (
              <div className="error" role="alert">
                <p>{error}</p>
                {!session && !loading && (
                  <button
                    type="button"
                    onClick={() => {
                      setLoading(true);
                      void loadSession();
                    }}
                  >
                    Проверить подключение и доступ
                  </button>
                )}
              </div>
            )}
            {navigationNotice && (
              <p className="error" role="status">
                {navigationNotice}
              </p>
            )}
            {loading ? (
              <p role="status">Проверяем доступ…</p>
            ) : session ? (
              <>
                {invalid ? (
                  <section className="connection" role="alert">
                    <h2>Раздел или филиал недоступен</h2>
                    <p>
                      Проверьте ссылку и свои назначения. Доступ к данным
                      проверяет сервер.
                    </p>
                    <button
                      type="button"
                      onClick={() =>
                        requestLeave(() =>
                          navigate({
                            clubId: "",
                            section: "home",
                            detail: "",
                            invalid: false,
                          }),
                        )
                      }
                    >
                      На главную
                    </button>
                  </section>
                ) : (
                  <>
                    {clubError && (
                      <div role="alert">
                        <p>{clubError}</p>
                        <button
                          type="button"
                          onClick={() => setClubAttempt((v) => v + 1)}
                        >
                          Попробовать снова
                        </button>
                      </div>
                    )}
                    {selectedId && !currentClub && !clubError && (
                      <p role="status">Проверяем доступ к филиалу…</p>
                    )}
                    <NavigationBoundary
                      key={`${session.user.id}:${session.user.isOwner}:${assignmentKey}:${selectedId}:${clubTimezone}:${route.section}`}
                      dirty={dirty}
                    >
                      {route.section === "home" &&
                        !parentOnly &&
                        !(trainerOnly && session.clubs.length === 1 && !selectedId) &&
                        (!selectedId || currentClub) && (
                          <Dashboard
                            clubId={selectedId}
                            displayName={session.user.displayName}
                            mode={
                              session.user.isOwner
                                ? "owner"
                                : canManage ||
                                    (!selectedId && hasAdministration)
                                  ? "administrator"
                                  : "trainer"
                            }
                          />
                        )}
                      {(route.section === "families" ||
                        (route.section === "home" && parentOnly)) &&
                        (parentOnly ||
                          (session.user.isParent && !canManage) ||
                          (currentClub && canManage)) && (
                          <Families
                            clubId={selectedId}
                            portal={!canManage}
                            detail={route.detail}
                            beforeNavigate={requestLeave}
                            onDetail={(detail) =>
                              navigate({ ...routeRef.current, detail })
                            }
                          />
                        )}
                      {route.section === "payments" &&
                        (parentOnly ||
                          (session.user.isParent && !canManage) ||
                          (currentClub && canManage)) && (
                          <Payments
                            clubId={selectedId}
                            portal={!canManage}
                            beforeNavigate={requestLeave}
                          />
                        )}
                      {route.section === "notifications" && (
                        <Notifications
                          onNavigate={(href) =>
                            requestLeave(() => window.location.assign(href))
                          }
                        />
                      )}
                      {route.section === "settings" && (
                        <ApplicationSettings
                          name={session.user.displayName}
                          role={roleLabel}
                          clubs={session.clubs}
                          collapsed={!sidebarOpen}
                          onCollapsed={(collapsed) =>
                            setSidebarOpen(!collapsed)
                          }
                        />
                      )}
                      {route.section === "profile" && (
                        <Profile
                          name={session.user.displayName}
                          role={roleLabel}
                          clubs={session.clubs}
                          clubId={selectedId}
                          trainerOnly={trainerOnly}
                        />
                      )}
                      {route.section === "statistics" &&
                        session.user.isOwner && (
                          <Reports clubId={selectedId} clubs={session.clubs} />
                        )}
                      {route.section === "clubs" && session.user.isOwner && (
                        <OwnerManagement
                          beforeNavigate={requestLeave}
                          mode="clubs"
                          onChanged={() => loadSession()}
                        />
                      )}
                      {route.section === "history" && (
                        <AuditHistory
                          selectedClubId={canManage ? selectedId : ""}
                        />
                      )}
                      {[
                        "sessions",
                        "participants",
                        "groups",
                        "journals",
                        ...(!parentOnly ? ["families", "payments"] : []),
                        ...(session.user.isOwner && staffView === "network"
                          ? []
                          : ["staff"]),
                      ].includes(route.section) &&
                        !selectedId &&
                        !parentOnly && (
                          <section className="connection">
                            <h2>
                              {trainerOnly
                                ? "Ваши назначенные занятия"
                                : "Выберите филиал"}
                            </h2>
                            {trainerOnly ? (
                              <>
                                <p>
                                  Откройте занятие нужного филиала с главной
                                  страницы. Назначения меняет администратор или
                                  владелец.
                                </p>
                                <button onClick={() => openSection("home")}>
                                  К назначенным занятиям
                                </button>
                              </>
                            ) : (
                              <p>Используйте выбор филиала в верхней панели.</p>
                            )}
                          </section>
                        )}
                      {route.section === "staff" && session.user.isOwner && (
                        <ToggleGroup
                          type="single"
                          className="staff-scopes"
                          aria-label="Область сотрудников"
                          orientation={mobile ? "vertical" : "horizontal"}
                          value={staffView}
                          onValueChange={(view) => {
                            if (view === "network" || view === "trainers")
                              requestLeave(() => setStaffView(view));
                          }}
                        >
                          <ToggleGroupItem value="network">
                            Сотрудники сети
                          </ToggleGroupItem>
                          <ToggleGroupItem value="trainers">
                            Тренеры филиала
                          </ToggleGroupItem>
                        </ToggleGroup>
                      )}
                      {currentClub && (
                        <>
                          {route.section === "participants" && canManage && (
                            <ParticipantRegistry
                              clubName={currentClub.name}
                              clubId={currentClub.id}
                              timezone={currentClub.timezone}
                              onSessionRefresh={() => loadSession()}
                              detailId={route.detail}
                              beforeNavigate={requestLeave}
                              onDetailChange={(detail) =>
                                navigate({ ...routeRef.current, detail })
                              }
                            />
                          )}
                          {route.section === "groups" && canManage && (
                            <GroupManagement
                              clubId={currentClub.id}
                              clubName={currentClub.name}
                              timezone={currentClub.timezone}
                              detailId={route.detail}
                              beforeNavigate={requestLeave}
                              onDetailChange={(detail) =>
                                navigate({ ...routeRef.current, detail })
                              }
                              onOpenSessions={(group) =>
                                requestLeave(() =>
                                  navigate({
                                    ...routeRef.current,
                                    section: "sessions",
                                    detail: "",
                                    group,
                                  }),
                                )
                              }
                            />
                          )}
                          {route.section === "sessions" && (
                            <>
                              <SessionCalendar
                                clubId={currentClub.id}
                                timezone={currentClub.timezone}
                                canManage={canManage}
                              />
                              {canManage && (
                                <ScheduleManagement
                                  clubId={currentClub.id}
                                  timezone={currentClub.timezone}
                                />
                              )}
                            </>
                          )}
                          {route.section === "journals" && (
                            <>
                              {canTrain && (
                                <TrainerSessions
                                  clubId={currentClub.id}
                                  timezone={currentClub.timezone}
                                />
                              )}
                              {canManage && (
                                <AdminJournals
                                  clubId={currentClub.id}
                                  timezone={currentClub.timezone}
                                />
                              )}
                            </>
                          )}
                          {route.section === "staff" &&
                            canManage &&
                            (!session.user.isOwner ||
                              staffView === "trainers") && (
                              <TrainerManagement
                                beforeNavigate={requestLeave}
                                clubName={currentClub.name}
                                clubId={currentClub.id}
                                isOwner={session.user.isOwner}
                                onSessionRefresh={() => loadSession()}
                              />
                            )}
                        </>
                      )}

                      {route.section === "staff" &&
                        session.user.isOwner &&
                        staffView === "network" && (
                          <OwnerManagement
                            beforeNavigate={requestLeave}
                            mode="staff"
                            onChanged={() => loadSession()}
                          />
                        )}
                    </NavigationBoundary>
                  </>
                )}
              </>
            ) : (
              <section
                className="connection login-panel"
                aria-labelledby="login-title"
              >
                <img
                  className="login-logo"
                  src={logo}
                  alt=""
                  width="64"
                  height="64"
                />
                <h2 id="login-title">PRIDE CLUB OS</h2>
                <p className="login-description">
                  Всё для вашего клуба в одном пространстве
                </p>
                <form onSubmit={(event) => void signIn(event)} aria-busy={busy}>
                  <label className="field">
                    Логин
                    <input
                      name="username"
                      autoComplete="username"
                      autoCapitalize="none"
                      spellCheck={false}
                      required
                      maxLength={100}
                      value={login}
                      onChange={(event) => setLogin(event.target.value)}
                      disabled={busy}
                    />
                  </label>
                  <label className="field">
                    Пароль
                    <input
                      name="password"
                      type="password"
                      autoComplete="current-password"
                      required
                      maxLength={128}
                      value={password}
                      onChange={(event) => setPassword(event.target.value)}
                      disabled={busy}
                    />
                  </label>
                  <button type="submit" disabled={busy}>
                    {busy ? "Входим…" : "Войти"}
                  </button>
                </form>
                <p className="help">
                  Нет доступа или забыли пароль? Обратитесь к администратору
                  клуба.
                </p>
              </section>
            )}
            <div id="app-install" ref={setInstallContainer} />
          </main>
          {more && session && (
            <Dialog
              className="more-sheet"
              title="Разделы PRIDE"
              onCancel={() => setMore(false)}
              cancelLabel="Закрыть меню"
            >
              <nav className="more-navigation" aria-label="Другие разделы">
                {links(false, true)}
                {!items.some((item) => !primary.includes(item.section)) && (
                  <p>Все доступные разделы находятся в нижней навигации.</p>
                )}
              </nav>
            </Dialog>
          )}
          {leave && session && (
            <Dialog
              title="Есть несохранённые изменения"
              onCancel={() => setLeave(null)}
              cancelLabel="Продолжить редактирование"
            >
              <p>При переходе введённые данные будут потеряны.</p>
              <button
                type="button"
                onClick={() => {
                  const action = leave;
                  setLeave(null);
                  if (!pendingMutations()) action();
                  else setNavigationNotice("Дождитесь окончания сохранения.");
                }}
              >
                Перейти без сохранения
              </button>
            </Dialog>
          )}
          {!session && <footer>Клуб дзюдо PRIDE</footer>}
        </div>
      </SidebarProvider>
    </TooltipProvider>
  );
}

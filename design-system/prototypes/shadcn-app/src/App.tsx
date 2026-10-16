import { useEffect, useRef, useState, type ReactNode } from "react";
import {
  Activity,
  ArrowRight,
  BookOpen,
  Building2,
  CalendarDays,
  Check,
  ChevronRight,
  CircleHelp,
  ExternalLink,
  History,
  LayoutDashboard,
  Library,
  MapPin,
  Search,
  Users,
  UserRound,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectGroup,
  SelectItem,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import {
  SidebarProvider,
  Sidebar,
  SidebarHeader,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
  SidebarInset,
  SidebarTrigger,
  SidebarRail,
} from "@/components/ui/sidebar";
import { Separator } from "@/components/ui/separator";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Input } from "@/components/ui/input";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import {
  Empty,
  EmptyHeader,
  EmptyTitle,
  EmptyDescription,
  EmptyContent,
} from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { TooltipProvider } from "@/components/ui/tooltip";
import {
  allowedSections,
  sections,
  roleNames,
  clubs,
  groups,
  participants,
  sessions,
  registries,
  statusNames,
  memberGroup,
  time,
  clubName,
  type Section,
  type Role,
  demoDate,
} from "./data";
import type { Session } from "../../../../frontend/src/features/dashboard/types";
import type { Card as Participant } from "../../../../frontend/src/features/participants/ParticipantEditor";
import logo from "../../../assets/logo-pride.png";
import { useIsMobile } from "@/hooks/use-mobile";
import {
  ProfileMenu,
  ProfileSettings,
  MobileNavigation,
  StatusDot,
  DateNavigation,
  dateLabel,
} from "@/components/workspace-tools";

type Page = Section | "registries";
type ViewState = "ready" | "loading" | "empty" | "error";
const icons = {
  home: LayoutDashboard,
  sessions: CalendarDays,
  participants: Users,
  groups: Library,
  journals: BookOpen,
  statistics: Activity,
  staff: UserRound,
  clubs: Building2,
  history: History,
  registries: Library,
};
const titles = { ...sections, registries: "Компоненты и registries" };
const descriptions: Record<Page, string> = {
  home: "Филиалы, занятия и посещаемость — в одном рабочем пространстве.",
  sessions: "Занятия выбранного филиала и состояние журналов.",
  participants: "Карточки участников, контакты и состав групп.",
  groups: "Группы и их текущий состав.",
  journals: "Посещаемость и завершение занятий.",
  statistics: "Только закрытые занятия. Отменённые исключены.",
  staff: "Тренеры и администраторы филиала.",
  clubs: "Все залы клуба и их рабочий контекст.",
  history: "Последние изменения в выбранном контексте.",
  registries: "Подборка для реестра, расписания и рабочих экранов PRIDE.",
};
function Picker({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger aria-label={label}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          {options.map((o) => (
            <SelectItem key={o.value} value={o.value}>
              {o.label}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  );
}
function Status({ status }: { status: Session["status"] }) {
  return (
    <Badge
      variant={
        status === "closed"
          ? "success"
          : status === "open"
            ? "warning"
            : "secondary"
      }
    >
      {status === "closed" && <Check data-icon="inline-start" />}
      {statusNames[status]}
    </Badge>
  );
}
function Panel({
  title,
  description,
  children,
  footer,
}: {
  title: string;
  description?: string;
  children: ReactNode;
  footer?: ReactNode;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent>{children}</CardContent>
      {footer && <CardFooter>{footer}</CardFooter>}
    </Card>
  );
}
function AppNav({
  page,
  navigate,
  allowed,
  role,
  profile,
}: {
  page: Page;
  navigate: (p: Page) => void;
  allowed: Section[];
  role: Role;
  profile: ReactNode;
}) {
  const go = navigate;
  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <a
          className="brand"
          href="#home"
          onClick={(e) => {
            e.preventDefault();
            go("home");
          }}
        >
          <img src={logo} alt="" width="44" height="44" />
          <span>
            <strong>PRIDE</strong>
            <small>Клуб дзюдо</small>
          </span>
        </a>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Рабочее пространство</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {allowed.map((p) => {
                const Icon = icons[p];
                return (
                  <SidebarMenuItem key={p}>
                    <SidebarMenuButton
                      asChild
                      isActive={p === page}
                      tooltip={titles[p]}
                    >
                      <a
                        href={`#${p}`}
                        aria-current={p === page ? "page" : undefined}
                        onClick={(e) => {
                          e.preventDefault();
                          go(p);
                        }}
                      >
                        <Icon />
                        <span>{titles[p]}</span>
                      </a>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
        <SidebarGroup className="mt-auto">
          <SidebarGroupLabel>Локальный шаблон</SidebarGroupLabel>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton
                asChild
                isActive={page === "registries"}
                tooltip="Подборка registries"
              >
                <a
                  href="#registries"
                  onClick={(e) => {
                    e.preventDefault();
                    go("registries");
                  }}
                >
                  <Library />
                  <span>Подборка registries</span>
                </a>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter>
        <Separator />
        {profile}
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}
export function App() {
  const [active, setActive] = useState(true);
  return (
    <TooltipProvider>
      <SidebarProvider
        style={
          {
            "--sidebar-width": "248px",
            "--sidebar-width-icon": "76px",
          } as React.CSSProperties
        }
      >
        {active ? (
          <Workspace
            onSignOut={() => {
              history.replaceState(null, "", "#signed-out");
              setActive(false);
            }}
          />
        ) : (
          <main className="signed-out">
            <img src={logo} width="72" height="72" alt="PRIDE" />
            <Empty>
              <EmptyHeader>
                <EmptyTitle>Вы вышли из демо</EmptyTitle>
                <EmptyDescription>
                  Локальный сеанс завершён. Откройте шаблон снова, чтобы
                  продолжить просмотр.
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                <Button
                  onClick={() => {
                    history.replaceState(null, "", "#home");
                    setActive(true);
                  }}
                >
                  Открыть демо
                </Button>
              </EmptyContent>
            </Empty>
          </main>
        )}
      </SidebarProvider>
    </TooltipProvider>
  );
}
function Workspace({ onSignOut }: { onSignOut: () => void }) {
  const isMobile = useIsMobile();
  const [profileName, setProfileName] = useState("Демо-профиль");
  const [settings, setSettings] = useState(false);
  const [showDemo, setShowDemo] = useState(true);
  const [date, setDate] = useState(demoDate);
  const [staffDetail, setStaffDetail] = useState<{
    name: string;
    assignments: string[];
  } | null>(null);
  const [role, setRole] = useState<Role>("owner");
  const [page, setPage] = useState<Page>(() => {
    const hash = location.hash.slice(1);
    return hash === "registries" || Object.hasOwn(sections, hash)
      ? (hash as Page)
      : "home";
  });
  const [club, setClub] = useState("all");
  const [state, setState] = useState<ViewState>("ready");
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("all");
  const [detail, setDetail] = useState<Participant | Session | null>(null);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const allowed = allowedSections(
    role === "owner",
    role === "owner" ? [] : [role],
    role === "administrator",
  );
  const accessibleClubs = role === "owner" ? clubs : clubs.slice(0, 2);
  const inClub = (id: string) =>
    club === "all" ? accessibleClubs.some((c) => c.id === id) : club === id;
  const visibleSessions = sessions.filter(
    (s) =>
      inClub(s.clubId) &&
      (role !== "trainer" || s.trainerName === "Елена Пример"),
  );
  const visiblePeople = participants.filter((p) => inClub(p.clubId));
  const visibleGroups = groups.filter(
    (g) =>
      inClub(g.clubId) &&
      (role !== "trainer" || visibleSessions.some((s) => s.groupId === g.id)),
  );
  function navigate(p: Page) {
    setPage(p);
    setQuery("");
    setStatus("all");
    setState("ready");
    setDetail(null);
    setStaffDetail(null);
    history.pushState(null, "", `#${p}`);
  }
  useEffect(() => {
    const onHash = () => {
      const next = location.hash.slice(1);
      if (next === "registries" || Object.hasOwn(sections, next)) {
        setPage(next as Page);
        setDetail(null);
        setQuery("");
        setStatus("all");
      }
    };
    window.addEventListener("hashchange", onHash);
    window.addEventListener("popstate", onHash);
    return () => {
      window.removeEventListener("hashchange", onHash);
      window.removeEventListener("popstate", onHash);
    };
  }, []);
  useEffect(() => {
    titleRef.current?.focus();
  }, [page]);
  const sectionAllowed = page === "registries" || allowed.includes(page);
  function switchRole(value: string) {
    const next = value as Role;
    setRole(next);
    setDate(demoDate);
    setClub(next === "owner" ? "all" : clubs[0].id);
    navigate("home");
  }
  const filteredPeople = visiblePeople.filter(
    (p) =>
      (status === "all" ||
        (status === "archived" ? p.archived : !p.archived)) &&
      p.fullName
        .toLocaleLowerCase("ru")
        .includes(query.toLocaleLowerCase("ru")),
  );
  const filteredSessions = visibleSessions.filter(
    (s) =>
      s.date === date &&
      (status === "all" || status === s.status) &&
      (s.groupName + " " + s.trainerName)
        .toLocaleLowerCase("ru")
        .includes(query.toLocaleLowerCase("ru")),
  );
  const closed = visibleSessions.filter((s) => s.status === "closed").length;
  const percentage = closed ? "75%" : "—";
  function SessionsList({ items }: { items: Session[] }) {
    return items.length ? (
      <div className="session-list">
        {items.map((s) => (
          <div className="session-row" key={s.id}>
            <div className="session-time">
              <strong>{time(s)}</strong>
              <small>60 минут</small>
            </div>
            <div className="session-info">
              <strong>{s.groupName}</strong>
              <span>
                {clubName(s.clubId)} · {s.trainerName}
              </span>
            </div>
            <Status status={s.status} />
            <Button
              variant="ghost"
              size="icon"
              aria-label={`Открыть занятие ${s.groupName} в ${time(s)}`}
              onClick={() => setDetail(s)}
            >
              <ChevronRight />
            </Button>
          </div>
        ))}
      </div>
    ) : (
      <NoResults
        reset={() => {
          setStatus("all");
          setQuery("");
          setDate(demoDate);
        }}
      />
    );
  }
  function Network() {
    return (
      <Panel
        title="Филиалы сети"
        description="Срез демонстрационных данных на 16 октября"
      >
        <Table className="adaptive-table">
          <TableHeader>
            <TableRow>
              <TableHead>Филиал</TableHead>
              <TableHead>Участники</TableHead>
              <TableHead>Занятия</TableHead>
              <TableHead>Посещаемость</TableHead>
              <TableHead>
                <span className="sr-only">Открыть</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {accessibleClubs
              .filter((c) => inClub(c.id))
              .map((c) => (
                <TableRow key={c.id}>
                  <TableCell>
                    <strong>{c.name}</strong>
                    <small className="cell-note">Europe/Minsk · 2 группы</small>
                  </TableCell>
                  <TableCell data-label="Участники">
                    {
                      participants.filter(
                        (p) => p.clubId === c.id && !p.archived,
                      ).length
                    }
                  </TableCell>
                  <TableCell data-label="Занятия">
                    {sessions.filter((s) => s.clubId === c.id).length}
                  </TableCell>
                  <TableCell data-label="Посещаемость">
                    {sessions.some(
                      (s) => s.clubId === c.id && s.status === "closed",
                    )
                      ? "75%"
                      : "—"}
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={`Выбрать ${c.name}`}
                      onClick={() => {
                        setClub(c.id);
                        navigate("sessions");
                      }}
                    >
                      <ArrowRight />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
          </TableBody>
        </Table>
        <p className="footnote">
          Посещаемость считается только по завершённым журналам.
        </p>
      </Panel>
    );
  }
  return (
    <>
      <a className="skip-link" href="#workspace">
        К содержимому
      </a>
      {!isMobile && (
        <AppNav
          page={page}
          navigate={navigate}
          allowed={allowed}
          role={role}
          profile={
            <ProfileMenu
              name={profileName}
              role={role}
              onSettings={() => setSettings(true)}
              onSignOut={onSignOut}
            />
          }
        />
      )}
      <SidebarInset>
        <header className="topbar">
          <div className="context">
            {!isMobile && <SidebarTrigger aria-label="Переключить меню" />}
            <Separator orientation="vertical" />
            <Building2 aria-hidden="true" />
            <Picker
              label="Рабочий филиал"
              value={club}
              onChange={(v) => {
                setClub(v);
                setDetail(null);
                setQuery("");
                setStatus("all");
              }}
              options={[
                ...(role === "owner"
                  ? [{ value: "all", label: "Вся сеть" }]
                  : []),
                ...accessibleClubs.map((c) => ({ value: c.id, label: c.name })),
              ]}
            />
          </div>
          <div className="topbar-date">
            <CalendarDays />
            16 октября 2026
          </div>
          <Badge variant="outline">Демо</Badge>
          {isMobile && (
            <ProfileMenu
              compact
              name={profileName}
              role={role}
              onSettings={() => setSettings(true)}
              onSignOut={onSignOut}
            />
          )}
        </header>
        <main id="workspace" className="workspace">
          <div className="page-heading">
            <div>
              <h1 ref={titleRef} tabIndex={-1}>
                {titles[page]}
              </h1>
              <p>{descriptions[page]}</p>
            </div>
            {page === "home" && (
              <Button onClick={() => navigate("sessions")}>
                К занятиям
                <ArrowRight data-icon="inline-end" />
              </Button>
            )}
          </div>
          {showDemo && (
            <div className="demo-controls">
              <div>
                <CircleHelp />
                <span>Шаблон на синтетических данных</span>
              </div>
              <div className="demo-pickers">
                <Picker
                  label="Роль в демо"
                  value={role}
                  onChange={switchRole}
                  options={Object.entries(roleNames).map(([value, label]) => ({
                    value,
                    label,
                  }))}
                />
                <Picker
                  label="Состояние экрана"
                  value={state}
                  onChange={(v) => setState(v as ViewState)}
                  options={[
                    { value: "ready", label: "Обычный экран" },
                    { value: "loading", label: "Загрузка" },
                    { value: "empty", label: "Нет данных" },
                    { value: "error", label: "Ошибка" },
                  ]}
                />
              </div>
            </div>
          )}
          {!sectionAllowed ? (
            <Alert>
              <AlertTitle>Раздел недоступен для этой роли</AlertTitle>
              <AlertDescription>
                Выберите доступный раздел в меню.
                <Button variant="outline" onClick={() => navigate("home")}>
                  На главную
                </Button>
              </AlertDescription>
            </Alert>
          ) : state === "loading" ? (
            <div
              aria-busy="true"
              aria-label="Загрузка данных"
              className="loading-state"
            >
              <Skeleton className="h-24 w-full" />
              <Skeleton className="h-72 w-full" />
              <span className="sr-only">Загрузка данных</span>
            </div>
          ) : state === "error" ? (
            <Alert variant="destructive">
              <AlertTitle>Не удалось загрузить данные</AlertTitle>
              <AlertDescription>
                Проверьте соединение и повторите попытку. Это демонстрация
                состояния ошибки.
                <Button variant="outline" onClick={() => setState("ready")}>
                  Повторить загрузку
                </Button>
              </AlertDescription>
            </Alert>
          ) : state === "empty" ? (
            <Empty>
              <EmptyHeader>
                <EmptyTitle>Здесь пока нет данных</EmptyTitle>
                <EmptyDescription>
                  Для выбранного контекста нет записей. В шаблоне можно
                  вернуться к демонстрационному набору.
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                <Button onClick={() => setState("ready")}>
                  Показать демоданные
                </Button>
              </EmptyContent>
            </Empty>
          ) : (
            <>
              {page === "home" && (
                <>
                  <div className="summary-strip">
                    {[
                      {
                        label: "Действующих участников",
                        value:
                          role === "trainer"
                            ? "—"
                            : visiblePeople.filter((p) => !p.archived).length,
                      },
                      { label: "Групп", value: visibleGroups.length },
                      {
                        label: "Занятий сегодня",
                        value: visibleSessions.length,
                      },
                      {
                        label: "Открытых журналов",
                        value: visibleSessions.filter(
                          (s) => s.status === "open",
                        ).length,
                      },
                    ].map((x) => (
                      <div key={x.label}>
                        <span>{x.label}</span>
                        <strong>{x.value}</strong>
                      </div>
                    ))}
                  </div>
                  <div className="dashboard-grid">
                    <div className="main-column">
                      {role === "owner" && <Network />}
                      <Panel
                        title={
                          role === "trainer"
                            ? "Мои занятия сегодня"
                            : "Занятия сегодня"
                        }
                        description="Пятница, 16 октября · время филиала"
                        footer={
                          <Button
                            variant="link"
                            onClick={() => navigate("sessions")}
                          >
                            Все занятия
                            <ArrowRight data-icon="inline-end" />
                          </Button>
                        }
                      >
                        <SessionsList
                          items={visibleSessions
                            .filter((s) => s.status !== "cancelled")
                            .slice(0, 3)}
                        />
                      </Panel>
                    </div>
                    <aside className="right-column">
                      <Panel
                        title="Требуют внимания"
                        description="Открытые журналы сегодняшних занятий"
                      >
                        <div className="attention">
                          <BookOpen />
                          <strong>
                            {visibleSessions.filter((s) => s.status === "open")
                              .length === 1
                              ? "1 открытый журнал"
                              : "Нет открытых журналов"}
                          </strong>
                          <p>
                            {visibleSessions.some((s) => s.status === "open")
                              ? "Завершите отметки, чтобы занятие попало в статистику."
                              : "Все журналы завершены или ещё не открыты."}
                          </p>
                          <Button
                            variant="outline"
                            onClick={() => navigate("journals")}
                          >
                            Перейти в журналы
                            <ArrowRight data-icon="inline-end" />
                          </Button>
                        </div>
                      </Panel>
                      <Panel title="Быстрый переход">
                        <div className="quick-links">
                          {allowed
                            .filter((p) =>
                              ["participants", "groups", "statistics"].includes(
                                p,
                              ),
                            )
                            .map((p) => {
                              const Icon = icons[p];
                              return (
                                <Button
                                  key={p}
                                  variant="ghost"
                                  onClick={() => navigate(p)}
                                >
                                  <Icon data-icon="inline-start" />
                                  {titles[p]}
                                  <ChevronRight data-icon="inline-end" />
                                </Button>
                              );
                            })}
                          {role === "trainer" && (
                            <Button
                              variant="ghost"
                              onClick={() => navigate("sessions")}
                            >
                              <CalendarDays data-icon="inline-start" />
                              Моё расписание
                              <ChevronRight data-icon="inline-end" />
                            </Button>
                          )}
                        </div>
                      </Panel>
                      <div className="context-note">
                        <MapPin />
                        <p>
                          Один зал — один филиал.
                          <br />
                          Выбранный контекст применяется к рабочим разделам.
                        </p>
                      </div>
                    </aside>
                  </div>
                </>
              )}
              {(page === "sessions" || page === "journals") && (
                <Panel
                  title={
                    page === "journals"
                      ? "Журналы занятий"
                      : `Расписание на ${dateLabel(date)}`
                  }
                  description="Демо-занятия доступны 16 октября · Europe/Minsk"
                >
                  <DateNavigation date={date} onChange={setDate} />
                  <div className="toolbar">
                    <div className="search-field">
                      <Search />
                      <Input
                        aria-label="Поиск занятий"
                        placeholder="Группа или тренер"
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                      />
                    </div>
                    <Picker
                      label="Статус занятия"
                      value={status}
                      onChange={setStatus}
                      options={[
                        { value: "all", label: "Все статусы" },
                        ...Object.entries(statusNames).map(
                          ([value, label]) => ({ value, label }),
                        ),
                      ]}
                    />
                  </div>
                  <SessionsList items={filteredSessions} />
                  {page === "sessions" && (
                    <p className="footnote">
                      Четыре режима: дни недели, чётность дат, выбранные даты,
                      разовое занятие.
                    </p>
                  )}
                </Panel>
              )}
              {page === "participants" && (
                <Panel
                  title="Реестр участников"
                  description={`Записей в выбранном контексте: ${filteredPeople.length}`}
                >
                  <div className="toolbar">
                    <div className="search-field">
                      <Search />
                      <Input
                        aria-label="Поиск участников"
                        placeholder="Поиск по имени"
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                      />
                    </div>
                    <Picker
                      label="Состояние карточки"
                      value={status}
                      onChange={setStatus}
                      options={[
                        { value: "all", label: "Все участники" },
                        { value: "active", label: "Действующие" },
                        { value: "archived", label: "Архив" },
                      ]}
                    />
                  </div>
                  {filteredPeople.length ? (
                    <Table className="adaptive-table">
                      <TableHeader>
                        <TableRow>
                          <TableHead>Участник</TableHead>
                          <TableHead>Дата рождения</TableHead>
                          <TableHead>Группа</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {filteredPeople.map((p) => (
                          <TableRow key={p.id}>
                            <TableCell>
                              <div className="person-heading">
                                <StatusDot
                                  active={!p.archived}
                                  label={p.archived ? "В архиве" : "Действует"}
                                />
                                <Button
                                  variant="name"
                                  onClick={() => setDetail(p)}
                                >
                                  {p.fullName}
                                </Button>
                              </div>
                              <small className="cell-note">
                                {clubName(p.clubId)}
                              </small>
                            </TableCell>
                            <TableCell data-label="Дата рождения">
                              {p.birthDate.split("-").reverse().join(".")}
                            </TableCell>
                            <TableCell data-label="Группа">
                              {p.archived ? "—" : memberGroup(p)?.name}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  ) : (
                    <NoResults
                      reset={() => {
                        setQuery("");
                        setStatus("all");
                      }}
                    />
                  )}
                </Panel>
              )}
              {page === "groups" && (
                <Panel
                  title="Состав групп"
                  description="Демонстрационный состав на 16 октября"
                >
                  <div className="group-list">
                    {visibleGroups.map((g) => (
                      <div key={g.id} className="entity-row">
                        <div>
                          <strong>{g.name}</strong>
                          <span>{clubName(g.clubId)}</span>
                          <small>
                            {participants
                              .filter(
                                (p) =>
                                  !p.archived && memberGroup(p)?.id === g.id,
                              )
                              .map((p) => p.fullName)
                              .join(", ")}
                          </small>
                        </div>
                        <Button
                          variant="outline"
                          onClick={() => {
                            setClub(g.clubId);
                            setDate(demoDate);
                            navigate("sessions");
                            setQuery(g.name);
                          }}
                        >
                          Занятия
                          <ArrowRight data-icon="inline-end" />
                        </Button>
                      </div>
                    ))}
                  </div>
                </Panel>
              )}
              {page === "statistics" && (
                <Panel
                  title="Посещаемость"
                  description="16 октября 2026 · текущий контекст"
                >
                  <div className="report-total">
                    <strong>{percentage}</strong>
                    <div>
                      <h2>Посещаемость закрытых занятий</h2>
                      <p>
                        {closed * 3} присутствий / {closed * 4} отметок
                      </p>
                    </div>
                  </div>
                  <Separator />
                  <Table className="adaptive-table">
                    <TableHeader>
                      <TableRow>
                        <TableHead>Завершено</TableHead>
                        <TableHead>Присутствия</TableHead>
                        <TableHead>Пропуски</TableHead>
                        <TableHead>Открыто</TableHead>
                        <TableHead>Запланировано</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      <TableRow>
                        <TableCell data-label="Завершено">{closed}</TableCell>
                        <TableCell data-label="Присутствия">
                          {closed * 3}
                        </TableCell>
                        <TableCell data-label="Пропуски">{closed}</TableCell>
                        <TableCell data-label="Открыто">
                          {
                            visibleSessions.filter((s) => s.status === "open")
                              .length
                          }
                        </TableCell>
                        <TableCell data-label="Запланировано">
                          {
                            visibleSessions.filter(
                              (s) => s.status === "planned",
                            ).length
                          }
                        </TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                  <p className="footnote">
                    Причины пропусков: неизвестна — {closed}, болезнь — 0,
                    уважительная — 0. При отсутствии закрытых отметок процент не
                    вычисляется.
                  </p>
                </Panel>
              )}
              {page === "clubs" && <Network />}
              {page === "staff" && (
                <Panel
                  title="Сотрудники филиалов"
                  description="Демонстрационные назначения"
                >
                  <div className="group-list">
                    {["Елена Пример", "Александр Демо"].map((name) => {
                      const assignments = accessibleClubs
                        .filter((c) => inClub(c.id))
                        .map((c) => c.name);
                      return (
                        <div className="entity-row staff-row" key={name}>
                          <Avatar size="lg">
                            <AvatarFallback>
                              {name
                                .split(" ")
                                .map((n) => n[0])
                                .join("")}
                            </AvatarFallback>
                          </Avatar>
                          <div className="entity-copy">
                            <div className="person-heading">
                              <StatusDot active label="Сотрудник активен" />
                              <Button
                                variant="name"
                                onClick={() =>
                                  setStaffDetail({ name, assignments })
                                }
                              >
                                {name}
                              </Button>
                            </div>
                            <span>{assignments.join(" · ")}</span>
                            <small>Тренер</small>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </Panel>
              )}
              {page === "history" && (
                <Panel
                  title="История изменений"
                  description="Синтетические события — пример представления"
                >
                  <div className="group-list">
                    {visibleSessions
                      .filter(
                        (s) => s.status === "closed" || s.status === "open",
                      )
                      .map((s) => (
                        <div className="entity-row" key={s.id}>
                          <History />
                          <div>
                            <strong>
                              {s.status === "closed"
                                ? "Завершён журнал"
                                : "Открыт журнал"}
                            </strong>
                            <span>
                              {s.groupName} · {clubName(s.clubId)}
                            </span>
                            <small>
                              {s.trainerName} · 16 октября, {time(s)}
                            </small>
                          </div>
                        </div>
                      ))}
                  </div>
                  {!visibleSessions.some(
                    (s) => s.status === "closed" || s.status === "open",
                  ) && <NoResults />}
                </Panel>
              )}
              {page === "registries" && (
                <div className="registry-list">
                  {registries.map((r) => (
                    <Card key={r.name}>
                      <CardHeader>
                        <div className="registry-heading">
                          <CardTitle>{r.name}</CardTitle>
                          <Badge variant="secondary">{r.fit}</Badge>
                        </div>
                        <CardDescription>{r.title}</CardDescription>
                      </CardHeader>
                      <CardContent>
                        <p>{r.text}</p>
                      </CardContent>
                      <CardFooter>
                        <Button variant="outline" asChild>
                          <a href={r.url} target="_blank" rel="noreferrer">
                            Документация
                            <ExternalLink data-icon="inline-end" />
                          </a>
                        </Button>
                      </CardFooter>
                    </Card>
                  ))}
                </div>
              )}
            </>
          )}
          <footer className="workspace-footer">
            <span>PRIDE · рабочее пространство</span>
            <span>Локальный шаблон · без подключения к API</span>
          </footer>
        </main>
      </SidebarInset>
      {isMobile && (
        <MobileNavigation
          page={page}
          role={role}
          allowed={allowed}
          navigate={navigate}
        />
      )}
      {settings && (
        <ProfileSettings
          open
          onOpenChange={setSettings}
          name={profileName}
          onSave={setProfileName}
          showDemo={showDemo}
          onShowDemo={setShowDemo}
        />
      )}
      <Sheet
        open={!!staffDetail}
        onOpenChange={(open) => {
          if (!open) setStaffDetail(null);
        }}
      >
        <SheetContent className="detail-sheet">
          <SheetHeader>
            <SheetTitle>{staffDetail?.name}</SheetTitle>
            <SheetDescription>
              Демонстрационная карточка сотрудника
            </SheetDescription>
          </SheetHeader>
          {staffDetail && (
            <div className="detail-body">
              <Avatar className="size-16">
                <AvatarFallback>
                  {staffDetail.name
                    .split(" ")
                    .map((n) => n[0])
                    .join("")}
                </AvatarFallback>
              </Avatar>
              <div className="person-heading">
                <StatusDot active label="Сотрудник активен" />
                <span>Сотрудник активен</span>
              </div>
              <dl>
                <dt>Роль</dt>
                <dd>Тренер</dd>
                <dt>Назначенные филиалы</dt>
                <dd>{staffDetail.assignments.join(", ")}</dd>
              </dl>
              <Alert>
                <AlertTitle>Назначения сотрудника</AlertTitle>
                <AlertDescription>
                  В приложении доступ ограничен назначенными филиалами и
                  занятиями. Здесь показаны синтетические назначения.
                </AlertDescription>
              </Alert>
            </div>
          )}
        </SheetContent>
      </Sheet>
      <Sheet
        open={!!detail}
        onOpenChange={(open) => {
          if (!open) setDetail(null);
        }}
      >
        <SheetContent className="detail-sheet">
          <SheetHeader>
            <SheetTitle>
              {detail && "fullName" in detail
                ? detail.fullName
                : detail?.groupName}
            </SheetTitle>
            <SheetDescription>
              Демонстрационная карточка · данные не сохраняются в CRM
            </SheetDescription>
          </SheetHeader>
          {detail &&
            ("fullName" in detail ? (
              <div className="detail-body">
                <Avatar className="size-16">
                  <AvatarFallback>
                    {detail.fullName
                      .split(" ")
                      .map((s) => s[0])
                      .join("")}
                  </AvatarFallback>
                </Avatar>
                <Badge variant={detail.archived ? "secondary" : "success"}>
                  {detail.archived ? "В архиве" : "Действует"}
                </Badge>
                <dl>
                  <dt>Филиал</dt>
                  <dd>{clubName(detail.clubId)}</dd>
                  <dt>Дата рождения</dt>
                  <dd>{detail.birthDate.split("-").reverse().join(".")}</dd>
                  <dt>Дата вступления</dt>
                  <dd>01.09.2026</dd>
                  <dt>Контакт</dt>
                  <dd>{detail.contactValue}</dd>
                  <dt>Группа</dt>
                  <dd>
                    {detail.archived
                      ? "Нет текущего состава"
                      : memberGroup(detail)?.name}
                  </dd>
                </dl>
                <Alert>
                  <AlertTitle>История состава</AlertTitle>
                  <AlertDescription>
                    В приложении состав хранится датированными интервалами. При
                    переводе предыдущая группа остаётся в истории.
                  </AlertDescription>
                </Alert>
              </div>
            ) : (
              <div className="detail-body">
                <Status status={detail.status} />
                <dl>
                  <dt>Филиал</dt>
                  <dd>{clubName(detail.clubId)}</dd>
                  <dt>Начало</dt>
                  <dd>16 октября · {time(detail)}</dd>
                  <dt>Тренер</dt>
                  <dd>{detail.trainerName}</dd>
                  <dt>Участников в снимке</dt>
                  <dd>
                    {detail.status === "planned"
                      ? "Снимок создаётся при открытии"
                      : detail.status === "cancelled"
                        ? "Не создавался"
                        : detail.roster}
                  </dd>
                </dl>
                <Alert>
                  <AlertTitle>
                    {detail.status === "cancelled"
                      ? "Занятие отменено"
                      : detail.status === "closed"
                        ? "Журнал завершён"
                        : "Демонстрация карточки занятия"}
                  </AlertTitle>
                  <AlertDescription>
                    {detail.status === "closed"
                      ? "3 присутствия, 1 пропуск. После закрытия исправляет администратор с обязательной причиной."
                      : detail.status === "cancelled"
                        ? "Отменённые занятия не входят в посещаемость."
                        : "Рабочие отметки и завершение журнала выполняются в основном приложении."}
                  </AlertDescription>
                </Alert>
                <Button
                  variant="outline"
                  onClick={() => {
                    setDetail(null);
                    navigate("journals");
                  }}
                >
                  К списку журналов
                  <ArrowRight data-icon="inline-end" />
                </Button>
              </div>
            ))}
        </SheetContent>
      </Sheet>
    </>
  );
}
function NoResults({ reset }: { reset?: () => void }) {
  return (
    <Empty>
      <EmptyHeader>
        <EmptyTitle>Записей не найдено</EmptyTitle>
        <EmptyDescription>
          Для этого контекста и фильтров нет записей.
        </EmptyDescription>
      </EmptyHeader>
      {reset && (
        <EmptyContent>
          <Button variant="outline" onClick={reset}>
            Сбросить фильтры
          </Button>
        </EmptyContent>
      )}
    </Empty>
  );
}

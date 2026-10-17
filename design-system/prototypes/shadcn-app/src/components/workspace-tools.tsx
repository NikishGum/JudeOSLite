import { useState, lazy, Suspense } from "react";
import {
  Activity,
  BookOpen,
  Building2,
  CalendarDays,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronsUpDown,
  History,
  LayoutDashboard,
  Library,
  LogOut,
  MoreHorizontal,
  Settings,
  UserRound,
  Users,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import {
  Field,
  FieldGroup,
  FieldLabel,
  FieldDescription,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
const DateCalendar = lazy(() => import("./date-calendar"));
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "@/components/ui/popover";
import {
  sections,
  roleNames,
  demoDate,
  type Role,
  type Section,
} from "../data";
import { cn } from "@/lib/utils";

type Page = Section | "registries";
export const navIcons = {
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
export function initials(name: string) {
  return name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((n) => n[0])
    .join("")
    .toLocaleUpperCase("ru");
}
export function StatusDot({
  active,
  label,
}: {
  active: boolean;
  label: string;
}) {
  return (
    <span
      className={cn(
        "status-dot",
        active ? "status-dot-active" : "status-dot-muted",
      )}
      role="img"
      aria-label={label}
      title={label}
    />
  );
}
export function ProfileMenu({
  name,
  role,
  onSettings,
  onSignOut,
  compact = false,
}: {
  name: string;
  role: Role;
  onSettings: () => void;
  onSignOut: () => void;
  compact?: boolean;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          className={cn("profile-trigger", compact && "profile-compact")}
          aria-label="Меню профиля"
        >
          <Avatar>
            <AvatarFallback>{initials(name)}</AvatarFallback>
          </Avatar>
          {!compact && (
            <span className="profile-copy">
              <strong>{name}</strong>
              <small>{roleNames[role]}</small>
            </span>
          )}
          {!compact && <ChevronsUpDown data-icon="inline-end" />}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        sideOffset={8}
        collisionPadding={12}
        side={compact ? "bottom" : "right"}
        className="profile-menu"
      >
        <DropdownMenuLabel>
          <strong>{name}</strong>
          <span className="menu-caption">{roleNames[role]} · демо-профиль</span>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuGroup>
          <DropdownMenuItem onSelect={onSettings}>
            <Settings />
            Настройки профиля
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={onSignOut}>
            <LogOut />
            Выйти из демо
          </DropdownMenuItem>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
export function ProfileSettings({
  open,
  onOpenChange,
  name,
  onSave,
  showDemo,
  onShowDemo,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  name: string;
  onSave: (v: string) => void;
  showDemo: boolean;
  onShowDemo: (v: boolean) => void;
}) {
  const [draft, setDraft] = useState(name);
  const [saved, setSaved] = useState(false);
  const [draftShow, setDraftShow] = useState(showDemo);
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="detail-sheet">
        <SheetHeader>
          <SheetTitle>Настройки профиля</SheetTitle>
          <SheetDescription>
            Настройки локального демо. При перезагрузке они сбросятся.
          </SheetDescription>
        </SheetHeader>
        <form
          className="detail-body"
          onSubmit={(e) => {
            e.preventDefault();
            if (draft.trim()) {
              onSave(draft.trim());
              onShowDemo(draftShow);
              setSaved(true);
            }
          }}
        >
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="profile-name">Имя в интерфейсе</FieldLabel>
              <Input
                id="profile-name"
                value={draft}
                onChange={(e) => {
                  setDraft(e.target.value);
                  setSaved(false);
                }}
                required
                maxLength={60}
              />
              <FieldDescription>
                Показывается в меню профиля и в инициалах.
              </FieldDescription>
            </Field>
            <Field orientation="horizontal">
              <div className="field-copy">
                <FieldLabel htmlFor="demo-tools">
                  Демонстрационные настройки
                </FieldLabel>
                <FieldDescription>
                  Показывать выбор роли и состояния экрана.
                </FieldDescription>
              </div>
              <Switch
                id="demo-tools"
                checked={draftShow}
                onCheckedChange={(value) => {
                  setDraftShow(value);
                  setSaved(false);
                }}
              />
            </Field>
          </FieldGroup>
          <Button type="submit" disabled={!draft.trim()}>
            Сохранить настройки
          </Button>
          {saved && (
            <p role="status" className="settings-saved">
              Настройки сохранены в этом демо-сеансе.
            </p>
          )}
        </form>
      </SheetContent>
    </Sheet>
  );
}
export function MobileNavigation({
  page,
  role,
  allowed,
  navigate,
}: {
  page: Page;
  role: Role;
  allowed: Section[];
  navigate: (p: Page) => void;
}) {
  const [more, setMore] = useState(false);
  const primary: Section[] =
    role === "owner"
      ? ["home", "sessions", "statistics"]
      : role === "administrator"
        ? ["home", "sessions", "participants"]
        : ["home", "sessions", "journals"];
  const secondary: Page[] = [
    ...allowed.filter((p) => !primary.includes(p)),
    "registries",
  ];
  const go = (p: Page) => {
    setMore(false);
    navigate(p);
  };
  return (
    <>
      <nav className="bottom-navigation" aria-label="Основная навигация">
        {primary.map((p) => {
          const Icon = navIcons[p];
          return (
            <button
              type="button"
              key={p}
              aria-current={page === p ? "page" : undefined}
              onClick={() => go(p)}
            >
              <Icon aria-hidden="true" />
              <span>{sections[p]}</span>
            </button>
          );
        })}
        <button
          type="button"
          aria-label="Ещё разделы"
          aria-haspopup="dialog"
          aria-expanded={more}
          aria-current={!primary.includes(page as Section) ? "page" : undefined}
          onClick={() => setMore(true)}
        >
          <MoreHorizontal aria-hidden="true" />
          <span>Ещё</span>
        </button>
      </nav>
      <Sheet open={more} onOpenChange={setMore}>
        <SheetContent side="bottom" className="more-sheet">
          <SheetHeader>
            <SheetTitle>Ещё</SheetTitle>
            <SheetDescription>
              Остальные разделы рабочего пространства
            </SheetDescription>
          </SheetHeader>
          <nav className="more-navigation" aria-label="Другие разделы">
            {secondary.map((p) => {
              const Icon = navIcons[p];
              return (
                <Button
                  key={p}
                  variant="ghost"
                  aria-current={page === p ? "page" : undefined}
                  onClick={() => go(p)}
                >
                  <Icon data-icon="inline-start" />
                  {p === "registries" ? "Подборка registries" : sections[p]}
                </Button>
              );
            })}
          </nav>
        </SheetContent>
      </Sheet>
    </>
  );
}
export const dateLabel = (value: string) =>
  new Intl.DateTimeFormat("ru-BY", {
    day: "numeric",
    month: "long",
    year: "numeric",
    timeZone: "Europe/Minsk",
  }).format(new Date(`${value}T12:00:00+03:00`));
const dayValue = (date: Date) =>
  new Intl.DateTimeFormat("en-CA", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    timeZone: "Europe/Minsk",
  }).format(date);
export function DateNavigation({
  date,
  onChange,
}: {
  date: string;
  onChange: (v: string) => void;
}) {
  const [open, setOpen] = useState(false);
  function move(delta: number) {
    const next = new Date(`${date}T12:00:00Z`);
    next.setUTCDate(next.getUTCDate() + delta);
    onChange(next.toISOString().slice(0, 10));
  }
  return (
    <div className="date-navigation">
      <div className="date-picker-group">
        <Button
          variant="ghost"
          size="icon"
          className="date-step"
          aria-label="Предыдущий день"
          onClick={() => move(-1)}
        >
          <ChevronLeft />
        </Button>
        <Popover open={open} onOpenChange={setOpen}>
          <PopoverTrigger asChild>
            <Button
              variant="ghost"
              className="date-select-trigger"
              aria-label="Открыть календарь"
            >
              <CalendarDays data-icon="inline-start" />
              {new Intl.DateTimeFormat("ru-BY", {
                day: "numeric",
                month: "long",
                timeZone: "Europe/Minsk",
              }).format(new Date(`${date}T12:00:00+03:00`))}
              <ChevronDown data-icon="inline-end" />
            </Button>
          </PopoverTrigger>
          <PopoverContent
            align="start"
            className="calendar-popover"
            collisionPadding={12}
          >
            <Suspense fallback={<p role="status">Загрузка календаря…</p>}>
              <DateCalendar
                mode="single"
                required
                selected={new Date(`${date}T12:00:00+03:00`)}
                defaultMonth={new Date(`${date}T12:00:00+03:00`)}
                timeZone="Europe/Minsk"
                weekStartsOn={1}
                labels={{
                  labelNext: () => "Следующий месяц",
                  labelPrevious: () => "Предыдущий месяц",
                }}
                onSelect={(day) => {
                  if (day) {
                    onChange(dayValue(day));
                    setOpen(false);
                  }
                }}
              />
            </Suspense>
          </PopoverContent>
        </Popover>
        <Button
          variant="ghost"
          size="icon"
          className="date-step"
          aria-label="Следующий день"
          onClick={() => move(1)}
        >
          <ChevronRight />
        </Button>
      </div>
      {date !== demoDate && (
        <Button
          className="date-reset"
          variant="link"
          onClick={() => onChange(demoDate)}
        >
          К демодню
        </Button>
      )}
    </div>
  );
}

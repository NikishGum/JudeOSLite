import { Children, isValidElement, useState, type ReactNode } from "react";
import {
  Building2,
  ChevronsUpDown,
  LogOut,
  Settings,
  UserRound,
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
  Sidebar,
  SidebarHeader,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
} from "@/components/ui/sidebar";
import { Dialog } from "../shared/ui/Dialog";
import { InterfaceSettings } from "../features/club-workspace/Settings";
import { cn } from "@/lib/utils";
import logo from "../../../design-system/assets/logo-pride.png";

export function initials(name: string) {
  return name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part[0])
    .join("")
    .toLocaleUpperCase("ru");
}
export function PersonAvatar({
  name,
  active = true,
  label = "Действующий аккаунт",
}: {
  name: string;
  active?: boolean;
  label?: string;
}) {
  return (
    <span className="person-avatar">
      <Avatar>
        <AvatarFallback>{initials(name)}</AvatarFallback>
      </Avatar>
      <StatusDot active={active} label={label} />
    </span>
  );
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
export function RuntimeProfile({
  name,
  role,
  clubs,
  compact,
  busy,
  onSignOut,
  collapsed,
  onCollapsed,
}: {
  name: string;
  role: string;
  clubs: { id: string; name: string; roles: string[] }[];
  compact: boolean;
  busy: boolean;
  onSignOut: () => void;
  collapsed: boolean;
  onCollapsed: (collapsed: boolean) => void;
}) {
  const [settings, setSettings] = useState(false);
  return (
    <div className="account">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            className={cn("profile-trigger", { "profile-compact": compact })}
            aria-label="Меню профиля"
          >
            <Avatar>
              <AvatarFallback>{initials(name)}</AvatarFallback>
            </Avatar>
            <span className="profile-copy">
              <strong>{name}</strong>
              <span>{role}</span>
            </span>
            {!compact && <ChevronsUpDown data-icon="inline-end" />}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          className="profile-menu"
          side={compact ? "bottom" : "right"}
          align="end"
          collisionPadding={12}
          sideOffset={8}
        >
          <DropdownMenuLabel>
            <strong>{name}</strong>
            <span className="menu-caption">{role}</span>
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem onSelect={() => setSettings(true)}>
              <Settings />
              Настройки приложения
            </DropdownMenuItem>
            <DropdownMenuItem disabled={busy} onSelect={onSignOut}>
              <LogOut />
              {busy ? "Выходим…" : "Выйти"}
            </DropdownMenuItem>
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenu>
      {settings && (
        <Dialog
          title="Настройки приложения"
          onCancel={() => setSettings(false)}
          cancelLabel="Закрыть настройки"
          className="profile-settings"
        >
          <div className="profile-identity">
            <UserRound aria-hidden="true" />
            <div>
              <strong>{name}</strong>
              <p>{role}</p>
            </div>
          </div>
          <InterfaceSettings collapsed={collapsed} onCollapsed={onCollapsed} />
          <h3>Доступные филиалы</h3>
          {clubs.length ? (
            <ul>
              {clubs.map((club) => (
                <li key={club.id}>
                  <Building2 aria-hidden="true" />
                  {club.name}
                </li>
              ))}
            </ul>
          ) : (
            <p>Нет текущих назначений.</p>
          )}
          <p className="help">
            Для изменения имени, пароля или назначений обратитесь к
            администратору клуба.
          </p>
        </Dialog>
      )}
    </div>
  );
}
export function RuntimeSidebar({
  links,
  onHome,
}: {
  links: ReactNode;
  onHome: (event: React.MouseEvent<HTMLAnchorElement>) => void;
}) {
  return (
    <Sidebar collapsible="icon" className="runtime-sidebar">
      <SidebarHeader>
        <a
          className="brand"
          href="/"
          onClick={onHome}
          aria-label="PRIDE — главная"
        >
          <img src={logo} width="44" height="44" alt="" />
          <span>
            <strong>PRIDE CLUB OS</strong>
            <small>Управление клубом</small>
          </span>
        </a>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <p className="sidebar-caption">Рабочее пространство</p>
          <nav
            className="sidebar workspace-navigation"
            aria-label="Разделы филиала"
          >
            <SidebarMenu>
              {Children.toArray(links).map((link, index) => (
                <SidebarMenuItem key={index}>
                  <SidebarMenuButton
                    asChild
                    isActive={
                      isValidElement<{ "aria-current"?: string }>(link) &&
                      !!link.props["aria-current"]
                    }
                    tooltip={
                      isValidElement<{ "aria-label"?: string }>(link)
                        ? link.props["aria-label"]
                        : undefined
                    }
                  >
                    {link}
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </nav>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter>
        <p className="sidebar-caption sidebar-signature">PRIDE · Клуб дзюдо</p>
      </SidebarFooter>
    </Sidebar>
  );
}

import { useEffect, useState } from "react";
import {
  Bell,
  CalendarDays,
  CheckCheck,
  CreditCard,
  FileText,
  Users,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { apiPost } from "../../shared/api";
import {
  Pagination,
  ResourceState,
  useCommand,
  useResource,
} from "./api-state";
import type { List, Notification } from "./types";

type Inbox = List<Notification> & { unreadCount: number };
const icons = {
  family: Users,
  payment: CreditCard,
  member_document: FileText,
  training_session: CalendarDays,
};
export function safeNotificationHref(value: string) {
  if (!value.startsWith("/") || value.startsWith("//")) return null;
  const url = new URL(value, window.location.origin);
  if (
    url.origin !== window.location.origin ||
    !/^\/(?:clubs\/[0-9a-f-]{36}\/)?(?:families|payments|sessions|notifications)(?:$|\/)/.test(
      url.pathname,
    )
  )
    return null;
  return url.pathname + url.search;
}
export function NotificationBell({ onOpen }: { onOpen: () => void }) {
  const inbox = useResource<Inbox>("/api/notifications?status=unread&limit=1");
  useEffect(() => {
    const refresh = () => {
      if (document.visibilityState === "visible") inbox.refresh();
    };
    // useResource owns the interval and focus/visibility revalidation.
    window.addEventListener("club-data-changed", refresh);
    return () => {
      window.removeEventListener("club-data-changed", refresh);
    };
  }, []);
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      className="notification-bell"
      onClick={onOpen}
      aria-label={`Уведомления${inbox.data?.unreadCount ? `, непрочитанных: ${inbox.data.unreadCount}` : ""}`}
    >
      <Bell />
      {!!inbox.data?.unreadCount && (
        <span className="notification-count">
          {inbox.data.unreadCount > 99 ? "99+" : inbox.data.unreadCount}
        </span>
      )}
    </Button>
  );
}
export function Notifications({
  onNavigate,
}: {
  onNavigate: (href: string) => void;
}) {
  const [status, setStatus] = useState("all"),
    [page, setPage] = useState(0);
  const resource = useResource<Inbox>(
    `/api/notifications?status=${status}&page=${page}&limit=25`,
  );
  const command = useCommand(resource.refresh);
  return (
    <Card className="notifications-page">
      <div className="module-toolbar">
        <ToggleGroup
          type="single"
          className="segmented-control"
          aria-label="Фильтр уведомлений"
          value={status}
          onValueChange={(value) => {
            if (value) {
              setStatus(value);
              setPage(0);
            }
          }}
        >
          <ToggleGroupItem value="all">Все</ToggleGroupItem>
          <ToggleGroupItem value="unread">
            Непрочитанные
            {resource.data ? ` · ${resource.data.unreadCount}` : ""}
          </ToggleGroupItem>
        </ToggleGroup>
        <Button
          variant="outline"
          disabled={!resource.data?.unreadCount || command.busy}
          onClick={() =>
            void command.run(() => apiPost("/api/notifications/read-all", {}))
          }
        >
          <CheckCheck />
          Прочитать все
        </Button>
      </div>
      {command.error && (
        <p className="error" role="alert">
          {command.error}
        </p>
      )}
      <ResourceState
        resource={resource}
        empty={resource.data?.items.length === 0}
        title={status === "unread" ? "Всё прочитано" : "Уведомлений пока нет"}
      >
        <p>
          Здесь появятся изменения занятий и новости по вашей работе в клубе.
        </p>
      </ResourceState>
      <ul className="notification-list">
        {resource.data?.items.map((item) => {
          const Icon = icons[item.kind] ?? Bell;
          const href = safeNotificationHref(item.href);
          return (
            <li key={item.id} className={item.read ? "" : "is-unread"}>
              <span className={`notification-kind kind-${item.kind}`}>
                <Icon aria-hidden="true" />
              </span>
              <div>
                <strong>{item.title}</strong>
                {item.body && <p>{item.body}</p>}
                <time dateTime={item.createdAt}>
                  {new Intl.DateTimeFormat("ru-BY", {
                    timeZone: "Europe/Minsk",
                    day: "numeric",
                    month: "long",
                    hour: "2-digit",
                    minute: "2-digit",
                  }).format(new Date(item.createdAt))}
                </time>
                <div className="notification-actions">
                  {href && (
                    <Button variant="link" onClick={() => onNavigate(href)}>
                      Перейти
                    </Button>
                  )}
                  {!item.read && (
                    <Button
                      variant="ghost"
                      disabled={command.busy}
                      aria-label={`Прочитано: ${item.title}`}
                      onClick={() =>
                        void command.run(() =>
                          apiPost(`/api/notifications/${item.id}/read`, {}),
                        )
                      }
                    >
                      Отметить прочитанным
                    </Button>
                  )}
                </div>
              </div>
              {!item.read && (
                <span
                  className="unread-dot"
                  aria-label="Непрочитанное"
                  role="img"
                />
              )}
            </li>
          );
        })}
      </ul>
      <Pagination page={page} list={resource.data} onPage={setPage} />
    </Card>
  );
}

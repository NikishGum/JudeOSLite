import { Building2, CalendarDays, Users } from "lucide-react";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Card } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { initials } from "../../app/RuntimeShell";
import { routePath } from "../../app/navigation";
import { sessionHref } from "../sessions/navigation";
import { ResourceState, useResource } from "./api-state";
import type { Panel } from "../dashboard/types";
import type { Session } from "../sessions/types";
import { day, today } from "./types";

export function Profile({
  name,
  role,
  clubs,
  clubId,
  trainerOnly,
}: {
  name: string;
  role: string;
  clubs: { id: string; name: string; roles: string[] }[];
  clubId: string;
  trainerOnly: boolean;
}) {
  const resource = useResource<Panel>(
    trainerOnly ? `/api/dashboard?${new URLSearchParams({ clubId })}` : null,
    false,
  );
  return (
    <Card className="profile-page">
      <div className="member-heading">
        <Avatar>
          <AvatarFallback>{initials(name)}</AvatarFallback>
        </Avatar>
        <div>
          <h2>{name}</h2>
          <p>{role}</p>
          <span className="status-badge success">Действующий аккаунт</span>
        </div>
      </div>
      <Tabs defaultValue="overview">
        <TabsList aria-label="Профиль сотрудника">
          <TabsTrigger value="overview">Основное</TabsTrigger>
          {trainerOnly && <TabsTrigger value="groups">Мои группы</TabsTrigger>}
          <TabsTrigger value="access">Филиалы</TabsTrigger>
        </TabsList>
        <TabsContent value="overview">
          <div className="profile-columns">
            <section>
              <h3>Профиль</h3>
              <dl className="facts-list">
                <div>
                  <dt>Имя</dt>
                  <dd>{name}</dd>
                </div>
                <div>
                  <dt>Роль</dt>
                  <dd>{role}</dd>
                </div>
                <div>
                  <dt>Филиалов</dt>
                  <dd>{clubs.length}</dd>
                </div>
              </dl>
              <p className="help">
                Для изменения данных обратитесь к администратору или владельцу
                клуба.
              </p>
            </section>
            {trainerOnly && (
              <section>
                <h3>Ближайшие тренировки</h3>
                <ResourceState resource={resource} />
                {resource.data && !resource.data.next && (
                  <p>Назначенных занятий пока нет.</p>
                )}
                {resource.data?.next && (
                  <div className="trainer-next">
                    <CalendarDays />
                    <strong>{resource.data.next.groupName}</strong>
                    <p>
                      {day(resource.data.next.date)} ·{" "}
                      {new Intl.DateTimeFormat("ru-BY", {
                        timeZone: resource.data.next.timezone,
                        hour: "2-digit",
                        minute: "2-digit",
                      }).format(new Date(resource.data.next.startsAt))}
                    </p>
                    <a
                      className="session-link session-primary"
                      href={sessionHref(resource.data.next.clubId, {
                        sessionId: resource.data.next.id,
                        from: resource.data.next.date,
                        through: resource.data.next.date,
                        journal: "trainer",
                      })}
                    >
                      Открыть занятие
                    </a>
                  </div>
                )}
              </section>
            )}
          </div>
        </TabsContent>
        {trainerOnly && (
          <TabsContent value="groups">
            <h3>Назначенные группы</h3>
            <ResourceState
              resource={resource}
              empty={resource.data?.groups.length === 0}
              title="Группы пока не назначены"
            />
            <ul className="compact-agenda">
              {resource.data?.groups.map((group) => (
                <li key={group.id}>
                  <Users />
                  <div>
                    <strong>{group.name}</strong>
                    <p>
                      {
                        resource.data?.clubs.find(
                          (club) => club.id === group.clubId,
                        )?.name
                      }{" "}
                      · участников: {group.members}
                    </p>
                    <a href={sessionHref(group.clubId, { groupId: group.id })}>
                      Расписание группы
                    </a>
                  </div>
                </li>
              ))}
            </ul>
          </TabsContent>
        )}
        <TabsContent value="access">
          <h3>Ваши филиалы</h3>
          <ul className="settings-clubs">
            {clubs.map((club) => (
              <li key={club.id}>
                <Building2 />
                <div>
                  <strong>{club.name}</strong>
                  {!trainerOnly && (
                    <a href={routePath({ clubId: club.id, section: "home" })}>
                      Открыть рабочий день
                    </a>
                  )}
                </div>
              </li>
            ))}
          </ul>
          {!clubs.length && <p>Нет текущих назначений.</p>}
        </TabsContent>
      </Tabs>
    </Card>
  );
}
export function TrainerProfile({
  trainerId,
  name,
  clubId,
  clubName,
}: {
  trainerId: string;
  name: string;
  clubId: string;
  clubName: string;
}) {
  const end = new Date(`${today()}T12:00:00Z`);
  end.setUTCDate(end.getUTCDate() + 31);
  const resource = useResource<Session[]>(
    `/api/clubs/${clubId}/sessions?from=${today()}&until=${end.toISOString().slice(0, 10)}`,
    false,
  );
  const sessions =
    resource.data?.filter(
      (session) =>
        session.trainerId === trainerId && session.status !== "cancelled",
    ) ?? [];
  const groups = [
    ...new Map(
      sessions.map((session) => [session.groupId, session.groupName]),
    ).entries(),
  ];
  return (
    <>
      <div className="member-heading">
        <Avatar>
          <AvatarFallback>{initials(name)}</AvatarFallback>
        </Avatar>
        <div>
          <h2>{name}</h2>
          <p>Тренер · {clubName}</p>
        </div>
      </div>
      <ResourceState resource={resource} />
      <Tabs defaultValue="schedule">
        <TabsList aria-label="Профиль тренера">
          <TabsTrigger value="schedule">Ближайшие занятия</TabsTrigger>
          <TabsTrigger value="groups">Группы</TabsTrigger>
        </TabsList>
        <TabsContent value="schedule">
          <p className="help">Назначенные занятия на ближайшие 30 дней.</p>
          {resource.data && !sessions.length && <p>Ближайших занятий нет.</p>}
          <ul className="compact-agenda">
            {sessions.slice(0, 20).map((session) => (
              <li key={session.id}>
                <CalendarDays />
                <time>
                  {day(session.date)}
                  <span>
                    {new Intl.DateTimeFormat("ru-BY", {
                      timeZone: session.timezone,
                      hour: "2-digit",
                      minute: "2-digit",
                    }).format(new Date(session.startsAt))}
                  </span>
                </time>
                <div>
                  <strong>{session.groupName}</strong>
                  <a
                    href={sessionHref(clubId, {
                      sessionId: session.id,
                      from: session.date,
                      through: session.date,
                      journal: "admin",
                    })}
                  >
                    Открыть занятие
                  </a>
                </div>
              </li>
            ))}
          </ul>
        </TabsContent>
        <TabsContent value="groups">
          <h3>Группы в ближайшем расписании</h3>
          <ul className="compact-agenda">
            {groups.map(([id, label]) => (
              <li key={id}>
                <Users />
                <a href={sessionHref(clubId, { groupId: id })}>{label}</a>
              </li>
            ))}
          </ul>
          {resource.data && !groups.length && (
            <p>Групп с занятиями в этом периоде нет.</p>
          )}
        </TabsContent>
      </Tabs>
    </>
  );
}

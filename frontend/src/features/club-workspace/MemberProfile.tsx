import { useState } from "react";
import { CalendarDays, Check, Users } from "lucide-react";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Dialog } from "../../shared/ui/Dialog";
import { initials } from "../../app/RuntimeShell";
import { ResourceState, useResource } from "./api-state";
import { Documents } from "./Documents";
import { MemberPayments } from "./Payments";
import { day, type MemberProfileData } from "./types";

const reasons = {
  unknown: "Причина неизвестна",
  illness: "Болезнь",
  excused: "Уважительная причина",
};
export function MemberProfile({
  clubId,
  membershipId,
  canManage,
  beforeNavigate,
  onClose,
}: {
  clubId: string;
  membershipId: string;
  canManage: boolean;
  beforeNavigate: (action: () => void) => void;
  onClose: () => void;
}) {
  const resource = useResource<MemberProfileData>(
    `/api/clubs/${clubId}/member-profiles/${membershipId}`,
    false,
  );
  const [tab, setTab] = useState("overview");
  const member = resource.data;
  return (
    <Dialog
      title={member?.fullName ?? "Карточка участника"}
      className="wide-panel member-profile"
      onCancel={() => beforeNavigate(onClose)}
      cancelLabel="Закрыть карточку"
    >
      <ResourceState resource={resource} />
      {member && (
        <>
          <div className="member-heading">
            <Avatar>
              <AvatarFallback>{initials(member.fullName)}</AvatarFallback>
            </Avatar>
            <div>
              <h2>{member.fullName}</h2>
              <p>
                {day(member.birthDate)} ·{" "}
                {member.groupName || "Группа пока не назначена"}
              </p>
              <span
                className={`status-badge ${member.archived ? "neutral" : "success"}`}
              >
                {member.archived ? "В архиве" : "Активный участник"}
              </span>
            </div>
          </div>
          <Tabs value={tab} onValueChange={setTab}>
            <TabsList aria-label="Карточка ребёнка">
              <TabsTrigger value="overview">Основное</TabsTrigger>
              <TabsTrigger value="groups">Группы</TabsTrigger>
              <TabsTrigger value="visits">Посещения</TabsTrigger>
              <TabsTrigger value="payments">Платежи</TabsTrigger>
              <TabsTrigger value="documents">Документы</TabsTrigger>
            </TabsList>
            <TabsContent value="overview">
              <div className="profile-columns">
                <section>
                  <h3>Данные участника</h3>
                  <dl className="facts-list">
                    <div>
                      <dt>Дата рождения</dt>
                      <dd>{day(member.birthDate)}</dd>
                    </div>
                    <div>
                      <dt>В клубе с</dt>
                      <dd>{day(member.joinedOn)}</dd>
                    </div>
                    <div>
                      <dt>Семья</dt>
                      <dd>{member.familyName ?? "Не указана"}</dd>
                    </div>
                    <div>
                      <dt>Контактное лицо</dt>
                      <dd>{member.contactName || "Не указано"}</dd>
                    </div>
                    <div>
                      <dt>Контакт</dt>
                      <dd>{member.contactValue || "Не указан"}</dd>
                    </div>
                    <div>
                      <dt>Тренер</dt>
                      <dd>{member.trainerName || "Не назначен"}</dd>
                    </div>
                  </dl>
                </section>
                <section>
                  <h3>Посещаемость</h3>
                  <div className="attendance-value">
                    {member.attendance.rate === null
                      ? "—"
                      : `${member.attendance.rate}%`}
                  </div>
                  <p className="help">По завершённым занятиям</p>
                  <dl className="mini-metrics">
                    <div>
                      <dt>Присутствия</dt>
                      <dd>{member.attendance.present}</dd>
                    </div>
                    <div>
                      <dt>Пропуски</dt>
                      <dd>{member.attendance.absent}</dd>
                    </div>
                  </dl>
                </section>
              </div>
              <section className="profile-section">
                <h3>Ближайшие занятия</h3>
                {!member.upcoming.length && (
                  <p className="help">Ближайших занятий пока нет.</p>
                )}
                <ul className="compact-agenda">
                  {member.upcoming.map((session) => (
                    <li key={session.id}>
                      <CalendarDays aria-hidden="true" />
                      <time>
                        {day(session.date)}
                        <span>{session.time}</span>
                      </time>
                      <div>
                        <strong>{session.groupName}</strong>
                        <p>{session.trainerName}</p>
                      </div>
                    </li>
                  ))}
                </ul>
              </section>
            </TabsContent>
            <TabsContent value="groups">
              <h3>Группы и история состава</h3>
              {!member.groups.length && (
                <p>Участник пока не записан в группу.</p>
              )}
              <ul className="compact-agenda">
                {member.groups.map((group) => (
                  <li key={`${group.id}:${group.from}`}>
                    <Users aria-hidden="true" />
                    <div>
                      <strong>{group.name}</strong>
                      <p>{group.trainerName || "Тренер не назначен"}</p>
                      <span className="help">
                        {day(group.from)} —{" "}
                        {group.until ? day(group.until) : "по настоящее время"}
                      </span>
                    </div>
                  </li>
                ))}
              </ul>
            </TabsContent>
            <TabsContent value="visits">
              <h3>Последние посещения</h3>
              {!member.attendance.history.length && (
                <p>Здесь появятся отметки после занятий.</p>
              )}
              <ul className="visit-list">
                {member.attendance.history.map((visit) => (
                  <li key={visit.id}>
                    <time>
                      {day(visit.date)}
                      <span>{visit.time}</span>
                    </time>
                    <strong>{visit.groupName}</strong>
                    <span
                      className={`status-badge ${visit.state === "present" ? "success" : visit.state === "absent" ? "warning" : "neutral"}`}
                    >
                      {visit.state === "present" ? (
                        <>
                          <Check />
                          Присутствует
                        </>
                      ) : visit.state === "absent" ? (
                        `Отсутствует · ${reasons[visit.absenceReason ?? "unknown"]}`
                      ) : (
                        "Не отмечено"
                      )}
                    </span>
                  </li>
                ))}
              </ul>
            </TabsContent>
            <TabsContent value="payments">
              <MemberPayments
                clubId={clubId}
                membershipId={membershipId}
                canManage={canManage}
                beforeNavigate={beforeNavigate}
              />
            </TabsContent>
            <TabsContent value="documents">
              <Documents
                clubId={clubId}
                membershipId={membershipId}
                canManage={canManage}
                beforeNavigate={beforeNavigate}
              />
            </TabsContent>
          </Tabs>
        </>
      )}
    </Dialog>
  );
}

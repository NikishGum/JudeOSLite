import { useId } from "react";
import {
  Building2,
  Monitor,
  ShieldCheck,
  Smartphone,
  UserRound,
} from "lucide-react";
import { Card } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { useIsMobile } from "@/hooks/use-mobile";

export type SettingsProps = {
  name: string;
  role: string;
  clubs: { id: string; name: string; roles: string[] }[];
  collapsed: boolean;
  onCollapsed: (collapsed: boolean) => void;
};
export function InterfaceSettings({
  collapsed,
  onCollapsed,
}: Pick<SettingsProps, "collapsed" | "onCollapsed">) {
  const mobile = useIsMobile();
  const settingId = useId();
  return (
    <>
      <h3>Интерфейс</h3>
      {!mobile && (
        <FieldGroup>
          <Field orientation="horizontal">
            <input
              id={settingId}
              type="checkbox"
              checked={collapsed}
              onChange={(e) => onCollapsed(e.target.checked)}
            />
            <FieldLabel htmlFor={settingId}>
              Сворачивать меню на компьютере
            </FieldLabel>
          </Field>
          <FieldDescription>
            Применяется сразу в текущей вкладке.
          </FieldDescription>
        </FieldGroup>
      )}
      <div className="settings-note">
        {mobile ? <Smartphone /> : <Monitor />}
        <p>
          {mobile
            ? "Основные разделы всегда под рукой в нижней панели. Остальные находятся в меню «Ещё»."
            : "Боковое меню можно свернуть, чтобы освободить место для работы."}
        </p>
      </div>
    </>
  );
}
export function ApplicationSettings(props: SettingsProps) {
  return (
    <Card className="settings-page">
      <Tabs defaultValue="profile">
        <TabsList aria-label="Настройки">
          <TabsTrigger value="profile">Профиль</TabsTrigger>
          <TabsTrigger value="clubs">Филиалы и доступ</TabsTrigger>
          <TabsTrigger value="interface">Интерфейс</TabsTrigger>
        </TabsList>
        <TabsContent value="profile">
          <div className="settings-section">
            <UserRound aria-hidden="true" />
            <div>
              <h2>{props.name}</h2>
              <p>{props.role}</p>
              <p className="help">
                Для изменения имени или пароля обратитесь к администратору
                клуба.
              </p>
            </div>
          </div>
          <div className="settings-section">
            <ShieldCheck aria-hidden="true" />
            <div>
              <h3>Ваш доступ</h3>
              <p>Разделы и данные определяются вашими назначениями в клубе.</p>
            </div>
          </div>
        </TabsContent>
        <TabsContent value="clubs">
          <h2>Назначенные филиалы</h2>
          {!props.clubs.length && (
            <p className="help">Нет текущих назначений.</p>
          )}
          <ul className="settings-clubs">
            {props.clubs.map((club) => (
              <li key={club.id}>
                <Building2 aria-hidden="true" />
                <div>
                  <strong>{club.name}</strong>
                  <p>
                    {club.roles
                      .map(
                        (role) =>
                          ({
                            owner: "Владелец",
                            administrator: "Администратор",
                            trainer: "Тренер",
                            parent: "Родитель",
                          })[role] ?? role,
                      )
                      .join(" · ")}
                  </p>
                </div>
              </li>
            ))}
          </ul>
          <p className="help">
            Назначения сотрудников и доступ семьи меняет администратор или
            владелец.
          </p>
        </TabsContent>
        <TabsContent value="interface">
          <InterfaceSettings
            collapsed={props.collapsed}
            onCollapsed={props.onCollapsed}
          />
        </TabsContent>
      </Tabs>
    </Card>
  );
}

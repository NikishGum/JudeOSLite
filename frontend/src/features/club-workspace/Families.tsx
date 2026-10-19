import { useDeferredValue, useState, type FormEvent } from "react";
import {
  ChevronRight,
  Mail,
  Phone,
  Plus,
  Search,
  UserPlus,
  Users,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Card } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Dialog } from "../../shared/ui/Dialog";
import { apiPost, apiPut } from "../../shared/api";
import { initials } from "../../app/RuntimeShell";
import {
  Pagination,
  ResourceState,
  useCommand,
  useResource,
} from "./api-state";
import { MemberProfile } from "./MemberProfile";
import { day, type Family, type List, type MemberSummary } from "./types";

type Navigate = (action: () => void) => void;
export function Families({
  clubId,
  portal = false,
  detail = "",
  onDetail,
  beforeNavigate,
}: {
  clubId: string;
  portal?: boolean;
  detail?: string;
  onDetail: (id: string) => void;
  beforeNavigate: Navigate;
}) {
  const [query, setQuery] = useState(""),
    [search, setSearch] = useState(""),
    [page, setPage] = useState(0);
  const [editing, setEditing] = useState<Family | "new" | null>(null),
    [member, setMember] = useState<MemberSummary | null>(null);
  const base = `/api/clubs/${clubId}/families` as const;
  const resource = useResource<List<Family>>(
    portal
      ? "/api/portal/families"
      : clubId
        ? `${base}?${new URLSearchParams({ q: search, page: String(page), limit: "25" })}`
        : null,
  );
  const selected = useResource<Family>(
    detail && !portal && clubId ? `${base}/${detail}` : null,
    false,
  );
  const family = portal
    ? (resource.data?.items.find((item) => item.id === detail) ??
      (resource.data?.items.length === 1 ? resource.data.items[0] : null))
    : selected.data;
  function closeDetail() {
    beforeNavigate(() => onDetail(""));
  }
  return (
    <div className="module-page families-page">
      {!portal && (
        <div className="module-toolbar">
          <form
            className="module-search"
            onSubmit={(event) => {
              event.preventDefault();
              setSearch(query.trim());
              setPage(0);
            }}
          >
            <label>
              <Search aria-hidden="true" />
              <span className="sr-only">Поиск семьи</span>
              <input
                type="search"
                placeholder="Фамилия, телефон или имя ребёнка"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                maxLength={200}
              />
            </label>
            <Button variant="outline" type="submit">
              Найти
            </Button>
          </form>
          <Button
            disabled={!clubId || resource.loading || resource.unavailable}
            onClick={() => setEditing("new")}
          >
            <Plus />
            Добавить семью
          </Button>
        </div>
      )}
      <ResourceState
        resource={resource}
        empty={resource.data?.items.length === 0}
        title={
          search
            ? "Семьи не найдены"
            : portal
              ? "К вашему аккаунту пока не привязана семья"
              : "Добавьте первую семью"
        }
      >
        <p>
          {portal
            ? "Администратор клуба поможет привязать детей и предоставить доступ."
            : search
              ? "Попробуйте другое имя или номер телефона."
              : "Объедините детей и контакты родителей в одной карточке."}
        </p>
      </ResourceState>
      {resource.data && !portal && resource.data.items.length > 0 && (
        <Card className="data-panel">
          <div className="family-table-heading" aria-hidden="true">
            <span>Семья</span>
            <span>Дети</span>
            <span>Контакты</span>
            <span />
          </div>
          <ul className="family-list">
            {resource.data.items.map((item) => (
              <li key={item.id}>
                <div className="person-cell">
                  <Avatar>
                    <AvatarFallback>{initials(item.name)}</AvatarFallback>
                  </Avatar>
                  <div>
                    <Button
                      variant="name"
                      className="family-name"
                      onClick={() => beforeNavigate(() => onDetail(item.id))}
                    >
                      {item.name}
                    </Button>
                    <p>
                      {item.accounts.length
                        ? "Доступ родителей подключён"
                        : "Без аккаунта родителя"}
                    </p>
                  </div>
                </div>
                <div className="family-children-summary">
                  {item.children.length ? (
                    item.children.map((child) => (
                      <span key={child.id}>{child.fullName}</span>
                    ))
                  ) : (
                    <span className="help">Дети не добавлены</span>
                  )}
                </div>
                <div className="family-contacts">
                  <span>{item.phone || "Телефон не указан"}</span>
                  <span>{item.email}</span>
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={`Открыть семью ${item.name}`}
                  onClick={() => beforeNavigate(() => onDetail(item.id))}
                >
                  <ChevronRight />
                </Button>
              </li>
            ))}
          </ul>
        </Card>
      )}
      {portal &&
        resource.data?.items.map((item) => (
          <Card key={item.id} className="portal-family">
            <div className="module-heading">
              <div>
                <h2>{item.name}</h2>
                <p className="help">Ваша семья в клубе PRIDE</p>
              </div>
              <span className="status-badge success">Семейный доступ</span>
            </div>
            <ChildrenList family={item} onOpen={setMember} />
            <div className="contact-line">
              <span>
                <Phone />
                {item.phone || "Телефон не указан"}
              </span>
              {item.email && (
                <span>
                  <Mail />
                  {item.email}
                </span>
              )}
            </div>
          </Card>
        ))}
      {!portal && (
        <Pagination page={page} list={resource.data} onPage={setPage} />
      )}
      {detail && !portal && (
        <Dialog
          title={family?.name ?? "Карточка семьи"}
          className="wide-panel"
          onCancel={closeDetail}
          cancelLabel="Закрыть семью"
        >
          <ResourceState resource={selected} />
          {family && (
            <FamilyDetails
              family={family}
              onEdit={() => setEditing(family)}
              onOpen={setMember}
              refresh={() => {
                selected.refresh();
                resource.refresh();
              }}
              beforeNavigate={beforeNavigate}
            />
          )}
        </Dialog>
      )}
      {editing && (
        <FamilyEditor
          key={editing === "new" ? "new" : `${editing.id}:${editing.version}`}
          base={base}
          family={editing === "new" ? null : editing}
          clubId={clubId}
          beforeNavigate={beforeNavigate}
          onClose={() => setEditing(null)}
          onSaved={(value) => {
            setEditing(null);
            resource.refresh();
            selected.refresh();
            onDetail(value.id);
          }}
        />
      )}
      {member &&
        (!portal ||
          resource.data?.items.some((item) =>
            item.children.some(
              (child) =>
                child.id === member.id && child.clubId === member.clubId,
            ),
          )) && (
          <MemberProfile
            clubId={member.clubId}
            membershipId={member.id}
            canManage={!portal}
            beforeNavigate={beforeNavigate}
            onClose={() => setMember(null)}
          />
        )}
    </div>
  );
}
function ChildrenList({
  family,
  onOpen,
}: {
  family: Family;
  onOpen: (child: MemberSummary) => void;
}) {
  return (
    <div className="children-grid">
      {!family.children.length && (
        <p className="help">Участники пока не добавлены в семью.</p>
      )}
      {family.children.map((child) => (
        <button
          className="child-card"
          key={child.id}
          onClick={() => onOpen(child)}
        >
          <Avatar>
            <AvatarFallback>{initials(child.fullName)}</AvatarFallback>
          </Avatar>
          <div>
            <strong>{child.fullName}</strong>
            <span>{day(child.birthDate)}</span>
            <p>
              {child.groupName || "Без группы"}
              {child.trainerName && ` · ${child.trainerName}`}
            </p>
            <span
              className={`status-badge ${child.archived ? "neutral" : "success"}`}
            >
              {child.archived ? "В архиве" : "Активный участник"}
            </span>
          </div>
          <ChevronRight aria-hidden="true" />
        </button>
      ))}
    </div>
  );
}
function FamilyDetails({
  family,
  onEdit,
  onOpen,
  refresh,
  beforeNavigate,
}: {
  family: Family;
  onEdit: () => void;
  onOpen: (child: MemberSummary) => void;
  refresh: () => void;
  beforeNavigate: Navigate;
}) {
  const [account, setAccount] = useState<
      "new" | Family["accounts"][number] | null
    >(null),
    [revoke, setRevoke] = useState<Family["accounts"][number] | null>(null);
  const command = useCommand(refresh);
  const base =
    `/api/clubs/${family.clubId}/families/${family.id}/accounts` as const;
  return (
    <>
      <div className="module-heading family-profile-heading">
        <div className="person-cell">
          <Avatar>
            <AvatarFallback>{initials(family.name)}</AvatarFallback>
          </Avatar>
          <div>
            <h2>{family.name}</h2>
            <p>{family.contactName || "Контактное лицо не указано"}</p>
          </div>
        </div>
        <Button variant="outline" onClick={onEdit}>
          Редактировать
        </Button>
      </div>
      <div className="contact-line">
        {family.phone && (
          <span>
            <Phone />
            {family.phone}
          </span>
        )}
        {family.email && (
          <span>
            <Mail />
            {family.email}
          </span>
        )}
      </div>
      <Tabs defaultValue="children">
        <TabsList aria-label="Данные семьи">
          <TabsTrigger value="children">
            Дети · {family.children.length}
          </TabsTrigger>
          <TabsTrigger value="access">Доступ родителей</TabsTrigger>
        </TabsList>
        <TabsContent value="children">
          <ChildrenList family={family} onOpen={onOpen} />
        </TabsContent>
        <TabsContent value="access">
          <div className="module-heading">
            <div>
              <h3>Аккаунты родителей</h3>
              <p className="help">Доступ только к детям этой семьи.</p>
            </div>
            <Button onClick={() => setAccount("new")}>
              <UserPlus />
              Создать аккаунт
            </Button>
          </div>
          {!family.accounts.length && (
            <p>
              Аккаунтов пока нет. Создайте логин и передайте пароль родителю
              лично.
            </p>
          )}
          <ul className="access-list">
            {family.accounts.map((user) => (
              <li key={user.id}>
                <div>
                  <strong>{user.displayName}</strong>
                  <span>Логин: {user.login}</span>
                </div>
                <Button variant="outline" onClick={() => setAccount(user)}>
                  Сменить пароль
                </Button>
                <Button
                  variant="ghost"
                  onClick={() => {
                    command.clear();
                    setRevoke(user);
                  }}
                >
                  Отозвать доступ
                </Button>
              </li>
            ))}
          </ul>
        </TabsContent>
      </Tabs>
      {account && (
        <ParentAccount
          family={family}
          account={account}
          base={base}
          beforeNavigate={beforeNavigate}
          onCancel={() => setAccount(null)}
          onSaved={() => {
            setAccount(null);
            refresh();
          }}
        />
      )}
      {revoke && (
        <Dialog
          title="Отозвать доступ к семье?"
          busy={command.busy}
          onCancel={() => setRevoke(null)}
        >
          <p>
            {revoke.displayName} больше не сможет просматривать детей, оплаты и
            документы этой семьи.
          </p>
          {command.error && (
            <p className="error" role="alert">
              {command.error}
            </p>
          )}
          <Button
            disabled={command.busy}
            onClick={() =>
              void command.run(
                () =>
                  apiPost(`${base}/${revoke.id}/revoke`, {
                    version: family.version,
                  }),
                () => setRevoke(null),
              )
            }
          >
            {command.busy ? "Отзываем…" : "Отозвать доступ"}
          </Button>
        </Dialog>
      )}
    </>
  );
}
function ParentAccount({
  family,
  account,
  base,
  onCancel,
  onSaved,
  beforeNavigate,
}: {
  family: Family;
  account: "new" | Family["accounts"][number];
  base: `/api/${string}`;
  onCancel: () => void;
  onSaved: () => void;
  beforeNavigate: Navigate;
}) {
  const [login, setLogin] = useState(""),
    [name, setName] = useState(
      account === "new" ? family.contactName : account.displayName,
    ),
    [password, setPassword] = useState("");
  const command = useCommand(onSaved);
  return (
    <Dialog
      title={account === "new" ? "Аккаунт родителя" : "Новый пароль родителя"}
      busy={command.busy}
      onCancel={() => beforeNavigate(onCancel)}
    >
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void command.run(() =>
            apiPost(
              account === "new" ? base : `${base}/${account.id}/reset-password`,
              account === "new"
                ? {
                    login: login.trim(),
                    displayName: name.trim(),
                    password,
                    version: family.version,
                  }
                : { password, version: family.version },
            ),
          );
        }}
        aria-busy={command.busy}
      >
        {account === "new" && (
          <>
            <label className="field">
              Имя родителя
              <input
                required
                maxLength={200}
                value={name}
                disabled={command.busy}
                onChange={(e) => setName(e.target.value)}
              />
            </label>
            <label className="field">
              Логин
              <input
                required
                maxLength={100}
                pattern="[a-zA-Z0-9][a-zA-Z0-9._@+\-]*"
                autoComplete="off"
                autoCapitalize="none"
                value={login}
                disabled={command.busy}
                onChange={(e) => setLogin(e.target.value)}
              />
              <span className="help">Латинские буквы, цифры и . _ @ + -</span>
            </label>
          </>
        )}
        <label className="field">
          Пароль
          <input
            required
            type="password"
            autoComplete="new-password"
            minLength={12}
            maxLength={128}
            value={password}
            disabled={command.busy}
            onChange={(e) => setPassword(e.target.value)}
          />
          <span className="help">
            Не менее 12 символов. Передайте пароль родителю лично.
          </span>
        </label>
        {command.error && (
          <p className="error" role="alert">
            {command.error}
          </p>
        )}
        <Button type="submit" disabled={command.busy}>
          {command.busy
            ? "Сохраняем…"
            : account === "new"
              ? "Создать аккаунт"
              : "Установить пароль"}
        </Button>
      </form>
    </Dialog>
  );
}
function FamilyEditor({
  base,
  family,
  clubId,
  onClose,
  onSaved,
  beforeNavigate,
}: {
  base: `/api/${string}`;
  family: Family | null;
  clubId: string;
  onClose: () => void;
  onSaved: (value: Family) => void;
  beforeNavigate: Navigate;
}) {
  const [name, setName] = useState(family?.name ?? ""),
    [contactName, setContactName] = useState(family?.contactName ?? ""),
    [phone, setPhone] = useState(family?.phone ?? ""),
    [email, setEmail] = useState(family?.email ?? "");
  const [query, setQuery] = useState(""),
    [ids, setIds] = useState(family?.children.map((child) => child.id) ?? []);
  const deferredQuery = useDeferredValue(query);
  const people = useResource<{ items: MemberSummary[]; hasMore: boolean }>(
    `/api/clubs/${clubId}/participants?q=${encodeURIComponent(deferredQuery)}&state=active`,
  );
  const command = useCommand();
  const choices = [
    ...new Map(
      [...(family?.children ?? []), ...(people.data?.items ?? [])].map(
        (person) => [person.id, person],
      ),
    ).values(),
  ];
  function submit(event: FormEvent) {
    event.preventDefault();
    const body = {
      name: name.trim(),
      contactName: contactName.trim(),
      phone: phone.trim(),
      email: email.trim(),
      membershipIds: ids,
      version: family?.version ?? 0,
    };
    void command.run(
      () =>
        family
          ? apiPut<Family>(`${base}/${family.id}`, body)
          : apiPost<Family>(base, body),
      onSaved,
    );
  }
  return (
    <Dialog
      title={family ? "Редактировать семью" : "Новая семья"}
      busy={command.busy}
      onCancel={() => beforeNavigate(onClose)}
    >
      <form onSubmit={submit} aria-busy={command.busy}>
        <label className="field">
          Название семьи
          <input
            required
            maxLength={200}
            value={name}
            disabled={command.busy}
            onChange={(e) => setName(e.target.value)}
            placeholder="Например, семья Ивановых"
          />
        </label>
        <label className="field">
          Контактное лицо
          <input
            maxLength={200}
            value={contactName}
            disabled={command.busy}
            onChange={(e) => setContactName(e.target.value)}
          />
        </label>
        <div className="form-columns">
          <label className="field">
            Телефон
            <input
              type="tel"
              maxLength={80}
              value={phone}
              disabled={command.busy}
              onChange={(e) => setPhone(e.target.value)}
            />
          </label>
          <label className="field">
            Электронная почта
            <input
              type="email"
              maxLength={254}
              value={email}
              disabled={command.busy}
              onChange={(e) => setEmail(e.target.value)}
            />
          </label>
        </div>
        <fieldset className="member-picker">
          <legend>
            <Users />
            Участники семьи · {ids.length}
          </legend>
          <label className="field">
            Найти участника
            <input
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              maxLength={200}
              disabled={command.busy}
            />
          </label>
          <ResourceState
            resource={people}
            empty={choices.length === 0}
            title="Участников не найдено"
          >
            <p>Сначала добавьте ребёнка в реестр участников.</p>
          </ResourceState>
          <div className="member-picker-list">
            {choices.map((person) => (
              <label className="check" key={person.id}>
                <input
                  type="checkbox"
                  disabled={command.busy}
                  checked={ids.includes(person.id)}
                  onChange={(e) =>
                    setIds(
                      e.target.checked
                        ? [...ids, person.id]
                        : ids.filter((id) => id !== person.id),
                    )
                  }
                />
                <span>
                  {person.fullName}
                  <small>{day(person.birthDate)}</small>
                </span>
              </label>
            ))}
          </div>
          {people.data?.hasMore && (
            <p className="help">
              Показаны первые результаты. Уточните поиск по имени.
            </p>
          )}
        </fieldset>
        {command.error && (
          <p className="error" role="alert">
            {command.error}
          </p>
        )}
        <Button type="submit" disabled={command.busy || people.loading}>
          {command.busy ? "Сохраняем…" : "Сохранить семью"}
        </Button>
      </form>
    </Dialog>
  );
}

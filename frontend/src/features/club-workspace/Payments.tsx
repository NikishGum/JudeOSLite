import { useState, type FormEvent } from "react";
import {
  Banknote,
  CreditCard,
  Plus,
  Search,
  Ticket,
  Wallet,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Dialog } from "../../shared/ui/Dialog";
import { apiPost, apiPut } from "../../shared/api";
import { nextDate } from "../sessions/types";
import {
  Pagination,
  ResourceState,
  useCommand,
  useResource,
} from "./api-state";
import {
  amountMinor,
  day,
  methodNames,
  money,
  paymentNames,
  subscriptionNames,
  today,
  type BillingSummary,
  type List,
  type MemberSummary,
  type Method,
  type Payment,
  type Plan,
  type Subscription,
} from "./types";

type Navigate = (action: () => void) => void;
export function Payments({
  clubId,
  portal = false,
  beforeNavigate,
}: {
  clubId: string;
  portal?: boolean;
  beforeNavigate: Navigate;
}) {
  const [tab, setTab] = useState("payments"),
    [status, setStatus] = useState("all"),
    [query, setQuery] = useState(""),
    [search, setSearch] = useState(""),
    [page, setPage] = useState(0),
    [create, setCreate] = useState(false);
  const [dates, setDates] = useState({
    from: today().slice(0, 7) + "-01",
    until: today(),
  });
  const base = `/api/clubs/${clubId}/payments` as const;
  const params = new URLSearchParams({
    q: search,
    page: String(page),
    limit: "25",
    status,
    ...dates,
  });
  const resource = useResource<List<Payment>>(
    portal
      ? `/api/portal/payments?${params}`
      : clubId
        ? `${base}?${params}`
        : null,
  );
  return (
    <div className="module-page payments-page">
      {!portal && clubId && <FinanceSummary clubId={clubId} dates={dates} />}
      <Tabs value={tab} onValueChange={setTab}>
        <div className="module-toolbar">
          <TabsList aria-label="Раздел платежей">
            <TabsTrigger value="payments">Платежи</TabsTrigger>
            <TabsTrigger value="subscriptions">Абонементы</TabsTrigger>
            {!portal && <TabsTrigger value="plans">Тарифы</TabsTrigger>}
          </TabsList>
          {!portal && (
            <Button
              disabled={!clubId || resource.loading || resource.unavailable}
              onClick={() => setCreate(true)}
            >
              <Plus />
              Добавить платёж
            </Button>
          )}
        </div>
        <TabsContent value="payments">
          <Card className="data-panel">
            <div className="payment-filters">
              <form
                className="module-search"
                onSubmit={(e) => {
                  e.preventDefault();
                  setSearch(query.trim());
                  setPage(0);
                }}
              >
                <label>
                  <Search aria-hidden="true" />
                  <span className="sr-only">Поиск платежа</span>
                  <input
                    type="search"
                    placeholder="Участник или назначение платежа"
                    maxLength={200}
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                  />
                </label>
                <Button variant="outline" type="submit">
                  Найти
                </Button>
              </form>
              <label className="field">
                Статус
                <select
                  value={status}
                  onChange={(e) => {
                    setStatus(e.target.value);
                    setPage(0);
                  }}
                >
                  <option value="all">Все платежи</option>
                  {Object.entries(paymentNames).map(([value, name]) => (
                    <option key={value} value={value}>
                      {name}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <div className="date-filter-row">
              <label className="field">
                С даты
                <input
                  type="date"
                  value={dates.from}
                  onChange={(e) => {
                    setDates({ ...dates, from: e.target.value });
                    setPage(0);
                  }}
                />
              </label>
              <label className="field">
                По дату
                <input
                  type="date"
                  min={dates.from}
                  value={dates.until}
                  onChange={(e) => {
                    setDates({ ...dates, until: e.target.value });
                    setPage(0);
                  }}
                />
              </label>
            </div>
            <ResourceState
              resource={resource}
              empty={resource.data?.items.length === 0}
              title="Платежей за этот период нет"
            >
              <p>Выберите другой период или добавьте первый платёж.</p>
            </ResourceState>
            {resource.data && (
              <PaymentRows
                items={resource.data.items}
                canManage={!portal}
                beforeNavigate={beforeNavigate}
                onChanged={resource.refresh}
              />
            )}
            <Pagination page={page} list={resource.data} onPage={setPage} />
          </Card>
        </TabsContent>
        <TabsContent value="subscriptions">
          <Subscriptions clubId={clubId} portal={portal} />
        </TabsContent>
        {!portal && (
          <TabsContent value="plans">
            <Plans clubId={clubId} beforeNavigate={beforeNavigate} />
          </TabsContent>
        )}
      </Tabs>
      {portal && <OnlinePaymentNotice />}
      {create && (
        <PaymentEditor
          clubId={clubId}
          beforeNavigate={beforeNavigate}
          onClose={() => setCreate(false)}
          onSaved={() => {
            setCreate(false);
            resource.refresh();
          }}
        />
      )}
    </div>
  );
}
export function FinanceSummary({
  clubId,
  dates = { from: today().slice(0, 7) + "-01", until: today() },
  chart = false,
  showUnavailable = false,
}: {
  clubId: string;
  dates?: { from: string; until: string };
  chart?: boolean;
  showUnavailable?: boolean;
}) {
  const resource = useResource<BillingSummary>(
    clubId
      ? `/api/clubs/${clubId}/billing/summary?${new URLSearchParams(dates)}`
      : null,
  );
  if (!resource.data && (!chart || (resource.unavailable && !showUnavailable)))
    return null;
  const summary = resource.data;
  if (chart)
    return (
      <Card className="finance-chart">
        <div className="module-heading">
          <div>
            <h2>Выручка за период</h2>
            <p className="help">Подтверждённые оплаты · BYN</p>
          </div>
          <Wallet aria-hidden="true" />
        </div>
        <ResourceState resource={resource} />
        {summary && (
          <>
            <strong className="finance-total">
              {money(summary.paidMinor)}
            </strong>
            {summary.byMonth.length ? (
              <div
                className="revenue-bars"
                role="img"
                aria-label={summary.byMonth
                  .map((month) => `${month.month}: ${money(month.paidMinor)}`)
                  .join("; ")}
              >
                {summary.byMonth.map((month) => (
                  <div key={month.month}>
                    <span
                      style={{
                        height: `${Math.max(2, (month.paidMinor / Math.max(1, ...summary.byMonth.map((item) => item.paidMinor))) * 100)}%`,
                      }}
                      title={money(month.paidMinor)}
                    />
                    <small>{month.month.slice(5)}</small>
                  </div>
                ))}
              </div>
            ) : (
              <p className="help">Оплат за выбранный период пока нет.</p>
            )}
          </>
        )}
      </Card>
    );
  return (
    <dl className="billing-metrics">
      <div>
        <dt>
          <Wallet />
          Получено за период
        </dt>
        <dd>{money(summary!.paidMinor)}</dd>
        <span>{summary!.paidCount} платежей</span>
      </div>
      <div>
        <dt>
          <CreditCard />
          Ожидает оплаты
        </dt>
        <dd>{money(summary!.pendingMinor)}</dd>
        <span>{summary!.pendingCount} счетов</span>
      </div>
    </dl>
  );
}
export function OnlinePaymentNotice() {
  return (
    <div className="online-payment-notice">
      <CreditCard aria-hidden="true" />
      <div>
        <strong>Онлайн-оплата через iPay</strong>
        <p id="ipay-availability">
          Скоро появится возможность оплатить счёт онлайн. Пока уточните способ
          оплаты у администратора.
        </p>
      </div>
      <Button variant="outline" disabled aria-describedby="ipay-availability">
        Оплатить онлайн
      </Button>
    </div>
  );
}
function PaymentRows({
  items,
  canManage,
  onChanged,
  beforeNavigate,
}: {
  items: Payment[];
  canManage: boolean;
  onChanged: () => void;
  beforeNavigate: Navigate;
}) {
  const [selected, setSelected] = useState<Payment | null>(null);
  return (
    <>
      {items.length > 0 && (
        <div
          className="payment-table"
          role="region"
          aria-label="Список платежей"
        >
          <table>
            <thead>
              <tr>
                <th>Дата</th>
                <th>Участник</th>
                <th>Сумма</th>
                <th>Способ</th>
                <th>Статус</th>
                <th>
                  <span className="sr-only">Действия</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {items.map((payment) => (
                <tr key={payment.id}>
                  <td data-label="Дата">
                    {day(payment.paidOn ?? payment.createdAt.slice(0, 10))}
                  </td>
                  <td>
                    <strong>{payment.fullName}</strong>
                    <span className="table-description">
                      {payment.description}
                    </span>
                  </td>
                  <td data-label="Сумма" className="money-value">
                    {money(payment.amountMinor)}
                  </td>
                  <td data-label="Способ">
                    {payment.method ? methodNames[payment.method] : "—"}
                  </td>
                  <td>
                    <span
                      className={`status-badge ${payment.status === "paid" ? "success" : payment.status === "pending" ? "warning" : "neutral"}`}
                    >
                      {paymentNames[payment.status]}
                    </span>
                  </td>
                  <td>
                    {canManage && payment.status !== "void" && (
                      <Button
                        variant="ghost"
                        onClick={() => setSelected(payment)}
                        aria-label={`Действия с платежом ${payment.fullName} ${money(payment.amountMinor)}`}
                      >
                        Подробнее
                      </Button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {selected && (
        <PaymentAction
          payment={selected}
          beforeNavigate={beforeNavigate}
          onClose={() => setSelected(null)}
          onSaved={() => {
            setSelected(null);
            onChanged();
          }}
        />
      )}
    </>
  );
}
function PaymentAction({
  payment,
  onClose,
  onSaved,
  beforeNavigate,
}: {
  payment: Payment;
  onClose: () => void;
  onSaved: () => void;
  beforeNavigate: Navigate;
}) {
  const [action, setAction] = useState<"confirm" | "void">(
      payment.status === "pending" ? "confirm" : "void",
    ),
    [method, setMethod] = useState<Method>("cash"),
    [paidOn, setPaidOn] = useState(today()),
    [reason, setReason] = useState("");
  const command = useCommand(onSaved);
  const base = `/api/clubs/${payment.clubId}/payments/${payment.id}` as const;
  return (
    <Dialog
      title="Платёж"
      busy={command.busy}
      onCancel={() => beforeNavigate(onClose)}
    >
      <div className="payment-receipt">
        <Banknote aria-hidden="true" />
        <strong>{money(payment.amountMinor)}</strong>
        <h3>{payment.fullName}</h3>
        <p>{payment.description}</p>
        <span
          className={`status-badge ${payment.status === "paid" ? "success" : "warning"}`}
        >
          {paymentNames[payment.status]}
        </span>
      </div>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void command.run(() =>
            apiPost(
              `${base}/${action}`,
              action === "confirm"
                ? { version: payment.version, method, paidOn }
                : { version: payment.version, reason: reason.trim() },
            ),
          );
        }}
      >
        {payment.status === "pending" && (
          <label className="field">
            Действие
            <select
              value={action}
              disabled={command.busy}
              onChange={(e) => setAction(e.target.value as "confirm" | "void")}
            >
              <option value="confirm">Подтвердить получение оплаты</option>
              <option value="void">Аннулировать счёт</option>
            </select>
          </label>
        )}
        {action === "confirm" ? (
          <>
            <label className="field">
              Способ оплаты
              <select
                value={method}
                onChange={(e) => setMethod(e.target.value as Method)}
                disabled={command.busy}
              >
                {Object.entries(methodNames).map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              Дата оплаты
              <input
                type="date"
                required
                max={today()}
                value={paidOn}
                onChange={(e) => setPaidOn(e.target.value)}
                disabled={command.busy}
              />
            </label>
            <p className="help">
              Подтвердите только фактически полученную оплату.
            </p>
          </>
        ) : (
          <label className="field">
            Причина аннулирования
            <textarea
              required
              maxLength={500}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              disabled={command.busy}
            />
            <span className="help">
              Аннулирование записи не возвращает деньги плательщику.
            </span>
          </label>
        )}
        {command.error && (
          <p className="error" role="alert">
            {command.error}
          </p>
        )}
        <Button type="submit" disabled={command.busy}>
          {command.busy
            ? "Сохраняем…"
            : action === "confirm"
              ? "Подтвердить оплату"
              : "Аннулировать запись"}
        </Button>
      </form>
    </Dialog>
  );
}
function Subscriptions({
  clubId,
  portal = false,
  membershipId = "",
}: {
  clubId: string;
  portal?: boolean;
  membershipId?: string;
}) {
  const params = new URLSearchParams({ membershipId, limit: "100" });
  const resource = useResource<List<Subscription>>(
    portal
      ? `/api/portal/subscriptions?${params}`
      : clubId
        ? `/api/clubs/${clubId}/subscriptions?${params}`
        : null,
  );
  return (
    <Card>
      <div className="module-heading">
        <h2>Абонементы</h2>
        <Ticket aria-hidden="true" />
      </div>
      <ResourceState
        resource={resource}
        empty={resource.data?.items.length === 0}
        title="Абонементов пока нет"
      >
        <p>Абонемент появится после выбора тарифа при создании платежа.</p>
      </ResourceState>
      <ul className="subscription-list">
        {resource.data?.items.map((item) => (
          <li key={item.id}>
            <div>
              <strong>{item.planName}</strong>
              <span>
                {day(item.validFrom)} — {day(nextDate(item.validUntil, -1))}
              </span>
              <span
                className={`status-badge ${item.status === "active" ? "success" : item.status === "pending" ? "warning" : "neutral"}`}
              >
                {subscriptionNames[item.status]}
              </span>
            </div>
            <div className="subscription-progress">
              <strong>
                {Math.max(0, item.visits - item.used)}{" "}
                <span>из {item.visits}</span>
              </strong>
              <progress
                value={Math.min(item.used, item.visits)}
                max={item.visits}
                aria-label={`Использовано занятий: ${item.used} из ${item.visits}`}
              />
              <p>Осталось занятий</p>
            </div>
          </li>
        ))}
      </ul>
    </Card>
  );
}
export function MemberPayments({
  clubId,
  membershipId,
  canManage,
  beforeNavigate,
}: {
  clubId: string;
  membershipId: string;
  canManage: boolean;
  beforeNavigate: Navigate;
}) {
  const [create, setCreate] = useState(false);
  const path = canManage
    ? (`/api/clubs/${clubId}/payments?membershipId=${membershipId}` as const)
    : (`/api/portal/payments?membershipId=${membershipId}` as const);
  const resource = useResource<List<Payment>>(path);
  return (
    <>
      <Subscriptions
        clubId={clubId}
        membershipId={membershipId}
        portal={!canManage}
      />
      <div className="module-heading profile-section">
        <h3>История платежей</h3>
        {canManage && (
          <Button
            disabled={resource.loading || resource.unavailable}
            onClick={() => setCreate(true)}
          >
            <Plus />
            Добавить платёж
          </Button>
        )}
      </div>
      <ResourceState
        resource={resource}
        empty={resource.data?.items.length === 0}
        title="Платежей пока нет"
      />
      {resource.data && (
        <PaymentRows
          items={resource.data.items}
          canManage={canManage}
          onChanged={resource.refresh}
          beforeNavigate={beforeNavigate}
        />
      )}
      {!canManage && <OnlinePaymentNotice />}
      {create && (
        <PaymentEditor
          clubId={clubId}
          membershipId={membershipId}
          beforeNavigate={beforeNavigate}
          onClose={() => setCreate(false)}
          onSaved={() => {
            setCreate(false);
            resource.refresh();
          }}
        />
      )}
    </>
  );
}
function Plans({
  clubId,
  beforeNavigate,
}: {
  clubId: string;
  beforeNavigate: Navigate;
}) {
  const resource = useResource<List<Plan>>(
    clubId ? `/api/clubs/${clubId}/billing/plans?limit=100` : null,
  );
  const [editing, setEditing] = useState<Plan | "new" | null>(null);
  return (
    <Card>
      <div className="module-heading">
        <div>
          <h2>Тарифы клуба</h2>
          <p className="help">
            Количество занятий, срок и стоимость абонемента.
          </p>
        </div>
        <Button
          disabled={resource.unavailable || !clubId}
          onClick={() => setEditing("new")}
        >
          <Plus />
          Создать тариф
        </Button>
      </div>
      <ResourceState
        resource={resource}
        empty={resource.data?.items.length === 0}
        title="Создайте первый тариф"
      >
        <p>После этого его можно будет выбрать при оформлении абонемента.</p>
      </ResourceState>
      <ul className="plan-list">
        {resource.data?.items.map((plan) => (
          <li key={plan.id}>
            <Ticket aria-hidden="true" />
            <div>
              <strong>{plan.name}</strong>
              <span>
                {plan.visits} занятий · {plan.days} дней
                {plan.archived ? " · В архиве" : ""}
              </span>
            </div>
            <strong>{money(plan.amountMinor)}</strong>
            <Button variant="outline" onClick={() => setEditing(plan)}>
              Изменить
            </Button>
          </li>
        ))}
      </ul>
      {editing && (
        <PlanEditor
          key={editing === "new" ? "new" : editing.id}
          clubId={clubId}
          plan={editing === "new" ? null : editing}
          beforeNavigate={beforeNavigate}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            resource.refresh();
          }}
        />
      )}
    </Card>
  );
}
function PlanEditor({
  clubId,
  plan,
  onClose,
  onSaved,
  beforeNavigate,
}: {
  clubId: string;
  plan: Plan | null;
  onClose: () => void;
  onSaved: () => void;
  beforeNavigate: Navigate;
}) {
  const [name, setName] = useState(plan?.name ?? ""),
    [amount, setAmount] = useState(
      plan ? (plan.amountMinor / 100).toFixed(2) : "",
    ),
    [visits, setVisits] = useState(plan?.visits ?? 12),
    [days, setDays] = useState(plan?.days ?? 30),
    [archived, setArchived] = useState(plan?.archived ?? false),
    [validation, setValidation] = useState("");
  const command = useCommand(onSaved);
  return (
    <Dialog
      title={plan ? "Изменить тариф" : "Новый тариф"}
      busy={command.busy}
      onCancel={() => beforeNavigate(onClose)}
    >
      <form
        onSubmit={(e) => {
          e.preventDefault();
          const value = amountMinor(amount);
          if (value === null) {
            setValidation(
              "Укажите сумму больше нуля, не более двух знаков после запятой.",
            );
            return;
          }
          setValidation("");
          const body = {
            name: name.trim(),
            amountMinor: value,
            visits,
            days,
            version: plan?.version ?? 0,
            ...(plan ? { archived } : {}),
          };
          const base = `/api/clubs/${clubId}/billing/plans` as const;
          void command.run(() =>
            plan ? apiPut(`${base}/${plan.id}`, body) : apiPost(base, body),
          );
        }}
      >
        <label className="field">
          Название
          <input
            required
            maxLength={200}
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={command.busy}
          />
        </label>
        <label className="field">
          Стоимость, BYN
          <input
            required
            inputMode="decimal"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            disabled={command.busy}
          />
        </label>
        <div className="form-columns">
          <label className="field">
            Занятий
            <input
              type="number"
              required
              min={1}
              max={366}
              value={visits}
              onChange={(e) => setVisits(Number(e.target.value))}
              disabled={command.busy}
            />
          </label>
          <label className="field">
            Срок, дней
            <input
              type="number"
              required
              min={1}
              max={366}
              value={days}
              onChange={(e) => setDays(Number(e.target.value))}
              disabled={command.busy}
            />
          </label>
        </div>
        {plan && (
          <label className="check">
            <input
              type="checkbox"
              checked={archived}
              onChange={(e) => setArchived(e.target.checked)}
              disabled={command.busy}
            />
            Убрать тариф в архив
          </label>
        )}
        {(validation || command.error) && (
          <p className="error" role="alert">
            {validation || command.error}
          </p>
        )}
        <Button type="submit" disabled={command.busy}>
          {command.busy ? "Сохраняем…" : "Сохранить тариф"}
        </Button>
      </form>
    </Dialog>
  );
}
function PaymentEditor({
  clubId,
  membershipId = "",
  onClose,
  onSaved,
  beforeNavigate,
}: {
  clubId: string;
  membershipId?: string;
  onClose: () => void;
  onSaved: () => void;
  beforeNavigate: Navigate;
}) {
  const [requestId] = useState(() => crypto.randomUUID());
  const [member, setMember] = useState(membershipId),
    [query, setQuery] = useState(""),
    [planId, setPlanId] = useState(""),
    [amount, setAmount] = useState(""),
    [description, setDescription] = useState(""),
    [mode, setMode] = useState<"paid" | "pending">("paid"),
    [method, setMethod] = useState<Method>("cash"),
    [paidOn, setPaidOn] = useState(today()),
    [from, setFrom] = useState(today()),
    [validation, setValidation] = useState("");
  const people = useResource<{ items: MemberSummary[]; hasMore: boolean }>(
    `/api/clubs/${clubId}/participants?q=${encodeURIComponent(query)}&state=active`,
  );
  const plans = useResource<List<Plan>>(
    `/api/clubs/${clubId}/billing/plans?limit=100`,
  );
  const command = useCommand(onSaved);
  const selectedPlan = plans.data?.items.find((item) => item.id === planId);
  function choosePlan(id: string) {
    setPlanId(id);
    const plan = plans.data?.items.find((item) => item.id === id);
    if (plan) {
      setAmount((plan.amountMinor / 100).toFixed(2));
      setDescription(plan.name);
    }
  }
  function submit(e: FormEvent) {
    e.preventDefault();
    const value = amountMinor(amount);
    if (value === null) {
      setValidation(
        "Проверьте сумму: нужны рубли и не более двух знаков копеек.",
      );
      return;
    }
    setValidation("");
    void command.run(() =>
      apiPost(`/api/clubs/${clubId}/payments`, {
        requestId,
        membershipId: member,
        description: description.trim(),
        amountMinor: value,
        method: mode === "paid" ? method : null,
        paidOn: mode === "paid" ? paidOn : null,
        planId: planId || null,
        validFrom: planId ? from : null,
      }),
    );
  }
  return (
    <Dialog
      title="Новый платёж / абонемент"
      className="payment-editor"
      busy={command.busy}
      onCancel={() => beforeNavigate(onClose)}
    >
      <form onSubmit={submit} aria-busy={command.busy}>
        <fieldset disabled={command.busy} className="plain-fieldset">
          <legend className="sr-only">Данные платежа</legend>
          {!membershipId && (
            <label className="field">
              Найти участника
              <input
                type="search"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                maxLength={200}
                placeholder="Фамилия или контакт"
              />
            </label>
          )}
          <label className="field">
            Участник
            <select
              required
              value={member}
              onChange={(e) => setMember(e.target.value)}
              disabled={!!membershipId || people.loading}
            >
              <option value="">Выберите участника</option>
              {people.data?.items.map((person) => (
                <option key={person.id} value={person.id}>
                  {person.fullName}
                </option>
              ))}
              {member &&
                !people.data?.items.some((person) => person.id === member) && (
                  <option value={member}>Участник из карточки</option>
                )}
            </select>
          </label>
          <ResourceState resource={people} />
          <label className="field">
            Тип оплаты
            <select
              value={mode}
              onChange={(e) => setMode(e.target.value as "paid" | "pending")}
            >
              <option value="paid">Зарегистрировать полученную оплату</option>
              <option value="pending">Выставить счёт к оплате</option>
            </select>
          </label>
          <label className="field">
            Тариф / абонемент
            <select
              value={planId}
              onChange={(e) => choosePlan(e.target.value)}
              disabled={plans.loading || plans.unavailable}
            >
              <option value="">Разовая оплата без абонемента</option>
              {plans.data?.items
                .filter((plan) => !plan.archived)
                .map((plan) => (
                  <option key={plan.id} value={plan.id}>
                    {plan.name} · {money(plan.amountMinor)}
                  </option>
                ))}
            </select>
          </label>
          {plans.error && (
            <p className="error" role="alert">
              {plans.error}
            </p>
          )}
          {selectedPlan && (
            <div className="plan-selection">
              <Ticket />
              <strong>{selectedPlan.visits} занятий</strong>
              <span>{selectedPlan.days} дней</span>
            </div>
          )}
          <label className="field">
            Назначение платежа
            <input
              required
              maxLength={300}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </label>
          <label className="field">
            Сумма, BYN
            <input
              required
              inputMode="decimal"
              readOnly={!!planId}
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
            />
          </label>
          {planId && (
            <label className="field">
              Абонемент действует с
              <input
                type="date"
                required
                value={from}
                onChange={(e) => setFrom(e.target.value)}
              />
            </label>
          )}
          {mode === "paid" && (
            <>
              <label className="field">
                Способ оплаты
                <select
                  value={method}
                  onChange={(e) => setMethod(e.target.value as Method)}
                >
                  {Object.entries(methodNames).map(([value, label]) => (
                    <option key={value} value={value}>
                      {label}
                    </option>
                  ))}
                </select>
              </label>
              <label className="field">
                Дата получения
                <input
                  type="date"
                  required
                  max={today()}
                  value={paidOn}
                  onChange={(e) => setPaidOn(e.target.value)}
                />
              </label>
            </>
          )}
          {mode === "pending" && (
            <p className="help">
              Счёт будет ожидать оплаты. Онлайн-оплата iPay появится после
              подключения клубом.
            </p>
          )}
          {(validation || command.error) && (
            <p className="error" role="alert">
              {validation || command.error}
            </p>
          )}
          <Button
            type="submit"
            disabled={
              command.busy || !member || people.loading || people.unavailable
            }
          >
            {command.busy
              ? "Сохраняем…"
              : mode === "paid"
                ? "Зарегистрировать оплату"
                : "Выставить счёт"}
          </Button>
        </fieldset>
      </form>
    </Dialog>
  );
}

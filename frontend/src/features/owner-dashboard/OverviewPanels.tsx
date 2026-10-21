import { Fragment } from "react";
import type { Report, Row, Totals } from "./types";
import { percentage } from "./types";

export const number = (value: number) => value.toLocaleString("ru-RU");
export const plural = (n: number, forms: [string, string, string]) =>
  forms[
    n % 100 >= 11 && n % 100 <= 19
      ? 2
      : n % 10 === 1
        ? 0
        : n % 10 >= 2 && n % 10 <= 4
          ? 1
          : 2
  ];
export const sessions = (n: number) =>
  number(n) + " " + plural(n, ["занятие", "занятия", "занятий"]);
export const low = (row: Totals) =>
  row.percentage !== null && row.percentage < 80;
export const clubLabel = (row: Row) =>
  row.clubName + (row.clubArchived ? " · в архиве" : "");
export function shortId(id: string, ids: string[]) {
  const suffix = id.slice(-8);
  return ids.some((other) => other !== id && other.endsWith(suffix))
    ? id
    : suffix;
}
export function groupLabel(row: Row, groups: Row[]) {
  const duplicate = groups.some(
    (other) =>
      other.groupId !== row.groupId &&
      other.clubId === row.clubId &&
      other.groupName === row.groupName &&
      other.groupArchived === row.groupArchived,
  );
  return (
    (row.groupName ?? "Группа") +
    (row.groupArchived ? " · в архиве" : "") +
    (duplicate
      ? " · " +
        shortId(
          row.groupId ?? "",
          groups.map((group) => group.groupId ?? ""),
        )
      : "")
  );
}
export function Arrow() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path d="m9 6 6 6-6 6" />
    </svg>
  );
}
export function Info() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <circle cx="12" cy="12" r="9" />
      <path d="M12 11v5M12 7v.1" />
    </svg>
  );
}
export function Reasons({ total }: { total: Totals }) {
  const fields = [
    ["illness", "Болезнь"],
    ["excused", "Уважительная"],
    ["unknown", "Неизвестна"],
  ] as const;
  return (
    <section className="panel reason-panel" aria-labelledby="reasons-title">
      <div className="section-head">
        <div>
          <h2 id="reasons-title">Почему пропускают</h2>
          <p>{number(total.absent)} пропусков в выбранном контексте</p>
        </div>
      </div>
      <div className="reason-bar" aria-hidden="true">
        {fields.map(([key]) => (
          <span
            key={key}
            style={{
              width:
                (total.absent ? (total[key] / total.absent) * 100 : 0) + "%",
            }}
          />
        ))}
      </div>
      <dl className="reason-list">
        {fields.map(([key, label]) => (
          <div key={key}>
            <dt>{label}</dt>
            <dd>
              {number(total[key])}
              <span>
                {total.absent
                  ? Math.round((total[key] / total.absent) * 100) + "%"
                  : "—"}
              </span>
            </dd>
          </div>
        ))}
      </dl>
      <p>
        {total.unknown
          ? "«Неизвестна» также включает неотмеченных при закрытии журнала."
          : total.absent
            ? "Причина указана у всех пропусков."
            : "В выбранном контексте пропусков нет."}
      </p>
    </section>
  );
}
export function Findings({
  report,
  onProblem,
}: {
  report: Report;
  onProblem: (kind: "open" | "low") => void;
}) {
  const below = report.clubs
    .filter(low)
    .sort((a, b) => a.percentage! - b.percentage!);
  const opened = report.clubs
    .filter((row) => row.open)
    .sort((a, b) => b.open - a.open);
  return (
    <section className="panel findings-panel" aria-labelledby="findings-title">
      <div className="section-head">
        <h2 id="findings-title">На что обратить внимание</h2>
      </div>
      {opened.length > 0 && (
        <div className="finding">
          <div className="finding-head">
            <span>Неполные данные</span>
            <span className="badge">
              {report.total.open}{" "}
              {plural(report.total.open, ["журнал", "журнала", "журналов"])}
            </span>
          </div>
          <p>
            {opened.map((row) => clubLabel(row) + " — " + row.open).join("; ")}.
            Эти занятия не входят в процент.
          </p>
          <button type="button" onClick={() => onProblem("open")}>
            Показать эти филиалы <Arrow />
          </button>
        </div>
      )}
      {below.length > 0 && (
        <div className="finding">
          <div className="finding-head">
            <span>Посещаемость ниже 80%</span>
            <span className="badge">
              {below.length} из{" "}
              {report.clubs.filter((row) => row.percentage !== null).length}
            </span>
          </div>
          <p>
            {below
              .map((row) => clubLabel(row) + " — " + percentage(row.percentage))
              .join("; ")}
            .
          </p>
          <button type="button" onClick={() => onProblem("low")}>
            Сравнить эти филиалы <Arrow />
          </button>
          <p className="help">
            80% — ориентир этого макета, не норматив клуба.
          </p>
        </div>
      )}
      {report.total.percentage === null ? (
        <div className="finding">
          <div className="finding-head">Пока нечего оценивать</div>
          <p>
            В этом контексте ещё нет отметок закрытых занятий. Процент появится
            после заполнения и закрытия журналов.
          </p>
        </div>
      ) : (
        !opened.length &&
        !below.length && (
          <div className="finding">
            <div className="finding-head">
              Отклонений по выбранным признакам нет
            </div>
            <p>
              Открытых журналов нет, посещаемость филиалов с отметками не ниже
              80%. Это ориентир сравнения из макета, не норматив клуба.
            </p>
          </div>
        )
      )}
    </section>
  );
}
export function BranchDetail({
  row,
  report,
  onGroup,
  onOpen,
}: {
  row: Row;
  report: Report;
  onGroup: (row: Row) => void;
  onOpen: () => void;
}) {
  return (
    <div className="detail-block">
      <h3>Группы филиала</h3>
      <div className="detail-facts">
        <span>
          Присутствия <strong>{number(row.present)}</strong>
        </span>
        <span>
          Пропуски <strong>{number(row.absent)}</strong>
        </span>
        <span>
          Запланировано <strong>{row.planned}</strong>
        </span>
        <span>
          Открыто <strong>{row.open}</strong>
        </span>
      </div>
      {report.groups
        .filter((group) => group.clubId === row.clubId)
        .map((group) => (
          <div
            className="group-line"
            key={group.groupId}
            data-group-id={group.groupId}
          >
            <button
              className="text-button"
              type="button"
              onClick={() => onGroup(group)}
            >
              {groupLabel(group, report.groups)} <Arrow />
            </button>
            <span className="rate">{percentage(group.percentage)}</span>
            <span className="muted">{sessions(group.closed)}</span>
          </div>
        ))}
      <p className="detail-reasons">
        Пропуски: болезнь — {row.illness}; уважительная — {row.excused};
        неизвестна — {row.unknown}.
      </p>
      {row.open > 0 && (
        <button className="text-button" type="button" onClick={onOpen}>
          Открытые журналы филиала: {row.open} <Arrow />
        </button>
      )}
    </div>
  );
}
export function Ranking({
  report,
  expanded,
  onExpand,
  onGroup,
  onOpen,
  order,
  onOrder,
}: {
  report: Report;
  expanded: Set<string>;
  onExpand: (id: string) => void;
  onGroup: (row: Row) => void;
  onOpen: (row: Row) => void;
  order: string;
  onOrder: (value: string) => void;
}) {
  const sorted = [...report.clubs].sort(
    (a, b) =>
      (order === "open"
        ? b.open - a.open
        : a.percentage === null
          ? b.percentage === null
            ? 0
            : 1
          : b.percentage === null
            ? -1
            : a.percentage - b.percentage) ||
      a.clubName.localeCompare(b.clubName, "ru") ||
      a.clubId.localeCompare(b.clubId),
  );
  return (
    <section className="panel ranking-panel" aria-labelledby="ranking-title">
      <div className="section-head">
        <div>
          <h2 id="ranking-title">Посещаемость по филиалам</h2>
          <p>Раскройте строку, чтобы увидеть группы</p>
        </div>
        <label>
          <span className="visually-hidden">Порядок филиалов</span>
          <select
            aria-label="Порядок филиалов"
            value={order}
            onChange={(e) => onOrder(e.target.value)}
          >
            <option value="rate">Сначала ниже</option>
            <option value="open">Сначала открытые</option>
          </select>
        </label>
      </div>
      <div className="rank-header" aria-hidden="true">
        <span>Филиал</span>
        <span>Посещаемость</span>
        <span>Занятия</span>
        <span>Открыто</span>
        <span />
      </div>
      {sorted.map((row) => (
        <Fragment key={row.clubId}>
          <div
            className={"rank-row " + (low(row) ? "is-low" : "")}
            data-club-id={row.clubId}
          >
            <div>
              <span className="branch-name">{clubLabel(row)}</span>
              <span className="branch-sub">
                {sessions(row.closed)}
                {row.open > 0 ? " · " + row.open + " открыто" : ""}
                {row.percentage === null ? " · нет отметок" : ""}
              </span>
            </div>
            <div className="rank-rate">
              <span className="rate">{percentage(row.percentage)}</span>
              <div className="bar-track" aria-hidden="true">
                <div
                  className="bar-fill"
                  style={{ width: (row.percentage ?? 0) + "%" }}
                />
              </div>
            </div>
            <span className="count numeric">{row.closed}</span>
            <span
              className={"count numeric " + (row.open ? "open-count" : "muted")}
            >
              {row.open || "—"}
            </span>
            <button
              className="row-open"
              type="button"
              aria-label={"Группы: " + clubLabel(row)}
              aria-expanded={expanded.has(row.clubId)}
              aria-controls={"branch-" + row.clubId}
              onClick={() => onExpand(row.clubId)}
            >
              <Arrow />
            </button>
          </div>
          <div id={"branch-" + row.clubId} hidden={!expanded.has(row.clubId)}>
            <BranchDetail
              row={row}
              report={report}
              onGroup={onGroup}
              onOpen={() => onOpen(row)}
            />
          </div>
        </Fragment>
      ))}
      {!sorted.length && <p>Филиалов пока нет.</p>}
      <p className="column-note">
        «—» в посещаемости: нет отметок. Филиалы без проведённых занятий
        сохранены.
      </p>
    </section>
  );
}

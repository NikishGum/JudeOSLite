import { useEffect, useRef, useState } from "react";
import { sessionHref } from "../sessions/navigation";
import { reportPeriod, type Period } from "./periods";
import { useAttendanceReport } from "./useAttendanceReport";
import { percentage, type ReportSelection, type Row } from "./types";
import {
  Arrow,
  Findings,
  groupLabel,
  shortId,
  Info,
  Ranking,
  Reasons,
  low,
  number,
  plural,
} from "./OverviewPanels";
import {
  readDetailClub,
  readReportSelection,
  reportParams,
  validReportDate,
} from "./navigation";
import "./statistics-overview.css";

type Detail = "" | "open" | "low";
export function OwnerDashboard() {
  const [selection, setSelection] = useState<ReportSelection>(
    () =>
      readReportSelection() ?? {
        ...reportPeriod("month"),
        clubId: "",
        groupId: "",
      },
  );
  const [period, setPeriod] = useState<Period>(() =>
    readReportSelection() ? "custom" : "month",
  );
  const [manual, setManual] = useState(() => ({
    from: selection.from,
    until: selection.until,
  }));
  const [rangeOpen, setRangeOpen] = useState(false),
    [rangeError, setRangeError] = useState("");
  const [expanded, setExpanded] = useState(new Set<string>()),
    [order, setOrder] = useState("rate");
  const [detail, setDetail] = useState<Detail>(() => {
    const value = new URLSearchParams(location.search).get("reportDetail");
    return value === "open" || value === "low" ? value : "";
  });
  const [detailClub, setDetailClub] = useState(readDetailClub);
  const { filters, report, loading, busy, error, refresh } =
    useAttendanceReport(selection);
  const detailHeading = useRef<HTMLHeadingElement>(null),
    overviewHeading = useRef<HTMLHeadingElement>(null);
  const previousDetail = useRef(detail);
  useEffect(() => {
    // The shell uses its own popstate navigation. Replacing query context must
    // not dispatch navigation or remount the current report during a refresh.
    history.replaceState(
      history.state,
      "",
      location.pathname + "?" + reportParams(selection, detail, detailClub),
    );
  }, [selection, detail, detailClub]);
  useEffect(() => {
    if (previousDetail.current !== detail)
      (detail ? detailHeading : overviewHeading).current?.focus();
    previousDetail.current = detail;
  }, [detail]);
  const { from, until, clubId, groupId } = selection;
  const clubs = new Map(filters?.clubs.map((club) => [club.id, club]));
  const selectedClub = clubs.get(clubId),
    selectedGroup = filters?.groups.find((group) => group.id === groupId);
  function changeSelection(next: ReportSelection) {
    setSelection(next);
    setExpanded(new Set());
    setDetail("");
    setDetailClub("");
  }
  function choosePeriod(next: Exclude<Period, "custom">) {
    setPeriod(next);
    setRangeOpen(false);
    setRangeError("");
    changeSelection({ ...selection, ...reportPeriod(next) });
  }
  function chooseGroup(row: Row) {
    changeSelection({
      ...selection,
      clubId: row.clubId,
      groupId: row.groupId ?? "",
    });
  }
  function showProblem(kind: "open" | "low", row?: Row) {
    setDetailClub(row?.clubId ?? "");
    setOrder(kind === "open" ? "open" : "rate");
    setDetail(kind);
  }
  const context =
    (selectedClub
      ? selectedClub.name + (selectedClub.archived ? " · в архиве" : "")
      : clubId
        ? "Выбранный филиал"
        : "Вся сеть") +
    (selectedGroup
      ? " · " +
        selectedGroup.name +
        (selectedGroup.archived ? " · в архиве" : "")
      : groupId
        ? " · Выбранная группа"
        : "");
  const dateLabel = (date: string) =>
    new Intl.DateTimeFormat("ru-RU", {
      day: "numeric",
      month: "long",
      year: "numeric",
      timeZone: "Europe/Minsk",
    })
      .format(new Date(date + "T12:00:00Z"))
      .replace(" г.", "");
  const rangeLabel =
    from === until
      ? dateLabel(from)
      : from.slice(0, 7) === until.slice(0, 7)
        ? new Date(from + "T12:00:00Z").getUTCDate() + "–" + dateLabel(until)
        : dateLabel(from) + " — " + dateLabel(until);
  const problemRows =
    report?.clubs.filter(
      (row) =>
        (detail === "open" ? row.open > 0 : low(row)) &&
        (!detailClub || row.clubId === detailClub),
    ) ?? [];
  const detailReport = report
    ? {
        ...report,
        clubs: problemRows,
        groups: report.groups.filter((row) =>
          problemRows.some((club) => club.clubId === row.clubId),
        ),
      }
    : null;
  return (
    <div className="statistics-frame">
      <section
        className="owner-dashboard statistics-overview"
        aria-label="Обзор посещаемости"
        aria-busy={busy}
      >
        <div className="page-heading">
          <p>Посещаемость и полнота данных.</p>
          <button
            className="button secondary refresh"
            type="button"
            aria-label="Обновить статистику"
            disabled={busy}
            onClick={refresh}
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M20 7v5h-5M4 17v-5h5M5.5 8a7 7 0 0 1 11.6-2L20 9M4 15l2.9 3A7 7 0 0 0 18.5 16" />
            </svg>
            <span>Обновить</span>
          </button>
        </div>
        <section className="filter-band" aria-label="Фильтры отчёта">
          <div className="filter-main">
            <div
              className="periods"
              role="group"
              aria-label="Быстрый период отчёта"
            >
              {(
                [
                  ["today", "Сегодня"],
                  ["week", "Неделя"],
                  ["month", "Месяц"],
                ] as const
              ).map(([value, label]) => (
                <button
                  key={value}
                  type="button"
                  aria-pressed={period === value}
                  onClick={() => choosePeriod(value)}
                >
                  {label}
                </button>
              ))}
              <button
                type="button"
                aria-pressed={period === "custom"}
                aria-expanded={rangeOpen}
                aria-controls="custom-range"
                onClick={() => setRangeOpen((value) => !value)}
              >
                Период
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="m8 10 4 4 4-4" />
                </svg>
              </button>
            </div>
            <div className="scope-fields">
              <label>
                Филиал
                <select
                  aria-label="Филиал отчёта"
                  value={clubId}
                  onChange={(e) =>
                    changeSelection({
                      ...selection,
                      clubId: e.target.value,
                      groupId: "",
                    })
                  }
                >
                  <option value="">Вся сеть</option>
                  {filters?.clubs.map((club) => (
                    <option key={club.id} value={club.id}>
                      {club.name}
                      {club.archived ? " · в архиве" : ""}
                      {filters?.clubs.filter((item) => item.name === club.name)
                        .length !== 1
                        ? " · " + club.id.slice(0, 8)
                        : ""}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Группа
                <select
                  aria-label="Группа отчёта"
                  value={groupId}
                  onChange={(e) => {
                    const group = filters?.groups.find(
                      (item) => item.id === e.target.value,
                    );
                    changeSelection({
                      ...selection,
                      groupId: e.target.value,
                      clubId: group?.clubId ?? clubId,
                    });
                  }}
                >
                  <option value="">Все группы</option>
                  {filters?.groups
                    .filter((group) => !clubId || group.clubId === clubId)
                    .map((group) => (
                      <option key={group.id} value={group.id}>
                        {group.name} · {clubs.get(group.clubId)?.name}
                        {group.archived ? " · в архиве" : ""}
                        {filters?.groups.filter(
                          (item) =>
                            item.name === group.name &&
                            item.clubId === group.clubId &&
                            item.archived === group.archived,
                        ).length !== 1
                          ? " · " +
                            shortId(
                              group.id,
                              filters?.groups.map((item) => item.id) ?? [],
                            )
                          : ""}
                      </option>
                    ))}
                </select>
              </label>
            </div>
          </div>
          <form
            id="custom-range"
            className="custom-range"
            hidden={!rangeOpen}
            noValidate
            onSubmit={(e) => {
              e.preventDefault();
              if (
                !validReportDate(manual.from) ||
                !validReportDate(manual.until) ||
                manual.from > manual.until
              ) {
                setRangeError(
                  "Укажите обе даты: начало периода должно быть не позже конца.",
                );
                return;
              }
              setRangeError("");
              setPeriod("custom");
              changeSelection({ ...selection, ...manual });
              setRangeOpen(false);
            }}
          >
            <label>
              Дата с
              <input
                aria-label="Период с"
                type="date"
                min="0001-01-01"
                max="9999-12-31"
                required
                value={manual.from}
                aria-invalid={!!rangeError}
                aria-describedby={rangeError ? "date-error" : undefined}
                onChange={(e) => setManual({ ...manual, from: e.target.value })}
              />
            </label>
            <label>
              Дата по
              <input
                aria-label="Период по"
                type="date"
                min="0001-01-01"
                max="9999-12-31"
                required
                value={manual.until}
                aria-invalid={!!rangeError}
                onChange={(e) =>
                  setManual({ ...manual, until: e.target.value })
                }
              />
            </label>
            <button className="button primary" type="submit">
              Применить
            </button>
            {rangeError && (
              <p className="form-error" id="date-error" role="alert">
                {rangeError}
              </p>
            )}
          </form>
        </section>
        <div className="overview-content">
          <div className="report-context">
            <h2 ref={overviewHeading} tabIndex={-1}>
              {context}
              <span className="context-date">{rangeLabel}</span>
            </h2>
            <details className="context-info">
              <summary aria-label="Контекст отчёта">
                <Info />
                <span>Контекст отчёта</span>
              </summary>
              <p>
                Филиал выбран только для этого отчёта. Общий рабочий филиал
                приложения не меняется. Обе даты включены.
              </p>
            </details>
            {selectedClub?.archived && (
              <p className="archive-note">
                Филиал в архиве. Исторические данные сохранены; рабочие действия
                недоступны.
              </p>
            )}
          </div>
          {loading && <p role="status">Считаем посещаемость…</p>}
          {error && (
            <div className="report-error" role="alert">
              <p>{error}</p>
              <button
                className="button secondary"
                type="button"
                disabled={busy}
                onClick={refresh}
              >
                Повторить загрузку статистики
              </button>
            </div>
          )}
          {report && (
            <>
              <section
                className="summary-panel"
                aria-label="Итоги посещаемости"
              >
                <dl className="metrics">
                  <div className="metric">
                    <dt>Посещаемость</dt>
                    <dd className="rate">
                      <span data-metric="percentage">
                        {percentage(report.total.percentage)}
                      </span>
                      <span className="metric-note">
                        {number(report.total.present + report.total.absent)}{" "}
                        отметок в закрытых журналах
                      </span>
                    </dd>
                  </div>
                  <div className="metric">
                    <dt>Проведено занятий</dt>
                    <dd>
                      <span data-metric="closed">
                        {number(report.total.closed)}
                      </span>
                      <span className="metric-note">
                        <span data-metric="planned">
                          {report.total.planned}
                        </span>{" "}
                        запланировано
                      </span>
                    </dd>
                  </div>
                  <div className="metric">
                    <dt>Присутствия</dt>
                    <dd>
                      <span data-metric="present">
                        {number(report.total.present)}
                      </span>
                      <span className="metric-note">
                        посещения, не спортсмены
                      </span>
                    </dd>
                  </div>
                  <div className="metric">
                    <dt>Пропуски</dt>
                    <dd>
                      <span data-metric="absent">
                        {number(report.total.absent)}
                      </span>
                      <span className="metric-note">
                        {report.total.percentage === null
                          ? "нет отметок"
                          : percentage(100 - report.total.percentage) +
                            " от всех отметок"}
                      </span>
                    </dd>
                  </div>
                </dl>
                <div className="summary-meta">
                  <span>Только закрытые занятия · отменённые исключены</span>
                  {report.total.open ? (
                    <button type="button" onClick={() => showProblem("open")}>
                      <Info />
                      <strong data-metric="open">
                        {report.total.open}
                      </strong>{" "}
                      {plural(report.total.open, [
                        "журнал не вошёл",
                        "журнала не вошли",
                        "журналов не вошли",
                      ])}{" "}
                      в расчёт <Arrow />
                    </button>
                  ) : (
                    <span>
                      Открытых журналов нет{" "}
                      <span className="visually-hidden" data-metric="open">
                        0
                      </span>
                    </span>
                  )}
                </div>
                <details className="mobile-facts">
                  <summary>Дополнительные показатели</summary>
                  <p>
                    Отметок: {report.total.present + report.total.absent}.
                    Запланировано: {report.total.planned}. Присутствия —
                    посещения, не уникальные спортсмены. Только закрытые
                    занятия; отменённые исключены.
                  </p>
                </details>
              </section>
              {!report.total.closed &&
              !report.total.open &&
              !report.total.planned ? (
                <section className="panel empty-state">
                  <h2>В этом периоде занятий нет</h2>
                  <p>Попробуйте другой период, филиал или группу.</p>
                </section>
              ) : !detail ? (
                <div className="overview-grid">
                  <Ranking
                    report={report}
                    expanded={expanded}
                    onExpand={(id) =>
                      setExpanded((previous) => {
                        const next = new Set(previous);
                        if (next.has(id)) next.delete(id);
                        else next.add(id);
                        return next;
                      })
                    }
                    onGroup={chooseGroup}
                    onOpen={(row) => showProblem("open", row)}
                    order={order}
                    onOrder={setOrder}
                  />
                  <div className="right-column">
                    <Findings report={report} onProblem={showProblem} />
                    <Reasons total={report.total} />
                  </div>
                </div>
              ) : (
                <>
                  <div className="detail-navigation">
                    <button
                      className="text-button"
                      type="button"
                      onClick={() => {
                        setDetail("");
                        setDetailClub("");
                      }}
                    >
                      К обзору сети
                    </button>
                    <h2 ref={detailHeading} tabIndex={-1}>
                      {detail === "open"
                        ? "Открытые журналы"
                        : "Филиалы с посещаемостью ниже 80%"}
                    </h2>
                    <p>
                      {detailClub ? clubs.get(detailClub)?.name : context} ·{" "}
                      {rangeLabel}
                    </p>
                  </div>
                  {detailReport && (
                    <Ranking
                      report={detailReport}
                      expanded={expanded}
                      onExpand={(id) =>
                        setExpanded((previous) => {
                          const next = new Set(previous);
                          if (next.has(id)) next.delete(id);
                          else next.add(id);
                          return next;
                        })
                      }
                      onGroup={chooseGroup}
                      onOpen={(row) => showProblem("open", row)}
                      order={order}
                      onOrder={setOrder}
                    />
                  )}
                  {detail === "open" && (
                    <section
                      className="panel open-journals"
                      aria-label="Список открытых журналов"
                    >
                      <h3>
                        Открытые занятия:{" "}
                        {number(
                          (report.openSessions ?? []).filter(
                            (item) => !detailClub || item.clubId === detailClub,
                          ).length,
                        )}
                      </h3>
                      {(report.openSessions ?? [])
                        .filter(
                          (item) => !detailClub || item.clubId === detailClub,
                        )
                        .map((item) => {
                          const club = clubs.get(item.clubId),
                            group = report.groups.find(
                              (g) => g.groupId === item.groupId,
                            );
                          return (
                            <article className="open-journal" key={item.id}>
                              <div>
                                <h3>
                                  {group
                                    ? groupLabel(group, report.groups)
                                    : "Группа"}
                                </h3>
                                <p>
                                  {club?.name} · {dateLabel(item.date)} ·{" "}
                                  {new Intl.DateTimeFormat("ru-RU", {
                                    hour: "2-digit",
                                    minute: "2-digit",
                                    timeZone: item.timezone,
                                  }).format(new Date(item.startsAt))}
                                </p>
                              </div>
                              {club?.archived ? (
                                <p>
                                  Филиал в архиве; рабочий журнал недоступен.
                                </p>
                              ) : (
                                <a
                                  className="button secondary report-session-link"
                                  href={
                                    sessionHref(item.clubId, {
                                      sessionId: item.id,
                                      groupId: item.groupId,
                                      from,
                                      through: until,
                                      view: "period",
                                      status: "open",
                                    }) +
                                    "&" +
                                    reportParams(selection, detail, detailClub)
                                  }
                                >
                                  Открыть журнал <Arrow />
                                </a>
                              )}
                            </article>
                          );
                        })}
                      {!report.total.open && (
                        <p>Открытых журналов в выбранном контексте нет.</p>
                      )}
                    </section>
                  )}
                </>
              )}
            </>
          )}
        </div>
        <footer className="report-footer">
          <details>
            <summary>Как считается посещаемость</summary>
            <p>
              Присутствия ÷ (присутствия + пропуски) × 100, только по закрытым
              занятиям. Отменённые и заменённые занятия исключены. Открытые и
              запланированные показаны отдельно. При отсутствии отметок — «—».
              Общий процент взвешен по отметкам, а не усреднён по филиалам.
              «Присутствия» — посещения занятий, а не уникальные спортсмены.
            </p>
            <p>
              Обе даты включены. Быстрые периоды — по Минску. Дата занятия
              определяется его сохранённым часовым поясом.
            </p>
            {report && (
              <p data-metric="reasons">
                Причины пропусков: неизвестна — {report.total.unknown}; болезнь
                — {report.total.illness}; уважительная — {report.total.excused}.
              </p>
            )}
          </details>
        </footer>
      </section>
    </div>
  );
}

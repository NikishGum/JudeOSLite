import type { ReportSelection } from "./types";

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
export function validReportDate(value: string) {
  return (
    /^\d{4}-\d{2}-\d{2}$/.test(value) &&
    value >= "0001-01-01" &&
    !Number.isNaN(Date.parse(value)) &&
    new Date(value + "T12:00:00Z").toISOString().slice(0, 10) === value
  );
}
export function readReportSelection(
  params = new URLSearchParams(location.search),
): ReportSelection | null {
  const from = params.get("reportFrom") ?? "",
    until = params.get("reportUntil") ?? "";
  const clubId = params.get("reportClub") ?? "",
    groupId = params.get("reportGroup") ?? "";
  return validReportDate(from) &&
    validReportDate(until) &&
    from <= until &&
    (!clubId || uuid.test(clubId)) &&
    (!groupId || uuid.test(groupId))
    ? { from, until, clubId, groupId }
    : null;
}
export function reportParams(
  selection: ReportSelection,
  detail = "",
  detailClub = "",
) {
  const params = new URLSearchParams({
    reportFrom: selection.from,
    reportUntil: selection.until,
  });
  if (selection.clubId) params.set("reportClub", selection.clubId);
  if (selection.groupId) params.set("reportGroup", selection.groupId);
  if (detail === "open" || detail === "low") params.set("reportDetail", detail);
  if (uuid.test(detailClub)) params.set("reportDetailClub", detailClub);
  return params;
}
/** Fixed internal route and validated scalar fields; never accept a return URL. */
export function reportReturnHref() {
  const selection = readReportSelection();
  return selection
    ? "/statistics?" +
        reportParams(
          selection,
          new URLSearchParams(location.search).get("reportDetail") ?? "",
          new URLSearchParams(location.search).get("reportDetailClub") ?? "",
        )
    : null;
}

export function readDetailClub() {
  const value =
    new URLSearchParams(location.search).get("reportDetailClub") ?? "";
  return uuid.test(value) ? value : "";
}

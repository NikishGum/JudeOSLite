import { useState } from "react";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { OwnerDashboard } from "../owner-dashboard/OwnerDashboard";
import { FinanceSummary } from "./Payments";
import { today } from "./types";

export function Reports({
  clubId,
  clubs,
}: {
  clubId: string;
  clubs: { id: string; name: string }[];
}) {
  const [selected, setSelected] = useState(clubId),
    [dates, setDates] = useState({
      from: today().slice(0, 7) + "-01",
      until: today(),
    });
  const valid = !!dates.from && !!dates.until && dates.from <= dates.until;
  return (
    <Tabs defaultValue="attendance">
      <TabsList aria-label="Вид отчёта">
        <TabsTrigger value="attendance">Посещаемость</TabsTrigger>
        <TabsTrigger value="finance">Финансы</TabsTrigger>
      </TabsList>
      <TabsContent value="attendance">
        <OwnerDashboard />
      </TabsContent>
      <TabsContent value="finance">
        <div className="module-page">
          <div className="connection">
            <h2>Финансовый отчёт</h2>
            <div className="report-finance-filters">
              <label className="field">
                Филиал отчёта
                <select
                  value={selected}
                  onChange={(e) => setSelected(e.target.value)}
                >
                  <option value="">Выберите филиал</option>
                  {clubs.map((club) => (
                    <option key={club.id} value={club.id}>
                      {club.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="field">
                С даты
                <input
                  type="date"
                  required
                  value={dates.from}
                  onChange={(e) => setDates({ ...dates, from: e.target.value })}
                />
              </label>
              <label className="field">
                По дату
                <input
                  type="date"
                  required
                  min={dates.from}
                  value={dates.until}
                  onChange={(e) =>
                    setDates({ ...dates, until: e.target.value })
                  }
                />
              </label>
            </div>
            {!valid && (
              <p className="error" role="alert">
                Укажите обе даты. Начало периода должно быть не позже конца.
              </p>
            )}
            {!selected && (
              <p className="help">
                Выберите филиал, чтобы посмотреть подтверждённые оплаты и
                ожидающие счета.
              </p>
            )}
          </div>
          {selected && valid && (
            <>
              <FinanceSummary clubId={selected} dates={dates} />
              <FinanceSummary
                clubId={selected}
                dates={dates}
                chart
                showUnavailable
              />
            </>
          )}
        </div>
      </TabsContent>
    </Tabs>
  );
}

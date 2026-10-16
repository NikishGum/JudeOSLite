import { Calendar } from "@/components/ui/calendar";
import { ru } from "date-fns/locale";
export default function DateCalendar(
  props: React.ComponentProps<typeof Calendar>,
) {
  return (
    <Calendar
      {...props}
      locale={ru}
      labels={{
        ...props.labels,
        labelNav: () => "Навигация календаря",
        labelDayButton: (day, modifiers) =>
          [
            new Intl.DateTimeFormat("ru-BY", {
              weekday: "long",
              day: "numeric",
              month: "long",
              year: "numeric",
              timeZone: "Europe/Minsk",
            }).format(day),
            modifiers.today ? "сегодня" : "",
            modifiers.selected ? "выбрано" : "",
          ]
            .filter(Boolean)
            .join(", "),
      }}
    />
  );
}

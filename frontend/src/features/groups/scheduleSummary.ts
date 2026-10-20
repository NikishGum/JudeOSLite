import { dateLabel, modeNames, type Rule } from '../schedule/types';

export function ruleSummary(rule: Rule) {
  const start = rule.startsTime.slice(0, 5);
  const minutes = Number(start.slice(0, 2)) * 60 + Number(start.slice(3)) + rule.durationMinutes;
  const end = `${String(Math.floor(minutes / 60) % 24).padStart(2, '0')}:${String(minutes % 60).padStart(2, '0')}`;
  const days = rule.mode === 'weekly' ? rule.weekdays?.map((day) => ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'][day - 1]).join(', ')
    : rule.mode === 'day_parity' ? `${rule.dayParity === 'even' ? 'Чётные' : 'Нечётные'} числа месяца`
      : rule.mode === 'date_list' ? `${rule.selectedDates?.slice(0, 3).map(dateLabel).join(', ')}${(rule.selectedDates?.length ?? 0) > 3 ? ` · ещё ${(rule.selectedDates?.length ?? 0) - 3} дат` : ''}` : modeNames[rule.mode];
  return `${days} · ${start}–${end}${minutes >= 1440 ? ' (+1 день)' : ''}`;
}

// A future version must not hide the version that is still effective today.
export function currentRules(rules: Rule[], groupId: string, today: string) {
  return rules.filter((rule) => rule.groupId === groupId && rule.mode !== 'once' && rule.from <= today && (!rule.until || today < rule.until)
    && (rule.mode !== 'date_list' || rule.selectedDates?.some((date) => date >= today)));
}
export function futureRules(rules: Rule[], groupId: string, today: string) {
  return rules.filter((rule) => rule.groupId === groupId && rule.mode !== 'once' && rule.from > today);
}

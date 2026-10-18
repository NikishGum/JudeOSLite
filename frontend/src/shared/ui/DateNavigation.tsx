import { lazy, Suspense, useState } from 'react';
import { CalendarDays, ChevronLeft, ChevronRight } from 'lucide-react';
import { ru } from 'date-fns/locale';
import { Button } from '@/components/ui/button';
import { Popover, PopoverTrigger, PopoverContent } from '@/components/ui/popover';
const Calendar = lazy(() => import('@/components/ui/calendar').then((module) => ({ default: module.Calendar })));
export function DateNavigation({ date, timezone, onChange, disabled, step = 1 }: { date: string; timezone: string; onChange: (date: string) => void; disabled: boolean; step?: number }) {
  const [open, setOpen] = useState(false);
  const selected = new Date(`${date}T12:00:00Z`);
  const valid = !Number.isNaN(selected.getTime());
  function move(delta: number) { if (!valid) return; const next = new Date(selected); next.setUTCDate(next.getUTCDate() + delta); onChange(next.toISOString().slice(0, 10)); }
  return <div className="date-picker-group" role="group" aria-label="Смена периода">
    <Button type="button" className="date-step" variant="ghost" size="icon" aria-label="Назад" disabled={disabled || !valid} onClick={() => move(-step)}><ChevronLeft /></Button>
    <Popover open={open && !disabled} onOpenChange={setOpen}><PopoverTrigger asChild><Button type="button" variant="ghost" className="date-select-trigger" disabled={disabled} aria-label="Открыть календарь"><CalendarDays data-icon="inline-start" />{valid ? new Intl.DateTimeFormat('ru-BY', { day: 'numeric', month: 'long', timeZone: 'UTC' }).format(selected) : 'Выбрать дату'}</Button></PopoverTrigger>
      <PopoverContent align="start" collisionPadding={0} className="calendar-popover"><Suspense fallback={<p role="status">Загружаем календарь…</p>}><Calendar locale={ru} mode="single" selected={valid ? selected : undefined} defaultMonth={valid ? selected : undefined} timeZone="UTC" weekStartsOn={1} labels={{ labelNav: () => 'Навигация календаря', labelNext: () => 'Следующий месяц', labelPrevious: () => 'Предыдущий месяц' }} onSelect={(day) => { if (day) { onChange(new Intl.DateTimeFormat('en-CA', { year: 'numeric', month: '2-digit', day: '2-digit', timeZone: 'UTC' }).format(day)); setOpen(false); } }} /></Suspense><p className="help">Локальная дата филиала · {timezone}</p></PopoverContent>
    </Popover>
    <Button type="button" className="date-step" variant="ghost" size="icon" aria-label="Вперёд" disabled={disabled || !valid} onClick={() => move(step)}><ChevronRight /></Button>
  </div>;
}

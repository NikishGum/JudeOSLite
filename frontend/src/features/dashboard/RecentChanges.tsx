import { routePath } from '../../app/navigation';
import { useReadBlock } from './useReadBlock';

type Change = { id: string; createdAt: string; actorName: string; clubName: string | null; entityType: string; action: string };
const names: Record<string, string> = { created: 'Создана запись', updated: 'Изменена запись', archived: 'Запись в архиве', restored: 'Возврат из архива', added: 'Участник включён в группу', excluded: 'Участник исключён из группы', transferred: 'Перевод участника', ended: 'Завершён период в группе', marked: 'Сохранена отметка', corrected: 'Исправлена закрытая отметка', journal_opened: 'Открыт журнал', journal_closed: 'Завершён журнал', cancelled: 'Отменено занятие', cancel: 'Отменено занятие', reschedule: 'Перенесено занятие', 'replace-trainer': 'Заменён тренер', trainer_replaced: 'Заменён тренер', trainer_created: 'Создан тренер', trainer_assignment_created: 'Назначен тренер', trainer_assignment_revoked: 'Отозвано назначение тренера' };
const stamp = new Intl.DateTimeFormat('ru-BY', { timeZone: 'Europe/Minsk', dateStyle: 'short', timeStyle: 'short' });

export function RecentChanges({ clubId }: { clubId: string }) {
  const block = useReadBlock<{ items: Change[] }>(`/api/audit/events?${new URLSearchParams({ clubId })}`);
  return <section className="dashboard-panel dashboard-changes" aria-labelledby="recent-changes-title" aria-busy={block.loading}>
    <div className="dashboard-section-heading"><h2 id="recent-changes-title">Последние изменения</h2><a href={routePath({ clubId, section: 'history' })}>Вся история</a></div>
    <p className="help">Разрешённая история · время по Минску.</p>
    {block.loading && <p role="status">Загружаем последние изменения…</p>}
    {block.error && <div role="alert" className="error"><p>{block.error}</p><button type="button" onClick={block.refresh}>Повторить загрузку изменений</button></div>}
    {block.data && (block.data.items.length ? <ol className="dashboard-change-list">{block.data.items.slice(0, 5).map((item) => <li key={item.id}><strong>{names[item.action] ?? 'Изменение записи'}</strong><p>{item.actorName} · {item.clubName ?? 'Сеть'}</p><time dateTime={item.createdAt}>{stamp.format(new Date(item.createdAt))}</time></li>)}</ol> : <p role="status">Изменений пока нет.</p>)}
  </section>;
}

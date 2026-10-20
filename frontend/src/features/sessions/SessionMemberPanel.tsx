import { useEffect, useRef } from 'react';
import { markLabel, originLabel, type Journal } from '../journals/types';

export function SessionMemberPanel({ journal, membershipId, onJournal }: { journal: Journal; membershipId: string; onJournal: () => void }) {
  const heading = useRef<HTMLHeadingElement>(null);
  const member = journal.roster.members.find((item) => item.membershipId === membershipId);
  const record = journal.records.find((item) => item.membershipId === membershipId);
  useEffect(() => {
    heading.current?.focus({ preventScroll: true });
    if (window.matchMedia('(max-width: 900px)').matches) heading.current?.scrollIntoView({ block: 'start' });
  }, [membershipId]);
  if (!member) return null;
  return <section className="session-member-panel" aria-labelledby="session-member-title">
    <h3 id="session-member-title" ref={heading} tabIndex={-1}>{member.fullName}</h3>
    <p className="help">{journal.session.groupName}{member.archived && ' · Карточка в архиве'}</p>
    <h4>Участие в тренировке</h4>
    {journal.roster.source === 'preliminary' ? <p>Спортсмен в предварительном составе. Отметка будет доступна после открытия журнала.</p>
      : record ? <dl className="session-member-facts">
        <div><dt>Посещаемость</dt><dd>{markLabel(record)}</dd></div>
        {record.markedAt && <div><dt>Отметка сохранена</dt><dd>{new Intl.DateTimeFormat('ru', { dateStyle: 'long', timeStyle: 'short', timeZone: journal.session.timezone }).format(new Date(record.markedAt))}</dd></div>}
        {record.origin && <div><dt>Способ отметки</dt><dd>{originLabel(record.origin)}</dd></div>}
      </dl> : <p>Данные отметки недоступны. Обновите журнал.</p>}
    <div className="session-member-actions"><button type="button" onClick={onJournal}>Вернуться к журналу</button><a className="session-link" href="#session-roster-title">К списку спортсменов</a></div>
  </section>;
}

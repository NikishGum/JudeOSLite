import { useState } from 'react';
import { dateLabel } from '../schedule/types';
import type { SessionRoster as Roster } from './types';
import { markLabel, type Record } from '../journals/types';

export function SessionRoster({ roster, cancelled, records, selectedMembershipId, onSelect }: {
  roster: Roster; cancelled: boolean; records: Record[]; selectedMembershipId: string | null; onSelect: (membershipId: string) => void;
}) {
  const [search, setSearch] = useState('');
  const members = roster.members.filter((m) => m.fullName.toLocaleLowerCase('ru').includes(search.trim().toLocaleLowerCase('ru')));
  return <section className="session-roster" aria-labelledby="session-roster-title">
    <h3 id="session-roster-title" tabIndex={-1}>Состав тренировки <span>· {roster.members.length}</span></h3>
    <p className="help">{roster.source === 'preliminary' ? `Предварительный состав на ${dateLabel(roster.date)}` : 'Зафиксированный состав тренировки'}</p>
    {roster.source === 'preliminary' && <p className="help">{cancelled ? 'Занятие отменено без снимка. Это предварительный состав, не история посещения.' : 'Состав может измениться до открытия журнала.'}</p>}
    <label className="field">Поиск по составу<input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Имя участника" /></label>
    <p className="help" role="status">Показано: {members.length} из {roster.members.length}</p>
    {members.length ? <ol className="session-roster-list">{members.map((m) => {
      const record = records.find((item) => item.membershipId === m.membershipId);
      return <li key={m.membershipId}><button type="button" className="session-roster-member" aria-label={`Посмотреть спортсмена: ${m.fullName}`} aria-pressed={selectedMembershipId === m.membershipId} onClick={() => onSelect(m.membershipId)}><span>{m.fullName}</span><small>{record ? markLabel(record) : 'Предварительный состав'}</small>{m.archived && <small>Карточка в архиве</small>}</button></li>;
    })}</ol> : <p>{search ? 'По этому имени никого нет.' : roster.source === 'snapshot' ? 'Снимок состава пуст.' : 'На дату тренировки в группе нет участников.'}</p>}
  </section>;
}

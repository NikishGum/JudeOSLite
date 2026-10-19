import { useEffect, useState } from 'react';
import { routePath } from '../../app/navigation';
import { sessionHref } from '../sessions/navigation';
import { nextDate } from '../sessions/types';
import { stateNames } from '../trainer-sessions/types';
import { OwnerOverview } from './OwnerOverview';
import { RecentChanges } from './RecentChanges';
import { calendarDate, sessionTime, type Mode, type Panel, type Session } from './types';
import { useReadBlock } from './useReadBlock';
import './dashboard.css';
import { BookOpenCheck, CalendarDays, Plus, RefreshCw, Users, UsersRound } from 'lucide-react';
import { FinanceSummary } from '../club-workspace/Payments';

const href = (session: Session) => sessionHref(session.clubId, { sessionId: session.id, groupId: session.groupId, from: session.date, through: session.date, view: 'day', journal: session.canMark ? 'trainer' : 'admin' });
const action = (session: Session) => session.canMark && session.status === 'open' ? 'Продолжить отметки' : session.status === 'open' ? 'Открыть журнал' : 'Открыть занятие';
function useCompactLayout() {
  const [compact, setCompact] = useState(() => window.matchMedia('(max-width: 900px)').matches);
  useEffect(() => {
    const media = window.matchMedia('(max-width: 900px)');
    const update = () => setCompact(media.matches);
    media.addEventListener('change', update);
    return () => media.removeEventListener('change', update);
  }, []);
  return compact;
}
function SessionFacts({ session, showDate = true }: { session: Session; showDate?: boolean }) {
  return <>{showDate && <p className="dashboard-session-date">{calendarDate(session.date)} · {session.timezone}</p>}
    {session.status === 'open' && <div className="dashboard-progress"><p>Отмечено {session.marked} из {session.roster}</p>{session.roster > 0 ? <progress aria-label="Прогресс журнала" value={session.marked} max={session.roster} /> : <p className="help">Состав журнала пуст.</p>}</div>}
    {session.status === 'planned' && <p className="help">Состав фиксируется при открытии журнала.</p>}
  </>;
}
function NextSession({ panel }: { panel: Panel }) {
  const session = panel.next;
  return <section className="dashboard-panel dashboard-next" aria-labelledby="next-session-title">
    {session ? <><div className="dashboard-next-top"><time dateTime={session.startsAt}>{sessionTime(session, session.startsAt)}<span>–{sessionTime(session, session.endsAt)}</span></time><span className={`dashboard-state ${session.status}`}>{stateNames[session.status]}</span></div>
      <h2 id="next-session-title">{session.groupName}</h2>
      <p className="dashboard-session-date">{calendarDate(session.date)} · {session.timezone}</p><a className="dashboard-primary" href={href(session)} data-session-id={session.id}>{action(session)}</a>
      <p>{panel.clubs.length > 1 && <>{panel.clubs.find((club) => club.id === session.clubId)?.name} · </>}{session.canMark ? 'Ваше занятие' : session.trainerName}</p><SessionFacts session={session} showDate={false} />
    </> : <><h2 id="next-session-title">{panel.mode === 'trainer' ? 'Ближайших назначенных занятий нет' : 'Ближайших занятий нет'}</h2><p>{panel.mode === 'trainer' ? 'Когда администратор назначит вам занятие, оно появится здесь.' : 'Открытых журналов и запланированных занятий с сегодняшнего дня пока нет.'}</p>{panel.clubs[0] && <a className="dashboard-primary" href={sessionHref(panel.clubs[0].id, {})}>Перейти к занятиям</a>}</>}
  </section>;
}
function Counters({ panel }: { panel: Panel }) {
  const fields = [ ...(panel.counters.participants === null ? [] : [{ key: 'participants', label: 'Активные участники', value: panel.counters.participants, icon: Users }]),
    { key: 'groups', label: panel.mode === 'trainer' ? 'Назначенные группы' : 'Активные группы', value: panel.counters.groups, icon: UsersRound },
    { key: 'today', label: 'Занятия сегодня', value: panel.counters.today, icon: CalendarDays }, { key: 'open', label: 'Открытые журналы', value: panel.counters.open, icon: BookOpenCheck } ];
  return <dl className="dashboard-counters" aria-label="Показатели главной">{fields.map((field) => <div key={field.key}><dt><field.icon aria-hidden="true" />{field.label}</dt><dd data-counter={field.key}>{field.value}</dd></div>)}</dl>;
}
function SessionList({ panel, unfinished = false }: { panel: Panel; unfinished?: boolean }) {
  const sessions = unfinished ? panel.unfinished : panel.today;
  const count = unfinished ? panel.counters.open : panel.counters.today;
  const title = unfinished ? 'Незавершённые журналы' : 'Сегодня';
  return <section className="dashboard-panel dashboard-agenda" aria-label={title}>
    <div className="dashboard-section-heading"><h2>{title}</h2><span>{count}</span></div>
    {!unfinished && <ul className="dashboard-local-dates">{panel.clubs.map((club) => <li key={club.id}><a href={routePath({ clubId: club.id, section: 'home' })}>{club.name}</a><span>{calendarDate(club.today)} · {club.timezone}</span></li>)}</ul>}
    {!sessions.length && <p className="help">{unfinished ? 'Открытых журналов нет.' : 'На локальную дату филиала занятий нет.'}</p>}
    <ul className="dashboard-session-list">{sessions.map((session) => <li key={session.id} data-session-id={session.id}>
      <div className="dashboard-agenda-top"><time dateTime={session.startsAt}>{sessionTime(session, session.startsAt)}–{sessionTime(session, session.endsAt)}</time><span className={`dashboard-state ${session.status}`}>{stateNames[session.status]}</span></div>
      <h3>{session.groupName}</h3><p>{panel.clubs.find((club) => club.id === session.clubId)?.name} · {session.trainerName}</p>
      <SessionFacts session={session} /><a href={href(session)}>{action(session)}</a>
    </li>)}</ul>
    {count > sessions.length && <p className="help">Показано {sessions.length} из {count}. Все записи — в разделе занятий соответствующего филиала.</p>}
    <div className="dashboard-links">{panel.clubs.map((club) => <a key={club.id} href={sessionHref(club.id, { from: club.today, through: club.today, view: 'day' })}>Занятия · {club.name}</a>)}</div>
  </section>;
}
function Groups({ panel }: { panel: Panel }) {
  return <section className="dashboard-panel dashboard-groups" aria-labelledby="home-groups-title"><h2 id="home-groups-title">{panel.mode === 'trainer' ? 'Ваши группы' : panel.clubs.length > 1 ? 'Группы сети' : 'Группы филиала'}</h2>
    {!panel.groups.length && <p className="help">{panel.mode === 'trainer' ? 'Назначенных активных групп пока нет.' : 'Активных групп пока нет. Создайте группу в разделе «Группы».'}</p>}
    <ul className="dashboard-group-list">{panel.groups.map((group) => {
      const club = panel.clubs.find((item) => item.id === group.clubId)!;
      const destination = club.canManage ? routePath({ clubId: group.clubId, section: 'groups', detail: group.id }) : sessionHref(group.clubId, { groupId: group.id, from: club.today, through: nextDate(club.today, 6), view: 'period' });
      return <li key={group.id}><a href={destination} data-group-id={group.id}><strong>{group.name}</strong><span>{club.name} · участников на {calendarDate(club.today)}: {group.members}</span></a></li>;
    })}</ul>
    {panel.counters.groups > panel.groups.length && <p className="help">Показано {panel.groups.length} из {panel.counters.groups} активных групп.</p>}
    <div className="dashboard-links">{panel.clubs.filter((club) => club.canManage).map((club) => <a key={club.id} href={routePath({ clubId: club.id, section: 'groups' })}>Все группы · {club.name}</a>)}</div>
  </section>;
}
export function Dashboard({ clubId, mode, displayName }: { clubId: string; mode: Mode; displayName: string }) {
  const compact = useCompactLayout();
  const block = useReadBlock<Panel>(`/api/dashboard?${new URLSearchParams({ clubId })}`);
  const panel = block.data?.mode === mode ? block.data : null;
  useEffect(() => { if (block.data && block.data.mode !== mode) window.dispatchEvent(new Event('session-changed')); }, [block.data, mode]);
  const primaryClub = panel?.clubs.find((club) => club.id === panel.next?.clubId && club.canManage) ?? panel?.clubs.find((club) => club.canManage);
  const noAssignments = panel && !panel.clubs.length;
  const secondary = <>{panel && !noAssignments && <Groups panel={panel} />}{mode !== 'trainer' && <RecentChanges clubId={clubId} />}</>;
  return <div className={`dashboard dashboard-${mode}`} data-dashboard-mode={mode}>
    <div className="dashboard-intro"><div><h2>Добро пожаловать, {displayName.trim().split(/\s+/)[0]}!</h2><p>{panel?.clubs.length === 1 ? panel.clubs[0].name : mode === 'owner' ? 'Обзор сети' : mode === 'administrator' ? 'Ваши филиалы' : 'Ваши назначения'}</p></div><button type="button" className="secondary" aria-label="Обновить главную" disabled={block.loading} onClick={() => window.dispatchEvent(new Event('attendance-changed'))}><RefreshCw aria-hidden="true" />Обновить</button></div>
    {block.loading && <p role="status">Загружаем рабочий день…</p>}
    {block.error && <div role="alert" className="dashboard-panel error"><p>{block.error}</p><button type="button" onClick={block.refresh}>Повторить загрузку дня</button></div>}
    {noAssignments && <section className="dashboard-panel"><h2>{mode === 'owner' ? 'Филиалы пока не добавлены' : 'Нет доступных назначений'}</h2><p>{mode === 'owner' ? 'Добавьте филиал, затем участников, группы и расписание.' : 'Обратитесь к владельцу клуба за назначением.'}</p>{mode === 'owner' && <a href="/clubs">Перейти к филиалам</a>}</section>}
    {mode !== 'owner' && compact && panel && !noAssignments && <NextSession panel={panel} />}
    {panel && !noAssignments && <Counters panel={panel} />}
    {primaryClub && <div className="dashboard-create"><a className="dashboard-primary" href={routePath({ clubId: primaryClub.id, section: 'participants', detail: 'new' })}><Plus aria-hidden="true" />Добавить участника · {primaryClub.name}</a></div>}
    <div className="dashboard-grid"><div className="dashboard-main">
      {mode === 'owner' ? <><OwnerOverview clubId={clubId} />{clubId && <FinanceSummary clubId={clubId} chart />}</> : !compact && panel && !noAssignments && <NextSession panel={panel} />}
      {!compact && secondary}
    </div><aside className="dashboard-side" aria-label="Рабочие занятия">
      {panel && !noAssignments && <><SessionList panel={panel} /><SessionList panel={panel} unfinished /></>}
    </aside>{compact && <div className="dashboard-lower">{secondary}</div>}</div>
  </div>;
}

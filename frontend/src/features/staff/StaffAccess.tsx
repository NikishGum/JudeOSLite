import { useEffect, useId, useRef, useState } from 'react';
import { ArrowLeft, ChevronRight, CircleMinus, KeyRound, LockKeyhole, Plus, Search, UnlockKeyhole } from 'lucide-react';
import { ApiError, apiGet, apiPost, apiPut } from '../../shared/api';
import { Dialog } from '../../shared/ui/Dialog';
import { PersonAvatar } from '../../app/RuntimeShell';
import { TrainerProfile } from '../club-workspace/Profile';
import { StaffEditor, type StaffDraft, type StaffEditorMode } from './StaffEditor';
import { accountState, assignmentLabel, staffRoles, type StaffAssignment, type StaffCard, type StaffDirectory } from './staff-types';
import './staff-access.css';

type Modal = { mode: StaffEditorMode | 'disable' | 'revoke' | 'profile'; target: StaffCard | null; assignment?: StaffAssignment };
const message = (cause: unknown) => cause instanceof ApiError ? cause.message : 'Нет соединения. Проверьте интернет и повторите действие.';
const titles = { add: 'Добавить сотрудника', account: 'Данные сотрудника', password: 'Сменить пароль', assignment: 'Добавить назначение', disable: 'Состояние доступа', revoke: 'Снять назначение', profile: 'Профиль тренера' };

export function StaffAccess({ beforeNavigate, onSessionRefresh }: { beforeNavigate: (action: () => void) => void; onSessionRefresh: () => Promise<void> }) {
  const filterId = useId();
  const [directory, setDirectory] = useState<StaffDirectory | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [attempt, setAttempt] = useState(0);
  const [selectedId, setSelectedId] = useState('');
  const [detailOpen, setDetailOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [role, setRole] = useState('');
  const [clubId, setClubId] = useState('');
  const [access, setAccess] = useState('');
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [modal, setModal] = useState<Modal | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [conflict, setConflict] = useState(false);
  const pending = useRef(false);
  const epoch = useRef(0);
  const heading = useRef<HTMLHeadingElement>(null);
  const listHeading = useRef<HTMLHeadingElement>(null);
  const listScroll = useRef(0);
  const root = useRef<HTMLElement>(null);

  useEffect(() => {
    const controller = new AbortController(); const generation = ++epoch.current;
    setLoading(true); setLoadError('');
    apiGet<StaffDirectory>('/api/staff-directory', controller.signal).then((result) => {
      if (controller.signal.aborted || generation !== epoch.current) return;
      setDirectory(result); setSelectedId((id) => id || result.staff[0]?.id || '');
    }).catch((cause: unknown) => { if (!controller.signal.aborted && generation === epoch.current) setLoadError(message(cause)); })
      .finally(() => { if (!controller.signal.aborted && generation === epoch.current) setLoading(false); });
    return () => { controller.abort(); epoch.current += 1; };
  }, [attempt]);

  const people = directory?.staff ?? [];
  const selected = people.find((person) => person.id === selectedId);
  const search = query.trim().toLocaleLowerCase('ru');
  const filtered = people.filter((person) => {
    // Role and club must match the same assignment, not two unrelated roles.
    const assignmentMatches = (!role && !clubId) || person.assignments.some((assignment) => (!role || assignment.role === role) && (!clubId || assignment.clubId === clubId));
    const stateMatches = !access || (access === 'disabled' ? person.disabled : access === 'enabled' ? !person.disabled : person.assignments.length === 0);
    return assignmentMatches && stateMatches && `${person.displayName} ${person.login}`.toLocaleLowerCase('ru').includes(search);
  });
  const selectedHidden = selected && !filtered.some((person) => person.id === selected.id);
  const unavailable = loading || !!loadError || busy;
  const clearFilters = () => { setQuery(''); setRole(''); setClubId(''); setAccess(''); };
  const focusCard = () => requestAnimationFrame(() => {
    if (!root.current?.isConnected) return;
    heading.current?.focus({ preventScroll: true });
    if (window.matchMedia('(max-width: 1100px)').matches) window.scrollTo({ top: 0, behavior: 'instant' });
  });
  function choose(person: StaffCard) {
    if (unavailable) return;
    beforeNavigate(() => { listScroll.current = window.scrollY; setSelectedId(person.id); setDetailOpen(true); setError(''); setNotice(''); focusCard(); });
  }
  function back() {
    setDetailOpen(false);
    requestAnimationFrame(() => {
      const row = root.current?.querySelector<HTMLButtonElement>(`[data-person-id="${selectedId}"]`);
      (row ?? listHeading.current)?.focus({ preventScroll: true }); window.scrollTo({ top: listScroll.current, behavior: 'instant' });
    });
  }
  function open(value: Modal) {
    if (unavailable) return;
    beforeNavigate(() => { setError(''); setNotice(''); setConflict(false); setModal(value); });
  }
  function cancel() { if (!pending.current) beforeNavigate(() => { setModal(null); setError(''); setConflict(false); }); }
  function reload() {
    if (pending.current) return;
    beforeNavigate(() => { setModal(null); setError(''); setConflict(false); setAttempt((value) => value + 1); });
  }
  async function mutate(operation: () => Promise<{ id?: string }>, success: string) {
    if (pending.current || unavailable) return;
    pending.current = true; setBusy(true); setError(''); setNotice(''); setConflict(false);
    const generation = epoch.current;
    try {
      const result = await operation();
      if (generation !== epoch.current) return;
      const id = result.id ?? modal?.target?.id ?? selectedId;
      setSelectedId(id); setDetailOpen(true); setModal(null);
      // The mutation succeeded. Disable further actions if the confirmed new
      // directory cannot be read; never keep using an old version or recipient.
      try {
        const updated = await apiGet<StaffDirectory>('/api/staff-directory');
        if (generation !== epoch.current) return;
        setDirectory(updated); setNotice(success); setLoadError('');
        requestAnimationFrame(() => {
          if (!(document.activeElement instanceof HTMLElement) || document.activeElement === document.body || !document.activeElement.isConnected) heading.current?.focus({ preventScroll: true });
        });
      } catch (cause) {
        if (generation !== epoch.current) return;
        setLoadError(`Изменение сохранено, но актуальный список не загружен. ${message(cause)}`);
      }
      if (generation === epoch.current) await onSessionRefresh();
    } catch (cause) {
      if (generation !== epoch.current) return;
      setError(message(cause)); setConflict(cause instanceof ApiError && cause.code === 'version_conflict');
    } finally {
      pending.current = false;
      if (generation === epoch.current) setBusy(false);
    }
  }
  function accountBase(person: StaffCard): `/api/${string}` {
    return directory?.canAssignExisting ? `/api/staff/${person.id}` : `/api/clubs/${person.assignments[0].clubId}/trainers/${person.id}`;
  }
  function assign(person: StaffCard, draft: StaffDraft) {
    return apiPost<{ id?: string }>(`/api/staff/${person.id}/${draft.role === 'trainer' ? 'trainer' : 'administrator'}-assignments`, { clubId: draft.clubId, version: person.version });
  }
  async function save(draft: StaffDraft) {
    if (!modal || !directory) return;
    const person = modal.target;
    if (modal.mode === 'add') {
      if (draft.existingId) {
        const existing = draft.existingAccount;
        if (existing) await mutate(async () => { await assign(existing, draft); return { id: existing.id }; }, 'Назначение добавлено существующему сотруднику.');
      } else {
        await mutate(() => apiPost(draft.role === 'trainer' ? `/api/clubs/${draft.clubId}/trainers` : '/api/staff', { login: draft.login, displayName: draft.displayName, password: draft.password, ...(draft.role === 'administrator' ? { clubIds: [draft.clubId] } : {}) }), 'Сотрудник создан с первоначальным назначением. Передайте ему пароль по защищённому каналу.');
      }
    } else if (person && modal.mode === 'assignment') {
      await mutate(() => assign(person, draft), 'Назначение добавлено. Остальные роли сохранены.');
    } else if (person && modal.mode === 'account') {
      await mutate(() => apiPut(`${accountBase(person)}/account`, { login: draft.login, displayName: draft.displayName, disabled: person.disabled, version: person.version }), 'Данные сотрудника сохранены. При смене логина прежние сессии завершены.');
    } else if (person && modal.mode === 'password') {
      await mutate(() => apiPost(`${accountBase(person)}/password`, { password: draft.password, version: person.version }), 'Пароль изменён. Прежние сессии завершены.');
    }
  }
  function confirm() {
    const person = modal?.target; if (!person) return;
    if (modal.mode === 'disable') {
      void mutate(() => apiPut(`${accountBase(person)}/account`, { login: person.login, displayName: person.displayName, disabled: !person.disabled, version: person.version }), person.disabled ? 'Аккаунт включён. Сотруднику нужно войти заново.' : 'Весь доступ отключён. Прежние сессии завершены.');
    } else if (modal.mode === 'revoke' && modal.assignment) {
      const assignment = modal.assignment;
      void mutate(() => apiPost(assignment.role === 'trainer' ? `/api/clubs/${assignment.clubId}/trainers/${person.id}/revoke` : `/api/staff/${person.id}/administrator-assignments/revoke`, { version: person.version, ...(assignment.role === 'administrator' ? { clubId: assignment.clubId } : {}) }), 'Назначение снято. Аккаунт, остальные роли и история сохранены.');
    }
  }
  return <section className="staff-access" ref={root} aria-label="Управление сотрудниками" aria-busy={busy} data-detail-open={detailOpen}>
    <div className="staff-access-top"><p className="help">Один аккаунт сотрудника — все его роли и филиалы.</p><button disabled={unavailable || !directory} onClick={() => open({ mode: 'add', target: null })}><Plus aria-hidden="true" />Добавить сотрудника</button></div>
    {notice && <p className="notice staff-access-feedback" role="status">{notice}</p>}
    {loadError && <div className="staff-access-feedback" role="alert"><p>{loadError}</p><button className="secondary" onClick={reload}>Повторить загрузку сотрудников</button></div>}
    {loading && <p role="status">Загружаем сотрудников и назначения…</p>}
    {directory && <div className="staff-access-workbench">
      <div className="staff-access-roster">
        <div className="staff-access-toolbar">
          <h2 ref={listHeading} tabIndex={-1} className="sr-only">Список сотрудников</h2>
          <label className="staff-access-search"><Search aria-hidden="true" /><span className="sr-only">Поиск по имени или логину</span><input type="search" placeholder="Имя или логин" value={query} disabled={busy} onChange={(event) => setQuery(event.target.value)} /></label>
          <button className="secondary staff-access-filter-toggle" aria-expanded={filtersOpen} aria-controls={`${filterId}-filters`} disabled={busy} onClick={() => setFiltersOpen((value) => !value)}>Фильтры{[role, clubId, access].filter(Boolean).length ? ` · ${[role, clubId, access].filter(Boolean).length} активных` : ''}<ChevronRight aria-hidden="true" /></button>
          <div className="staff-access-filters" id={`${filterId}-filters`} data-expanded={filtersOpen}>
            <div className="field"><label htmlFor={`${filterId}-role`}>Роль</label><select id={`${filterId}-role`} value={role} disabled={busy} onChange={(event) => setRole(event.target.value)}><option value="">Все роли</option><option value="trainer">Тренер</option>{directory.canAssignExisting && <option value="administrator">Администратор</option>}</select></div>
            <div className="field"><label htmlFor={`${filterId}-club`}>Филиал сотрудника</label><select id={`${filterId}-club`} value={clubId} disabled={busy} onChange={(event) => setClubId(event.target.value)}><option value="">Все филиалы</option>{directory.clubs.map((club) => <option key={club.id} value={club.id}>{club.name}{club.archived ? ' (архив)' : ''}</option>)}</select></div>
            <div className="field"><label htmlFor={`${filterId}-access`}>Состояние доступа</label><select id={`${filterId}-access`} value={access} disabled={busy} onChange={(event) => setAccess(event.target.value)}><option value="">Любое состояние</option><option value="enabled">Аккаунт включён</option><option value="disabled">Аккаунт отключён</option><option value="unassigned">Без назначений</option></select></div>
          </div>
          <div className="staff-access-count"><span role="status">Показано {filtered.length} из {people.length}</span>{(query || role || clubId || access) && <button className="secondary" disabled={busy} onClick={clearFilters}>Сбросить фильтры</button>}</div>
        </div>
        {filtered.length === 0 && <div className="staff-access-empty"><h3>{people.length ? 'Сотрудники не найдены' : 'Пока нет сотрудников'}</h3><p>{people.length ? 'Измените поиск или сбросьте фильтры.' : 'Добавьте сотрудника и выдайте первое назначение в действующий филиал.'}</p></div>}
        <ul className="staff-access-list">{filtered.map((person) => <li key={person.id}><button className="staff-access-person" data-person-id={person.id} aria-current={person.id === selectedId ? 'true' : undefined} disabled={unavailable} onClick={() => choose(person)}>
          <PersonAvatar name={person.displayName} active={!person.disabled} label={accountState(person)} /><span className="staff-access-person-copy"><strong>{person.displayName}</strong><small>{person.login}</small><small>{[...new Set(person.assignments.map((assignment) => staffRoles[assignment.role]))].join(', ') || 'Без назначений'}{person.assignments.length ? ` · ${new Set(person.assignments.map((assignment) => assignment.clubId)).size} фил.` : ''}</small><small className={person.disabled ? 'staff-access-warning' : ''}>{accountState(person)}</small></span><ChevronRight aria-hidden="true" />
        </button></li>)}</ul>
        <p className="staff-access-foot">{directory.canAssignExisting ? 'Аккаунт общий для сети. Каждое назначение задаёт одну роль в одном филиале.' : 'Здесь только тренеры ваших филиалов. Общие аккаунты меняет владелец.'}</p>
      </div>
      <aside className="staff-access-card" aria-label="Карточка доступа">
        <button className="secondary staff-access-back" onClick={back} disabled={busy}><ArrowLeft aria-hidden="true" />К списку сотрудников</button>
        {selected ? <>
          <header className="staff-access-card-head"><PersonAvatar name={selected.displayName} active={!selected.disabled} label={accountState(selected)} /><div><h2 tabIndex={-1} ref={heading}>{selected.displayName}</h2><span className={`staff-access-state ${selected.disabled || !selected.assignments.length ? 'staff-access-warning' : ''}`}>{accountState(selected)}</span></div></header>
          {selectedHidden && <p className="staff-access-filter-notice" role="status">Выбранный сотрудник не соответствует фильтрам. Карточка по-прежнему относится к {selected.displayName}. <button className="secondary" onClick={clearFilters} disabled={busy}>Показать в списке</button></p>}
          <div className="staff-access-account"><div><span className="help">Логин</span><p>{selected.login}</p></div>{selected.canManageAccount && <button className="secondary" disabled={unavailable} onClick={() => open({ mode: 'account', target: selected })}>Изменить данные</button>}</div>
          <section className="staff-access-card-section"><h3>Управление доступом</h3>
            {selected.canManageAccount ? <>
              <button className="staff-access-action" disabled={unavailable} onClick={() => open({ mode: 'password', target: selected })}><KeyRound aria-hidden="true" /><span>Сменить пароль</span><ChevronRight aria-hidden="true" /></button>
              <button className={`staff-access-action ${!selected.disabled ? 'staff-access-danger' : ''}`} disabled={unavailable} onClick={() => open({ mode: 'disable', target: selected })}>{selected.disabled ? <UnlockKeyhole aria-hidden="true" /> : <LockKeyhole aria-hidden="true" />}<span>{selected.disabled ? 'Включить доступ' : 'Отключить весь доступ'}</span><ChevronRight aria-hidden="true" /></button>
              <p className="help">Отключение запрещает вход и завершает все сессии аккаунта. Назначения и история сохраняются. Включение требует нового входа.</p>
            </> : <p className="help">Данные, пароль и состояние этого аккаунта меняет владелец. Вы можете снять доступное назначение своего филиала.</p>}
          </section>
          <section className="staff-access-card-section"><h3>Роли и филиалы <span className="help">{selected.assignments.length}</span></h3><p className="help">Каждое назначение даёт роль в одном филиале.</p>
            {selected.assignments.length === 0 && <p>Назначений нет. Рабочие разделы недоступны; аккаунт и история сохранены.</p>}
            <ul className="staff-access-assignments">{selected.assignments.map((assignment) => <li key={`${assignment.role}:${assignment.clubId}`}><div><strong>{assignment.clubName}{assignment.archived ? ' (архив)' : ''}</strong><p>{staffRoles[assignment.role]}</p>{assignment.role === 'trainer' && <button className="staff-access-profile" disabled={unavailable} onClick={() => open({ mode: 'profile', target: selected, assignment })}>Занятия и группы</button>}</div>{assignment.canRevoke && <button className="secondary staff-access-revoke" disabled={unavailable} aria-label={`Снять назначение: ${assignmentLabel(assignment)}`} onClick={() => open({ mode: 'revoke', target: selected, assignment })}><CircleMinus aria-hidden="true" /></button>}</li>)}</ul>
            {directory.canAssignExisting && <button className="secondary staff-access-add-role" disabled={unavailable} onClick={() => open({ mode: 'assignment', target: selected })}><Plus aria-hidden="true" />Добавить назначение</button>}
          </section>
        </> : <div className="staff-access-empty"><h2 ref={heading} tabIndex={-1}>{selectedId ? 'Сотрудник больше недоступен' : 'Выберите сотрудника'}</h2><p>{selectedId ? 'Его назначения или ваши права изменились. Выберите запись в списке; действия для прежнего сотрудника закрыты.' : 'Рядом откроется карточка с данными, ролями и управлением доступом.'}</p></div>}
      </aside>
    </div>}
    {directory && <button className="secondary staff-access-refresh" disabled={busy || loading} onClick={reload}>Обновить сотрудников</button>}
    {modal && directory && <Dialog title={modal.mode === 'disable' ? (modal.target?.disabled ? 'Включить доступ' : 'Отключить весь доступ') : titles[modal.mode]} className="staff-access-dialog" busy={busy} onCancel={cancel} cancelLabel="Закрыть форму сотрудника">
      {modal.target && <p className="staff-access-recipient"><strong>{modal.target.displayName}</strong><br />{modal.target.login}</p>}
      {error && <p className="error" role="alert">{error}</p>}
      {conflict && <p className="help">Не повторяйте действие со старой версией. <button className="secondary" disabled={busy} onClick={reload}>Загрузить актуальные данные</button></p>}
      {['add', 'account', 'password', 'assignment'].includes(modal.mode) && <StaffEditor mode={modal.mode as StaffEditorMode} target={modal.target} directory={directory} busy={busy} conflict={conflict} onSubmit={save} onCancel={cancel} />}
      {modal.mode === 'disable' && <><p>{modal.target?.disabled ? 'Сотрудник сможет войти заново. Старые сессии не восстановятся; назначения останутся прежними.' : 'Будет запрещён вход во все филиалы и семейный кабинет этого аккаунта. Все сессии завершатся. Сотрудник, назначения и история сохранятся.'}</p><div className="actions"><button disabled={busy || conflict} onClick={confirm}>{busy ? 'Сохраняем…' : modal.target?.disabled ? 'Подтвердить включение' : 'Подтвердить отключение'}</button><button className="secondary" disabled={busy} onClick={cancel}>Отмена</button></div></>}
      {modal.mode === 'revoke' && <><p><strong>{modal.assignment && assignmentLabel(modal.assignment)}</strong></p><p>Снимется только это назначение. Остальные роли, аккаунт и история сохранятся. Последнее назначение закроет рабочие разделы, но не отключит вход. Будущие занятия автоматически не переназначаются и не отменяются.</p><div className="actions"><button disabled={busy || conflict} onClick={confirm}>{busy ? 'Снимаем…' : 'Подтвердить снятие назначения'}</button><button className="secondary" disabled={busy} onClick={cancel}>Отмена</button></div></>}
      {modal.mode === 'profile' && modal.target && modal.assignment && <TrainerProfile trainerId={modal.target.id} name={modal.target.displayName} clubId={modal.assignment.clubId} clubName={modal.assignment.clubName} />}
    </Dialog>}
  </section>;
}

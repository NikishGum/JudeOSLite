import { useEffect, useRef, useState } from 'react';
import { ApiError, apiGet, apiPost, apiPut } from '../../shared/api';
import { TrainerEditor, type Draft, type Mode, type Trainer } from './TrainerEditor';
import { Dialog } from '../../shared/ui/Dialog';
import { PersonAvatar } from '../../app/RuntimeShell';
import './staff.css';

const message = (cause: unknown) => cause instanceof ApiError ? cause.message : 'Нет соединения. Проверьте интернет и повторите действие.';
export function TrainerManagement({ clubId, clubName, isOwner, onSessionRefresh, beforeNavigate }: { clubId: string; clubName: string; beforeNavigate: (action: () => void) => void; isOwner: boolean; onSessionRefresh: () => Promise<void> }) {
  const [trainers, setTrainers] = useState<Trainer[]>([]);
  const [roster, setRoster] = useState<Trainer[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [attempt, setAttempt] = useState(0);
  const [selected, setSelected] = useState<Trainer | null>(null);
  const [mode, setMode] = useState<Mode | null>(null);
  const [revoke, setRevoke] = useState<Trainer | null>(null);
  const [candidate, setCandidate] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [conflict, setConflict] = useState(false);
  const [busy, setBusy] = useState(false);
  const pending = useRef(false);
  const epoch = useRef(0);
  const heading = useRef<HTMLHeadingElement>(null);
  const base: `/api/${string}` = `/api/clubs/${clubId}/trainers`;
  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setLoadError(''); setTrainers([]); setRoster([]);
    Promise.all([apiGet<Trainer[]>(base, controller.signal), isOwner ? apiGet<Trainer[]>('/api/trainer-accounts', controller.signal) : Promise.resolve([])])
      .then(([local, all]) => { if (!controller.signal.aborted) { setTrainers(local); setRoster(all); } })
      .catch((cause: unknown) => { if (!controller.signal.aborted) setLoadError(message(cause)); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [base, isOwner, attempt]);
  useEffect(() => () => { epoch.current += 1; }, []);
  useEffect(() => { if (mode || revoke) heading.current?.focus(); }, [mode, revoke]);
  function close() { setMode(null); setSelected(null); setRevoke(null); setError(''); setConflict(false); }
  function open(trainer: Trainer | null, next: Mode) { beforeNavigate(() => { close(); setSelected(trainer); setMode(next); setNotice(''); }); }
  async function mutate(action: () => Promise<unknown>, success: string) {
    if (pending.current) return;
    pending.current = true; setBusy(true); setError(''); setNotice(''); setConflict(false);
    const generation = epoch.current;
    try {
      await action(); if (generation !== epoch.current) return;
      close(); setCandidate(''); setNotice(success); setAttempt((value) => value + 1);
    } catch (cause) {
      if (generation !== epoch.current) return;
      setError(message(cause));
      if (cause instanceof ApiError && (cause.code === 'version_conflict' || cause.code === 'account_owner_required')) setConflict(true);
      if (cause instanceof ApiError && (cause.code === 'csrf_failed' || cause.code === 'club_archived' || cause.status === 404)) await onSessionRefresh();
    } finally { if (generation === epoch.current) { pending.current = false; setBusy(false); } }
  }
  async function save(draft: Draft) {
    if (mode === 'new') await mutate(() => apiPost(base, { login: draft.login, displayName: draft.displayName, password: draft.password }), 'Тренер создан. Передайте ему данные для входа.');
    else if (selected) {
      const path: `/api/${string}` = isOwner ? `/api/staff/${selected.id}` : `${base}/${selected.id}`;
      if (mode === 'password') await mutate(() => apiPost(`${path}/password`, { version: selected.version, password: draft.password }), 'Пароль изменён. Прежние сессии завершены.');
      else await mutate(() => apiPut(`${path}/account`, { login: draft.login, displayName: draft.displayName, disabled: draft.disabled, version: selected.version }), 'Аккаунт сохранён.');
    }
  }
  const employee = roster.find((item) => item.id === candidate);
  return <section className="connection staff-management" aria-labelledby="trainers-title">
    <div className="section-heading"><div><h2 id="trainers-title">Тренеры филиала</h2><p className="help context-name">{clubName}</p></div><button disabled={busy} onClick={() => open(null, 'new')}>Добавить тренера</button></div>
    <p className="help">Для занятий доступны действующие тренеры этого филиала. Отзыв назначения сохраняет сотрудника и историю.</p>
    {notice && <p role="status">{notice}</p>}
    {!mode && error && <p className="error" role="alert">{error}</p>}
    {!mode && conflict && <><p>Правки остались в форме. Обновление списка закроет форму и сбросит их.</p><button className="secondary" disabled={busy} onClick={() => { close(); setAttempt((value) => value + 1); }}>Загрузить актуальный список</button></>}
    {mode && <Dialog className="subject-panel" title={mode === 'new' ? 'Новый тренер' : mode === 'password' ? `Смена пароля: ${selected?.displayName}` : `Аккаунт: ${selected?.displayName}`} busy={busy} cancelLabel="Закрыть форму" onCancel={() => beforeNavigate(close)}>
      <p className="help context-name">{clubName}</p>
      {error && <p className="error" role="alert">{error}</p>}
      {conflict && <><p className="help">Правки остались в форме. Перечитывание сбросит их.</p><button className="secondary" disabled={busy} onClick={() => { close(); setAttempt((value) => value + 1); }}>Загрузить актуальный список</button></>}
      <TrainerEditor key={`${selected?.id ?? 'new'}:${mode}`} trainer={selected} mode={mode} busy={busy} onSave={save} onCancel={close} />
    </Dialog>}
    {revoke && <div className="staff-card" role="region" aria-label="Подтверждение отзыва"><h3 ref={heading} tabIndex={-1}>Отозвать назначение: {revoke.displayName}</h3>
      <p>Тренер потеряет доступ к занятиям этого филиала. Назначения других филиалов и исторический автор отметок сохранятся. Вернуть назначение сможет владелец.</p>
      <div className="actions"><button disabled={busy} onClick={() => void mutate(() => apiPost(`${base}/${revoke.id}/revoke`, { version: revoke.version }), 'Назначение отозвано. История сохранена.')}>Подтвердить отзыв</button><button className="secondary" disabled={busy} onClick={close}>Отмена</button></div>
    </div>}
    {loading && <p role="status">Загружаем тренеров…</p>}
    {loadError && <div role="alert"><p>{loadError}</p><button className="secondary" onClick={() => setAttempt((value) => value + 1)}>Попробовать снова</button></div>}
    {!loading && !loadError && <>
      {trainers.length === 0 && <p>Тренеры ещё не назначены. Добавьте аккаунт для занятий этого филиала{isOwner ? ' или выберите существующего сотрудника ниже' : ''}.</p>}
      <ul className="staff-list">{trainers.map((trainer) => <li key={trainer.id}><div className="staff-identity"><PersonAvatar name={trainer.displayName} active={!trainer.disabled} label={trainer.disabled ? 'Аккаунт отключён' : 'Доступен для занятий'} /><div className="staff-copy"><h3>{trainer.displayName}</h3><p className="staff-role">Тренер · {trainer.login}</p><p className="staff-assignments">{clubName}</p>
        {!trainer.canManageAccount && <p className="help">Аккаунт меняет владелец. Вы управляете назначением этого филиала.</p>}</div></div>
        <div className="actions">{trainer.canManageAccount && <><button className="secondary" disabled={busy} onClick={() => open(trainer, 'account')}>Изменить аккаунт</button><button className="secondary" disabled={busy} onClick={() => open(trainer, 'password')}>Сменить пароль</button></>}
          <button className="secondary" disabled={busy} onClick={() => beforeNavigate(() => { close(); setRevoke(trainer); setNotice(''); })}>Отозвать назначение</button></div>
      </li>)}</ul>
      {isOwner && <div className="staff-card"><h3>Существующий сотрудник</h3><p className="help">Владелец может назначить сотрудника тренером выбранного филиала или изменить его общий аккаунт, в том числе после отзыва всех назначений.</p>
        <div className="field"><label htmlFor="trainer-account-select">Аккаунт сотрудника</label><select id="trainer-account-select" disabled={busy} value={candidate} onChange={(event) => setCandidate(event.target.value)}><option value="">Выберите сотрудника</option>{roster.map((item) => <option key={item.id} value={item.id}>{item.displayName} · {item.login}{item.disabled ? ' · отключён' : ''}</option>)}</select></div>
        {employee && <div className="actions"><button disabled={busy || employee.trainerClubIds?.includes(clubId)} onClick={() => void mutate(() => apiPost(`/api/staff/${employee.id}/trainer-assignments`, { clubId, version: employee.version }), 'Назначение тренером добавлено.')}>{employee.trainerClubIds?.includes(clubId) ? 'Уже назначен' : 'Назначить тренером'}</button>
          <button className="secondary" disabled={busy} onClick={() => open(employee, 'account')}>Изменить аккаунт сотрудника</button><button className="secondary" disabled={busy} onClick={() => open(employee, 'password')}>Сменить пароль сотрудника</button></div>}
      </div>}
    </>}
    <button className="secondary" disabled={busy || loading} onClick={() => { close(); setCandidate(''); setAttempt((value) => value + 1); }}>Обновить тренеров</button>
  </section>;
}

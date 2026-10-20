import { useEffect, useId, useRef, useState, type FormEvent } from 'react';
import { apiGet, ApiError } from '../../shared/api';
import type { StaffCard, StaffDirectory, StaffRole } from './staff-types';

export type StaffDraft = { login: string; displayName: string; password: string; clubId: string; role: StaffRole; existingId: string; existingAccount?: StaffCard };
export type StaffEditorMode = 'add' | 'account' | 'password' | 'assignment';

export function StaffEditor({ mode, target, directory, busy, conflict, onSubmit, onCancel }: {
  mode: StaffEditorMode; target: StaffCard | null; directory: StaffDirectory; busy: boolean; conflict: boolean;
  onSubmit: (draft: StaffDraft) => Promise<void>; onCancel: () => void;
}) {
  const id = useId();
  const [existing, setExisting] = useState(false);
  const [draft, setDraft] = useState<StaffDraft>({ login: target?.login ?? '', displayName: target?.displayName ?? '', password: '', clubId: '', role: 'trainer', existingId: '' });
  const [confirmation, setConfirmation] = useState('');
  const [error, setError] = useState('');
  const [accountQuery, setAccountQuery] = useState('');
  const [found, setFound] = useState<StaffCard[]>([]);
  const [finding, setFinding] = useState(false);
  const [searched, setSearched] = useState(false);
  const lookup = useRef<AbortController | null>(null);
  useEffect(() => () => lookup.current?.abort(), []);
  const needsPassword = mode === 'password' || (mode === 'add' && !existing);
  const needsAssignment = mode === 'add' || mode === 'assignment';
  const candidates = searched ? found : directory.staff;
  const employee = existing ? candidates.find((person) => person.id === draft.existingId) : target;
  const duplicate = !!employee?.assignments.some((assignment) => assignment.clubId === draft.clubId && assignment.role === draft.role);
  const field = (key: keyof StaffDraft, value: string) => setDraft((current) => ({ ...current, [key]: value }));
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy || finding || conflict || duplicate) return;
    if (needsPassword && (draft.password !== confirmation || new TextEncoder().encode(draft.password).length < 12 || new TextEncoder().encode(draft.password).length > 128)) {
      setError(draft.password !== confirmation ? 'Пароли не совпадают.' : 'Пароль должен содержать 12–128 байт.'); return;
    }
    setError('');
    try { await onSubmit({ ...draft, existingId: existing ? draft.existingId : '', existingAccount: existing ? employee ?? undefined : undefined }); }
    finally { setDraft((current) => ({ ...current, password: '' })); setConfirmation(''); }
  }
  async function findAccounts() {
    lookup.current?.abort(); const controller = new AbortController(); lookup.current = controller;
    setFinding(true); setError(''); setFound([]); setSearched(false); field('existingId', '');
    try {
      const result = await apiGet<StaffCard[]>(`/api/staff-candidates?query=${encodeURIComponent(accountQuery.trim())}`, controller.signal);
      if (!controller.signal.aborted) { setFound(result); setSearched(true); }
    } catch (cause) {
      if (!controller.signal.aborted) setError(cause instanceof ApiError ? cause.message : 'Не удалось найти аккаунт. Проверьте соединение и повторите поиск.');
    } finally { if (!controller.signal.aborted) setFinding(false); }
  }
  return <form className="staff-access-form" onSubmit={(event) => void submit(event)} aria-busy={busy}>
    <fieldset disabled={busy}>
      {mode === 'add' && directory.canAssignExisting && <div className="field"><label htmlFor={`${id}-method`}>Способ добавления</label><select id={`${id}-method`} value={existing ? 'existing' : 'new'} onChange={(event) => { lookup.current?.abort(); setFinding(false); setFound([]); setSearched(false); setExisting(event.target.value === 'existing'); setDraft((current) => ({ ...current, password: '', existingId: '' })); setConfirmation(''); }}>
        <option value="new">Новый аккаунт</option><option value="existing">Существующий аккаунт</option>
      </select></div>}
      {mode === 'add' && existing && <>
        <label className="field">Найти существующий аккаунт<input type="search" maxLength={100} value={accountQuery} placeholder="Имя или логин, от 3 символов" autoComplete="off" onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); if (!finding && accountQuery.trim().length >= 3) void findAccounts(); } }} onChange={(event) => { lookup.current?.abort(); setFinding(false); setFound([]); setSearched(false); field('existingId', ''); setAccountQuery(event.target.value); }} /></label>
        <button type="button" className="secondary" disabled={finding || accountQuery.trim().length < 3} onClick={() => void findAccounts()}>{finding ? 'Ищем…' : 'Найти аккаунты'}</button>
        <p className="help">Здесь можно найти в том числе аккаунт родителя. Назначение добавит ему рабочую роль; остальные доступы сохранятся. Проверяйте имя и логин выбранного человека.</p>
        {searched && <p role="status">По поиску найдено: {found.length}{found.length === 30 ? '. Показаны первые 30 — уточните имя или логин.' : '.'}</p>}
        <div className="field"><label htmlFor={`${id}-person`}>Аккаунт сотрудника</label><select id={`${id}-person`} required disabled={finding} value={draft.existingId} onChange={(event) => field('existingId', event.target.value)}>
          <option value="">Выберите аккаунт</option>{candidates.map((person) => <option key={person.id} value={person.id}>{person.displayName} · {person.login}{person.disabled ? ' · отключён' : ''}</option>)}
        </select>{employee && <p className="staff-access-selected" aria-live="polite"><span className="help">Выбранный аккаунт</span><br /><strong>{employee.displayName}</strong><br />Логин: {employee.login}</p>}<p className="help">Добавится назначение выбранному аккаунту. Имя, логин и пароль сохранятся.</p></div>
      </>}
      {(mode === 'account' || (mode === 'add' && !existing)) && <>
        <label className="field">Имя сотрудника<input required maxLength={200} autoComplete="off" value={draft.displayName} onChange={(event) => field('displayName', event.target.value)} /></label>
        <label className="field">Логин сотрудника<input required maxLength={100} pattern="[a-zA-Z0-9][a-zA-Z0-9._@+\-]*" autoCapitalize="none" autoComplete="off" spellCheck={false} value={draft.login} onChange={(event) => field('login', event.target.value)} /></label>
        <p className="help">Латинские буквы, цифры и . _ @ + −. {mode === 'account' && 'Смена логина завершит все прежние сессии.'}</p>
      </>}
      {needsPassword && <>
        <label className="field">{mode === 'password' ? 'Новый пароль' : 'Первоначальный пароль'}<input type="password" required maxLength={128} autoComplete="new-password" value={draft.password} onChange={(event) => field('password', event.target.value)} aria-describedby={`${id}-password-help`} /></label>
        <label className="field">Повторите пароль<input type="password" required maxLength={128} autoComplete="new-password" value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></label>
        <p className="help" id={`${id}-password-help`}>12–128 байт. Передайте пароль сотруднику лично или по защищённому каналу. Поля очищаются после отправки. {mode === 'password' && 'Все прежние сессии будут завершены.'}</p>
      </>}
      {needsAssignment && <>
        <div className="field"><label htmlFor={`${id}-role`}>Роль</label><select id={`${id}-role`} value={draft.role} onChange={(event) => field('role', event.target.value)}><option value="trainer">Тренер</option>{directory.canAssignExisting && <option value="administrator">Администратор</option>}</select></div>
        <div className="field"><label htmlFor={`${id}-club`}>Филиал назначения</label><select id={`${id}-club`} required value={draft.clubId} onChange={(event) => field('clubId', event.target.value)}><option value="">Выберите филиал</option>{directory.clubs.filter((club) => !club.archived).map((club) => <option key={club.id} value={club.id}>{club.name}</option>)}</select></div>
        {duplicate && <p role="status">Это назначение уже есть. Выберите другую роль или филиал.</p>}
        {employee?.disabled && <p className="help">Аккаунт отключён. Новое назначение не включит вход — доступ включается отдельно в карточке.</p>}
        {!directory.clubs.some((club) => !club.archived) && <p role="status">Нет доступных действующих филиалов для назначения.</p>}
      </>}
      {error && <p className="error" role="alert">{error}</p>}
      <div className="actions"><button type="submit" disabled={finding || conflict || duplicate || (needsAssignment && !directory.clubs.some((club) => !club.archived))}>{busy ? 'Сохраняем…' : mode === 'add' ? (existing ? 'Добавить назначение' : 'Создать сотрудника') : mode === 'assignment' ? 'Добавить назначение' : mode === 'password' ? 'Сменить пароль' : 'Сохранить данные'}</button><button type="button" className="secondary" onClick={onCancel}>Отмена</button></div>
    </fieldset>
  </form>;
}

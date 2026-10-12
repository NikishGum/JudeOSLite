import { useState, type FormEvent } from 'react';

export type Trainer = { id: string; login: string; displayName: string; disabled: boolean; version: number; canManageAccount: boolean; trainerClubIds?: string[] };
export type Draft = { login: string; displayName: string; disabled: boolean; password: string };
export type Mode = 'new' | 'account' | 'password';

export function TrainerEditor({ trainer, mode, busy, onSave, onCancel }: { trainer: Trainer | null; mode: Mode; busy: boolean; onSave: (draft: Draft) => Promise<void>; onCancel: () => void }) {
  const [login, setLogin] = useState(trainer?.login ?? '');
  const [displayName, setDisplayName] = useState(trainer?.displayName ?? '');
  const [disabled, setDisabled] = useState(trainer?.disabled ?? false);
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [error, setError] = useState('');
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (busy) return;
    if (mode !== 'account' && password !== confirmation) { setError('Пароли не совпадают.'); return; }
    setError('');
    try { await onSave({ login, displayName, disabled, password }); }
    finally { setPassword(''); setConfirmation(''); }
  }
  return <form onSubmit={(event) => void submit(event)} aria-busy={busy}>
    <fieldset disabled={busy}>
      {mode !== 'password' && <>
        <label className="field">Имя тренера<input required maxLength={200} value={displayName} onChange={(event) => setDisplayName(event.target.value)} autoComplete="off" /></label>
        <label className="field">Логин тренера<input required maxLength={100} value={login} onChange={(event) => setLogin(event.target.value)} autoComplete="off" autoCapitalize="none" spellCheck={false} /></label>
      </>}
      {mode !== 'account' && <>
        <label className="field">{mode === 'new' ? 'Первоначальный пароль' : 'Новый пароль'}<input type="password" required minLength={12} maxLength={128} autoComplete="new-password" value={password} onChange={(event) => setPassword(event.target.value)} /></label>
        <label className="field">Повторите пароль<input type="password" required minLength={12} maxLength={128} autoComplete="new-password" value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></label>
        <p className="help">12–128 байт. Передайте пароль сотруднику лично или по защищённому каналу. После отправки поля очищаются.</p>
        {mode === 'password' && <p>Все прежние сессии будут завершены. Сотрудник войдёт с новым паролем.</p>}
      </>}
      {mode === 'account' && <>
        <label className="staff-checkbox"><input type="checkbox" checked={disabled} onChange={(event) => setDisabled(event.target.checked)} />Отключить аккаунт</label>
        <p className="help">Отключение завершает все сессии. Смена логина также требует нового входа. Назначения и история сохраняются.</p>
      </>}
      {error && <p className="error" role="alert">{error}</p>}
      <div className="actions"><button type="submit">{busy ? 'Сохраняем…' : mode === 'new' ? 'Создать тренера' : 'Сохранить'}</button><button type="button" className="secondary" onClick={onCancel}>Отмена</button></div>
    </fieldset>
  </form>;
}

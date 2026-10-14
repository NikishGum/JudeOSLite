import { useEffect, useRef, useState, type FormEvent } from 'react';
import { ApiError, apiGet, apiPost, apiPut } from '../../shared/api';
import './clubs.css';

type Club = { id: string; name: string; address: string; timezone: string; archived: boolean; version: number };
type Staff = { id: string; login: string; displayName: string; disabled: boolean; clubIds: string[]; version: number };
type ClubDraft = { name: string; address: string; timezone: string };
type StaffDraft = { login: string; displayName: string; password: string; clubIds: string[]; disabled: boolean };
const emptyClub: ClubDraft = { name: '', address: '', timezone: 'Europe/Minsk' };
const emptyStaff: StaffDraft = { login: '', displayName: '', password: '', clubIds: [], disabled: false };
const message = (cause: unknown) => cause instanceof ApiError ? cause.message : 'Нет соединения. Проверьте интернет и повторите действие.';

export function OwnerManagement({ onChanged, mode = 'clubs' }: { mode?: 'clubs' | 'staff'; onChanged: () => Promise<void> }) {
  const [clubs, setClubs] = useState<Club[]>([]);
  const [staff, setStaff] = useState<Staff[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [attempt, setAttempt] = useState(0);
  const [busy, setBusy] = useState(false);
  const [clubEditor, setClubEditor] = useState<Club | 'new' | null>(null);
  const [clubDraft, setClubDraft] = useState(emptyClub);
  const [staffEditor, setStaffEditor] = useState<Staff | 'new' | null>(null);
  const [staffDraft, setStaffDraft] = useState(emptyStaff);
  const [archive, setArchive] = useState<Club | null>(null);
  const pending = useRef(false);
  const epoch = useRef(0);
  const editorRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setLoadError('');
    Promise.all([apiGet<Club[]>('/api/clubs', controller.signal), apiGet<Staff[]>('/api/staff', controller.signal)])
      .then(([clubList, staffList]) => { if (!controller.signal.aborted) { setClubs(clubList); setStaff(staffList); } })
      .catch((cause: unknown) => { if (!controller.signal.aborted) setLoadError(message(cause)); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [attempt]);
  useEffect(() => () => { epoch.current += 1; }, []);
  useEffect(() => { if (clubEditor || staffEditor || archive) editorRef.current?.focus(); }, [clubEditor, staffEditor, archive]);

  function closeEditor() { setClubEditor(null); setStaffEditor(null); setArchive(null); setStaffDraft(emptyStaff); }
  function editClub(value: Club | 'new') {
    closeEditor(); setClubEditor(value); setError(''); setNotice('');
    setClubDraft(value === 'new' ? emptyClub : { name: value.name, address: value.address, timezone: value.timezone });
  }
  function editStaff(value: Staff | 'new') {
    closeEditor(); setStaffEditor(value); setError(''); setNotice('');
    setStaffDraft(value === 'new' ? emptyStaff : { login: value.login, displayName: value.displayName, password: '', clubIds: value.clubIds, disabled: value.disabled });
  }
  async function save(action: () => Promise<unknown>, success: string) {
    if (pending.current) return;
    pending.current = true; setBusy(true); setError(''); setNotice('');
    const generation = epoch.current;
    try {
      await action(); if (generation !== epoch.current) return;
      closeEditor(); setNotice(success); setAttempt((value) => value + 1); await onChanged();
    } catch (cause) {
      if (generation !== epoch.current) return;
      setError(message(cause) + (cause instanceof ApiError && cause.code === 'version_conflict' ? ' Закройте форму и откройте запись заново, чтобы увидеть актуальные данные.' : ''));
      // Keep the unsaved draft on conflict; the user reopens the refreshed record.
      if (cause instanceof ApiError && cause.status === 409) setAttempt((value) => value + 1);
      if (cause instanceof ApiError && cause.code === 'csrf_failed') await onChanged();
    } finally {
      if (generation === epoch.current) { pending.current = false; setBusy(false); setStaffDraft((value) => ({ ...value, password: '' })); }
    }
  }
  function saveClub(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!clubEditor) return;
    void save(() => clubEditor === 'new' ? apiPost('/api/clubs', clubDraft) : apiPut(`/api/clubs/${clubEditor.id}`, { ...clubDraft, version: clubEditor.version }), 'Филиал сохранён.');
  }
  function saveStaff(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!staffEditor) return;
    void save(() => staffEditor === 'new' ? apiPost('/api/staff', { login: staffDraft.login, displayName: staffDraft.displayName, password: staffDraft.password, clubIds: staffDraft.clubIds }) :
      apiPut(`/api/staff/${staffEditor.id}`, { displayName: staffDraft.displayName, clubIds: staffDraft.clubIds, disabled: staffDraft.disabled, version: staffEditor.version }), 'Аккаунт и назначения сохранены. Доступ обновлён.');
  }
  function toggleClub(id: string, checked: boolean) { setStaffDraft((value) => ({ ...value, clubIds: checked ? [...value.clubIds, id] : value.clubIds.filter((item) => item !== id) })); }

  return <section className="connection management" aria-labelledby="management-title">
    <div className="section-heading"><div><p className="eyebrow">Управление сетью</p><h2 id="management-title">{mode === 'clubs' ? 'Филиалы' : 'Администраторы сети'}</h2></div>
      <button className="secondary" disabled={busy || loading} onClick={() => setAttempt((value) => value + 1)}>Обновить список</button></div>
    {error && <p className="error" role="alert">{error}</p>}
    {notice && <p role="status">{notice}</p>}
    {loading && <p role="status">Загружаем филиалы и сотрудников…</p>}
    {loadError && <div role="alert"><p>{loadError}</p><button className="secondary" disabled={busy} onClick={() => setAttempt((value) => value + 1)}>Попробовать снова</button></div>}

    {clubEditor && <form className="editor" onSubmit={saveClub} aria-busy={busy}>
      <h3 ref={editorRef} tabIndex={-1}>{clubEditor === 'new' ? 'Новый филиал' : 'Редактирование филиала'}</h3>
      <fieldset disabled={busy}>
        <label className="field">Название филиала<input required maxLength={200} value={clubDraft.name} onChange={(event) => setClubDraft({ ...clubDraft, name: event.target.value })} /></label>
        <label className="field">Адрес зала<input maxLength={500} value={clubDraft.address} onChange={(event) => setClubDraft({ ...clubDraft, address: event.target.value })} /></label>
        <label className="field">Часовой пояс<input required maxLength={100} aria-describedby="timezone-help" value={clubDraft.timezone} onChange={(event) => setClubDraft({ ...clubDraft, timezone: event.target.value })} /></label>
        <p id="timezone-help" className="help">Название зоны, например Europe/Minsk. Время занятий будет отображаться в этой зоне.</p>
        <div className="actions"><button type="submit">{busy ? 'Сохраняем…' : 'Сохранить филиал'}</button><button className="secondary" type="button" onClick={closeEditor}>Отмена</button></div>
      </fieldset>
    </form>}

    {staffEditor && <form className="editor" onSubmit={saveStaff} aria-busy={busy}>
      <h3 ref={editorRef} tabIndex={-1}>{staffEditor === 'new' ? 'Новый администратор' : 'Аккаунт и назначения'}</h3>
      <fieldset disabled={busy}>
        <label className="field">Имя сотрудника<input required maxLength={200} value={staffDraft.displayName} onChange={(event) => setStaffDraft({ ...staffDraft, displayName: event.target.value })} /></label>
        {staffEditor === 'new' ? <>
          <label className="field">Логин администратора<input required maxLength={100} autoCapitalize="none" autoComplete="off" spellCheck={false} value={staffDraft.login} onChange={(event) => setStaffDraft({ ...staffDraft, login: event.target.value })} /></label>
          <label className="field">Первоначальный пароль<input required type="password" minLength={12} maxLength={128} autoComplete="new-password" value={staffDraft.password} onChange={(event) => setStaffDraft({ ...staffDraft, password: event.target.value })} aria-describedby="password-help" /></label>
          <p id="password-help" className="help">12–128 байт. Передайте пароль сотруднику лично или по защищённому каналу. После запроса поле очищается.</p>
        </> : <><p>Логин: <strong>{staffEditor.login}</strong></p><label className="check"><input type="checkbox" checked={staffDraft.disabled} onChange={(event) => setStaffDraft({ ...staffDraft, disabled: event.target.checked })} />Отключить аккаунт во всех филиалах</label>
          <p className="help">Отключение сразу завершает все сессии. Повторное включение потребует нового входа. История сохраняется.</p></>}
        <fieldset className="assignments"><legend>Филиалы администратора</legend>
          {clubs.length === 0 && <p>Сначала добавьте филиал.</p>}
          {clubs.map((item) => <label className="check" key={item.id}><input type="checkbox" checked={staffDraft.clubIds.includes(item.id)} disabled={item.archived && (staffEditor === 'new' || !staffEditor.clubIds.includes(item.id))} onChange={(event) => toggleClub(item.id, event.target.checked)} />
            <span>{item.name}{item.archived ? ' · В архиве' : ''}</span></label>)}
        </fieldset>
        <p className="help">Можно назначить несколько филиалов. Снятие назначения сразу закрывает права администратора в филиале; роль тренера, если есть, сохраняется.</p>
        <div className="actions"><button type="submit" disabled={staffEditor === 'new' && staffDraft.clubIds.length === 0}>{busy ? 'Сохраняем…' : 'Сохранить администратора'}</button><button className="secondary" type="button" onClick={closeEditor}>Отмена</button></div>
      </fieldset>
    </form>}

    {archive && <div className="editor" role="region" aria-labelledby="archive-title">
      <h3 id="archive-title" ref={editorRef} tabIndex={-1}>Архивировать «{archive.name}»?</h3>
      <p>Филиал исчезнет из рабочего выбора. Новые назначения и будущие занятия будут запрещены. Сотрудники, назначения и история сохранятся; существующие занятия не удаляются и не отменяются автоматически.</p>
      <div className="actions"><button disabled={busy} onClick={() => void save(() => apiPost(`/api/clubs/${archive.id}/archive`, { version: archive.version }), 'Филиал перенесён в архив. История сохранена.')}>{busy ? 'Архивируем…' : 'Подтвердить архивирование'}</button><button className="secondary" disabled={busy} onClick={closeEditor}>Отмена</button></div>
    </div>}

    {mode === 'clubs' && <><div className="section-heading"><h3>Филиалы</h3><button disabled={busy || loading || !!loadError} onClick={() => editClub('new')}>Добавить филиал</button></div>
    {!loading && !loadError && clubs.length === 0 && <p>Каждый зал добавляется как отдельный филиал.</p>}
    <ul className="management-list">{clubs.map((item) => <li key={item.id}><div><h4>{item.name}</h4><p>{item.address || 'Адрес не указан'}<br />{item.timezone} · {item.archived ? 'В архиве' : 'Действующий'}</p></div><div className="actions">
      <button className="secondary" disabled={busy || loading} onClick={() => editClub(item)} aria-label={`Редактировать филиал ${item.name}`}>Редактировать</button>
      {!item.archived && <button className="secondary" disabled={busy || loading} onClick={() => { closeEditor(); setError(''); setNotice(''); setArchive(item); }} aria-label={`Архивировать филиал ${item.name}`}>В архив</button>}</div></li>)}</ul>
    </>}
    {mode === 'staff' && <><div className="section-heading"><h3>Сотрудники и назначения</h3><button disabled={busy || loading || !!loadError || !clubs.some((item) => !item.archived)} onClick={() => editStaff('new')}>Создать администратора</button></div>
    {!loading && !loadError && staff.length === 0 && <p>Создайте отдельный аккаунт для каждого сотрудника.</p>}
    <ul className="management-list">{staff.map((item) => <li key={item.id}><div><h4>{item.displayName}</h4><p>{item.login} · {item.disabled ? 'Аккаунт отключён' : 'Аккаунт включён'}</p><p>Администратор: {item.clubIds.map((id) => { const club = clubs.find((value) => value.id === id); return `${club?.name ?? 'Филиал'}${club?.archived ? ' (архив)' : ''}`; }).join(', ') || 'нет назначений'}</p></div>
      <button className="secondary" disabled={busy || loading} onClick={() => editStaff(item)} aria-label={`Изменить назначения ${item.displayName}`}>Изменить назначения</button></li>)}</ul>
    </>}
  </section>;
}

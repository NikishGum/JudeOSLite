import { useState, type FormEvent } from 'react';

export type Card = {
  id: string; participantId: string; clubId: string; fullName: string; birthDate: string; participantVersion: number;
  joinedOn: string; contactKind: 'phone' | 'email'; contactValue: string; contactName: string;
  archived: boolean; version: number; canEditIdentity: boolean;
};
export type Draft = { fullName: string; birthDate: string; joinedOn: string; contactKind: 'phone' | 'email'; contactValue: string; contactName: string };
const empty: Draft = { fullName: '', birthDate: '', joinedOn: '', contactKind: 'phone', contactValue: '', contactName: '' };

export function ParticipantEditor({ card, busy, today, onSave, onCancel }: {
  card: Card | null; busy: boolean; today: string; onSave: (draft: Draft) => void; onCancel: () => void;
}) {
  const [draft, setDraft] = useState<Draft>(() => card ? { fullName: card.fullName, birthDate: card.birthDate, joinedOn: card.joinedOn,
    contactKind: card.contactKind, contactValue: card.contactValue, contactName: card.contactName } : empty);
  const identityAllowed = !card || card.canEditIdentity;
  function submit(event: FormEvent<HTMLFormElement>) { event.preventDefault(); onSave(draft); }
  return <form onSubmit={submit} aria-busy={busy}>
    <fieldset disabled={busy}>
      <label className="field">ФИО участника<input required maxLength={200} autoComplete="off" disabled={!identityAllowed} value={draft.fullName} onChange={(event) => setDraft({ ...draft, fullName: event.target.value })} /></label>
      <label className="field">Дата рождения<input required type="date" min="0001-01-01" max={today} disabled={!identityAllowed} value={draft.birthDate} onChange={(event) => setDraft({ ...draft, birthDate: event.target.value })} /></label>
      {!identityAllowed && <p className="help">ФИО и дата рождения общие для нескольких филиалов; их меняет владелец. Контакт и состояние этой карточки доступны вам.</p>}
      <label className="field">Дата вступления<input required type="date" min={draft.birthDate || '0001-01-01'} max={today} value={draft.joinedOn} onChange={(event) => setDraft({ ...draft, joinedOn: event.target.value })} /></label>
      <label className="field">Контакт родителя или взрослого<select value={draft.contactKind} onChange={(event) => setDraft({ ...draft, contactKind: event.target.value as Draft['contactKind'] })}>
        <option value="phone">Телефон</option><option value="email">Электронная почта</option>
      </select></label>
      <label className="field">{draft.contactKind === 'phone' ? 'Номер телефона' : 'Адрес электронной почты'}<input required type={draft.contactKind === 'phone' ? 'tel' : 'email'} maxLength={254} autoComplete="off" aria-describedby="participant-contact-help" value={draft.contactValue} onChange={(event) => setDraft({ ...draft, contactValue: event.target.value })} /></label>
      <p className="help" id="participant-contact-help">Телефон: 7–15 цифр, допустимы + в начале, пробелы, скобки и дефисы. Один семейный контакт можно указать у нескольких участников.</p>
      <label className="field">Имя контактного лица (необязательно)<input maxLength={200} autoComplete="off" value={draft.contactName} onChange={(event) => setDraft({ ...draft, contactName: event.target.value })} /></label>
      <div className="actions"><button type="submit">{busy ? 'Сохраняем…' : 'Сохранить участника'}</button><button className="secondary" type="button" onClick={onCancel}>Отмена</button></div>
    </fieldset>
  </form>;
}

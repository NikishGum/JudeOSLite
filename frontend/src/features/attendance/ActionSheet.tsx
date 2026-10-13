import { useEffect, useId, useRef, type ReactNode } from 'react';

type Props = { title: string; children: ReactNode; busy: boolean; onCancel: () => void; cancelLabel?: string };

// Native dialog makes the background inert; Tab navigation is explicit.
export function ActionSheet({ title, children, busy, onCancel, cancelLabel = 'Отмена' }: Props) {
  const dialog = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  useEffect(() => {
    const element = dialog.current!;
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    element.showModal();
    element.querySelector<HTMLElement>('[data-initial-focus]')?.focus();
    return () => {
      element.close();
      document.body.style.overflow = overflow;
      if (previous?.isConnected) previous.focus();
    };
  }, []);
  return <dialog ref={dialog} className="attendance-sheet" aria-labelledby={titleId} aria-busy={busy}
    onKeyDown={(event) => {
      if (event.key !== 'Tab') return;
      event.preventDefault();
      const buttons = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('button:not(:disabled)'));
      if (!buttons.length) { event.currentTarget.querySelector<HTMLElement>('[data-initial-focus]')?.focus(); return; }
      const current = buttons.findIndex((button) => button === document.activeElement);
      const next = current < 0 ? (event.shiftKey ? buttons.length - 1 : 0)
        : (current + (event.shiftKey ? -1 : 1) + buttons.length) % buttons.length;
      buttons[next].focus();
    }}
    onCancel={(event) => { event.preventDefault(); if (!busy) onCancel(); }}>
    <h4 id={titleId} tabIndex={-1} data-initial-focus>{title}</h4>
    {children}
    <button className="attendance-secondary" disabled={busy} onClick={onCancel}>{cancelLabel}</button>
  </dialog>;
}

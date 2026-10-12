import { useEffect, useId, useRef, type ReactNode } from 'react';

type Props = { title: string; children: ReactNode; busy: boolean; onCancel: () => void; cancelLabel?: string };

// Native modal dialog keeps keyboard focus inside and makes the page behind it inert.
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
      const buttons = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('button:not(:disabled)'));
      const first = buttons[0]; const last = buttons.at(-1);
      if (!first) { event.preventDefault(); event.currentTarget.querySelector<HTMLElement>('[data-initial-focus]')?.focus(); }
      else if (event.shiftKey && (document.activeElement === first || document.activeElement?.hasAttribute('data-initial-focus'))) { event.preventDefault(); last?.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    }}
    onCancel={(event) => { event.preventDefault(); if (!busy) onCancel(); }}>
    <h4 id={titleId} tabIndex={-1} data-initial-focus>{title}</h4>
    {children}
    <button className="attendance-secondary" disabled={busy} onClick={onCancel}>{cancelLabel}</button>
  </dialog>;
}

import { X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useEffect, useId, useRef, type ReactNode } from 'react';

let modalCount = 0;
let originalOverflow = '';

export function Dialog({ title, children, busy = false, onCancel, cancelLabel = 'Отмена', className = '' }: {
  title: string; children: ReactNode; busy?: boolean; onCancel: () => void; cancelLabel?: string; className?: string;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  useEffect(() => {
    const element = dialog.current!;
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    if (modalCount === 0) originalOverflow = document.body.style.overflow;
    modalCount += 1; document.body.style.overflow = 'hidden';
    element.showModal();
    element.querySelector<HTMLElement>('[data-initial-focus]')?.focus();
    return () => {
      element.close(); modalCount -= 1;
      document.body.style.overflow = modalCount ? 'hidden' : originalOverflow;
      if (previous?.isConnected) previous.focus();
    };
  }, []);
  return <dialog ref={dialog} className={cn('pride-dialog', className)} aria-labelledby={titleId} aria-busy={busy}
    onCancel={(event) => { event.preventDefault(); if (!busy) onCancel(); }}
    onKeyDown={(event) => {
      if (event.key !== 'Tab') return;
      event.preventDefault();
      const targets = Array.from(event.currentTarget.querySelectorAll<HTMLElement>('button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex="0"]')).filter((el) => el.getClientRects().length && !el.closest('[hidden], [inert]'));
      if (!targets.length) { event.currentTarget.querySelector<HTMLElement>('[data-initial-focus]')?.focus(); return; }
      const current = targets.indexOf(document.activeElement as HTMLElement);
      targets[current < 0 ? (event.shiftKey ? targets.length - 1 : 0) : (current + (event.shiftKey ? -1 : 1) + targets.length) % targets.length].focus();
    }}>
    <header className="dialog-header"><h4 id={titleId} tabIndex={-1} data-initial-focus>{title}</h4><button type="button" className="secondary dialog-top-close" aria-label={`Закрыть панель «${title}»`} title={cancelLabel} disabled={busy} onClick={onCancel}><X aria-hidden="true" /></button></header>
    {children}
    <button type="button" className="secondary attendance-secondary" disabled={busy} onClick={onCancel}>{cancelLabel}</button>
  </dialog>;
}

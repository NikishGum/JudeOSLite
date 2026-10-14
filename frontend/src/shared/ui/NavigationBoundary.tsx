import { useEffect, useRef, type ReactNode } from 'react';
import { pendingMutations } from '../app-lifecycle';

// Values stay in a WeakMap, never in history, storage or lifecycle messages.
export function NavigationBoundary({ children, dirty }: { children: ReactNode; dirty: React.RefObject<() => boolean> }) {
  const root = useRef<HTMLDivElement>(null);
  const baselines = useRef(new WeakMap<HTMLFormElement, string>());
  const changed = useRef(new Set<HTMLFormElement>());
  const snapshot = (form: HTMLFormElement) => JSON.stringify(Array.from(new FormData(form).entries()));
  // Controlled fields often have no name. Include their current values as well.
  const values = (form: HTMLFormElement) => snapshot(form) + JSON.stringify(Array.from(form.querySelectorAll<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>('input,select,textarea')).map((el) => [el.value, 'checked' in el ? el.checked : null]));
  useEffect(() => {
    const check = () => { for (const form of changed.current) if (!form.isConnected || values(form) === baselines.current.get(form)) changed.current.delete(form); return changed.current.size > 0; };
    dirty.current = check;
    const unload = (event: BeforeUnloadEvent) => { if (dirty.current() || pendingMutations()) { event.preventDefault(); event.returnValue = ''; } };
    window.addEventListener('beforeunload', unload);
    return () => { window.removeEventListener('beforeunload', unload); changed.current.clear(); if (dirty.current === check) dirty.current = () => false; };
  }, [dirty]);
  return <div ref={root} onFocusCapture={(event) => {
    const form = (event.target as HTMLInputElement).form;
    if (form && !baselines.current.has(form)) baselines.current.set(form, values(form));
  }} onChangeCapture={(event) => {
    const field = event.target as HTMLInputElement;
    const form = field.form;
    if (!form || field.type === 'search') return;
    if (values(form) === baselines.current.get(form)) changed.current.delete(form); else changed.current.add(form);
  }}>{children}</div>;
}

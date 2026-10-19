import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { beginAppReload, cancelAppReload, pendingMutations } from '../shared/app-lifecycle';

interface InstallPrompt extends Event {
  prompt(): Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}
export function PwaStatus({ beforeUpdate, installContainer }: { beforeUpdate?: (action: () => void) => void; installContainer?: HTMLElement | null }) {
  const [online, setOnline] = useState(navigator.onLine);
  const [standalone, setStandalone] = useState(() => matchMedia('(display-mode: standalone)').matches || !!(navigator as Navigator & { standalone?: boolean }).standalone);
  const [install, setInstall] = useState<InstallPrompt | null>(null);
  const [waiting, setWaiting] = useState<ServiceWorker | null>(null);
  const [saving, setSaving] = useState(pendingMutations());
  const [applying, setApplying] = useState(false);
  const [notice, setNotice] = useState('');
  useEffect(() => {
    const connection = () => setOnline(navigator.onLine);
    const unavailable = () => setOnline(false);
    const reachable = () => setOnline(true);
    const mutation = () => setSaving(pendingMutations());
    const prompt = (event: Event) => { event.preventDefault(); setInstall(event as InstallPrompt); };
    const installed = () => { setInstall(null); setStandalone(true); };
    const display = matchMedia('(display-mode: standalone)');
    const displayChanged = () => setStandalone(display.matches);
    window.addEventListener('online', connection); window.addEventListener('offline', connection);
    window.addEventListener('network-unavailable', unavailable); window.addEventListener('network-reachable', reachable);
    window.addEventListener('mutation-state', mutation); window.addEventListener('beforeinstallprompt', prompt);
    window.addEventListener('appinstalled', installed); display.addEventListener('change', displayChanged);
    return () => {
      window.removeEventListener('online', connection); window.removeEventListener('offline', connection);
      window.removeEventListener('network-unavailable', unavailable); window.removeEventListener('network-reachable', reachable);
      window.removeEventListener('mutation-state', mutation); window.removeEventListener('beforeinstallprompt', prompt);
      window.removeEventListener('appinstalled', installed); display.removeEventListener('change', displayChanged);
    };
  }, []);
  useEffect(() => {
    if (!import.meta.env.PROD || !('serviceWorker' in navigator)) return;
    let disposed = false;
    let registration: ServiceWorkerRegistration | undefined;
    let worker: ServiceWorker | null = null;
    const checkWaiting = () => { if (!disposed) setWaiting(registration?.waiting ?? null); };
    const updateFound = () => {
      worker?.removeEventListener('statechange', checkWaiting);
      worker = registration?.installing ?? null;
      worker?.addEventListener('statechange', checkWaiting);
    };
    const check = () => { if (document.visibilityState === 'visible' && navigator.onLine) void registration?.update().catch(() => {}); };
    void navigator.serviceWorker.register('/sw.js', { scope: '/', updateViaCache: 'none' }).then((value) => {
      if (disposed) return;
      registration = value; checkWaiting(); updateFound(); registration.addEventListener('updatefound', updateFound);
    }).catch(() => { if (!disposed) setNotice('Установка пока недоступна. Можно продолжать работу в браузере.'); });
    window.addEventListener('online', check); document.addEventListener('visibilitychange', check);
    navigator.serviceWorker.addEventListener('controllerchange', checkWaiting);
    return () => {
      disposed = true; registration?.removeEventListener('updatefound', updateFound);
      worker?.removeEventListener('statechange', checkWaiting);
      window.removeEventListener('online', check); document.removeEventListener('visibilitychange', check);
      navigator.serviceWorker.removeEventListener('controllerchange', checkWaiting);
    };
  }, []);
  async function installApp() {
    if (!install) return;
    try { await install.prompt(); await install.userChoice; setInstall(null); }
    catch { setNotice('Откройте меню браузера и выберите установку приложения.'); }
  }
  function applyUpdate() {
    if (!waiting || !beginAppReload()) return;
    setApplying(true); setNotice('');
    // Only this tab reloads. Other tabs can finish their requests normally.
    const changed = () => { window.clearTimeout(timeout); window.location.reload(); };
    const timeout = window.setTimeout(() => {
      navigator.serviceWorker.removeEventListener('controllerchange', changed);
      cancelAppReload(); setApplying(false); setNotice('Не удалось обновить. Попробуйте снова.');
    }, 15_000);
    navigator.serviceWorker.addEventListener('controllerchange', changed, { once: true });
    waiting.postMessage({ type: 'ACTIVATE_UPDATE' });
  }
  const installHelp = !standalone && <details className="install-help"><summary>Установить PRIDE на устройство</summary>
      <p>iPhone / Safari: «Поделиться» → «На экран Домой» → «Добавить».</p>
      <p>Android / Chrome: меню ⋮ → «Установить приложение» или «Добавить на главный экран». На ПК в Chrome / Edge используйте значок установки в адресной строке.</p>
      <p className="help">Установленное приложение открывается отдельным окном. Для входа и работы нужен интернет.</p>
      {install && <button onClick={() => void installApp()}>Установить приложение</button>}
    </details>;
  return <aside className="pwa-status" aria-label="Приложение PRIDE">
    {!online && <p className="error" role="alert">Нет сети. Операции недоступны. Изменения не сохраняются без связи; после подключения повторите действие.</p>}
    {notice && <p role="status">{notice}</p>}
    {waiting && <div className="pwa-update"><p role="status">Доступно обновление. {saving ? 'Дождитесь окончания сохранения.' : 'Перед обновлением сохраните введённые данные.'}</p><button disabled={!!saving || applying || !online} onClick={() => beforeUpdate ? beforeUpdate(applyUpdate) : applyUpdate()}>{applying ? 'Обновляем…' : 'Обновить приложение'}</button></div>}
    {installContainer === undefined ? installHelp : installContainer ? createPortal(installHelp, installContainer) : null}
  </aside>;
}

import { useEffect, useState } from 'react';
import { ApiError, apiGet } from '../../shared/api';

/** Independent, uncached read blocks; an old scope never survives a new key. */
export function useReadBlock<T>(path: `/api/${string}`) {
  const [attempt, setAttempt] = useState(0);
  const key = JSON.stringify([path, attempt]);
  const [state, setState] = useState<{ key: string; data: T | null; loading: boolean; error: string }>({ key: '', data: null, loading: true, error: '' });
  useEffect(() => {
    const refresh = () => { if (document.visibilityState === 'visible') setAttempt((value) => value + 1); };
    window.addEventListener('focus', refresh);
    window.addEventListener('attendance-changed', refresh);
    document.addEventListener('visibilitychange', refresh);
    const timer = window.setInterval(refresh, 30_000);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener('focus', refresh);
      window.removeEventListener('attendance-changed', refresh);
      document.removeEventListener('visibilitychange', refresh);
    };
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    setState({ key, data: null, loading: true, error: '' });
    apiGet<T>(path, controller.signal).then((data) => {
      if (!controller.signal.aborted) setState({ key, data, loading: false, error: '' });
    }).catch((cause: unknown) => {
      if (controller.signal.aborted) return;
      setState({ key, data: null, loading: false, error: cause instanceof ApiError ? cause.message : 'Нет соединения. Повторите загрузку этого блока.' });
      if (cause instanceof ApiError && [403, 404].includes(cause.status)) window.dispatchEvent(new Event('session-changed'));
    });
    return () => controller.abort();
  }, [path, key]);
  return { ...(state.key === key ? state : { data: null, loading: true, error: '' }), refresh: () => setAttempt((value) => value + 1) };
}

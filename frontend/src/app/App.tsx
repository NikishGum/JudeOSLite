import { useEffect, useState } from 'react';
import logo from '../../../design-system/assets/logo-pride.png';
import { ApiError, apiGet } from '../shared/api';

type Connection = { state: 'loading' | 'ready' | 'error'; message: string };

export function App() {
  const [attempt, setAttempt] = useState(0);
  const [connection, setConnection] = useState<Connection>({ state: 'loading', message: 'Подключаемся к системе…' });
  useEffect(() => {
    const controller = new AbortController();
    apiGet<{ status: string }>('/api/readyz', controller.signal)
      .then((result) => {
        if (result.status !== 'ok') throw new Error('Unexpected readiness response');
        setConnection({ state: 'ready', message: 'Соединение с системой установлено.' });
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) return;
        setConnection({ state: 'error', message: error instanceof ApiError ? error.message : 'Нет соединения. Проверьте интернет и попробуйте снова.' });
      });
    return () => controller.abort();
  }, [attempt]);

  function reconnect() {
    setConnection({ state: 'loading', message: 'Подключаемся к системе…' });
    setAttempt((value) => value + 1);
  }

  return (
    <>
      <a className="skip-link" href="#workspace">К основному содержимому</a>
      <header className="header">
        <div className="header-inner">
          <a className="brand" href="/" aria-label="PRIDE — главная">
            <img src={logo} width="56" height="56" alt="" />
            <span>Клуб дзюдо <strong>PRIDE</strong></span>
          </a>
          <span className="header-label">Для сотрудников клуба</span>
        </div>
      </header>
      <main id="workspace" className="workspace" tabIndex={-1}>
        <p className="eyebrow">PRIDE · Вместе к результату</p>
        <h1>Рабочее пространство клуба</h1>
        <p className="intro">Каждая тренировка имеет значение.</p>
        <section className="connection" aria-labelledby="connection-title" aria-busy={connection.state === 'loading'}>
          <h2 id="connection-title">Подключение к системе</h2>
          <p role={connection.state === 'error' ? 'alert' : 'status'}>{connection.message}</p>
          <button onClick={reconnect} disabled={connection.state === 'loading'}>
            {connection.state === 'loading' ? 'Подключение…' : connection.state === 'error' ? 'Попробовать снова' : 'Проверить соединение'}
          </button>
        </section>
      </main>
      <footer>Клуб дзюдо PRIDE</footer>
    </>
  );
}

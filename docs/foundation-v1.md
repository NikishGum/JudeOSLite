# Рабочая основа V1-01

12.10.2026. Область — [#149](https://github.com/NikishGum/JudeOSLite/issues/149). Один Go-сервис, React и PostgreSQL. Нет предметных таблиц/CRUD, аккаунтов, финансов, iPay, Excel или медицинских сущностей. Реальное VPS-развёртывание, HTTPS и PWA остаются отдельными задачами.

## Закреплённый стек и источники

Версии сверены 12.10.2026 по первичным источникам, не по старой памяти модели. Прямые npm-зависимости закреплены точно, транзитивные — package-lock.json; Go — go.mod/go.sum. Образы Go/Node/PostgreSQL имеют точные patch-теги; Debian bookworm-slim обновляется в пределах своего дистрибутива.

| Компонент | Версия | Первичный источник |
| --- | --- | --- |
| Go / net/http | 1.27.2 | [JSON выпуска](https://go.dev/dl/?mode=json), [политика двух поддерживаемых major](https://go.dev/doc/devel/release) |
| pgx | 5.11.0 | [официальный релиз](https://github.com/jackc/pgx/releases/tag/v5.11.0), go.mod автора требует Go ≥1.25 |
| PostgreSQL | 18.6 | [поддерживаемые версии](https://www.postgresql.org/support/versioning/) |
| Node.js | 24.19.0 LTS | [релизы и поддержка](https://nodejs.org/en/about/previous-releases), [Docker Official Image](https://hub.docker.com/_/node) |
| React / React DOM | 19.3.0 | [официальный репозиторий](https://github.com/facebook/react), npm registry react/react-dom |
| TypeScript | 7.0.2 | [официальный репозиторий](https://github.com/microsoft/TypeScript), npm registry typescript |
| Vite / React plugin | 8.3.4 / 6.1.2 | [Vite](https://github.com/vitejs/vite), [плагин](https://github.com/vitejs/vite-plugin-react), npm registry: Node ^20.19 или ≥22.12 |
| React TypeScript types | 19.3.0 | [DefinitelyTyped](https://github.com/DefinitelyTyped/DefinitelyTyped), npm registry @types/react и @types/react-dom |

## HTTP и UI

- `GET /api/healthz`: 200, процесс обслуживает запросы; доступность БД не влияет.
- `GET /api/readyz`: 200 после проверки БД; 503 при недоступной БД или остановке. Ответы не кешируются.
- Успешный ответ проверки: `{"status":"ok"}`.
- Ошибка: `{"error":{"code":"not_ready","message":"Система временно недоступна."}}`; неизвестный API — not_found/404, неподдерживаемый метод — method_not_allowed/405. Технические детали/DSN не выдаются.
- Go отдаёт статику, неизвестные пути без расширения возвращают SPA index.html; отсутствующий файл или API никогда не маскируются HTML со статусом 200. Хешированные assets кешируются на год, index требует перепроверки.
- Общий клиент frontend/src/shared/api.ts обрабатывает JSON-ошибки, отмену и same-origin GET. Сессии/CSRF добавляются в V1-02; нет фиктивного auth.
- Минимальная оболочка использует оригинальный логотип, локальный Rubik и существующие CSS-токены. Есть узкий экран, фокус, клавиатурная ссылка перехода к содержимому, состояния подключения/ошибки/повтора. Персональных данных и фиктивных меню нет.

## Миграции и запуск

`go run ./cmd/migrate` — единственный runner. SQL встроен в бинарник из backend/migrations, формат `0001_name.sql`, непрерывные номера с 1. Session advisory lock сериализует runners; файл и запись public.schema_migrations фиксируются одной транзакцией. Контрольная сумма SHA-256 не разрешает менять уже применённые SQL. Новые изменения — новый файл; unknown/changed/missing история отвергается. Down-миграций нет; откат данных выполняется из проверенного backup по будущей операции V1-17.

0001 создаёт только schema `pride`. Предметные таблицы последующих задач создаются там, SQL явно указывает schema; history runner хранится отдельно в public.schema_migrations. Сервер миграции сам не запускает, при запуске проверяет всю embedded историю. Compose ждёт успешный migrate до запуска app.

SIGINT/SIGTERM снимает readiness, даёт HTTP-запросам до 10 секунд завершиться, затем закрывает pool; Compose даёт 15 секунд. Есть HTTP-таймауты и непривилегированный runtime-пользователь. Базовый Compose — заготовка для production с app на localhost и без публичного порта db; это ещё не HTTPS-развёртывание.

В управляемом окружении с корпоративным CA сборка принимает **необязательный** BuildKit secret `proxy_ca`: `docker build --secret id=proxy_ca,src="$CODEX_PROXY_CERT" -f deploy/Dockerfile -t judeoslite:ci .`. CA подключается только к скачиванию зависимостей и не попадает в финальный образ. Обычный Docker build не требует этого секрета. Не отключать TLS-проверки.

## Проверяемые границы

CI: TypeScript/frontend build; gofmt, vet, race; реальные PostgreSQL-тесты пустой БД, отката SQL, четырёх runners, повторного применения, сверки/отказа checksum; ошибка отсутствующей конфигурации; production image; Compose clean startup/restart. Результаты конкретного запуска записываются в issue при завершении.

Локально 12.10.2026 выполнены перечисленные проверки и npm audit (0 vulnerabilities). Дополнительно проверены отсутствие POSTGRES_PASSWORD в Compose, 503 readiness/200 liveness при остановке БД и восстановление, SIGTERM с exit 0, Vite API proxy и локальные ресурсы. Chromium проверен на ширинах 320/390/1280: нет горизонтальной прокрутки/JS-ошибок, кнопка ≥44 px, текст 16 px, ошибка и повтор подключения работают. Actions закреплены по SHA официальных releases checkout 7.0.1, setup-go 7.0.0, setup-node 7.1.0; успешность удалённого CI проверяется до доставки в main.

Работающие бизнес-сценарии, реальные iPhone/Android, установка PWA, VPS и пользовательская приёмка этим фундаментом не проверены и не объявляются готовыми.

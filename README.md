# JudeOSLite

Внутренняя CRM PRIDE. Создана рабочая основа V1-01: Go API, React-оболочка, PostgreSQL, миграции и CI. Вход сотрудников и предметные функции реализуются следующими назначенными задачами.

## Быстрый запуск из чистого checkout

Нужны Docker Engine с Compose v2 и свободный локальный порт 8080. Версии стека и первичные источники — [foundation-v1.md](docs/foundation-v1.md).

```sh
cp deploy/.env.example deploy/.env
# Для локальной проверки в примере есть тестовый пароль; замените его в deploy/.env.
docker compose --env-file deploy/.env -f deploy/compose.yml up --build -d --wait
```

Откройте http://localhost:8080. Compose ждёт БД, единственная команда `migrate` применяет SQL, затем стартует Go со статикой React. Повторный запуск не удаляет данные и не применяет миграцию второй раз. БД хранится в именованном volume PostgreSQL 18; её порт в базовом Compose не опубликован.

```sh
docker compose --env-file deploy/.env -f deploy/compose.yml logs app migrate
docker compose --env-file deploy/.env -f deploy/compose.yml down
```

`down` сохраняет БД. Не добавляйте `--volumes` к остановке рабочей системы. Пример пароля предназначен только для локальной разработки. На VPS задайте отдельные секреты вне Git; приложение слушает localhost, HTTPS-прокси и реальное развёртывание входят в V1-17.

## Разработка с Go и Vite на хосте

Нужны Go 1.27.2 и Node 24.19.0 (`.nvmrc`). Команды из корня:

```sh
cp deploy/.env.example deploy/.env
docker compose --env-file deploy/.env -f deploy/compose.yml -f deploy/compose.dev.yml up -d db --wait
cd frontend
npm ci
npm run build
cd ../backend
export DATABASE_URL='host=127.0.0.1 port=5432 dbname=judeoslite user=judeoslite sslmode=disable'
# Введите пароль из deploy/.env, не сохраняйте настоящий пароль в истории команд.
read -r -s -p 'Пароль БД: ' PGPASSWORD
export PGPASSWORD
go run ./cmd/migrate
go run ./cmd/server
```

В другом терминале запустите `cd frontend && npm run dev` и откройте http://localhost:5173. Vite проксирует `/api` на Go :8080. Go проверяет наличие собранной статики даже в этом режиме. Overlay публикует БД только на 127.0.0.1 и используется только локально.

Обязательная конфигурация Go — `DATABASE_URL`; pgx также понимает `PGUSER`, `PGPASSWORD`, `PGDATABASE`. Значения по умолчанию: `HTTP_ADDR=:8080`, `STATIC_DIR=../frontend/dist` (относительно backend). В образе статика в `/app/static`. Без БД, применённых миграций или статики сервер завершается с ошибкой; пароли и URL подключения не логируются.

## Проверки

```sh
cd frontend && npm ci && npm run build
cd ../backend
go vet ./...
go test -race ./...
go build ./cmd/server ./cmd/migrate
```

Реальные интеграционные тесты миграций требуют `TEST_DATABASE_URL` к **тестовой** PostgreSQL и право CREATE DATABASE. Они создают отдельную временную БД и удаляют только её. Без этой переменной тест явно пропускается; CI задаёт её и выполняет тесты.

```sh
docker build -f deploy/Dockerfile -t judeoslite:ci .
bash deploy/check.sh
```

Последние команды запускаются из корня. Smoke использует отдельный disposable Compose-проект, порт 18080 и удаляет его тестовый volume. Проверяет чистый запуск, повтор миграций, полный перезапуск и закрытый порт БД. CI выполняет эти же проверки и сборку production-образа.

## Контекст проекта

- [План: 18 полных задач](docs/development-plan-v1.md) и [GitHub issues](docs/github-issues-v1.md).
- [Совместная работа](docs/collaboration.md): агент автоматически занимает свободную задачу через общую очередь GitHub; исполнитель и In progress видны в issues, один issue — один автоматически названный чат. In review/ревью коллеги необязательны.
- [Архитектура](docs/architecture-v1.md), [схема БД](docs/database/schema-v1.md), [диаграммы](docs/database/diagrams-v1.md).
- [Промпт исполнителя с переименованием чата и назначением](docs/task-agent-prompt.md), [промпт планировщика](docs/planning-agent-prompt.md).
- [База знаний](docs/knowledge-base/README.md) и [память](docs/knowledge-base/memory.md); агентам начинать с них и [AGENTS.md](AGENTS.md).
- [Оформление](DESIGN.md), [дизайн-система](design-system/README.md), [витрина](design-system/preview.html).

Финансы/iPay, Excel и отдельные справки отложены; прежние требования сохранены в [истории](docs/archive/README.md). Фундамент развиваемый, без инфраструктуры для предполагаемой большой нагрузки.

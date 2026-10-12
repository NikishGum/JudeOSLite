# JudeOSLite

Внутренняя CRM PRIDE. Работают Go/React/PostgreSQL, вход сотрудников, серверные права, управление филиалами/администраторами/тренерами, ручной реестр участников и группы с датированной историей состава. Четыре режима расписания, датированные версии и генерация занятий реализованы; журнал тренера и поштучные исправления закрытых отметок администратором реализованы.

Группы/переводы/атомарное начальное включение: [groups-v1.md](docs/groups-v1.md).

Расписание/предпросмотр/генерация/история: [schedule-v1.md](docs/schedule-v1.md).

Журнал филиала/коррекции/история: [corrections-v1.md](docs/corrections-v1.md).
Статистика владельца/контрольные суммы: [reports-v1.md](docs/reports-v1.md).

## Быстрый запуск из чистого checkout

Нужны Docker Engine с Compose v2 и свободный локальный порт 8080. Версии стека и первичные источники — [foundation-v1.md](docs/foundation-v1.md).

```sh
cp deploy/.env.example deploy/.env
# Для локальной проверки в примере есть тестовый пароль; замените его в deploy/.env.
docker compose --env-file deploy/.env -f deploy/compose.yml up --build -d --wait
```

Создайте первого владельца командой с вводом пароля без отображения:

```sh
docker compose --env-file deploy/.env -f deploy/compose.yml exec app \
  /app/create-owner --login owner --name 'Владелец сети'
```

Откройте http://localhost:8080 и войдите. Compose ждёт БД, единственная команда `migrate` применяет SQL, затем стартует Go со статикой React. Повторный запуск не удаляет данные и не применяет миграцию второй раз. БД хранится в именованном volume PostgreSQL 18; её порт в базовом Compose не опубликован.

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
go run ./cmd/create-owner --login owner --name 'Владелец сети'
export APP_ORIGIN=http://localhost:5173 AUTH_INSECURE_LOCAL=true
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
go build ./cmd/server ./cmd/migrate ./cmd/create-owner
```

Origin, защищённые cookie, bootstrap и API — [auth-v1.md](docs/auth-v1.md). Для Go-статики на :8080 задайте APP_ORIGIN=http://localhost:8080. На HTTPS используйте рабочий origin и AUTH_INSECURE_LOCAL=false.

После входа владелец добавляет залы и создаёт администраторов в «Филиалы и администраторы». Несколько назначений, отзыв, отключение и архив — [clubs-v1.md](docs/clubs-v1.md). Пароль нового сотрудника передайте лично/по защищённому каналу.

Выберите филиал, чтобы вручную создать/найти/изменить участника, архивировать и вернуть карточку. ФИО и семейные контакты могут совпадать. Общие ФИО/дату рождения нескольких филиалов меняет владелец, контакты изолированы — [participants-v1.md](docs/participants-v1.md). Тренеру полный реестр недоступен; состав своего фактического занятия доступен в журнале.

В «Тренеры филиала» администратор создаёт тренера, меняет одиночный аккаунт или отзывает локальное назначение. Общий аккаунт и привязку существующего сотрудника меняет владелец. Смена пароля/логина отзывает прежние сессии; история остаётся — [trainers-v1.md](docs/trainers-v1.md).

Реальные интеграционные тесты миграций и доступа требуют `TEST_DATABASE_URL` к **тестовой** PostgreSQL и право CREATE DATABASE. Они создают отдельную временную БД и удаляют только её. Без этой переменной тест явно пропускается; CI задаёт её и выполняет тесты.

```sh
docker build -f deploy/Dockerfile -t judeoslite:ci .
bash deploy/check.sh
```

Последние команды запускаются из корня. Smoke использует отдельный disposable Compose-проект, порт 18080 и удаляет его тестовый volume. Проверяет чистый запуск, повтор миграций, полный перезапуск и закрытый порт БД. CI выполняет эти же проверки и сборку production-образа.

## Контекст проекта

- [План первой версии](docs/development-plan-v1.md) и [GitHub issues](docs/github-issues-v1.md).
- [Совместная работа](docs/collaboration.md): агент автоматически занимает свободную задачу через общую очередь GitHub; исполнитель и In progress видны в issues, один issue — один автоматически названный чат. In review/ревью коллеги необязательны.
- [Архитектура](docs/architecture-v1.md), [схема БД](docs/database/schema-v1.md), [диаграммы](docs/database/diagrams-v1.md).
- [Промпт исполнителя с переименованием чата и назначением](docs/task-agent-prompt.md), [промпт планировщика](docs/planning-agent-prompt.md).
- [База знаний](docs/knowledge-base/README.md) и [память](docs/knowledge-base/memory.md); агентам начинать с них и [AGENTS.md](AGENTS.md).
- [Оформление](DESIGN.md), [дизайн-система](design-system/README.md), [витрина](design-system/preview.html).

Финансы/iPay, Excel и отдельные справки отложены; прежние требования сохранены в [истории](docs/archive/README.md). Фундамент развиваемый, без инфраструктуры для предполагаемой большой нагрузки.

Ранний HTTPS-тестовый стенд и ручная подготовка VPS владельцем: [operations-test.md](docs/operations-test.md). Реальная серверная приёмка ещё не подтверждена.

Календарь, разовые занятия, отмена, замена тренера и перенос конкретного сеанса: [V1-08](docs/sessions-v1.md). Тренер видит собственные фактические занятия после обновления; отмена сохраняет историю без автоматических пропусков.

Тренер открывает «Мои занятия и журнал» на выбранную локальную дату и проходит последовательные карточки кнопками. Отсутствие выбирается в нижней панели причин; сохранение подтверждает сервер. Сводка позволяет исправить предыдущую отметку до завершения. Подтверждение завершения показывает число неотмеченных, которые атомарно станут absent/unknown. [API и снимок V1-09](docs/attendance-v1.md), [мобильный сценарий V1-10](docs/attendance-mobile-v1.md). Коррекции администратора после закрытия — V1-11.

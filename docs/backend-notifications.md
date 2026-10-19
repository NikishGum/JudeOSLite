# RD-12 / #204 — персональные уведомления

19.10.2026, Europe/Minsk. [Issue204](https://github.com/NikishGum/JudeOSLite/issues/204),
[HTTP-контракт](backend-contracts/README.md#уведомления--204). Основа `main fc07a78`,
ветка `codex/rd-12-notifications`. Новая миграция0014, пакет notifications,
две строки server wiring и собственный browser smoke в существующем CI.
Frontend/auth и миграции0001–0013 сохраняются.

Исходный запрос очереди: StarMadeGalaxy,
`6d0f556e8b034b8ca2d5e28513646b47`,
[run39281361187](https://github.com/NikishGum/JudeOSLite/actions/runs/39281361187).
Единственный исполнитель/In progress проверен. Лог исходного job118156110186
подтвердил `acquired:true` в15:53:58UTC, request/actor совпали. Ранее CLI API
показывал queued/unknown; проверка через GitHub-коннектор дала success и полный
лог, после чего штатная wait_for_claim подтвердила тот же запрос. Причина
расхождения не установлена. Повторного dispatch/прямого назначения нет.

## События и атомарность

Одна таблица notifications: получатель, филиал, исходная семья/карточка/занятие,
стабильный event_key, фиксированные kind/title/body, created_at/read_at.
Составные FK сохраняют границу филиала; уникальность `(user_id,event_key)`
и `ON CONFLICT DO NOTHING` исключают дубли доставки, включая конкурентный replay.
Индексы по user/created_at DESC/id DESC и отдельный partial unread.

| Источник | События | Идентичность |
| --- | --- | --- |
| Транзакционный audit семей | created/updated/account_created/password_reset/access_revoked | entity ID/action/version |
| Транзакционный audit платежей | payment_created/payment_paid/payment_voided | entity ID/action/version |
| Транзакционный audit документов | member_document_uploaded/deleted | entity ID/action/version |
| INSERT/UPDATE training_sessions | назначение, замена, перенос, отмена, смена версии расписания | session ID/version |

AFTER-триггер аудита переиспользует существующие атомарные записи модулей.
Триггер занятий покрывает также hourly generator, у которого нет human audit
actor. Open/mark/close/correction не отправляют новости расписания. Ошибка доставки
откатывает предметную операцию, аудит и все уведомления. Незафиксированные строки
не видны читателям; rollback не оставляет inbox. Нет фонового polling аудита,
брокера, внешнего сервиса или окна между commit и доставкой. Миграция не рассылает
исторические события задним числом. Повтор генератора не создаёт новых событий.

Получатели на момент события: enabled owner всей сети, текущие administrator
филиала, явные family_access исходной семьи для семейных/детских событий,
только фактический trainer занятия с действующей trainer-ролью. Собственные
действия включены. Прежний тренер после замены не получает событие о новом
назначении и теряет чтение всех прежних сообщений этого занятия.

## Живой доступ, счётчики и ссылки

GET `/api/notifications?status=all|unread&page=0&limit=25` возвращает обязательные
`items,total,hasMore,unreadCount`; пустые items — `[]`. Порядок createdAt DESC/id
DESC стабилен и при равных timestamps. page0–1000000, limit1–100; неверные,
неизвестные и дублированные параметры400. total относится к выбранному status,
unreadCount — ко всем доступным непрочитанным сообщениям этого пользователя,
включая запрос пустой дальней страницы.

Все операции используют прежний auth.Require/Origin/CSRF и повторный
auth.LockSession внутри транзакции. User/session locks сериализуют disable,
отзыв сессии и изменение staff-ролей. Один SQL statement использует единый
снимок предметных прав для items/total/unreadCount; read и read-all применяют
тот же scope непосредственно в UPDATE. Права из cached middleware user не
используются для предметного допуска. После commit отзыва последующее чтение
не включает старые сообщения или их счётчики.

Scope всегда начинается с конкретного получателя, даже для owner. Далее нужны
живые owner/admin-права либо исходная family_access и актуальная связь именно
этого ребёнка, либо actual trainer_id/current trainer-role/non-superseded занятие.
Parent+trainer не получает чужие семейные события. Unlink скрывает детские
payment/document-сообщения; общие сообщения доступной семьи остаются. Перевод
ребёнка в другую семью не передаёт старые сообщения новой семье. Revoke скрывает
оставшиеся семейные сообщения. Архив филиала скрывает parent/trainer inbox;
owner/admin сохраняют разрешённую историю. Снятие одной роли не отнимает
независимое право этого человека.

POST `/{id}/read` и `/read-all` принимают только JSON `{}` до1KiB.
Они идемпотентны, сохраняют первоначальный read_at и отвечают `{"status":"ok"}`.
Read-all обновляет только доступные unread записи. Чужой/скрытый/неверный UUID404;
анонимный401, CSRF403, неверный JSON/Content-Type400. Если сессия отозвана уже
после middleware, внутренняя проверка возвращает404. Ответы/ошибки no-store/nosniff.

В таблице нет произвольного href. Сервер строит его только из фиксированных
путей и UUID/дат БД: owner/admin `/clubs/{id}/families|payments`, parent
`/families|payments`, trainer `/clubs/{id}/sessions?session=...&from=...&through=...`.
Ссылка на семью несёт её detail; ссылка на занятие — фактическую локальную дату.
Названия/тексты фиксированы, для занятия body содержит только дату/время.
Нет семейных контактов, имён детей, сумм/назначения платежа, медицинского типа,
названия/имени/содержимого файла, пароля или свободного текста причины.
Существующая клиентская проверка href сохранена. Push/email/SMS не подключаются.

## Воспроизводимые проверки

```sh
cd backend
# TEST_DATABASE_URL — отдельный PostgreSQL18.6 с правом CREATE DATABASE.
go vet ./...
go test -race ./...
go build ./cmd/server ./cmd/migrate ./cmd/create-owner
cd ..
npm run build --prefix frontend
docker build --secret id=proxy_ca,src=/etc/ssl/certs/ca-certificates.crt -f deploy/Dockerfile -t judeoslite:ci .
python3 backend/tests/browser/notifications.py
PWA_BROWSER=webkit python3 backend/tests/browser/notifications.py
```

Go/PG проверяет реальные source HTTP mutations, две семьи/два филиала, смешанные
роли, pagination/tied timestamps/empty page, read/read-all/чужой ID/CSRF,
unlink/transfer/revoke/replace/disable/session/архив, privacy и разрешённые href.
Восемь одновременных replay одного event и read не создают дублей; незавершённая
транзакция/rollback семей и занятий невидимы; отказ inbox-триггера откатывает
HTTP payment и аудит. Настоящий schedule.Generate и повтор покрыты отдельно.
Upgrade0013→0014/rerun сохраняет прежние private bytea/семью/сессию/пароль,
не создаёт старую рассылку и не дублирует новое событие.

Browser harness запускает production Go и disposable PostgreSQL, существующий
React без подмены запросов: bell/read/read-all/reload/переход родителя, независимый
SQL, счета/confirm/documents/family events, мобильная pagination, trainer→занятие,
replacement/cancel/unlink/revoke/logout. Сам по себе этот smoke не является
физической PWA-проверкой или приёмкой сотрудников #188.

Окончательные результаты проверок, SHA/CI/Deploy и готовность main — в live issue.

Локально19.10: Go1.27.2 vet/полный race-suite с PostgreSQL18.6 (включая прежние
auth/access/training/attendance), отдельный повтор конечного notifications-кода,
upgrade/rerun, production image, frontend build,14 coordination и43 deployment
unit tests — PASS. Новый browser smoke PASS Chromium151.0.7922.34 и WebKit26.5:
родитель390×844, owner1280×900, реальные UI-команды и независимый SQL.
WebKit в cloud использует private Debian13 libraries/skip host-package validation;
в CI остаётся штатный `install --with-deps`, без пропуска продуктовых assertions.

Первичные остановки новых тестов сохранены в отчёте: wrapper для отзыва после
middleware не выставлял внешний no-store, а expiry fixture нарушал прежний
CHECK expires_at>created_at. Исправлены только fixture/header и обе даты.
Первый browser проход измерял ширину во время перестройки workspace после
viewport: scroll1304 при inner1280, затем нормальная раскладка без изменения UI.
Конечный harness ждёт измеряемую ширину до5s, при отказе сохраняет геометрию/снимок;
проверка отсутствия overflow сохранена. Оба конечных browser прохода PASS.
Физические устройства/установка PWA и приёмка сотрудников не объявляются PASS.

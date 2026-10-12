# История изменений V1-20

12.10.2026. По поручению владельца, [#170](https://github.com/NikishGum/JudeOSLite/issues/170). Зависимость V1-07/#155 готова; функция самостоятельна относительно V1-08.

## Что видит сотрудник

Раздел «История изменений»: действие, время, автор, филиал, тип записи и устойчивый ID. Фильтры: все разрешённые филиалы, конкретный филиал, даты включительно и действие; для owner также события сети без филиала. На странице до 50 событий, новые сверху; кнопки следующей/предыдущей страницы и обновления. Время и границы дат — Europe/Minsk. Имена авторов/филиалов текущие: старый аудит не содержит снимков имён. Отключённый автор и архивный филиал явно подписаны; их история сохраняется.

Это просмотр факта операции. Raw before_state/after_state/change_reason и данные изменяемого объекта не возвращаются; нельзя редактировать/удалять/отменять событие. Нет ссылок, способных открыть объект чужого филиала, и запросов текущих персональных карточек ради подписи. Тип + полный ID идентифицируют запись. Будущие действия сохраняются в ленте с нейтральной подписью до добавления перевода.

## Контракт и права

- GET /api/audit/filters: isOwner, clubs [{id,name,archived}], actions (только из разрешённого scope).
- GET /api/audit/events: items [{id,createdAt,actorId,actorName,actorDisabled,clubId,clubName,clubArchived,entityType,entityId,action}], nextCursor.
- Query: clubId (пусто = все разрешённые; network = только owner; UUID = филиал), from/until YYYY-MM-DD включительно, action, cursor. Даты/порядок/повторяющиеся/неизвестные query параметры и повреждённый cursor дают 400. Чужой/несуществующий филиал и network у admin одинаково дают 404.
- Owner видит сеть и архив. Administrator — только события с club_id его текущих administrator-назначений, включая архивные; trainer-роли не расширяют scope. Trainer без administrator и сотрудник без назначений получают 403. Без живой сессии — 401. Все ответы no-store, технические ошибки/DSN не выдаются.
- Read-транзакция повторяет live actor/session/роль через auth.LockClubAdministrator; триггер назначения блокирует staff_users, поэтому все изменения ролей сериализованы с чтением. SQL scope, списки фильтров и каждая страница проверяют только текущие разрешённые филиалы. Owner-флаг из middleware не является решением о доступе.
- Cursor: (created_at,id) в порядке DESC; tied timestamps упорядочены ID. Cursor привязан к фильтрам, но не выдаёт разрешения; новые верхние события не смещают следующую страницу. Обновление начинает с первой страницы.
- UI отменяет устаревшие запросы, очищает события перед новым чтением и размонтируется при смене пользователя/прав/выбранного филиала. Live filters перечитываются на каждой странице, при focus/visibility и каждые 30 секунд. При отказе авторизации раздел скрывается; нет хранения аудита в localStorage.

Схема/миграции и существующие writers не меняются. Пока audit_events содержит только события, которые записывают предметные операции; это не журнал всех HTTP-запросов или успешных входов. Для будущих функций достаточно писать scoped audit_events как раньше. Персональные payload не расширять.

## Проверки

PostgreSQL/API tests: два филиала, owner/admin/trainer, несколько назначений, network/чужой ID, фильтры дат/действия, no-store и приватность, архив/отключённый автор, 55 tied timestamps и новый insert между страницами, cursor с другими фильтрами, отзыв роли/сессии/disable/owner после auth middleware.

Команда из корня с отдельной тестовой PostgreSQL (тесты создают/удаляют собственные БД):

```bash
TEST_DATABASE_URL='postgres://test:test-only@127.0.0.1:55432/test?sslmode=disable' go -C backend test -race ./internal/audit
```

Браузерная регрессия — frontend/e2e/audit_v1.py; синтетическая fixture — backend/internal/audit/testdata/browser_fixture.go (отказывает любой БД кроме loopback audit_browser с user=test). запуск только на disposable loopback app с синтетическим owner, не на реальном реестре. Итоговые SHA/CI/браузерные результаты — в live issue #170. Реальные устройства/PWA и полная предметная приёмка остаются своими задачами.

Для браузерного прогона создать отдельную `audit_browser` на тестовой PostgreSQL, затем из корня:

```bash
export DATABASE_URL='postgres://test:test-only@127.0.0.1:55432/audit_browser?sslmode=disable'
go -C backend run ./cmd/migrate
go -C backend run ./internal/audit/testdata/browser_fixture.go
npm --prefix frontend ci
npm --prefix frontend run build
HTTP_ADDR=127.0.0.1:8082 APP_ORIGIN=http://127.0.0.1:8082 AUTH_INSECURE_LOCAL=true go -C backend run ./cmd/server
# В другом терминале, после установки Playwright/Chromium:
python3 frontend/e2e/audit_v1.py
```

Fixture одноразовая, отказывает повторному созданию владельца; после теста удалять только эту disposable БД. Персональные/реальные записи не использовать.

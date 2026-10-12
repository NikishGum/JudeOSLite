# Тренеры филиала V1-04

12.10.2026, [#152](https://github.com/NikishGum/JudeOSLite/issues/152). Владелец или администратор выбирает филиал и открывает «Тренеры филиала». Создание ручное: имя, свободный логин, первоначальный пароль с подтверждением. Публичные имена с сайта не импортируются в аккаунты. Пароль передают лично или по защищённому каналу; API его не возвращает, браузер/аудит не сохраняют, поля очищаются после попытки отправки. Используются существующие Argon2id, Origin/CSRF и сессии из [auth-v1.md](auth-v1.md).

## Права и история

Все endpoints проходят auth.Require. Owner работает со всей сетью; administrator — только с trainer-назначениями своего филиала. Trainer не получает управление сотрудниками или общий реестр. Чужие/несуществующие/неверные ID дают одинаковый 404. Локальный ответ не содержит чужих club ID, ролей и числа назначений.

Администратор меняет имя, логин, пароль и disabled только у non-owner аккаунта, имеющего ровно одно назначение — trainer его филиала. Любая дополнительная роль, в том числе administrator или назначение архивного филиала, оставляет общие изменения владельцу. Даже администратор нескольких филиалов не меняет общий аккаунт. canManageAccount сообщает доступность действия; сервер повторяет проверку под блокировкой. Отзыв своего trainer-назначения разрешён и для общего аккаунта; остальные роли/филиалы сохраняются.

Занятый нормализованный логин даёт 409 login_taken без автоматического присоединения, в том числе после отзыва назначения. Только owner выбирает существующего non-owner сотрудника из списка сети и добавляет trainer-роль; administrator-роли сохраняются. Owner также меняет аккаунт после отзыва всех назначений. Owner-аккаунты эти API не редактируют и не назначают тренерами.

Отзыв удаляет только текущую trainer-роль, увеличивает version аккаунта и пишет аудит. Сам staff_users и исторические FK сохраняются. DELETE сотрудника нет; будущие marked_by/trainer_id ссылаются на **staff_users.id**, не на удаляемый staff_club_roles.id. Отзыв действует на следующий RequireClub/RequireTrainer без повторного входа, другие филиалы остаются доступны. Если других ролей нет, me возвращает пустой список филиалов; сессия сама по себе не разрешает занятия.

Disable отзывает все сессии триггером 0002; включение требует нового входа. Миграция 0005 отзывает сессии при смене password_hash/login, включая прямую SQL-правку. Смена имени не отзывает сессию. Новый пароль хешируется с новой солью, прежние сессии прекращаются в той же транзакции. Архив запрещает создание/привязку и локальную правку аккаунта, но разрешает историческое чтение и отзыв; owner меняет общие данные через сетевой endpoint.

## API

`B = /api/clubs/{clubID}/trainers`. Trainer: id,login,displayName,disabled,version,canManageAccount. trainerClubIds выдаются только в списке owner и содержат trainer-назначения, не administrator.

| Запрос | Контракт |
| --- | --- |
| GET B | Owner/admin: локальные назначения, включая отключённых; сортировка по имени/ID |
| GET B?available=true | Только включённые тренеры активного филиала; [{id,displayName}] для выбора занятий |
| POST B | {login,displayName,password}; 201 с Trainer, роль всегда trainer только этого филиала |
| PUT B/{staffID}/account | {login,displayName,disabled,version}; 200 с Trainer; общие данные по правилу выше |
| POST B/{staffID}/password | {password,version}; 200 с Trainer, сессии отозваны |
| POST B/{staffID}/revoke | {version}; 200 status=ok; отзыв только trainer этого филиала |
| GET /api/trainer-accounts | Только owner: все non-owner сотрудники, включая отключённых/без ролей; Trainer + trainerClubIds |
| POST /api/staff/{staffID}/trainer-assignments | Только owner: {clubId,version}; 200 status=ok; повтор назначения с актуальной версией безопасен |
| PUT /api/staff/{staffID}/account | Только owner: {login,displayName,disabled,version}; 200 с Trainer |
| POST /api/staff/{staffID}/password | Только owner: {password,version}; 200 с Trainer |

Логин/пароль валидируются по auth; имя 1–200 символов после trim, без управляющих символов. Неизвестные JSON-поля (role/isOwner/clubIds), неверные версии/секреты отвергаются. 400 invalid_input; 403 account_owner_required; 404 not_found; 409 login_taken/version_conflict/club_archived; 429 при занятых двух слотах хеширования; безопасный 503 при сбое БД. Ответы no-store. Формы сохраняют несекретный ввод при ошибке; явное обновление после конфликта закрывает форму и сбрасывает правки. Двойная отправка блокируется, чтения отменяются при смене сотрудника/филиала, запоздалые результаты не возвращают прежние данные.

## Транзакции и передача контекста

Запись повторно проверяет enabled actor и live session: staff_users FOR SHARE, staff_sessions FOR SHARE, administrator-роль своего филиала FOR SHARE. Затем целевой non-owner staff_users FOR UPDATE, локальная trainer-роль/число всех назначений и version; lifecycle guard club FOR SHARE. Создание блокирует club перед вставкой нового аккаунта. Owner-назначение и отзыв тоже блокируют целевой staff_users и увеличивают version. 0005 блокирует тот же parent staff_users при INSERT/DELETE роли; ключи назначения неизменяемы. Новое назначение не пересекает проверку единственного филиала. 0001–0004 не меняются, upgrade сохраняет ID/пароли/роли/сессии.

Изменение и audit_events атомарны. Действия trainer_created/trainer_assignment_created/trainer_assignment_revoked/trainer_account_updated/trainer_password_changed сохраняют actor, club scope, staff ID, version, assigned/disabled и признаки loginChanged/displayNameChanged. Без паролей/хешей/токенов, логинов/ФИО и чужих назначений в локальном payload. Ошибка аудита откатывает аккаунт, роль и отзыв сессий.

`auth.RequireTrainer(ctx, tx, staffID, clubID)` проверяет enabled сотрудника и текущую trainer-роль с FOR SHARE внутри назначения/замены будущего занятия. V1-07/V1-08 используют его после проверки прав actor и `clubs.RequireActive` в той же транзакции. Для доступа к существующему занятию V1-09 и далее дополнительно проверяют текущую роль trainer в club и **фактический trainer_id занятия**; одной роли недостаточно. Таблицы/API занятий/посещаемости здесь ещё не создаются, автоматических отмен при отзыве нет. Исторические авторы сохраняются по staff ID.

## Проверки

```sh
cd frontend && npm ci && npm run build
cd ../backend
go vet ./...
TEST_DATABASE_URL='postgres://test:test-only@127.0.0.1:5432/test?sslmode=disable' go test -race ./...
go build ./cmd/server ./cmd/migrate ./cmd/create-owner
```

PostgreSQL 18.6: создание/вход, нормализованный дубль без присоединения, unknown fields/CSRF/эскалация, owner/admin/trainer/чужой ID; общий аккаунт двух филиалов, дополнительная administrator-роль, локальный отзыв/owner-привязка, исторический FK, смена логина/пароля, disable/re-enable, selector, archive, устаревшие/конкурентные версии, гонка нового назначения и правки, отзыв actor после middleware, rollback/privacy аудита. Общий suite проверяет upgrade с данными V1-02, concurrent runners/rerun/checksum и прежние auth/clubs/participants.

Chromium 151: формы → Go API → PostgreSQL → UI, создание/вход, duplicate login, один POST при двойном submit, reset и завершение сессии, owner-привязка/восстановление, общий аккаунт/локальный отзыв, network failure/retry, устаревшая форма, disable/selector/reload, изоляция второго admin. 320/390/1280 px без overflow/JS-ошибок, кнопки ≥44 px. Исправлена явная связь подписи списка сотрудников с select. Записи и отсутствие секретов/ФИО в аудите сверены с БД. Production Docker build и Compose clean/rerun/restart/readiness recovery прошли; коммит/CI — в отчёте issue.

Локальный синтетический стенд не заменяет реальные iPhone/Android, HTTPS/VPS/PWA или пользовательскую приёмку; они остаются последующим задачам.

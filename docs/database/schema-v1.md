# PostgreSQL: минимальная схема первой версии

Версия 2.0, 12.10.2026. **Предложение для будущих таблиц; фактическая схема — миграции.** Только текущая посещаемость/реестр; прежние 26 таблиц, включая финансы/медицину/импорт, сохранены в [истории](../archive/README.md). Таблицы добавляются при реализации зависимой задачи. 0002 создаёт clubs, staff_users, staff_club_roles, staff_sessions; 0003 добавляет version в clubs/staff_users и audit_events; 0004 — participants/club_memberships с отдельными версиями общей и локальной карточки. Контракты — [auth-v1.md](../auth-v1.md), [clubs-v1.md](../clubs-v1.md), [participants-v1.md](../participants-v1.md). 0006 создаёт training_groups/group_memberships: даты, версии, составные FK и запрет перекрытий на parent lock. [Контракт групп](../groups-v1.md). 0007 добавляет schedule_rules и training_sessions, датированные версии, локальные даты/зоны, version и superseded_at для сохранения заменённых будущих планов. [Контракт расписания](../schedule-v1.md). 0008 защищает историю существующих занятий; календарь/отмена/замена/перенос — [V1-08](../sessions-v1.md). 0009 создаёт attendance_records с составными FK, NULL/manual/close_default, версиями и защитой снимка; [журнал V1-09](../attendance-v1.md). 0010 добавляет change_reason и разрешает только поштучную correction закрытой записи с новой версией; [V1-11](../corrections-v1.md).

## Соглашения

- 12 таблиц одного приложения; UUID PK и created_at timestamptz. Изменяемые записи имеют updated_at; version нужен там, где UI может одновременно редактировать запись, прежде всего карточки, правила, сеансы и отметки.
- club_id ограничивает данные. Для связей группы/состава/сеанса/отметки — UNIQUE(id, club_id) и составные FK на соответствующий клуб. Это целостность, а серверные права проверяются отдельно.
- Даты состава/правила — [from, until), NULL конец открыт. Время сеанса timestamptz; календарь/чётность в clubs.timezone. NULL и смысл концов явно документировать.
- Архив вместо удаления используемой истории, ON DELETE RESTRICT. Основные данные обычные колонки/отношения, не EAV/универсальный JSON.
- Изменения внутри обычных транзакций; операции журнала блокируют сеанс, генератор защищён UNIQUE. Одного процесса достаточно, защита сохраняет корректность при случайном повторе запуска.

## Таблицы и поля

| Таблица | Поля кроме id/created_at | Назначение и ограничения |
| --- | --- | --- |
| clubs | name text, address text nullable, timezone text, archived_at timestamptz nullable | Филиал/зал. Название не идентификатор; Europe/Minsk по умолчанию |
| staff_users | login text, display_name text, password_hash text, is_owner bool, disabled_at timestamptz nullable | UNIQUE нормализованного login; хеш не возвращать клиенту |
| staff_club_roles | staff_user_id FK, club_id FK, role text | UNIQUE(user, club, role), role administrator/trainer; общий owner в staff_users |
| staff_sessions | staff_user_id FK, token_hash bytea, csrf_token_hash bytea, expires_at, revoked_at nullable | UNIQUE(token_hash), хранить хеши, проверять текущие роли/disabled; чистить истёкшие |
| participants | full_name text, birth_date date, version integer, updated_at | Общая идентичность. В ручном v1 полная дата; ФИО не UNIQUE. Поле birth_year понадобится позже при Excel, сейчас миграция не нужна |
| club_memberships | participant_id FK, club_id FK, joined_on date, contact_kind text, contact_value text, contact_name nullable, archived_at nullable, version integer, updated_at | UNIQUE(participant, club), UNIQUE(id,club_id) для будущих исторических FK; локальный контакт; kind phone/email, семейный контакт не UNIQUE. Новая связь блокирует participant для сериализации с правкой общей идентичности |
| training_groups | club_id FK, name text, archived_at nullable, version integer, updated_at | UNIQUE(id, club_id); простой справочник без вместимости/программ |
| group_memberships | club_id FK, group_id FK, club_membership_id FK, valid_from date, valid_until date nullable, version integer, updated_at | Состав на дату. Одна локальная карточка максимум в одной группе на дату; интервалы не пересекаются, until > from. Перевод атомарный |
| schedule_rules | club_id FK, group_id FK, series_id uuid, trainer_id FK staff_users, mode text, weekdays smallint[] nullable, day_parity text nullable, selected_dates date[] nullable, starts_time time, duration_minutes integer, valid_from date, valid_until date nullable | mode weekly/day_parity/date_list; CHECK ровно соответствующего набору режима. Паритет even/odd по дню месяца, weekdays ISO 1–7. Версия правила новой строкой периода, общий series_id; UNIQUE(series_id, valid_from) |
| training_sessions | club_id FK, group_id FK, schedule_rule_id nullable FK, starts_at/ends_at timestamptz, trainer_id FK, status text, roster_frozen_at nullable, closed_at nullable, cancelled_at nullable, cancellation_reason nullable, is_manual_override bool | status planned/open/closed/cancelled; одноразовый rule NULL; Partial UNIQUE(group_id, starts_at) WHERE superseded_at IS NULL, в том числе для отменённых. starts_on/timezone сохраняют исходную дату вхождения/зону (не меняются при переносе); рабочий календарь V1-08 использует фактическую локальную дату starts_at; version, superseded_at сохраняют историю заменённых ordinary planned. Конец > начало. Сохранять отмену/замену при генерации |
| attendance_records | club_id FK, training_session_id FK, club_membership_id FK, source_group_membership_id FK, state text nullable, absence_reason text nullable, marked_by nullable FK, marked_at nullable, origin text nullable, change_reason text nullable, version integer | Одна строка снимка на участника: UNIQUE(session, membership). state present/absent или NULL до закрытия; absence_reason unknown/illness/excused только при absent; present/NULL без причины. origin manual/close_default/correction |
| audit_events | club_id nullable FK, actor_id FK, entity_type text, entity_id uuid, action text, before_state/after_state jsonb nullable, change_reason text nullable | Малый аудит значимых операций. Для отметки старое/новое состояние/причина/версия, без контактного payload/пароля. Аудит не является источником текущего состояния |

Все FK на club-scoped сущности согласуют club_id. attendance_records связывает сеанс и локального участника одного клуба; source_group_membership_id относится к той же группе/участнику снимка. Это обеспечить подходящими составными ключами или проверкой операции под блокировкой и тестом целостности, а не доверять форме.

Назначение trainer_id требует актуальной роли trainer именно в этом клубе и включённого аккаунта; проверять в операции. Нет отдельной group_trainers: доступ группы выводится из назначенных правил/сеансов. Исторический автор не исчезает при отзыве роли.

V1-04 / 0005 не добавляет таблиц: отзывает сессии при смене password_hash/login и сериализует INSERT/DELETE staff_club_roles с правкой аккаунта через staff_users lock; ключи роли неизменяемы. Локальный отзыв удаляет только trainer-роль, сохраняет staff ID/FK и audit_events. Для назначения использовать `auth.RequireTrainer` внутри транзакции; исторические trainer_id/marked_by ссылаются на staff_users.id, не на строку назначения. Права/контракт — [trainers-v1.md](../trainers-v1.md).

## Инварианты операций

1. Создание участника/локальной карточки/начальной группы — одна транзакция. Идентичность общая, контакт локальный; автоматического merge нет.
2. Интервалы одной пары состава и версии одного расписания не пересекаются. Реализовать через транзакционную проверку с блокировкой родительской записи; если используется range EXCLUDE, обосновать расширение, не требовать его заранее.
3. Генерация повторяема: UNIQUE сеанса, отмены/override/open/closed не перезаписываются. Для смены правила проверяются затронутые будущие занятия, не производится скрытое удаление истории.
4. Открытие фиксирует снимок один раз из состава на дату. Повторное открытие не пересобирает его. Обе операции берут блокировку сеанса.
5. mark/close/cancel/replace также блокируют сеанс; закрытие заполняет все NULL как absent/unknown и ставит closed в одной транзакции. Обязательность всех отметок — инвариант операции, а не межстрочный CHECK.
6. cancelled не создаёт отсутствий. Администраторская коррекция closed — по одной с change_reason и проверкой version, аудит в той же транзакции.
7. Отчёт считает только closed и актуальные отметки; cancelled исключён, planned/open отдельно. Состав/причины и число сеансов не смешивать в агрегировании.

## Индексы по реальным запросам

club_memberships(club_id, archived_at), training_groups(club_id, archived_at), group_memberships(group_id, valid_from, valid_until), schedule_rules(club_id, group_id, valid_from), training_sessions(club_id, starts_at), training_sessions(trainer_id, starts_at), attendance_records(training_session_id), audit_events(entity_type, entity_id, created_at), staff_sessions(expires_at). Уникальные ключи дают свои индексы; не создавать дубли без плана запросов.

Точная SQL-форма, обязательные NULL/NOT NULL, CHECK и имена миграций уточняются задачей, которая добавляет таблицу, и проверяются на PostgreSQL. Не добавлять финансовые/медицинские поля «про запас».

[Архитектура](../architecture-v1.md), [диаграммы](diagrams-v1.md), [план](../development-plan-v1.md).

# PostgreSQL: схема JudeOSLite v1

Версия: 1.0, 11.10.2026. Статус: предложенная схема для планирования, не выполненные SQL-миграции. Основа — [архитектура](../architecture-v1.md), [ответы заказчика](../knowledge-base/crm-context.md), [протокол iPay](../knowledge-base/ipay-protocol.md). [Визуальные схемы](diagrams-v1.md) используют те же названия таблиц.

## 1. Соглашения

- 26 небольших таблиц: 4 доступа, 9 участников/занятий/справок, 10 финансов/провайдера, 3 аудита/импорта. Это обычные отношения, не 26 сервисов. Отдельных таблиц родителей-пользователей, бронирований и экипировки сейчас нет.
- `NN` — NOT NULL, `?` — NULL допустим. Все таблицы имеют `id uuid PK` и `created_at timestamptz NN`; UUID создаёт приложение. Исключений с другим PK нет.
- Изменяемые справочники, принадлежности, расписания, занятия, отметки, справки, интервалы цен и настройки имеют `updated_at timestamptz NN`, `version integer NN >= 1`. Клиент присылает ожидаемую version; несовпадение даёт конфликт без перезаписи. Служебные состояния попыток/импорта изменяются под блокировкой транзакции.
- Деньги — `bigint` в копейках BYN; валюта `char(3) = '933'`. Цена/начисление могут быть нулевыми, платёж положителен. Подписанные изменения хранятся в корректировках/зачёте. Нет float, «оплачено» как произвольного флага и сохранённого редактируемого баланса.
- Месяц — `date` первого числа. История принадлежности/расписания/цены использует полуоткрытые периоды `[from, until)`; NULL until означает отсутствие конца. Медицинский период в UI включает последний день: `[starts_on, ends_on]`. Это различие отражается в проверках.
- Время занятий и событий — `timestamptz`, регулярный час — `time`, локальные даты считаются в `clubs.timezone`, предложение — Europe/Minsk. Сырой DateTime iPay отдельно до подтверждения часового пояса B07.
- По умолчанию ссылки защищены `ON DELETE RESTRICT`. История не удаляется при архивировании. Каскад финансовых записей не разрешён; очистка истёкших сессий/временного импорта — отдельный служебный процесс.
- Статусы — `text` с ограниченным CHECK; нет универсальных EAV/JSON для основных сущностей. JSONB допускается только для обезличенного аудита, нормализованной строки импорта и минимального набора полей протокола.
- В пределах клуба используется `club_id uuid NN`. Scoped-таблицы предоставляют `UNIQUE(id, club_id)` и составные FK для связанных данных. Дублирование club_id обеспечивает целостность связи, но не заменяет авторизацию.

**Техническое основание:** составные FK и UNIQUE выражают соответствие ключей; CHECK не проверяет произвольные другие строки. Суммы распределений проверяются транзакцией под блокировками. См. [PostgreSQL Constraints](https://www.postgresql.org/docs/current/ddl-constraints.html) и [Explicit Locking](https://www.postgresql.org/docs/current/explicit-locking.html). Для непересекающихся периодов можно использовать EXCLUDE по диапазону; понадобится btree_gist для UUID-равенства: [Range Types](https://www.postgresql.org/docs/current/rangetypes.html). Конкретные версии и расширения фиксируются до миграций.

## 2. Реестр таблиц

| Область | Таблицы |
| --- | --- |
| Доступ | [T01 clubs](#t01), [T02 staff_users](#t02), [T03 staff_club_roles](#t03), [T04 staff_sessions](#t04) |
| Реестр | [T05 participants](#t05), [T06 club_memberships](#t06) |
| Занятия | [T07 training_groups](#t07), [T08 group_trainers](#t08), [T09 group_memberships](#t09), [T10 schedule_rules](#t10), [T11 training_sessions](#t11), [T12 attendance_records](#t12) |
| Медицинские сведения | [T13 medical_records](#t13) |
| Начисления | [T14 fee_terms](#t14), [T15 charges](#t15), [T16 charge_adjustments](#t16) |
| Провайдер и платежи | [T17 payment_services](#t17), [T18 payment_accounts](#t18), [T19 payment_attempts](#t19), [T20 ipay_events](#t20), [T21 payments](#t21), [T22 payment_reversals](#t22), [T23 payment_allocations](#t23) |
| Аудит/импорт | [T24 audit_events](#t24), [T25 import_batches](#t25), [T26 import_rows](#t26) |

Общие id/created_at/updated_at/version по соглашению выше не повторяются в каждом списке полей. Все дополнительные поля перечислены; знак FK указывает связь, составные варианты уточнены в разделе целостности.

<a id="t01"></a>
## T01. clubs — клуб как отдельный зал

| Поле | Тип | Смысл |
| --- | --- | --- |
| name | text NN | Название клуба/зала |
| address | text ? | Адрес |
| timezone | text NN | IANA-зона локального расписания |
| archived_at | timestamptz ? | Архивирование без удаления |

Уникальность адреса/названия не заменяет ID. Фильтры по состоянию. Архивирование клуба с незавершёнными операциями имеет отдельную проверку, не обнуляет долги.

<a id="t02"></a>
## T02. staff_users — сотрудник и вход

| Поле | Тип | Смысл |
| --- | --- | --- |
| login | text NN | Нормализованный идентификатор входа |
| display_name | text NN | Имя сотрудника |
| password_hash | text NN | Хеш проверенной библиотеки, не пароль |
| is_owner | boolean NN | Общий доступ владельца |
| disabled_at | timestamptz ? | Отключённый вход |

UNIQUE(login). Владелец не задаётся NULL-club ролью: отдельный флаг избегает неоднозначности. Создание/изменение владельца — ограниченная операция по B05, не обычное поле публичной формы. Парольный хеш никогда не возвращается API.

<a id="t03"></a>
## T03. staff_club_roles — доступ сотрудника к клубу

| Поле | Тип | Смысл |
| --- | --- | --- |
| staff_user_id | uuid NN FK staff_users | Сотрудник |
| club_id | uuid NN FK clubs | Клуб |
| role | text NN | administrator / trainer |

UNIQUE(staff_user_id, club_id, role). Схема допускает разные назначения одного сотрудника; права на их выдачу по B05. Администратор ограничен клубом, тренеру отдельно нужны группы/фактические занятия. Отзыв роли не удаляет исторические отметки этого сотрудника; сессия не кеширует старые права бессрочно.

<a id="t04"></a>
## T04. staff_sessions — серверные сессии

| Поле | Тип | Смысл |
| --- | --- | --- |
| staff_user_id | uuid NN FK staff_users | Пользователь |
| token_hash | bytea NN | Хеш случайного токена, не cookie в открытом виде |
| csrf_token_hash | bytea NN | Связь с CSRF-проверкой сессии |
| expires_at | timestamptz NN | Срок жизни |
| revoked_at | timestamptz ? | Выход/отзыв |

UNIQUE(token_hash); индекс staff_user_id и expires_at. Служебная очистка истёкших строк допустима; staff_users не удаляется ради очистки. Доступ одновременно проверяет disabled_at, expires_at, revoked_at и актуальные роли.

<a id="t05"></a>
## T05. participants — идентичность участника

| Поле | Тип | Смысл |
| --- | --- | --- |
| full_name | text NN | ФИО без обязательного машинного разделения |
| birth_date | date ? | Полная дата |
| birth_year | smallint ? | Известный год при неполном импорте |
| archived_at | timestamptz ? | Общая архивная карточка |

Новое создание требует birth_date на уровне операции; импорт может сохранить только год после решения B08. Если оба поля заданы, год совпадает с датой. ФИО/год/телефон не UNIQUE. Индекс поиска по нормализованному имени + id; перед добавлением расширения поиска обосновать потребность. Не угадывать компоненты Name iPay из full_name; поле Name там необязательное, отображение согласовать по B07.

<a id="t06"></a>
## T06. club_memberships — карточка участника в клубе

| Поле | Тип | Смысл |
| --- | --- | --- |
| participant_id | uuid NN FK participants | Единая идентичность |
| club_id | uuid NN FK clubs | Граница данных/финансов |
| status | text NN | active / paused / archived; процессы паузы по B02 |
| joined_on | date NN | Дата первого внесения/вступления, не начало начисления |
| contact_kind | text NN | phone / email |
| contact_value | text NN | Контакт родителя/взрослого |
| contact_name | text ? | Имя контактного лица, если известно |
| archived_at | timestamptz ? | Архивирование локальной карточки |

UNIQUE(participant_id, club_id); UNIQUE(id, club_id). Связь постоянна: возврат не создаёт новую идентичность; история периодов групп/цены отдельна. Не интерпретировать status как бухгалтерское погашение. Контакт принадлежит клубу, а не открыт всей сети администраторам. Индексы (club_id, status, id), participant_id. Валидация контакта по виду; семейный контакт повторяется у детей.

<a id="t07"></a>
## T07. training_groups — группа

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб |
| name | text NN | Название |
| archived_at | timestamptz ? | Архивирование |

UNIQUE(id, club_id); индекс (club_id, archived_at, name). Возрастные категории, вместимость и программы не нужны как обязательные справочники для подтверждённого CRUD; добавить позже при реальном запросе. Состав/тренеры/расписание вынесены из массива в карточке.

<a id="t08"></a>
## T08. group_trainers — датированное назначение тренера

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб |
| group_id | uuid NN FK training_groups | Группа того же клуба |
| staff_user_id | uuid NN FK staff_users | Тренер |
| valid_from | date NN | Начало назначения |
| valid_until | date ? | Конец, не включается |

valid_until > valid_from, если задан; непересечение периодов для (group_id, staff_user_id). Сервис проверяет роль trainer в этом клубе и включённого сотрудника. Смена назначения не меняет фактического тренера прошедшего занятия. Индексы group_id и staff_user_id + периоды.

<a id="t09"></a>
## T09. group_memberships — история состава

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб |
| group_id | uuid NN FK training_groups | Группа |
| club_membership_id | uuid NN FK club_memberships | Участник клуба |
| valid_from | date NN | Начало принадлежности |
| valid_until | date ? | Конец, не включается |

UNIQUE(id, club_id, group_id, club_membership_id) для связи снимка занятия. Непересечение одной пары группа–участник; valid_until > valid_from. Перевод — закрытие одного периода и новый период в транзакции. Схема способна хранить несколько групп, но правила параллельных/гостевых посещений не утверждены; v1 не создаёт их автоматически. Индексы (group_id, valid_from, valid_until), club_membership_id.

<a id="t10"></a>
## T10. schedule_rules — регулярное расписание

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб |
| group_id | uuid NN FK training_groups | Группа |
| trainer_id | uuid NN FK staff_users | Плановый тренер |
| series_key | uuid NN | Стабильный ключ одного регулярного пункта расписания между его версиями |
| weekday | smallint NN | ISO-день 1–7 |
| starts_local | time NN | Местное время начала |
| duration_minutes | integer NN | Положительная продолжительность |
| valid_from | date NN | Начало действия |
| valid_until | date ? | Конец, не включается |

UNIQUE(id, club_id, group_id, series_key). Версии одного series_key не пересекаются по периоду и принадлежат одной группе: изменение выполняется под блокировкой группы. Проверять принадлежность trainer клубу/группе. Проверка конфликтующих правил для группы — сервисная операция; запретить точные дубли/пересечения одинакового слота. Изменение с даты закрывает прежнее правило, создаёт новое с тем же series_key; дополнительный регулярный пункт получает новый ключ. Индекс (group_id, weekday, valid_from, valid_until).

<a id="t11"></a>
## T11. training_sessions — конкретное занятие

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб |
| group_id | uuid NN FK training_groups | Группа |
| schedule_rule_id | uuid ? FK schedule_rules | Источник; NULL для согласованного исторического импорта |
| series_key | uuid ? | Ключ регулярного пункта, совпадает с заданным правилом |
| occurrence_on | date NN | Исходная локальная дата генерации |
| starts_at / ends_at | timestamptz NN | Фактическое время, ends_at > starts_at |
| trainer_id | uuid NN FK staff_users | Фактический тренер; snapshot |
| status | text NN | planned / open / closed / cancelled |
| manually_changed | boolean NN | Защита ручной отмены/замены от регенерации |
| roster_created_at | timestamptz ? | Фиксация состава |
| closed_at | timestamptz ? | Закрытие по B04 |
| change_reason | text ? | Причина отмены/замены/исправления |

UNIQUE(series_key, occurrence_on) для заданного ключа; UNIQUE(group_id, starts_at) для всех состояний, включая cancelled; UNIQUE(id, club_id, group_id). schedule_rule_id и series_key либо оба заданы, либо оба NULL. Индексы (club_id, starts_at), (trainer_id, starts_at). Генератор блокирует группу и ищет уже созданное занятие по series_key/occurrence_on, включая отменённое и перенесённое. Новая версия правила не дублирует его и не снимает manually_changed. Исправление времени не меняет occurrence_on. Изменение обычного будущего planned-занятия допускается только явной операцией обновления расписания; повтор генератора сам его не переписывает. Повторное занятие в занятом или отменённом слоте не создаётся скрытым INSERT.

<a id="t12"></a>
## T12. attendance_records — состав и отметки занятия

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб |
| group_id | uuid NN FK training_groups | Группа для составных FK |
| session_id | uuid NN FK training_sessions | Занятие |
| club_membership_id | uuid NN FK club_memberships | Участник клуба |
| group_membership_id | uuid NN FK group_memberships | Источник датированного состава |
| status | text NN | unmarked / present / absent / sick |
| marked_by | uuid ? FK staff_users | Последний автор отметки |
| marked_at | timestamptz ? | Время отметки |
| change_reason | text ? | Обязательна для административной прошлой правки |

UNIQUE(session_id, club_membership_id). Связь session и group_membership проверяет одинаковые клуб/группу/участника. Для unmarked marked_by/marked_at NULL, для отмеченных заполнены; автор не подставляется клиентом. При открытии занятия строки фиксируются транзакционно. Удаление принадлежности запрещено, завершение её периода не меняет снимок. Индекс (club_membership_id, session_id). История изменений — audit_events; unmarked не попадает в количество отсутствовавших до согласованного B04.

<a id="t13"></a>
## T13. medical_records — допуск и период болезни

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб |
| club_membership_id | uuid NN FK club_memberships | Участник клуба |
| kind | text NN | fitness_clearance / sickness |
| starts_on | date NN | Подтверждённое начало |
| ends_on | date ? | Последний день; правила NULL — B09 |
| recorded_by | uuid NN FK staff_users | Кто внёс |
| revoked_at | timestamptz ? | Отмена ошибочной записи с аудитом |

UNIQUE(id, club_id, club_membership_id). Если ends_on задан, ends_on >= starts_on. БД допускает неполный конец, но UI болезни требует согласованный период; бессрочную болезнь не считать утверждённой. Нет диагнозов, файлов справок и свободного медицинского досье. Индекс (club_membership_id, kind, starts_on). Не изменяет fee_terms/charges автоматически.

<a id="t14"></a>
## T14. fee_terms — месячный взнос по интервалам

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб |
| club_membership_id | uuid NN FK club_memberships | Участник клуба |
| effective_from_month | date NN | Первое число первого месяца |
| effective_until_month | date ? | Первое число месяца прекращения, не включается |
| amount_minor | bigint NN | Месячная сумма >= 0 |
| currency | char(3) NN | 933 |
| created_by | uuid NN FK staff_users | Администратор/владелец по B05 |

UNIQUE(id, club_id, club_membership_id). Непересекающиеся интервалы для club_membership_id; начало/конец — первые числа, конец > начала. Индекс (club_membership_id, effective_from_month). Изменение цены с месяца закрывает старый интервал и создаёт новый в транзакции с блокировкой membership. Закрытие интервала для остановки оплаты зависит от B01/B02. Не выводить billing-период из joined_on или статуса active без решения заказчика.

<a id="t15"></a>
## T15. charges — обязательство за месяц

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб |
| club_membership_id | uuid NN FK club_memberships | Участник клуба |
| billing_month | date NN | Первое число месяца |
| kind | text NN | monthly_fee / opening_balance |
| source_fee_term_id | uuid ? FK fee_terms | Источник цены; для monthly_fee обязателен |
| base_amount_minor | bigint NN | Снимок исходной суммы >= 0 |
| currency | char(3) NN | 933 |
| due_on | date ? | Согласованная дата оплаты; не подставлять 7-е без B03 |

UNIQUE(club_membership_id, billing_month, kind); UNIQUE(id, club_id, club_membership_id). opening_balance — предусмотренное место начального долга, но его применение запрещено до B01. Сумма/источник/период не меняются обычным PATCH после выставления; корректировки отдельны. Индекс (club_id, billing_month), (club_membership_id, billing_month). paid/unpaid/overdue вычисляются, не хранятся как произвольные поля.

<a id="t16"></a>
## T16. charge_adjustments — ручные корректировки

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб |
| club_membership_id | uuid NN FK club_memberships | Для границы и связи основания |
| charge_id | uuid NN FK charges | Корректируемый месяц |
| delta_minor | bigint NN | Подписанное изменение, не 0 |
| reason | text NN | Обязательное объяснение |
| medical_record_id | uuid ? FK medical_records | Основание болезни того же участника/клуба |
| created_by | uuid NN FK staff_users | Ответственный |
| request_key | text NN | Повторяемый ключ команды |
| request_fingerprint | text NN | Хеш нормализованного содержимого команды |

UNIQUE(club_membership_id, request_key); одинаковый ключ с другим содержимым — 409, не новая правка. Только добавление записей; исправление ошибочной корректировки — обратное изменение с причиной. Итог обязательства не отрицателен; проверка суммы под блокировкой charge, не CHECK с запросом к другим строкам. Если уменьшение создаёт превышение зачёта, операция должна в той же транзакции вернуть превышение в нераспределённые деньги через T23; соответствующая политика по B03. Нельзя оставить отрицательный остаток/потерять переплату. Индекс charge_id.

<a id="t17"></a>
## T17. payment_services — договорная услуга iPay

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб-получатель в модели |
| connection_key | text NN | Несекретный alias договорного подключения |
| route_key | text NN | Несекретный ключ маршрута callback |
| service_no | varchar(9) ? | Код услуги, если известен; не угадывать |
| mode | text NN | disabled / test / production |
| last_success_at | timestamptz ? | Последний корректный обработанный обмен |

UNIQUE(route_key), UNIQUE(connection_key, service_no) для заданного service_no; UNIQUE(id, club_id). route_key уникален во всей установке, поскольку URL callback не включает connection_key. service_no — строка цифр, ведущие нули сохраняются. Секрет подписи/credentials хранятся вне таблицы по connection_key; route_key не заменяет подпись. Отсутствие ServiceNo в запросе требует однозначного маршрута по B07. Несколько услуг на один клуб допустимы, но реальные коды не придуманы. Финансовую свежесть нельзя выводить только из запроса ServiceInfo: успешный обмен не означает получение всех результатов/реестра.

<a id="t18"></a>
## T18. payment_accounts — привязка лицевого счёта

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб |
| service_id | uuid NN FK payment_services | Услуга этого клуба |
| club_membership_id | uuid NN FK club_memberships | Получатель зачёта |
| personal_account | varchar(30) NN | Строка счёта без числового преобразования |
| disabled_at | timestamptz ? | Запрет новых попыток, история остаётся |

UNIQUE(service_id, personal_account); UNIQUE(id, club_id, service_id, club_membership_id). Счёт не переиспользуется другим участником. В первой модели привязка к одному участнику клуба; семейные/групповые счета ждут реального сценария B07. Нельзя выдавать внутренний UUID за принятый ЕРИП-номер без проверки формата. Индекс club_membership_id. Общий статус участника не мешает подтвердить ранее начатую оплату/разобрать долг.

<a id="t19"></a>
## T19. payment_attempts — принятая попытка оплаты

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Разрешённый клуб |
| service_id | uuid NN FK payment_services | Услуга |
| payment_account_id | uuid NN FK payment_accounts | Счёт, проверенный при TransactionStart |
| club_membership_id | uuid NN FK club_memberships | Получатель, фиксируется на старте |
| connection_key | text NN | Подключение, должно совпадать с service |
| provider_transaction_id | varchar(12) NN | TransactionId iPay; строка цифр |
| provider_request_id | varchar(8) NN | RequestId цепочки, не единственный ключ идемпотентности |
| provider_trx_id | varchar(12) NN | ServiceProvider_TrxId CRM, стабильный для повтора |
| amount_minor | bigint NN | Принятая сумма > 0 |
| currency | char(3) NN | 933 |
| target_month | date ? | Снимок целевого месяца, только если выбран B03 |
| allocation_policy_key | text ? | Версия согласованной политики зачёта |
| status | text NN | pending / succeeded / failed |
| storn_status | text NN | none / requested / succeeded / failed |
| completed_at | timestamptz ? | Время принятого результата |

UNIQUE(connection_key, provider_transaction_id), UNIQUE(service_id, provider_trx_id), UNIQUE(id, club_id, club_membership_id). Область уникальности iPay пока предложение: подтвердить B07; до ответа чужое совпадение не объединять и не кредитовать, а отправлять на разбор. provider_trx_id должен укладываться в 12 символов — полный UUID не подходит. Повтор TransactionStart возвращает прежний ID, сумму/получателя не меняет. Сумма берётся отсюда для результата. Противоречивые параметры повтора не переписывают попытку. Индексы (service_id, status, created_at), payment_account_id.

<a id="t20"></a>
## T20. ipay_events — надёжно принятые сообщения

| Поле | Тип | Смысл |
| --- | --- | --- |
| service_id | uuid NN FK payment_services | Определено доверенным маршрутом |
| attempt_id | uuid ? FK payment_attempts | NULL при неразобранной цепочке |
| request_type | text NN | ServiceInfo / TransactionStart / TransactionResult / StornStart / StornResult |
| correlation_key | text NN | Тип и подтверждённый ID цепочки/операции |
| semantic_fingerprint | varchar(64) NN | Хеш значимых полей, не транспортной даты |
| provider_datetime_raw | varchar(14) NN | DateTime до подтверждения зоны |
| canonical_fields | jsonb NN | Белый список полей: ID/счёт/валюта/сумма/результат; без Name/Address/подписей |
| processing_state | text NN | received / processed / quarantined |
| delivery_count | integer NN | Количество одинаковых доставок >= 1 |
| last_received_at | timestamptz NN | Последний повтор |
| processed_at | timestamptz ? | Надёжная обработка |
| issue_code | text ? | Код разбора, без секретов/сырого медицинского текста |

UNIQUE(service_id, request_type, correlation_key, semantic_fingerprint). Так повтор объединяется, а конфликт значимых полей сохраняется отдельным вариантом. Одинаковый RequestId разных типов не является повтором одного события. DateTime не создаёт новый денежный эффект. Финансовая уникальность дополнительно обеспечивается T19/T21/T22, не только хешем сообщения.

Сохраняются только корректно аутентифицированные/валидированные сообщения; лог отказов не должен копировать секреты. ServiceInfo можно не сохранять постоянно, оставить минимальную диагностику; финансовые события сохраняются. Валидный неизвестный результат — quarantined без выдуманной суммы. Ответ формируется по договорному типу, не по processing_state автоматически. Индекс (service_id, processing_state, created_at), attempt_id. Служебное повторное применение не выполняет платёж дважды.

<a id="t21"></a>
## T21. payments — факт успешного платежа

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб |
| club_membership_id | uuid NN FK club_memberships | Участник клуба |
| attempt_id | uuid NN FK payment_attempts | Подтверждённая попытка |
| confirmation_event_id | uuid NN FK ipay_events | Принятый успешный результат |
| amount_minor | bigint NN | Полученная сумма > 0, равна попытке |
| currency | char(3) NN | 933 |
| confirmed_at | timestamptz NN | Время обработки успеха CRM |
| provider_occurred_at | timestamptz ? | Время провайдера после подтверждения зоны |

UNIQUE(attempt_id), UNIQUE(confirmation_event_id), UNIQUE(id, club_id, club_membership_id). Запись неизменяемая; нет UI редактирования суммы/статуса. Сторно и разнесение отдельны. Технически успешный нераспределённый платёж остаётся фактом, не исчезает при ошибке выбора периода. Индекс (club_id, confirmed_at), (club_membership_id, confirmed_at). Исторический импорт платежей требует отдельного подтверждённого источника; не имитировать iPay attempt для текста «оплачено» из Excel.

<a id="t22"></a>
## T22. payment_reversals — подтверждённое сторнирование

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб |
| club_membership_id | uuid NN FK club_memberships | Участник |
| payment_id | uuid NN FK payments | Исходный платёж |
| attempt_id | uuid NN FK payment_attempts | Та же попытка, что у исходного платежа; для FK результата |
| confirmation_event_id | uuid NN FK ipay_events | Успешный StornResult |
| amount_minor | bigint NN | Подтверждённая сумма > 0 |
| confirmed_at | timestamptz NN | Время обработки |

UNIQUE(payment_id), UNIQUE(confirmation_event_id), UNIQUE(id, club_id, club_membership_id, payment_id). Для предложенной v1 — одно полное сторно, amount равна платежу. Частичный/второй отличающийся результат уходит на разбор B07, не теряется и не проходит как поддержанный сценарий. Это ограничение предложения, а не доказанное ограничение iPay. Удалять исходный платёж нельзя. Неуспешное StornResult не создаёт эту запись. Индекс payment_id.

<a id="t23"></a>
## T23. payment_allocations — история зачёта денег

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб |
| club_membership_id | uuid NN FK club_memberships | Один получатель платежа/начисления |
| payment_id | uuid NN FK payments | Платёж |
| charge_id | uuid NN FK charges | Начисление |
| amount_minor | bigint NN | > 0 зачёт, < 0 отмена части прежнего зачёта; не 0 |
| reverses_allocation_id | uuid ? FK payment_allocations | Для отрицательной записи — исходная положительная |
| payment_reversal_id | uuid ? FK payment_reversals | Если причина — провайдерское сторно |
| reason_code | text NN | payment / provider_storno / reallocation |
| created_by | uuid ? FK staff_users | NULL у операции провайдера |
| entry_key | text NN | Уникальный детерминированный ключ записи |

UNIQUE(entry_key); UNIQUE(id, club_id, club_membership_id, payment_id, charge_id). Положительная запись не имеет reverses_allocation_id; отрицательная имеет его и совпадает с оригиналом по платежу/начислению/клубу/участнику. Сумма всех отмен конкретного оригинала не превышает положительную сумму. Индексы payment_id, charge_id, reverses_allocation_id.

Записи добавляются, не удаляются и не заменяются. При сторно добавляются отрицательные записи для оставшихся зачётов; нераспределённый остаток тоже аннулируется через T22. При ручном исправлении распределения — отрицательная запись и новый зачёт в транзакции. Банк-платёж не изменяется. Входящие повторы используют стабильные entry_key; отрицательные ключи учитывают ID оригинальной записи, чтобы отмена нескольких зачётов не конфликтовала.

<a id="t24"></a>
## T24. audit_events — минимальный аудит

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid ? FK clubs | NULL только для общесетевого действия |
| actor_id | uuid ? FK staff_users | NULL у jobs/провайдера |
| actor_kind | text NN | staff / system / ipay |
| entity_type | text NN | Разрешённый вид объекта |
| entity_id | uuid NN | ID объекта, полиморфный указатель |
| action | text NN | Создание/изменение/архивирование и др. |
| changed_fields | jsonb NN | Белый список значимых полей/дельт |
| reason | text ? | Причина, когда обязательна |
| request_key | text ? | Корреляция с операцией |

Индекс (club_id, created_at), (entity_type, entity_id, created_at). entity_id не является FK на одну таблицу: полиморфизм только в аудите, не в основных связях. Запись с изменением — одна транзакция. Аудит не копирует пароль, токены, полный контакт/Name/Address XML; персональные медицинские заметки не собираются. Не выдавать JSON аудита всем тренерам. Срок хранения — эксплуатационное решение до запуска.

<a id="t25"></a>
## T25. import_batches — управляемый импорт

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб назначения |
| uploaded_by | uuid NN FK staff_users | Кто загрузил |
| source_digest | varchar(64) NN | Хеш файла, не сам файл |
| source_label | text NN | Безопасное название |
| format_version | text NN | Версия согласованного маппинга Excel |
| state | text NN | draft / validated / applying / applied / failed |
| apply_request_key | text ? | Идемпотентный ключ подтверждения |
| applied_at | timestamptz ? | Применение |

UNIQUE(id, club_id); уникальный непустой (club_id, apply_request_key). Хеш файла служит предупреждением о повторе, не единственным ключом идентичности участника. Повтор того же batch возвращает результат. Сырой Excel хранится временно вне Git и удаляется по согласованному сроку; для первого импорта не требуется облачное файловое хранилище. Индекс (club_id, state, created_at).

<a id="t26"></a>
## T26. import_rows — проверенные исходные строки

| Поле | Тип | Смысл |
| --- | --- | --- |
| club_id | uuid NN FK clubs | Клуб batch |
| batch_id | uuid NN FK import_batches | Пакет |
| source_locator | text NN | Лист/блок/строка, не ФИО как ключ |
| normalized_fields | jsonb NN | Только нужные поля карточки/группы |
| validation_errors | jsonb NN | Ошибки полей без большого raw-лога |
| state | text NN | pending / ready / rejected / applied |
| resolved_membership_id | uuid ? FK club_memberships | Проверенное совпадение/созданная карточка своего клуба |
| resolved_group_id | uuid ? FK training_groups | Согласованная группа |

UNIQUE(batch_id, source_locator); составная связь batch/клуб и результата/клуб. Персональные normalized_fields ограничены теми же правами и сроком хранения, что реестр; не попадут в Git/fixtures. Год без даты не превращается в фиктивную дату. Неизвестное «б/о» — ошибка/запрос уточнения, не автоматическая льгота. Для первой версии импорт карточек/состава; перенос старых отметок и денежных остатков отдельно по B01/B08, не молчаливое расширение этого JSON.

<a id="integrity"></a>
## 3. Составные связи и межстрочная целостность

| Дочерняя запись | Связь, которую необходимо обеспечить |
| --- | --- |
| group_trainers / group_memberships | (group_id, club_id) → training_groups; участник членства также в этом клубе |
| schedule_rules | (group_id, club_id) → training_groups; тренер имеет допустимое назначение |
| training_sessions | (group_id, club_id) → training_groups; для заданного правила также (schedule_rule_id, club_id, group_id, series_key) → schedule_rules |
| attendance_records | (session_id, club_id, group_id) → training_sessions; (group_membership_id, club_id, group_id, club_membership_id) → group_memberships |
| medical_records / fee_terms / charges | (club_membership_id, club_id) → club_memberships |
| charges | Заданный source_fee_term принадлежит тому же membership/клубу |
| charge_adjustments | charge и заданная medical_record совпадают по membership/клубу |
| payment_accounts | service и membership в одном клубе |
| payment_attempts | account совпадает по service/club/membership; connection_key совпадает с service |
| payments | attempt совпадает по club/membership; confirmation_event относится к той же attempt |
| payment_reversals | payment совпадает по club/membership; event связан с его attempt |
| payment_allocations | payment и charge совпадают по club/membership; original allocation и reversal относятся к тому же payment |
| import_rows | batch, resolved membership/group совпадают по club |

Для перечисленных полных кортежей родители получают необходимые UNIQUE, а дети — составные FK. Совпадение connection_key гарантировать UNIQUE(id, club_id, connection_key) у service и соответствующим FK attempt. Для event с заданной attempt: (attempt_id, service_id) → attempts с UNIQUE(id, service_id). Event предоставляет UNIQUE(id, attempt_id); payment/reversal ссылаются на (confirmation_event_id, attempt_id). Payments дополнительно предоставляет UNIQUE(id, club_id, club_membership_id, attempt_id) для ссылки reversal на исходную попытку. Конкретные вспомогательные UNIQUE обязательны в миграционном плане, даже если не повторены в таблице полей.

Общих одиночных FK недостаточно для защиты от склейки двух клубов. Служебные event-связи с NULL attempt до разбора разрешены; переход к обработанному событию отдельно проверяется. Проверка ролей, даты действия, суммы нескольких строк и допустимого перехода — сервисная транзакция; CHECK с произвольным SELECT не используется.

<a id="transactions"></a>
## 4. Транзакционные операции и порядок блокировок

| Операция | Атомарный результат и защита |
| --- | --- |
| Новый участник/перевод | Карточка/членство/период группы + аудит; блокировка membership, проверка версий/клуба |
| Изменение цены с месяца | Закрытие прежнего fee_term и новый интервал + аудит; без пересечения |
| Генерация занятия | Блокировка группы, проверка слота/ручных изменений, INSERT с уникальным ключом; повтор не меняет историю |
| Открытие занятия | Блокировка session, создание snapshot attendance, статус/roster_created_at; повтор возвращает тот же состав |
| Прошлая отметка | Права администратора, ожидаемая version, причина и аудит |
| Месячное начисление | Исторический fee_term + snapshot charges; UNIQUE защищает повтор; существующая сумма не обновляется |
| Корректировка начисления | Блокировка получателя и charge, проверка итогов, adjustment + нужные отрицательные allocations + аудит по B03 |
| TransactionStart | Проверка trusted service/account; attempt и событие; стабильный provider_trx_id возвращается только после commit |
| Успешный результат | Блокировка получателя, затем attempt, событие, один payment, зачёт или сохранённый нераспределённый остаток по B03; commit до ACK |
| Подтверждённое сторно | Состояние попытки, reversal и обратные allocations; повтор не создаёт второй reversal |
| Импорт batch | Блокировка batch; все готовые карточки/сопоставления и состояние applied вместе. Для большого файла дробление только с явным прогрессом/ключами, не неявное частичное применение |

Предложенный единый порядок финансовых блокировок: club_membership → attempt/payment → charge по ID → allocation по ID. Операции без финполучателя (неизвестные события) сохраняют только inbox. Не брать сначала attempt, а затем membership в другом кодовом пути. Каждый финансовый путь держит membership-блокировку до commit, включая корректировку и сторно. В миграционном/сервисном плане отдельно проверить отсутствие deadlock при двух платежах и корректировке.

<a id="balances"></a>
## 5. Вычисляемые итоги

Для одного начисления:

```text
обязательство = base_amount_minor + SUM(charge_adjustments.delta_minor)
зачтено = SUM(payment_allocations.amount_minor)
остаток = обязательство - зачтено
```

Для одного платежа:

```text
доступные деньги = payments.amount_minor - SUM(payment_reversals.amount_minor)
нераспределённые деньги = доступные деньги - SUM(payment_allocations.amount_minor)
```

NULL SUM трактуется как 0. Инварианты: обязательство >= 0; 0 <= зачтено <= обязательство; 0 <= суммарный зачёт платежа <= его доступные деньги. Переплата сохраняется как нераспределённая сумма, не теряется и не превращается автоматически в следующий месяц. Правила автоматического применения по B03. Получение платежа, его зачёт и банковское зачисление — разные показатели.

До определения политики не показывать «просрочен» на основе выдуманной даты/периода. Неразобранное событие не равно подтверждённому платежу. После обработки сторно возвращённый долг объясняется исходным payment/reversal/allocations, а не перезаписанной оплатой.

## 6. Индексы и миграционная последовательность

Индексы строятся под перечисленные запросы; FK на стороне ребёнка при необходимости индексируется отдельно. UNIQUE родителя автоматически не индексирует FK ребёнка. Не добавлять индекс на каждую колонку или materialized dashboard заранее.

Порядок групп миграций для планировщика: clubs/users/roles/sessions → participants/memberships → groups/assignments/schedule/sessions/attendance → medical/fee_terms/charges/adjustments → services/accounts/attempts/events/payments/reversals/allocations → audit/import. Цикл adjustment/medical или event/attempt разбивается созданием таблиц и последующим добавлением FK, если это нужно. Это группы зависимостей, не разрешение выполнить один огромный пункт «создать всю БД».

Проверки будущих миграций: чистая установка и обновление предыдущего состояния; wrong-club FK, перекрывающиеся интервалы, повтор месячного начисления/слота/payment, сохранение ведущих нулей, нулевой взнос, денежные ограничения. Блокировки/суммы/переходы проверять интеграционными сценариями сервиса с PostgreSQL.

## 7. Где схема намеренно не выбирает бизнес-ответ

Полные даты импорта B08; остановка оплаты B01/B02; сумма/зачёт/предоплата B03; закрытие unmarked B04; роли B05; статистика B06; ключи/подпись/сторно/сверка iPay B07; сроки справок B09. Возможность поля не означает разрешённый workflow. Изменение этих условий требует обновить схему, диаграммы и архитектуру согласованно.

Будущие кабинеты смогут ссылаться на participants, но их таблицы и связи представителей сейчас не создаются. Существующий платёжный ledger не является универсальной бухгалтерией. Аудит/импорт/inbox — служебные части, а не отдельная event-sourcing платформа.

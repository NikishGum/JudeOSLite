# Краткая память PRIDE CRM

## Переход на выбранный дизайн — 14.10.2026

По прямому поручению заказчика в чате прототипа создан отдельный этап из 6 законченных задач RD-01–RD-06 (#183–#188), 8 прямых зависимостей, включая завершённый V1-18/#166. Цель — перенести выбранную Working Panel в React с реальными API и полными возможностями v1; отдельный HTML/CSS/JS-прототип 8016b74 остаётся референсом. Владельцу обзор, администратору/тренеру занятия, на телефоне основное действие раньше второстепенных показателей. Переключатель роли, демоданные и жёсткие даты прототипа в production не переносятся. Финансы/iPay, Excel и справки остаются отложенными.

[План](../development-plan-redesign.md), [карточки](../implementation-tasks-redesign.json), [реестр](../github-issues-redesign.json). Ту же безопасную очередь расширили двумя отдельными реестрами; новые задачи первоначально Todo без исполнителей. Это создание задач, не реализация приложения. Фактические статусы и готовность всегда проверять в live GitHub, не по этой памяти. Первую RD-01/#183 поручать отдельному чату; затем RD-02–RD-04 с согласованием общих файлов, RD-05 главные экраны и RD-06 QA. Поручение переноса не заменяет итоговую пользовательскую приёмку.

Обновлено: **14.10.2026**. Источник актуальных решений — прямые запросы и ответы заказчика в обсуждении плана/issues. Предыдущая полная память/исследование от 11.10.2026 доступна в [истории](../archive/README.md); её финансовые требования для v1 заменены отложенным этапом.

## Текущие подтверждённые решения

- Внутренняя CRM сотрудников: React, Go, PostgreSQL, VPS/Docker Compose; mobile-first и PWA. Каждый зал — отдельный филиал/club.
- Владелец управляет всей сетью, администратор своими филиалами, тренер назначенными группами/занятиями. Несколько назначений сотруднику разрешены; администратор создаёт тренеров своего филиала и выбирает их для занятий.
- Первая версия **без контроля оплат и любой финансовой реализации**: ручной реестр, группы/история состава, расписание, посещаемость, статистика владельца. Excel и отдельные медицинские справки оба позже.
- Расписание: выбранные дни недели, чётные/нечётные даты, выбор дат календаря или разовое занятие. Чётность числа дня месяца — явно обозначенное инженерное допущение; остальные четыре режима подтверждены.
- Тренер видит последовательные карточки, только кнопки, без свайпов; сохранение перед переходом. Отсутствие: неизвестна/болезнь/уважительная. Нижняя панель — предложение мобильного UX.
- Тренер завершает с неотмеченными: они атомарно становятся absent/unknown. Тренер исправляет до закрытия; после — администратор по одной с обязательной причиной, без ограничения давности. Справка сама не меняет отметку.
- Отчёт владельца: филиал/группа/период, проведённые занятия, присутствия/пропуски/причины, процент present/(present+absent) по закрытым. Отменённые исключены, незавершённые отдельно.
- Проверить iPhone/Safari, Android/Chrome; на ПК Chrome/Edge. Связь нужна, офлайн-очереди нет. Нет родительского доступа.
- Фундамент должен позволять добавлять функции/сущности, но быть простым и не исходить из сильного масштабирования. Приоритет — скорость первой работающей версии.
- Новый план — **18 полных задач, 27 прямых зависимостей**, предел 20 включая QA/развёртывание/приёмку. Старые 148 issues удалены по поручению заказчика; требования сохранены в Git, отложенные функции без текущих issues.
- Для координации достаточно исполнителей/меток issues. Последнее уточнение заказчика 12.10.2026 заменяет ручное назначение: агент по поручению пользователя автоматически занимает свободную готовую задачу через `python3 scripts/claim_issue.py НОМЕР`. Общая очередь GitHub назначает аккаунт отправителя и In progress; второй запрос получает отказ, даже от другого чата того же аккаунта. Чужое назначение не менять, перед кодом проверять успешный запрос/статус и зависимости в main. In review/ревью коллеги не обязательны. [Правила](../collaboration.md).
- Уточнение заказчика 12.10.2026: один issue — один новый чат, чаты создаёт пользователь. Агент сам переименовывает свой чат через set_thread_title; точное название «V1-xx · #номер — краткий результат» и инструкция переименования входят в один копируемый стартовый промпт. При завершении агент рекомендует следующую доступную задачу, её занимает новый чат. Решения остаются в Git. Первый пользовательский приоритет — спортсмены, контакты и состав групп, без обхода технических зависимостей. [Передача контекста](../collaboration.md#одна-задача--один-чат).
- Согласованные изменения коммитить и отправлять, только файлы задачи, без force push; проверять отправку. Реализация — по отдельному поручению после проверки live назначения и статуса.
- Решение владельца 13.10.2026 для #165: существующий VPS/test.judopride.tech, root-команды владельцем; backup только локально, ежедневно 03:00 Europe/Minsk, семь последних завершённых плюс current/previous/operation references, incomplete/unknown сохранять. Offsite исключён из критериев. Потеря VPS/диска означает потерю данных и копий. [Протокол](https://github.com/NikishGum/JudeOSLite/issues/165#issuecomment-6140917093).

## Предложения, а не дополнительные требования

Один Go-монолит/React/PostgreSQL; pgx, net/http, явные SQL; 12 таблиц по мере задач, миграции, клубные ключи/права, датированные интервалы, снимки состава, малый аудит и необходимые блокировки. Нет пустых модулей будущих финансов/медицины или дополнительной инфраструктуры. Общее изменение многоклубного аккаунта/идентичности — владельцу; локальное — администратору. Подробно [архитектура](../architecture-v1.md) и [схема](../database/schema-v1.md).

## Сохранённый будущий этап

iPay только после доступа ЛК и уточнения взаимодействия/договора. Ранее согласованы ручная сумма неполного месяца, месяцы остановки/возобновления, точная оплата выставленной суммы без частичной/предоплаты, неоплаченная сумма текущего месяца. Долги прошлых месяцев и последовательный переход к формированию оплат через CRM **оставлены открытыми и обязательно обсуждаются перед будущими платёжными тестами**.

Медицинские документы отдельные; конец обоих видов может быть неизвестен; отметки исправляются по одной. При будущем Excel можно хранить только год рождения, совпадения вручную, «б/о» неизвестно и не преобразуется автоматически. Это не задачи текущей версии.

[Контекст CRM](crm-context.md), [iPay](ipay.md), [история прежнего комплекта](../archive/README.md). Публичное исследование и PDF датированы; не выдавать их за действующие тарифы/договор. Секретов и персональных реестров в памяти нет. qdrant-memory не обнаружен, память поддерживается в Git.

## Проверенный результат переноса — 12.10.2026

Созданы #149–#166: 18 полных карточек и 27 штатных зависимостей, тексты и обе стороны связей сверены с источником. Старых issues нет; все новые Todo без исполнителей. Автоматика меток опубликована, 18 открытий и реальное восстановление Todo успешно проверены. [Реестр](../github-issues-v1.md). Разработка приложения не начата; первая задача #149 назначается человеком.

## Рабочая основа V1-01 — 12.10.2026

Источник: отдельное поручение заказчика выполнить #149, затем подтверждение назначения. Live issue проверено: один NikishGum, In progress, зависимостей нет. Созданы Go net/http/pgx, React/TypeScript/Vite, PostgreSQL/Compose, один SQL runner с блокировкой и checksum, JSON-ошибки/API-клиент, оболочка на PRIDE и CI. Инструкции/версии — [foundation-v1.md](../foundation-v1.md) и корневой README.

Проверены сборки, реальные PostgreSQL-тесты миграций, чистый Compose/restart и восстановление readiness после недоступности БД, отсутствие обязательной конфигурации, SIGTERM и Chromium 320/390/1280 px. Предметные функции, вход, реальные устройства/PWA/VPS и приёмка ещё не реализованы/не проверены; следующую задачу автоматически не брать. Завершение в main и актуальный статус смотреть в #149, не по датированному срезу памяти.

## Ранний тестовый стенд (12.10.2026)

По прямому поручению владельца добавлена V1-19 / #167: автоматический деплой успешного CI main на test.judopride.tech. Зависит от готовой V1-01; V1-17 переиспользует инфраструктуру после V1-13 и проверяет полную CRM/предметный restore. Работа агента только в Cloud; VPS, GitHub Secrets и реальные проверки выполняет владелец. DEPLOY_PATH=/srv/judeos-test, config=/srv/judeos-test-config. Доступ judeos-deploy только через root-owned ограниченный controller.py и forced SSH command, без Docker-группы и общего sudo.

Подготовленная реализация и ручные команды — [operations-test.md](../operations-test.md), результаты — [test-deploy-validation.md](../test-deploy-validation.md). Bootstrap только staging: прежний контроллер/ключи/sudoers/firewall сохраняются; активация нового протокола по подтверждённому hash — явная root-операция владельца. Учитывается готовая V1-02: APP_ORIGIN HTTPS, первый владелец создаётся скрытым терминальным prompt после deploy. GitHub Environment test и Secrets настроены владельцем; подключение Cloud не имеет API-прав управления environment (403), Secrets агент не заполнял.

Первый реальный deploy подтверждён: [Actions 38250186366, attempt 2](https://github.com/NikishGum/JudeOSLite/actions/runs/38250186366) success, main `90e09c322c13c854a51f816d11b1bb7e21084fa1`; curl владельца вернул HTTPS readyz `ok` и совпадающий SHA release.json. PR #168 и #169 merged; исправления #169 доставлены в main `2d13c7f0fda736e24cd024c878999cd1f63a5e04`. Владелец вручную применил исправления: прежний sshd ForceCommand переопределял команду ключа; transient systemd-run требует явного TimeoutStartSec=infinity (0 давал немедленный deadline и SIGTERM). В #169 также добавлены bounded retries GitHub API 5xx/connection failures. 29 Cloud tests и CI кодового commit 2e640ea прошли. При продолжении #167 владелец создал backup `20261012T142857-81978086`, восстановил его в отдельную `restorecheck_b3b1063d62ad`, сверил migration history и 2 staff/1 club. Несовместимый rollback отказал без изменения state. Агент проверил success CI 38263182332 и автоматического Deploy test 38263524825, публичные HTTPS readyz/SHA `2d13c7f`. Финальные persistence/совместимый rollback, SHA и live завершение — в [#167](https://github.com/NikishGum/JudeOSLite/issues/167); закрывать только после результатов владельца. Отчётный docs-only релиз позволяет проверить rollback между SHA с одинаковой схемой/кодом. Не объявлять V1-17/PWA/offsite выполненными.

## Доступ сотрудников V1-02 — 12.10.2026

По поручению #150: захват через очередь подтверждён для StarMadeGalaxy, [run 38233441350](https://github.com/NikishGum/JudeOSLite/actions/runs/38233441350). Добавлены auth API/UI, серверные 12-часовые сессии с хешами, Argon2id, Origin/CSRF, ограничение входа и интерактивный bootstrap первого владельца. Disable отзывает сессии DB-trigger; роли читаются заново. 0002 создаёт staff-таблицы и минимальный clubs: V1-03 развивает управление существующей таблицей. Контракт/запуск/границы — [auth-v1.md](../auth-v1.md).

Owner видит сеть; administrator/trainer — назначенные филиалы, несколько ролей разрешены. RequireClub не заменяет проверку фактического тренера занятия в будущих предметных задачах. Локальный HTTP включается явно только для loopback origin; на VPS нужны HTTPS origin и Secure cookie. Limiter процессный, не доверяет forwarded headers. Реальные устройства/HTTPS/VPS/приёмка не объявлены проверенными; live завершение и CI смотреть в issue. Следующая техническая зависимость реестра — V1-03 / #151; её занимает отдельный новый чат.

## Филиалы и администраторы V1-03 — 12.10.2026

По поручению #151: acquired=true для StarMadeGalaxy, [run 38236951454](https://github.com/NikishGum/JudeOSLite/actions/runs/38236951454), request 720739135cf747e18a74a79a657f716f. Owner API/UI управляет существующими clubs, создаёт администраторов, меняет имя/несколько назначений, отзывает роль и отключает общий аккаунт. 0003 добавляет версии и audit_events; прежние миграции/ID/назначения/сессии сохраняются. Trainer-роли не удаляются при замене administrator-назначений. Подробности — [clubs-v1.md](../clubs-v1.md).

Архив сохраняет историю/назначения, убирает филиал из рабочего выбора и запрещает новые назначения. Для будущих занятий V1-07/V1-08 должны вызывать clubs.RequireActive внутри транзакции после RequireClub: shared lock исключает гонку с архивированием. Таблицы/API занятий пока не создаются. Audit атомарен с изменением и не содержит пароли, токены, логины/имена сотрудников и контактный payload. Версии предотвращают тихую перезапись; ошибки сохраняют форму, пароль очищается. PostgreSQL/API/race, миграционный upgrade с данными V1-02, Chromium 320/390/1280, Docker/Compose проверены; реальные устройства/HTTPS/VPS/приёмка остаются следующими задачами. Live завершение/CI — в issue. После V1-03 доступны V1-05/#153 (приоритет ручного реестра) и V1-04/#152; новый чат проверяет актуальные зависимости/назначение и сам занимает выбранную задачу.

## Ручной реестр V1-05 — 12.10.2026

По поручению #153: acquired=true для StarMadeGalaxy, [run 38243514412](https://github.com/NikishGum/JudeOSLite/actions/runs/38243514412), request e6bc7f812cd04801ab6ea95495bdd908. 0004 добавляет participants и club_memberships; clubs/audit_events/auth сохраняются. Scoped API/UI: ручное создание, поиск/пагинация, карточка, изменение, архив/возврат. Полная дата явно вводится; ФИО/семейный контакт не уникальны, автоматического сопоставления/merge/присоединения нет. [Контракт](../participants-v1.md).

Администратор меняет идентичность при единственной связи с его филиалом; связи других филиалов, включая архивные, требуют owner. Контакт/состояние локальны, canEditIdentity не раскрывает чужие контакты. Identity/version и membership/version отдельные; транзакция блокирует actor/роль, активный club, participant, membership. Новая связь также блокирует participant. Audit атомарен, хранит версии/состояние/признаки правок, без ФИО/даты рождения/контактов. Архив не удаляет ID/историю; UNIQUE(id,club_id) готов для FK состава/отметок. В V1-06 ID карточки — membership.id, а не participantId; начальная группа должна добавляться в одной транзакции создания.

PostgreSQL/API/race, даты/контакты/дубли, общая идентичность/изоляция, конфликты/блокировки, немедленный отзыв/disable и отзыв во время запроса, исторический составной FK, rollback/privacy аудита, Chromium 156 320/390/1280 с формами/сбоем/retry/двойным submit/архивом/reload, Docker/Compose проверены. Live завершение/SHA/CI — в issue; реальные устройства/HTTPS/VPS/PWA/приёмка остаются будущими задачами. Для групп нужна также V1-04/#152; следующую свободную готовую задачу занимает отдельный новый чат.

## Тренеры филиала V1-04 — 12.10.2026

По поручению #152: acquired=true для StarMadeGalaxy, [run 38247326219](https://github.com/NikishGum/JudeOSLite/actions/runs/38247326219), request 1b65e36513834dd891bae21a74718f4f. API/UI ручного создания тренера, selector, имя/логин/disable/reset и локальный отзыв. Администратор меняет только аккаунт с единственным trainer-назначением своего филиала; любое дополнительное назначение/роль требует owner. Чужие назначения не раскрываются. Занятый логин не присоединяет аккаунт; owner привязывает существующего сотрудника, сохраняя administrator-роли. [Контракт](../trainers-v1.md).

0005 переиспользует staff/roles/sessions/clubs/audit_events: login/password change отзывает все сессии; role insert/delete блокирует staff_users для сериализации с проверкой одиночного аккаунта. Version/audit атомарны; actor/session/локальная administrator-роль проверяются повторно под блокировкой. Отзыв удаляет только trainer-роль, staff ID/история сохраняются. Будущие marked_by/trainer_id ссылаются на staff_users.id; V1-07/V1-08 используют auth.RequireTrainer внутри транзакции после прав actor и clubs.RequireActive; V1-09 дополнительно проверяет фактического тренера занятия. Учёт занятий здесь не добавляется.

PostgreSQL/race, права/изоляция/общий аккаунт/отзыв/сессии, дубли/эскалация, конкуренция версии/нового назначения, исторический FK, rollback/privacy, Chromium 151 320/390/1280 с create/login/reset/attach/revoke/restore/disable/reload/network failure/retry/двойным submit/устаревшей формой, Docker/Compose проверены. Live SHA/CI/завершение — в issue. Реальные устройства/HTTPS/VPS/PWA/приёмка остаются будущими задачами. После #152 и готовой #153 следующий шаг — V1-06/#154: группы и датированный состав; новый чат проверяет live доступность и занимает задачу через очередь.


## Группы и история состава V1-06 — 12.10.2026

По поручению #154: acquired=true для StarMadeGalaxy, [run 38251295831](https://github.com/NikishGum/JudeOSLite/actions/runs/38251295831), request 0205eaac593440b4b17faf57c70f490e. 0006 добавляет training_groups/group_memberships с составными FK по club, версиями и защитой дат/пересечений на participant/card lock. Состав использует **club_memberships.id**, не participantId. Карточка, начальная группа и аудит создаются атомарно. [Контракт/передача](../groups-v1.md).

Owner/admin API/UI: создание/переименование/архив групп, выбор карточки из реестра, включение/исключение/перевод с датой, возврат новым периодом, состав на выбранную дату и вся история. Интервалы [from,until), null открыт, перевод сохраняет прежний конец. Инженерное решение v1 — максимум одна группа карточке на дату; гостевых/параллельных включений нет. Начало не раньше joined_on; исключение/перевод строго внутри периода, пустой период/удаление исторического ID не допускаются. Архив сохраняет периоды; новая запись в архивный club/group/card запрещена, исключение/перевод из архивной группы разрешён. Исторический состав включает архивные карточки с подписью, имена показываются актуальные.

Actor/session/administrator-роль повторно блокируются внутри записи (auth.LockClubAdministrator также в реестре), затем active club → participant → card → целевая группа → интервал. Audit атомарен и без ФИО/рождения/контактов. В V1-07/V1-08 использовать groups.ReadGroup(active=true) после прав actor/clubs.RequireActive вместе с auth.RequireTrainer. Trainer-доступ выводится из правил/фактических занятий, отдельной group_trainers нет. Таблиц занятий/отметок ещё нет: V1-09 фиксирует snapshot один раз; последующие переводы/архивы его не пересобирают.

PostgreSQL 18.6/race: границы, bounded transfer/возврат/исключение, перекрытия/конкурентные add/transfer, изоляция/неверный participantId, права/CSRF, rollback initial create и второго аудита перевода, исторический составной FK, архив/restore, отзыв после middleware, disable/club archive/privacy; прежние suites и миграции прошли. Браузерная проверка выявила и исправила одинаковые React keys соседних предметных компонентов: старые DOM-блоки исчезают при смене филиала. Регрессия сохранена в frontend/e2e/groups_v1.py. Chromium 151/production Docker/Compose и CI/SHA — в отчёте issue. Реальные устройства/HTTPS/VPS/PWA/журналы и пользовательская приёмка остаются следующим задачам. Следующая техническая задача — V1-07/#155; новый чат проверяет live доступность и занимает её через очередь.


## Расписание V1-07 — 12.10.2026

По поручению #155: acquired=true для StarMadeGalaxy, [run 38259181649](https://github.com/NikishGum/JudeOSLite/actions/runs/38259181649), request 0acce92905794d67b0858fe2a85e86d4. Основа main 56e9f6d с готовой #154. 0007 добавляет schedule_rules/training_sessions; четыре отдельных режима API/UI и обязательный предпросмотр, датированные версии series_id/[from,until). Чётность числа месяца — прежнее инженерное допущение. [Контракт/передача](../schedule-v1.md).

Weekly/parity создают будущие даты текущего и следующего локального месяца; старт/каждый час в Go-процессе. Date_list — до 1000 уникальных дат сразу, в том числе дальних; once — одно занятие без rule. Версия сохраняет зону филиала, starts_on/абсолютное время; длительность elapsed minutes, неоднозначное DST отвергается. Генерация не пишет прошлое задним числом. Общий PG advisory transaction lock/уникальный живой слот защищают повтор/конкуренцию. Архив/отзыв/disable прекращают продление, сохраняют историю.

После actor/session/administrator внутри транзакции используются clubs.RequireActive, groups.ReadGroup(active=true), auth.RequireTrainer. Trainer GET ограничен своим trainer_id правил/занятий. Изменение последней версии повторяет предпросмотр под lock, stale ввод/состояние даёт 409; меняет только будущие обычные planned. Они сохраняются с superseded_at/ID, новая версия/планы отдельные; отмены/manual/open/closed/frozen/past остаются. Исключение или уже начавшийся planned старой серии резервирует исходную локальную дату даже при смене времени. Audit атомарен и без персонального/секретного payload.

V1-08 переиспользует существующие training_sessions и guards, блокирует session FOR UPDATE, проверяет version/status/superseded_at; отмена/override в самой записи. Сохранять исходный starts_on вхождения при переносе времени (или явно пересмотреть резервирование даты). V1-09 создаёт snapshot один раз и проверяет фактического trainer_id/текущую роль. Retired планы не являются отменой/проведённым занятием, исключаются из рабочего календаря и статистики.

Go/PostgreSQL/race: 28/29/30/31, month/year/local timezone/DST, several weekdays/far dates/once, preview=созданные даты, повтор/перезапуск/4 одновременных генератора, concurrent submit, isolation/CSRF/guards/actor revoke, датированные версии/исключения/исторические IDs/snapshot, stale preview, audit rollback и дальний one-off conflict. Browser регрессия сохранена в frontend/e2e/schedule_v1.py; production Docker/Compose/CI и точный SHA — в отчёте issue. Реальные устройства/HTTPS/VPS/PWA/приёмка остаются следующим задачам. Следующая техническая задача — V1-08/#156, новый чат проверяет live доступность и занимает через очередь.


## Календарь и исключения V1-08 — 12.10.2026

По поручению #156: acquired=true для StarMadeGalaxy, [run 38263996950](https://github.com/NikishGum/JudeOSLite/actions/runs/38263996950), request 07345d5e388c43f2aed1a4df44f3cd0b. Изолированная ветвь codex/v1-08-sessions от main 2d13c7f с готовой #155/CI. Переиспользованы training_sessions и календарь/lock V1-07, 0008 только guards истории. [Контракт/передача](../sessions-v1.md).

Owner/admin календарь по группе/периоду, once без rule, отмена planned/open с обязательной причиной и подтверждением open, фактическая замена planned/open. Снимок/marks/старые авторы сохраняются, absent не создаётся, запись cancelled закрыта; closed не отменяется/не переписывается. Перенос только будущего planned без frozen. Source starts_on/rule/timezone неизменны: календарь использует фактическую локальную дату starts_at, sourceDate резервирует исходное вхождение при regeneration/dated change. Retired планы исключены. Actor/session/admin и club/group/trainer guards, session FOR UPDATE/version/status/superseded_at и атомарный аудит без имён/контактов/свободного текста причины. Отказ/сбой сохраняет ввод, явный reset после конфликта; late responses не возвращают прежний club/user.

Trainer GET ограничен фактическим trainer_id; после refresh видны отмены/замены, прежний trainer теряет заменённый сеанс. V1-09 обязана проверять фактическое назначение/текущую роль на каждом journal запросе, создавать snapshot один раз на фактическую дату, писать только open/live и не вызывать absent при cancellation. Cancelled/retired исключать из статистики даже при сохранённых marks. Полный журнал здесь не реализован.

Go/PostgreSQL -race полный suite, migrations/ветвление истории/regeneration/concurrent commands/audit rollback/isolation/lifecycle/actor revoke; Chromium 151 create retry/double submit/stale/open warning/closed UI/snapshot authors/actual trainer refresh/move/club switch/reload и 320/390/1280 проверены. Browser regression — frontend/e2e/sessions_v1.py. Production Docker/Compose и финальные SHA/CI — в отчёте issue. Реальные устройства/HTTPS/VPS/PWA/приёмка остаются следующим задачам. Следующая техническая задача — V1-09/#157, занимает отдельный новый чат после live проверки.


## Просмотр истории V1-20 — 12.10.2026

По прямому поручению владельца создана #170, зарегистрирована с зависимостью от готовой #155 и занята StarMadeGalaxy: acquired=true, [run 38266273401](https://github.com/NikishGum/JudeOSLite/actions/runs/38266273401), request bad977442cae41908b9d51919a6c41f2. [Контракт](../audit-v1.md). Только чтение существующих audit_events, без схемы/миграций/writer-правок.

Owner видит сеть/архив/null club_id; admin — события своих текущих administrator-назначений, включая архив; trainer-роли не расширяют scope. Публичный envelope (автор/время/филиал/тип/ID/действие), raw states/reasons/контакты/пароли не выдаются. Имена актуальные, прежних снимков имён нет. Scope и фильтры серверные; actor/session/назначения блокируются внутри чтения. Cursor (created_at,id) привязан к фильтрам, 50 событий; даты включительно Europe/Minsk. UI отменяет старые запросы и очищает данные при смене scope/аккаунта, перечитывает live filters каждые 30 секунд/focus.

PostgreSQL/API/race: owner/admin/trainer, два филиала/multiple roles, privacy/no-store, archive/disabled author, tied timestamps + insert между страницами, invalid cursor/dates и live отзыв после middleware; прежние suites на объединённом V1-08 прошли. Chromium 151: owner/admin/trainer/archived-admin, фильтры/даты/pages/empty/error/retry/смена аккаунта и филиала, 320/390/1280 без overflow/JS errors. Регрессия frontend/e2e/audit_v1.py и disposable fixture сохранены. Итоговый SHA/CI/deploy/завершение смотреть в live #170.


## Журнал занятия V1-09 — 12.10.2026

По поручению #157: acquired=true для StarMadeGalaxy, [run 38270791266](https://github.com/NikishGum/JudeOSLite/actions/runs/38270791266), request 2136c3c8707e4c6da4bb61d40eff6625. Ветка codex/v1-09-journal от актуального main c122b2a с готовой #156/CI. 0009 добавляет attendance_records: immutable snapshot UUID/card/source interval, составные FK одного club/group/session, NULL/present/absent, причины unknown/illness/excused, marked_by/at/origin/version. [Контракт](../attendance-v1.md).

Текущий actor/live session/trainer-role и actual trainer_id проверяются на каждом запросе внутри транзакции. Lock actor/session/role → schedule → active club для записи → session FOR UPDATE; совместим с отменой/заменой. Snapshot создаётся один раз на фактическую локальную дату starts_at, включая архивные карточки с действующим периодом; последующие переводы/архивы не пересобирают его. Имена/архив показываются актуальные, без контактов/рождения. Инженерное правило открытия — с локального дня занятия; прошлый planned допустим. Owner/admin отдельный журнал — V1-11, здесь только actual trainer с текущей ролью.

Значимая mark меняет record/session версии и атомарный audit old/new state/reason/author/time/origin/version; identical retry в open не создаёт изменения. Close одной транзакцией заполняет NULL absent/unknown/close_default, пишет каждую историю и closed. Любая ошибка откатывает всё. Повтор open/close безопасен без пересборки/повторного аудита; closed mark запрещена. Cancelled сохраняет NULL/manual/authors без default-отсутствий; replacement не переписывает marked_by. Cancelled/retired исключать из статистики; closed пустой сеанс считается проведённым. V1-11 расширяет DB guard/check для correction новой миграцией, старые миграции не менять.

Trainer UI: свои занятия/дата/статусы, открытие, список отдельных mark forms, сводка/предупреждение/явное закрытие, reload и чтение closed. Network failure сохраняет ввод, нет ложного успеха; double submit/409/reset/смена scope обработаны. Последовательные мобильные карточки/панель причин ещё V1-10. Audit UI распознаёт journal_opened/journal_closed/marked и attendance_record.

Полный Go/PostgreSQL18.6 -race suite и migrations прошли: повторы/empty/boundaries/local date/moved occurrence/transfer/archive/причины/версии/isolation/SQL guards/rollback/privacy/replacement/authors/отзыв после middleware, 12 open+mark/close и 8 cancel/close гонок. Chromium154: формы→API→DB→UI/retry/double submit/stale/close/autodefaults/reload/cancel/replacement/logout; 320/390/1280 без overflow/JS errors, touch targets≥44. Регрессия frontend/e2e/attendance_v1.py; production Docker/Compose clean/rerun/restart/readiness и сохранение snapshot/marks/history после DB restart проверены. Итоговый SHA/CI/main/Done смотреть live issue. Реальные устройства/PWA/приёмка остаются будущими задачами. Следующая рекомендация V1-10/#158; новый чат проверяет live свободу и занимает через очередь.

## Мобильная посещаемость V1-10 — 12.10.2026

По поручению #158: acquired=true для StarMadeGalaxy, [run 38274610103](https://github.com/NikishGum/JudeOSLite/actions/runs/38274610103), request a4ef0b1ce80a44bca5a1be27f8296186. Ветка codex/v1-10-mobile-attendance от main 19f4258 с готовой #157/успешным CI. [Контракт](../attendance-mobile-v1.md). Переиспользованы Journal/API/versions V1-09, без backend/миграций/auth/Compose правок.

Последовательная карточка/две кнопки/прогресс, нижняя панель трёх причин, сводка с возвратом/исправлением до закрытия. ФИО/группа/порядок/архив и короткий стабильный код membershipId различают карточки без раскрытия контактов/рождения. Переход только после подтверждения сервера к следующему неотмеченному, потом сводка. После reload — серверный журнал/первый unmarked. Нет свайпов/офлайн-очереди. Завершение доступно при unmarked/empty, показывает число будущих absent/unknown; отмена ничего не закрывает.

Native dialog + явный Tab/Shift+Tab cycle, focus heading/return, Escape/cancel, safe area/прокрутка короткого экрана; основные цели ≥56 px. Ошибка сохраняет текущую карточку/причину, явный retry; lost response после DB commit не даёт ложного перехода, identical retry не удваивает аудит. 409 блокирует stale writes до явного reload; 401/404 очищают недоступное; поздние ответы после unmount игнорируются. Closed/cancelled — сводка без записи.

Регрессия frontend/e2e/attendance_v1.py адаптирована к совместному V1-09/V1-10 сценарию и disposable attendance158-db/browser158: real UI→API→PostgreSQL→UI, presence/все причины/touch/keyboard/cancel/double click/delay/lost response/retry/correction/reload/409/close/default absences/empty/cancelled/replacement/revoke/logout, 320/390/1280 и 320×480. TypeScript/Vite/production Docker, gofmt/vet и полный PostgreSQL18.6 -race suite прошли. Финальные SHA/CI/merge/Done — в live issue; реальные устройства/HTTPS/PWA/приёмка остаются следующим задачам. Следующая предметная задача — V1-11/#159 (зависит от #157); новый чат проверяет live свободу и занимает через очередь.

## Журнал администратора и коррекции V1-11 — 12.10.2026

По поручению #159: acquired=true для StarMadeGalaxy, [run 38274821296](https://github.com/NikishGum/JudeOSLite/actions/runs/38274821296), request d2ed64842a7b4fbda49f4f9929b9c1c1. Ветка codex/v1-11-corrections от main 19f4258 с готовой #157/CI; #158 доставлена другим чатом через PR #173 (main 06dd736); её мобильные карточки и документация объединены перед доставкой #159. [Контракт](../corrections-v1.md).

Owner в выбранном клубе/admin своих клубов читает journal по группе/локальной дате, frozen snapshot/отметки и историю записи. Closed correction только по одной, обязательная changeReason отдельно от absenceReason, без давности; архив карточки/группы/перевод допустимы, archived club только чтение. 0010 расширяет CHECK/guard новой миграцией, snapshot и closed session неизменны. Record version всегда проверяется, даже для identical stale; актуальное совпадающее значение no-op. Значимая правка state/reason/author/time/origin=correction/changeReason/version и old/new/change_reason audit=corrected одной транзакцией. Present очищает absenceReason; прошлые причины/авторы остаются. Тренер не правит closed, его чтение не выдаёт объяснение коррекции.

Lock actor/live session/admin-role → schedule → active club при записи → session FOR UPDATE. Live отзыв после middleware запрещает correction. History возвращает только явные поля для записи этого session/club, без контактов/рождения/секретов/raw payload. UI сохраняет ввод при сбое, блокирует double submit, 409 требует явного reload/reset, смена scope/401/404 очищают недоступные данные. V1-12 считает актуальные records закрытых, cancelled/retired исключает, closed empty учитывает.

Go/PostgreSQL18.6/race полный suite, отдельная 0009→0010 с closed snapshot, historical archive/transfer/all reasons/versions/isolation/no-op/privacy/CSRF, live revoke/session/disable после middleware, 8 correction races, audit rollback и SQL guards прошли. Chromium UI→API→DB→UI filters/history/reason/network retry/double submit/409/reload/owner/admin/trainer/revoke/logout, 320/390/1280 без overflow/JS errors и кнопки≥44. Регрессия frontend/e2e/corrections_v1.py. Production Docker/Compose clean startup/rerun/restart/readiness и отдельный restart БД с полными fingerprints records/sessions/audit прошли. Итоговые CI/SHA/main — в live отчёте issue. Реальные устройства/PWA/приёмка остаются следующим задачам.

## Статистика владельца V1-12 — 12.10.2026

По поручению #160: acquired=true для StarMadeGalaxy, [run 38278692771](https://github.com/NikishGum/JudeOSLite/actions/runs/38278692771), request 5859774a377f44e3a6b0811fa4735a29. От актуального main 77de4d0 с готовой #159/PR #174 и успешным CI; отдельная ветвь codex/v1-12-owner-statistics/worktree. [Контракт/контрольный пример](../reports-v1.md).

Owner-only API/UI сети: филиал/группа/включительный период по фактическому starts_at в сохранённой зоне сеанса (исторический UTC-offset учитывать), conducted closed отдельно от marks, planned/open отдельно, cancelled/retired исключены. Один SQL per-session + GROUPING SETS даёт согласованные уровни сеть/филиал/группа; текущие исправленные attendance_records, без join к текущему составу/audit. Closed empty считается; denominator=0 → null/«—». API без округления, UI до 0,1 процентного пункта; взвешенный общий процент, same-name groups различаются UUID/филиалом. Архив/перевод сохраняет исторические суммы; новых миграций/таблиц/кешей нет.

Actor/live session/is_owner повторно блокируются внутри чтения, admin/trainer/несколько ролей не расширяют доступ. Только агрегаты/названия, без персональных карточек/контактов/объяснений. UI abort/clear на смене фильтра/ошибке/аккаунте, retry сохраняет фильтры. Настоящая коррекция в том же окне вызывает attendance-changed и немедленно обновляет отчёт; другое окно — focus/30 секунд/явная кнопка. Начальный месяц Europe/Minsk; фильтры независимы от рабочего филиала.

Полный Go/PostgreSQL18.6 -race suite, gofmt/vet/build и TypeScript/Vite прошли; контроль сети 3 closed, 3 present/4 absent =42,9%, после correction 4/3=57,1%, reasons суммируются в absent. Исторические границы/пустые группы/без групп/перевод/архив/права/privacy/no-store/live revoke и конкурентная согласованность проверены. Chromium154 UI→API→PostgreSQL→UI: фильтры/период/null/настоящая UI-коррекция с мгновенным пересчётом/сбой и retry/поздний ответ/выход и роли, 320/390/1280 без overflow/JS errors, цели≥44 px. Регрессия frontend/e2e/reports_v1.py. Production Docker build прошёл; итоговые CI/SHA/main — в live отчёте issue. Реальные устройства/PWA/приёмка остаются следующим задачам.

## Адаптация и PWA V1-13 — 12.10.2026

По поручению #161: acquired=true для StarMadeGalaxy, [run 38281740152](https://github.com/NikishGum/JudeOSLite/actions/runs/38281740152), request 4a59d495ffb741ffa68f7d9859fb9b28. Ветка codex/v1-13-pwa от main 6b75a0a с готовыми #158/#159/#160 и успешными CI; другого активного чата по задаче нет. [Контракт/установка/проверка](../pwa-v1.md).

Manifest/PRIDE PNG 192/512 + maskable/Apple touch, standalone и safe area; подсказки iPhone/Safari, Android/Chrome и ПК. Build plugin генерирует хешированный публичный HTML shell и точный allowlist JS/CSS/fonts/logo/icons. Cache Storage только versioned static, API/персональные/навигационные ответы не сохраняются, offline queue/client DB нет. Offline shell показывает вход/недоступность и явный повтор; navigator.onLine дополняется фактическими ошибками API. Клиент не отправляет мутацию при явном offline и не сообщает ложный успех.

Выход/401/смена сотрудника очищают компоненты/CSRF/пароль/филиал; BroadcastChannel передаёт только changed и очищает соседние вкладки. Старые auth responses не возвращают прежний экран; pagehide очищает DOM перед back/forward snapshot, persisted pageshow перечитывает me. Worker update остаётся waiting до явной кнопки; общий API mutation counter до чтения ответа защищает сохранение. Reload только в инициировавшей вкладке, другие сохранения не обрываются. Несохранённый ввод не переживает явную перезагрузку — UI предупреждает сохранить его.

Chromium154 persistent profile: manifest/installability/icons, сигналы install/standalone, три роли UI→Go→PostgreSQL18.6→UI (admin group/trainer/participant/once, trainer mark/close, admin correction, owner current SQL totals), 320/390/1280 без overflow/JS errors и цели≥44; панель 320×480/focus/Escape. Static-only cache после личных действий/выхода, offline mark/no false save и offline reload/API navigation, две вкладки/logout/account swap/late response, expiry journal/pagehide/pageshow, два настоящих frontend builds/waiting/save guard/new cache/сохранение другой вкладки прошли. Регрессия frontend/e2e/pwa_v1.py включена в CI; только disposable pwa161-* Docker resources. Полный Go/PostgreSQL -race suite/gofmt/vet/build, TypeScript/Vite, production Docker, Compose clean/rerun/restart/readiness и 29 deploy/11 coordination tests прошли. Итоговые CI/SHA/main — в live issue. Физические iPhone/Android/Chrome/Edge и реальная установка — #164/V1-16; полная HTTPS/backup/предметный restore — #165/V1-17 с готовой #167. Следующий чат проверяет live свободу #165 и не создаёт второй deploy pipeline.

## Эксплуатация V1-17 — 13.10.2026, серверная приёмка завершена

Исходный acquired=true для StarMadeGalaxy сохранён: [run 38287158290](https://github.com/NikishGum/JudeOSLite/actions/runs/38287158290), request 5938abafee5749c8be895ffb20275f5f. По прямому поручению продолжение в том же чате без нового claim, один исполнитель/In progress, #161/#167 closed и результаты/CI в main проверены. PR #177 с полным backup/restore/restart доставлен в main 39ce7b7; [актуальные операции](../operations-v1.md).

[Итог владельца 13.10.2026](https://github.com/NikishGum/JudeOSLite/issues/165#issuecomment-6140917093): reviewed root-утилиты из 1b480de установлены под deploy flock, hashes/preflight PASS. HTTPS UI illness → closed correction present, история и 1 closed/1 present/100% (50% были только примером) подтверждены. Backup 20261013T073342-d469e4ce реально восстановлен в restorecheck_10f99ca0b181 с точным сравнением каждой строки/uploads, непустыми schedule_rules=1/training_sessions=7/attendance_records=1/audit_events=21 и 10 migrations. Restart-check сохранил DB/files/HTTPS/SHA; проверочная DB удалена. Внешний 5432 probe дал TCP timeout из сети владельца. Firefox: secure context/active controlling SW, 11 static-only /assets entries без API/query, обе cookie Secure/HttpOnly/Strict/Path=/HostOnly. Физические iPhone/Android/Chrome/Edge — #164.

Владелец утвердил local-only 03:00 Europe/Minsk, семь последних завершённых плюс referenced current/previous/operation. Прежнее обязательное offsite условие заменено; S3/SFTP/restic не нужны для закрытия. Код service переведён на admin.py backup-local с maintenance_backup/общим lock, успешной HTTPS readiness перед cleanup; новая retention сохраняет unknown/incomplete/corrupt/extra files/links, проверяет metadata/checksums, распознаёт legacy. Таймер без jitter, Persistent=true/AccuracySec=1s. Offsite-код/пример и зависимость restic в CI удалены; прежние server secret files не изменяются. Продолжение от main 1b480de, ветка codex/v1-17-local-backup; коммит a55ffd0/PR #180 доставлен в main 195e539. Уточнение проверки activating workers — main 3a8d56a. 43 policy/recovery tests, real disposable regular backup/retention/restore, systemd syntax/calendar прошли; CI 38368620120 и Deploy test 38369260362 успешны. Новый claim не выполнялся.

[Финальная приёмка service/timer владельцем](https://github.com/NikishGum/JudeOSLite/issues/165#issuecomment-6142229067): на VPS checkout 3a8d56a, четыре source hashes и installed admin.py/operations.py OK, verify/preflight PASS. Настоящий service Result=success/ExecMainStatus=0, штатное inactive/dead; local-backup.phase=ready, новая копия 20261013T090815-6f48fc14, completed_at=13.10.2026 09:08:24 UTC. Retention keep_last=7/completed=7/removed=12/preserved_unknown=0; referenced previous 20261012T191202-a8bc16dd и current 20261013T082808-099bf4c2 сохранены. Timer enabled/active, 03:00 Europe/Minsk, Persistent=yes/Accuracy=1s/jitter=0, следующий запуск 14.10.2026 00:00 UTC (03:00 Minsk). Первый календарный запуск ещё не наступил: подтверждены ручной запуск service и активное расписание. Все согласованные серверные критерии #165 приняты; физические устройства и браузерная матрица — следующий V1-16/#164. Новый чат проверяет live закрытие #161/#165, свободу #164 и занимает её через очередь; рекомендация не резервирует issue.

## Проверка доступа V1-14 — 12.10.2026

По поручению #162: NikishGum, acquired=true, [run 38289087021](https://github.com/NikishGum/JudeOSLite/actions/runs/38289087021), request 0f5cbf6b51474def98fccefc6b64c4b3; отдельная ветка codex/v1-14-access от main f8a95ea с готовыми #158/#159/#160 и успешным CI. [Матрица/команды/границы](../access-testing-v1.md).

Восемь интеграционных тестов backend/tests/integration/access: реальные PostgreSQL18.6/миграции/HTTP handlers/сессии/CSRF, два филиала и 11 профилей (несколько назначений и смешанные роли). Чтение/корректные запрещённые команды, чужие ID/поиск/hasMore/пагинация аудита, privacy/no-store, общий аккаунт/идентичность, actual trainer/replacement/closed journal, live revoke/disable/re-enable/logout проверены. Fingerprint 12 таблиц до/после отказов исключает частичные записи и аудит. Разрешённые административные команды также проходят. Тренерский endpoint требует живой trainer и фактического назначения даже для owner/admin; административный журнал доступен по своим филиалам. Нарушений в покрытых сценариях не обнаружено, production handlers/миграции не менялись. Пакет автоматически исполняется существующим CI go test -race ./... с TEST_DATABASE_URL. Полный Go -race/vet/build и frontend build прошли; итоговые SHA/CI/main — в live issue. Гонки/контроль отчёта остаются #163, реальные устройства/HTTPS/PWA — #164/#165.

## Расписание, история и гонки V1-15 — 12.10.2026

По поручению #163: NikishGum, acquired=true, [run 38292554241](https://github.com/NikishGum/JudeOSLite/actions/runs/38292554241), request 57e987ab8b9648e09172869855162e06; ветка codex/v1-15-training от main 7d2b343. #159/#160 closed, реализации d02997e/9c6ec7d и успешный CI проверены в main; другого чата по #163 не обнаружено. [Матрица/команды/границы](../testing-training-v1.md).

Десять интеграционных тестов backend/tests/integration/training: реальные HTTP handlers/сессии/CSRF/PostgreSQL18.6/миграции, синтетические отдельные БД. Все четыре режима, вручную перечисленные границы/чётность/год/2040, первый и повторный запуск четырёх генераторов, без backfill, смена версии/реальные open/closed снимки/отмена/замена/перевод/архив, шесть FK 23503. По шесть open/open, mark/close, close/close, cancel/close, replacement/mark, corrections одной/разных записей; строки/версии/авторы/defaults/аудит и повтор закрытия. Полный rollback при промежуточном/финальном отказе аудита и второго INSERT генерации подтверждён fingerprint. Независимый ledger сети: 4 closed, 4 present/4 absent=50%; correction даёт 5/3=62,5%, empty/cancelled/retired/planned/open/период/историческая зона/перевод/архив и восемь report/correction гонок сохраняют согласованные уровни. Production handlers/миграции не менялись: воспроизводимых дефектов в покрытых сценариях не обнаружено.

Пакет автоматически включён существующим CI go test -race ./... с TEST_DATABASE_URL; без URL тесты skip и не подтверждают готовность. Полный Go -race/vet/build и frontend build проверены; итоговые SHA/CI/main — в live issue. Ограниченные гонки не заменяют нагрузочное испытание. На момент завершения #163 серверная приёмка #165 блокировала #164/#166; её последующий результат см. в разделе V1-17 выше. Реальные устройства/PWA и итоговая приёмка остаются #164/#166.

## Браузеры и мобильная PWA V1-16 — 13.10.2026, согласованные устройства проверены

По поручению #164: acquired=true для StarMadeGalaxy, [run 38376322238](https://github.com/NikishGum/JudeOSLite/actions/runs/38376322238), request 23d3281934d84743998f0efc2641ba6b. Ветка codex/v1-16-browser-pwa от main a1b9e33; #161/#165 CLOSED/Done, af40c56/a55ffd0 и успешные CI 38285072512/38374642603/Deploy test 38375311612 проверены в main; другого активного чата по задаче нет. [Сценарий и протокол](../testing-v1.md).

Расширена существующая disposable frontend/e2e/pwa_v1.py: admin через UI создаёт пять участников/группу/trainer и четыре режима; независимо перечисленные даты/preview/API, double submit. Trainer карточки/три причины/default unknown, offline/до commit/lost response после настоящего commit/identical retry без второго audit/reload, closed/cancelled/live role revoke. Admin correction C illness→present, независимый итог SQL/UI 1 closed/2 present/3 absent/unknown=2/illness=0/excused=1/40%. Worker/cache/logout/expiry/две сборки/update guard сохранены; добавлен WebKit и честные границы fault injection. Chromium154/WebKit26.5, production frontend/Docker и публичный read-only HTTPS/SW/static-only cache проверены; финальные повтор/CI/SHA/main — live issue. Продуктовые API/миграции/UI не менялись, подтверждённых блокеров в покрытых сценариях не выявлено.

Первая часть доставлена через PR #181/main 88edc2e; CI main 38380209243 и Deploy test 38381097912 success, HTTPS SHA совпал. По просьбе владельца локальный агент выполнял вспомогательное тестирование без claim/поручения/коммитов; облачный исполнитель сохраняет исходное назначение. Получен LOCAL-QA-164 REPORT 13.10.2026 15:13 Europe/Minsk. MacBook Pro Mac17,9 arm64/macOS27.0.1 (26A434), Chrome154.0.8037.98 и Edge155.0.4283.45: реальные HTTPS формы/карточки/40%, штатные .app/standalone/relaunch/session/logout PASS. iPhone13/iOS26.6.2 (UA Safari Version/26.6.1): Home Screen установка/relaunch/session/logout PASS по наблюдениям владельца; ordinary Safari USB Inspector подтвердил формы/кеш/390×739, panel/safe area/keyboard/landscape и offline-before-save подтверждены владельцем. После network restore/reload A unmarked; точный текст ошибки и transient success не записаны. Console установленной iPhone PWA неуправляема, её численные standalone/focus/cache не измерены. Android недоступен.

Mac исходные E2E на 88edc2e FAIL: D1 требует один cache при waiting B (Chromium151/Chrome154/Edge155); D2 select24px, D3 Tab на BODY в Mac WebKit26.5. Диагностические обходы не считаются PASS исходных тестов. Продолжение в том же облачном чате: StarMadeGalaxy/In progress/original run проверены, локальный проверщик idle, ветка codex/v1-16-browser-fixes от 88edc2e. D1 regression удерживает A отдельной вкладкой, проверяет A/B static-only до explicit activation и единственный B после. D2 appearance:none/CSS arrow сохраняют native HTML select и цель≥44. D3 явно выполняет каждый Tab/Shift+Tab в ActionSheet; точный порядок проверяется E2E. Backend/миграции не меняются. После Linux/CI/main требуется Mac повтор без обходов и краткий retest select/panel/relaunch на физическом iPhone: предыдущие physical PASS относятся к 88edc2e.

Пароли/личные данные/браузерные профили и raw-транскрипты не сохранять. Отдельные QA164 данные на HTTPS оставлены для повторения, trainer-role восстановлена owner UI. Физический Android BLOCKED, HTTPS разрыв во время save NOT RUN, дополнительные iPhone проверки перечислены в тематическом протоколе. #164 не закрывать по эмуляции; продолжение в этом чате без нового claim. #166 пока заблокирована #164; других свободных готовых задач нет. Итоговые CI/SHA/main и новые ручные результаты — live issue/[протокол](../testing-v1.md).

D1–D3 доставлены через PR #182 (ffcf7ae/2017cc4, main a358b87), push/PR CI 38398590151/38398597996 success. Первый main CI 38400282537 остановился в WebKit на disabled initial-group select; затем один локальный повтор остановился на schedule form, следующий полный WebKit прошёл. Причина этих остановок не доказана, не объявлять их исправленными по одному успешному повтору. Добавлена диагностическая печать loading/error/busy/HTTP failures при таком FAIL; проверки не форсируются и не получают скрытых retries. Final CI/deploy/SHA и нужный Mac/physical повтор — live #164. Первоначальный FAIL сохранять в отчёте.

Последующий main 929483d прошёл полный CI 38403460788 и Deploy test 38404823284; независимый HTTPS readyz=ok/release SHA совпал. **Новое решение заказчика 13.10.2026 в основном чате #164:** Android/Chrome, его физическая установка/relaunch и матрица отложены («не будем его пока проверять, чтобы таска не была заблокирована»); возможные ошибки разбирать отдельно. Прежний Android BLOCKED заменён на DEFERRED / NOT RUN и больше не препятствует закрытию #164. Не объявлять Android проверенным. Остались повтор неизменённых E2E на Mac Chromium/Chrome/Edge/WebKit и select/panel/relaunch на реальном iPhone. Вспомогательный локальный чат возвращает LOCAL-QA-164 RETEST REPORT через владельца, без claim/правок исходников/коммитов/закрытия. Основной облачный исполнитель сохраняет исходное назначение и завершает #164 после принятого Mac/iPhone повтора; #166 до этого остаётся зависимой задачей.

Принят LOCAL-QA-164 RETEST REPORT владельца 13.10.2026 18:21:51 Europe/Minsk: чистый f532e6b, production build и четыре исходных E2E по одному запуску, SHA256 скрипта 929ed7df…92a неизменен. Mac Chromium151.0.7922.34/Chrome154.0.8037.98/Edge155.0.4283.45/WebKit26.5 exit0, D1/D2/D3 и ledger40% PASS. Реальные обновлённые Chrome/Edge .app/select/формы/date/contact/panel/focus/browser-context offline/native quit/relaunch/session/logout PASS. iPhone13/iOS26.6.2 native popup/portrait формы/date/contact/save/readback/полный admin quit/relaunch/logout через Mirroring; panel/landscape/trainer relaunch/физический offline по владельцу плюс API. Actual installed-PWA Inspector теперь работает: standalone/390×797/select48.39/один кеш11assets подтверждены. Прежнее console-ограничение снято; экранная клавиатура/поворот обновлённой формы и дополнительные проверки NOT RUN, старые PASS не подменяют их. Android DEFERRED / NOT RUN. Main f532e6b CI38406879104/Deploy38407933373 success, HTTPS SHA совпал. Полный HTTPS40% на88edc2e сохранён как история; его journal не переписывался. Прежние form-loading FAIL не воспроизвелись, причина неизвестна, не объявлять её исправленной.

По новой UX-находке длинной страницы добавлены ссылки доступных разделов и focus/scroll выбранного trainer-журнала: кнопка подтверждения видна, server planned/records0 до явного подтверждения. Регрессия создаёт 12 cancelled disposable SQL fixtures и проверяет 320×480/ссылки/цели≥44. Только навигация; API/права/миграции не меняются. Последняя поправка проверяется Linux Chromium/WebKit/CI, ручная матрица выше привязана к f532e6b. Исходный claim основного чата сохраняется. Итоговые commit/main/CI/deploy/закрытие #164 — live issue; после фактического завершения следующая #166, зависимости #162/#163/#165 уже закрыты. Raw отчёты/профили/секреты не хранить в Git.

Main навигации 3621a0c/CI38425435001: Chromium PASS, WebKit FAIL на синтетическом persisted pageshow второй вкладки (5 s, DOM «Проверяем доступ…»), не на select/новом scroll. Fixture уточнён: restored page явно foreground/visible, timeout и privacy/fresh-me ожидания сохранены; при FAIL timed API/visibility диагностика без секретов. Первоначальную задержку не считать доказанно исправленной без причины; сохранить FAIL и последующие результаты. Завершение #164 после нового успешного main CI/Deploy.

## Приёмка и ручной запуск V1-18 — подготовка 13.10.2026

По поручению заказчика #166 начата от main 32c9ed48161a35888ea73c72555740c50060678c.
Все прямые #162/#163/#164/#165 CLOSED; результаты 9e27ae2/8db3da5/32c9ed4/a1b9e33
входят в main, CI38427133370/Deploy38427990038 success. #164 фактически завершена:
согласованный Mac/iPhone повтор принят, последняя навигация прошла Chromium/WebKit/
CI; Android DEFERRED / NOT RUN, пользовательскую понятность переходов проверяет #166.
Это уточняет предыдущий датированный блокер #164, не стирая историю FAIL/ограничений.

#166 была свободна, другой работающий чат не обнаружен. Автоматический claim:
StarMadeGalaxy, acquired=true, request 025adf279792451bac2ef7274dac747b,
[run38429419630](https://github.com/NikishGum/JudeOSLite/actions/runs/38429419630),
один исполнитель/In progress. Продолжать в этом чате по сохранённому run без нового
claim. Независимый публичный HTTPS readyz=ok/release32c9ed4 подтверждён с проверкой TLS.

Подготовлены [протокол](../acceptance-v1.md), независимый расчёт по реальным журналам
и [памятка ролям](../staff-guide-v1.md), связанные из операций/указателя. Заказчик
выбрал сразу несколько филиалов и поручил придумать обозначения/роли: выбраны
F1/F2, A1/T1 и A2/T2 раздельно, O1 принимает сеть, G1/G2/G3, малые контрольные
наборы 6+4. Это выбранный сценарий, не реальные подтверждённые залы/данные.
По следующему прямому поручению заказчик предоставил доступ для самостоятельного
HTTPS-прохода. Исполнитель через UI создал выбранный новый контроль: F1/F2,
A1/A2/T1/T2 и R1, G1/G2/G3, 6+4 синтетические карточки; SQL/QA164 не использованы.
Cloud Chromium154.0.8037.57/main32c9ed4: четыре режима (включая odd/even границу),
три роли обоих филиалов, ссылки/13-item список320×480/confirm planned-records0,
три причины/reload/offline-before-send/close defaults/correction/две истории,
перевод6→5+1/архив-возврат/immutable snapshot6, replacement/denied old save/open
cancel с пятью NULL и ещё11 planned cancellations, чужие404/admin-report403 PASS.
Ledger F1 3/3=50%→4/2=66,7%, F2 3/1=75%; сумма контроля60%→70%, отмены не влияют,
нулевой знаменатель«—». Для шести ролей cache11 static-only/logout/reload login PASS,
использованные сессии завершены; контроль 13.10.2026 19:30–19:47 Europe/Minsk.
Пароль/сессии/полные приватные сценарии в Git не сохранены.
Это фактический контроль исполнителя по согласованному набору, не физический
повтор устройств и не пользовательский signoff. Реальные рабочие соответствия/
данные, понимание сотрудниками, дата ручного запуска и итог заказчик ещё не
подтвердил. Контроль оставлен в CRM, QA164 не изменён. Реальные
сведения и пароли вводить только в CRM; в Git/чат — обозначения/агрегаты. QA164/CI
не являются пользовательской приёмкой. Сценарии, искажающие реальную посещаемость,
только на отдельно согласованном контрольном наборе; архив не убирает его историю
из сетевого отчёта. #166 не закрывать до фактической приёмки/согласованного запуска.

Документация V1-18 доставлена в main d9063fc (без force, SHA/чистое дерево проверены).
Branch CI38433629156 success, main CI38433900712 FAIL на WebKit synthetic restored
page, Deploy38434694993 skipped; стенд сохранил32c9ed4. Новая диагностика показывает
visible/pending новый me/отменённые клубные чтения; причина задержки не доказана.
Реальный HTTPS-контроль выше успешен, от CI-fixtures отделён. Уточняется fixture:
networkidle перед synthetic persisted контрактом обоих движков, прежний5s timeout/
privacy/диагностика сохранены; добавлен настоящий уход на readyz/back с
fresh me200/admin/no-owner. Native BFCache не измеряется; очистка DOM остаётся
в отдельном synthetic persisted контракте обоих движков.
Backend/продуктовые TSX/права/схема не меняются. Локальная WebKit26.5 новая регрессия
PASS с приватными runtime-библиотеками (system package validation отключена из-за
non-root Cloud, сами UI/API/DB assertions нет); обязательный стандартный CI без этого
флага и итоговые Chromium/доставка/Deploy ещё проверяются, результат в live #166.

Локальный окончательный WebKit сначала FAIL на отсутствующей форме выбора группы
расписания (30s), затем fixture стал ждать GET schedule200 после refresh/save до
следующего editor; диагностика включает select/fill, прежние таймауты/один POST
при двойном submit сохранены. Последующий полный WebKit26.5 PASS, counts2/4/2/1,
40%; источник первого form FAIL не объявлять доказанно устранённым. Native back
проверяет fresh me/роль, BFCache не измеряет. Итоговый стандартный CI/Deploy — live#166.
Окончательные локальные Chromium154.0.8037.57 и WebKit26.5 полные регрессии PASS,
включая оба persisted/native контракта,2/4/2/1 и40%; отдельны от HTTPS F1/F2 и
пользовательского signoff. Проверки Python/Markdown links/diff и frontend build PASS.

## V1-18 — решение заказчика о вымышленном объёме, 13.10.2026

Последний ответ на вопрос о готовности реальных филиалов/данных и ручном запуске:
«Нет, просто придумай вымышленное, и всё». Он снимает описанное выше ожидание
реальных рабочих данных/запуска для завершения #166. По предыдущим поручениям
выбраны несколько филиалов и самостоятельный проход исполнителя; поэтому итог
#166 — фактически выполненный новый HTTPS-контроль F1/F2 с вымышленными карточками,
независимым расчётом и короткой памяткой. План/JSON/live issue согласованы с этим
решением; зависимости/очередь не изменяются. Это решение заказчика об объёме,
а не сведения о личном просмотре интерфейса или подтверждение обучения сотрудников.

Предыдущий результат fe6488817e3fb0677aa5b514bb7e7a2507af90ac в main:
CI38439989277 (ветвь), CI38439997309 (main), Deploy38441055587 success;
независимые HTTPS readyz=ok/release=fe64888 с проверкой TLS. Стандартный CI без
локального skip package-check: Go vet/race/PG/миграции, frontend, Chromium/WebKit,
43 operation tests, Compose/restart, disposable HTTPS/backup/точный restore.
Исходные FAIL38433900712 и локальная отсутствующая schedule-форма сохранены;
причина продуктового сбоя не объявляется доказанно исправленной, BFCache не измерен.

Реальные залы/сотрудники/персональный реестр, получение/понимание памятки, отдельная
пользовательская оценка переходов и ручной рабочий запуск DEFERRED / NOT RUN.
Дата/ответственный запуска не придуманы. Android DEFERRED / NOT RUN, не блокер.
QA164/CI не считаются пользовательской приёмкой. Контрольные данные остаются в CRM
и исторических суммах; при будущем реальном запуске фильтровать рабочие филиалы.
Закрытие #166 допускается после доставки согласованного результата/проверок в main;
это не объявление начала работы реального клуба. Итоговый SHA/CI/deploy/статус —
в live #166. Следующий этап по отдельному поручению: реальные назначения/карточки
через защищённую CRM, обучение/проверка понимания, дата/ответственный запуска и
протокол рабочей приёмки. Других открытых готовых issues v1 при решении нет;
новое issue/назначение здесь не создавалось.


## Оболочка PRIDE RD-01 — 14.10.2026

По прямому поручению #183, NikishGum/acquired=true, [run38535824077](https://github.com/NikishGum/JudeOSLite/actions/runs/38535824077), request ce84f36e4c1c4351b700ded7bff80b26. Изолированный codex/rd-01-shell от main b80412f, готовый #166/CI/отсутствие другого чата проверены. [Контракт навигации](../redesign-navigation.md), [протокол](../redesign-shell-validation.md). React Working Panel: desktop sidebar, верхний club/user, mobile три раздела+«Ещё», адаптивный общий Dialog. Все v1-компоненты доступны в отдельных разделах; multi-role union без preview role switch, главная пока только реальный контекст. URL /clubs/:id/:section и participant ?detail=membershipId; legacy club URL, history/reload/direct link. Неподходящий scope объясняется без предметных данных. Серверные права/CSRF/сессии/очистка DOM/pagehide/pageshow и static-only PWA сохранены, SW узнаёт новые публичные маршруты. Черновики/beforeunload/диалог перехода и pending-save guard; отправленные команды не отменяются. OwnerManagement разделён только mode=clubs/staff, без потери назначений/операций. Прототип неизменён. RD-02–RD-06 не реализованы; общие app/shared/router/токены согласовать до дальнейших изменений. Фактические результаты/CI/main/завершение смотреть в live #183, не по датированному статусу памяти.

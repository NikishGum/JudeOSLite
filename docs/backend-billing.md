# RD-10 / #202 — счета, ручные оплаты и абонементы

19.10.2026, Europe/Minsk. [Issue](https://github.com/NikishGum/JudeOSLite/issues/202),
[HTTP/JSON-контракт](backend-contracts/README.md). Прямое поручение заказчика
разрешает ручные финансы на этом этапе; прежнее исключение финансов из v1
не отменяет эту задачу. iPay не подключается.

Claim: NikishGum, request `722c3b23f6e64ae9b564923c9103ae40`,
[run39257237627](https://github.com/NikishGum/JudeOSLite/actions/runs/39257237627),
acquired=true/единственный исполнитель/In progress. Основа main `30577ff`,
отдельные `codex/rd-10-billing` и `/workspace/rd-10-billing`.
#201 закрыта, PR205/main `a1069fa8de683910a46af451b3a694fd8ed65060`,
Build39251553406/Coordination39251553370/Deploy39252781627 success проверены.
Область server wiring/CI опубликована в issue до изменения. Frontend #200,
auth helpers #201, documents0013/#203 и notifications0014/#204 не меняются.

## Поведение

`0012_billing.sql` создаёт billing_plans, payments и subscriptions. Денежные
значения — bigint копейки BYN, вход1–999999999, без float/кошелька/частичного
распределения/импорта долгов. Ручные методы: cash/card/transfer/erip.
Запись erip означает ручную фиксацию сотрудником, не подтверждение шлюзом.

Реализованы все URL контракта: GET/POST/PUT тарифов, GET/POST платежей,
POST confirm/void, GET subscriptions и billing/summary; родительские GET
portal/payments и portal/subscriptions. Все поля контрактных объектов присутствуют,
nullable поля сериализуются как null, пустые коллекции как [], а не null.
JSON ограничен16KiB; неизвестные поля/лишний JSON/неверный Content-Type,
неполные nullable поля PaymentInput, дробные деньги, неправильные даты/UUID,
несогласованные method+paidOn и planId+validFrom отклоняются400.

Создание тарифа требует version0, изменение —текущую version и archived.
Счёт с обоими method/paidOn=null имеет pending; ручная оплата —paid.
План должен быть действующим в этом филиале, сумма равна его текущей цене;
несовпадение/архивный тариф409. Без тарифа сотрудник явно задаёт полную сумму,
в том числе для неполного месяца. Нет автоматического начисления старых долгов.

requestId —глобально уникальный UUID. Transaction advisory lock по ключу
сериализует одновременные повторы; JSONB хранит исходное типизированное тело.
Одинаковое тело возвращает текущую запись201 (также после confirm/void), другое
тело или тот же ключ другого филиала409. Повтор не создаёт второй платёж,
абонемент или аудит. Проверка уже созданного ключа предшествует чтению текущего
тарифа: последующее изменение/архив тарифа не ломает потерянный ответ создания.

Confirm блокирует платёж, сравнивает version, переводит pending→paid с увеличением
версии. Устаревшая версия409; paid с текущей версией и теми же method/paidOn —
успешный no-op без нового аудита; менять уже подтверждённые факты повторным confirm
нельзя409. Void требует текущую версию и непустую причину1–500 символов,
увеличивает версию и отменяет абонемент в той же транзакции. После void confirm
и повторный void409; запись/полученная сумма сохраняются. Это аннулирование
учётной записи, не возврат денег плательщику.

Абонемент сохраняет название/visits/days тарифа. validUntil=validFrom+days,
интервал [validFrom,validUntil), смежные интервалы разрешены. Все неаннулированные
интервалы, включая pending, резервируют даты. PostgreSQL btree_gist exclusion
не допускает пересечения одного membership даже при прямой/конкурентной SQL-записи.
Составной FK не позволяет связать абонемент с платежом другого ребёнка/филиала.
Void освобождает интервал; последующая запись не меняет прежнюю историю.

used вычисляется по present закрытых журналов на фактическую локальную дату
занятия в сохранённом timezone. Open/planned/cancelled/superseded не расходуют
занятия. Коррекции видны при следующем чтении; used может превышать visits.
Приоритет статуса: cancelled → pending неоплаченного счёта → expired при наступлении
исключающей даты конца → exhausted при used>=visits → pending до начала → active.
День сравнивается с текущей датой филиала. Неоплаченный истёкший счёт остаётся
pending до явного confirm/void, без автоматического списания/аннулирования.

Фильтры платежей: q (имя/описание), status (paid/pending/void; пустой/all=все),
from/until включительно, membershipId. Дата paid/void ранее оплаченного —paidOn,
неоплаченного —createdAt в timezone филиала. Summary использует те же правила дат,
исключает void из сумм/количеств; byMonth содержит только месяцы с paid, YYYY-MM
по paidOn, по возрастанию. Пустая summary —нулевые суммы/счётчики и byMonth[].
Тарифы сортируются name/id, платежи createdAt DESC/id, абонементы по созданию
платежа DESC/id. page0–1000000/limit1–100; total и hasMore вычисляются сервером.
Клубные/member индексы и UUID-предикаты сохраняют индексируемую границу выборки.

## Права и транзакции

Используется один существующий auth, cookie/Origin/CSRF. Каждая операция повторно
проверяет живую сессию/учётную запись; staff маршруты —owner/admin своего филиала.
Trainer и родитель без staff-прав не читают журнал/summary/тарифы и не пишут финансы.
Скрытый UUID/чужой филиал404; анонимный401; CSRF403. Архив запрещает записи409,
owner/admin сохраняют чтение истории; родителю архивный филиал не виден.

Portal всегда ограничен явными family_access/family_children, даже для parent+staff.
Связи/филиал удерживаются FOR SHARE; для каждой карточки вызывается существующий
families.LockMemberReader в той же транзакции. Unlink/revoke/архив/отключение
аккаунта/отзыв staff-назначения закрывают будущие чтения. Ответ не содержит
контакты семьи, чужие membership или данные аутентификации. Все финансовые
ответы/ошибки, включая неподдержанный метод на финансовом URL, no-store.

Чтения используют REPEATABLE READ: total/items, summary/byMonth и посещения
относятся к одному снимку. Живые блокировки отзыва прав сохраняются; конфликт
снимка возвращает409 и требует перечитать данные. Мутации READ COMMITTED,
с блокировками actor/session/role/club, request key/member/plan либо payment;
версия, предметные строки и аудит коммитятся вместе. Аудит содержит IDs,
version/status/amount/method/paidOn/voidReason, без имени ребёнка и семейных контактов.
Действия payment_created/payment_paid/payment_voided и plan_created/plan_updated
готовы для потребителя #204. Его таблицы/уведомления здесь не создаются.

## Проверки

Воспроизводимые команды (с настоящим TEST_DATABASE_URL):

```sh
(cd backend && go vet ./... && go test -race ./... && go build ./cmd/server ./cmd/migrate ./cmd/create-owner)
npm ci --prefix frontend && npm run build --prefix frontend
docker build --secret id=proxy_ca,src=/etc/ssl/certs/ca-certificates.crt -f deploy/Dockerfile -t judeoslite:ci .
python3 backend/tests/browser/billing.py
PWA_BROWSER=webkit python3 backend/tests/browser/billing.py
python3 frontend/e2e/pwa_v1.py
PWA_BROWSER=webkit python3 frontend/e2e/pwa_v1.py
python3 -m unittest discover -s deploy/test -p 'test_*.py'
```

Новые Go/PG сценарии: точные минимальная/максимальная суммы и aggregate>1млрд,
конкурентные8 повторы UUID/другое тело/аудит1, confirm race200+409/повтор/no-op,
void/reason/version, rollback при отказе аудита, снимок тарифа/архив/цена,
смежные/конкурирующие интервалы/освобождение при void, calendar-статусы,
закрытые present на обеих границах, used>visits и настоящие HTTP-коррекции,
q/status/date/member/page/mandatory fields, семья A/B в одном/разных филиалах,
parent+trainer/чужой UUID, unlink/revoke/disabled/admin revoke, архив/CSRF/no-store.
Upgrade0011→0012/rerun сохраняет прежние staff sessions/roles/family_access и
архивную карточку; составные FK проверяются на настоящем PostgreSQL.

Браузерный billing.py запускает production image и отдельную PostgreSQL18.6.
Owner входит через UI; существующие формы создают тариф/счёт, подтверждают erip
и аннулируют с причиной. Ответы сверяются с SQL. Настоящие browser fetch проверяют
конкурентную идемпотентность, snapshot, посещения/коррекцию через journal API,
родителя A/B/чужой филиал, unlink/revoke, trainer/disabled, CSRF/no-store и rerun.
Ни route fulfill, ни мок-сервер в приёмке backend не используются. Сценарий
добавлен в стандартный CI Chromium/WebKit перед прежними регрессиями.

Фактические результаты окончательного прогона и ссылки доставки фиксируются
ниже и в live issue. Физические устройства и приёмка сотрудников #188 не входят
в backend-приёмку и не объявляются выполненными.

### Фактический локальный результат

- Go1.27.2: vet/build, полный `go test -race ./...` с PostgreSQL18.6 —PASS,
  без пропуска PG. Окончательные billing/миграционные дополнения проверены отдельно.
- Production frontend typecheck/build и Docker image —PASS.
- billing.py: Chromium154.0.8037.57 и настоящий WebKit26.5 —PASS; UI/Go/PG,
  восемь concurrent retry, SQL/audit, коррекции и граница семей сохранены.
- Deployment unit43 и coordination14 —PASS.
- Прежние UI/PWA, training/reports/dashboard, новый frontend contract harness,
  Compose/TLS/deploy/backup restore входят в обязательный полный CI; точные
  результаты текущего SHA/доставки публикуются в live issue.

Non-root Cloud: WebKit использует недостающие Debian13 библиотеки в приватном
browser cache. PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1 отключает только
проверку системного расположения пакетов; реальный движок и assertions сохраняются.
CI устанавливает штатные --with-deps и не использует этот флаг.

Исходные локальные ошибки нового harness: неправильный URL correction, затем
exact label поиска select (текст options входит в label) и запуск предыдущего
образа без status=all. Исправлены URL/локатор и пересобран образ; assertions не
убирались. Окончательные оба billing-прогона exit0. Первоначальный main30577ff
CI39256443467 имеет прежние browser failures до billing; они не объявляются
исправленными по новой реализации. Последующий полный CI проверяет сохранённые
старые assertions вместе с billing.

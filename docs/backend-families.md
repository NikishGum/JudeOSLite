# RD-09 / #201 — семьи и родительский доступ

19.10.2026, Europe/Minsk. [Issue](https://github.com/NikishGum/JudeOSLite/issues/201),
[общий HTTP/JSON-контракт](backend-contracts/README.md).
Назначение NikishGum получено штатной очередью:
[run39244310462](https://github.com/NikishGum/JudeOSLite/actions/runs/39244310462),
request `e99062df12724daea11fc2d400868310`. Основа `cf0948f`, отдельная ветвь
`codex/rd-09-families`; #198 уже в main. Frontend #200 не является зависимостью.

## Реализация

`0011_families.sql` добавляет только families/family_children/family_access.
Семья принадлежит филиалу; состав — явные существующие membership UUID. Составные
FK не допускают связь между филиалами; UNIQUE membership_id разрешает одному
membership только одну семью, включая конкурирующие запросы. Телефон/ФИО не
используются для поиска идентичности или слияния. Архивные карточки сохраняют
связь и историю. Никаких таблиц платежей, файлов или уведомлений в этой миграции нет.

`backend/internal/families/` реализует все маршруты контракта:
список/создание семьи, чтение/изменение, создание аккаунта, reset-password/revoke,
portal и member-profile. Коллекции используют page/limit/q, стабильный name/id,
полный total и hasMore. limit1–100, page0–1000000. Состав ограничен100 ID,
дубли/неверные UUID отклоняются; JSON ограничен32KiB, неизвестные поля и лишний
JSON отклоняются. Создание/обновление/аккаунты требуют явную version; создание0,
дальнейшие записи сравнивают версию под блокировкой семьи и увеличивают её.
Конфликт/занятый логин/занятый membership —409 version_conflict без частичной записи.

Родитель — тот же staff_users/staff_sessions; cookie/Argon2id/Origin/CSRF остаются
прежними. Публичной регистрации нет. Пароль12–128 байт, хеширование ограничено
двумя параллельными запросами семейного модуля. Ответ возвращает только Family,
без пароля/хеша. Сброс меняет credential и отзывает все старые сессии в транзакции.
Revoke удаляет только указанную family_access: другие семьи, staff-роли и аккаунт
сохраняются. Без семейных или staff-ролей человек может войти, но не читать данные.

Создание accounts с существующим login возвращает409, не привязывает существующую
личность и не меняет её пароль. Контракт не содержит операции привязки существующего
человека; автоматического сопоставления нет. Существующий staff+parent учитывается
при проверке объединённых прав, но не создаётся через перезапись занятого login.

Глобальный аккаунт со staff/owner-правами либо семьями другого филиала сбрасывает
только owner. Старые trainer account/password endpoints тоже запрещают локальному
admin менять идентичность с family_access; локальное снятие trainer-назначения
остаётся доступным. Это закрывает возможность захвата другого независимого доступа
через одно локальное назначение. Уточнение опубликовано в #201 до доставки.

## Граница доступа

Все семейные мутации заново проверяют живую сессию/учётную запись и актуальную
administrator-роль под блокировками в той же транзакции, где записываются данные,
версия и аудит. Owner/admin читают и меняют свои семьи; parent читает только
явно связанные семьи через portal и только конкретные связанные membership через
member-profile. Trainer не получает семейные контакты, даже если видит группу.
Скрытый/чужой/невалидный ID возвращает одинаковый404 без раскрытия существования.

`auth/me` добавляет user.isParent, а доступные clubs и `/api/auth/clubs/{clubID}` —
role parent и только метаданные филиала. Staff-проверка RequireClub сохраняет
прежнее значение: обнаруженный parent-клуб сам по себе не открывает реестр,
группы, тренеров, журнал, dashboard или owner API. Staff и parent роли складываются.
Архивные филиалы исключены из parent discovery, portal и профилей; owner/admin
могут читать историю. Семейные изменения/создание/сброс в архиве запрещены409,
отзыв доступа разрешён. Архив/отзыв staff/session не обходятся старым me.

Общий helper `families.LockMemberReader(ctx, tx, clubID, membershipID)` предназначен
также #202/#203. Вызывать только внутри auth.Require, в той же транзакции, где
читаются приватные данные, и удерживать её до завершения чтения. Helper повторно
проверяет живую сессию и owner/admin либо конкретные family_children/family_access;
держит блокировки сессии, назначения или семейных связей. Отзыв связи ждёт уже
начатое защищённое чтение; новые чтения после commit отзыва не проходят. Trainer
или присутствие клуба в me не заменяют эту проверку. Portal также держит семейную
строку/доступ до загрузки ответа, поэтому unlink/revoke не оставляют старый состав.

Аудит семей содержит только club/actor/entity/account UUID, action и version;
ФИО, контакты, логины, пароли/хеши и медицинские сведения туда не записываются.
Все ответы семейного/профильного API, включая ошибки, no-store.

## Профиль

MemberProfile возвращает все обязательные поля; отсутствие связи —null,
неизвестные имена группы/тренера —пустая строка, списки —[]. Группа определяется
по текущей дате в timezone филиала и датированному составу; тренер —из ближайшего
доступного занятия группы. В истории групп возвращаются действующие и завершённые
интервалы, тренер —последнее занятие этого интервала. Нет выдуманного постоянного
назначения тренера группе: v1 хранит его в расписании/занятии.

История —последние20 записей open/closed, включая null неотмеченного open;
процент и полные present/absent —только closed, cancelled исключены. Коррекции
учитываются при следующем чтении. Rate=null при нулевом знаменателе.
Дата/время истории берутся из сохранённого timezone занятия. Upcoming —ближайшие20
planned/open с ends_at>now(), без cancelled/superseded; до открытия по датированному
составу и активной карточке, после открытия по frozen roster. Архив/перевод не
переписывают снимки посещений.

## Проверки

Выполняются на синтетических данных, PostgreSQL18.6 и Go1.27.2. Секреты и реальные
реестры клуба не используются. Встроенный `/usr/bin/go` среды —другая программа;
Go1.27.2 взят из официального закреплённого golang image. Docker build использует
CA secret, TLS-проверка не отключается.

```sh
export PATH=/workspace/shared/rd09-go/bin:$PATH
export TEST_DATABASE_URL='postgres://test:test-only@127.0.0.1:55432/test?sslmode=disable'
(cd backend && go vet ./... && go test -race ./... && go build ./cmd/server ./cmd/migrate ./cmd/create-owner)
npm ci --prefix frontend && npm run build --prefix frontend
docker build --secret id=proxy_ca,src=/etc/ssl/certs/ca-certificates.crt -f deploy/Dockerfile -t judeoslite:ci .
python3 backend/tests/browser/families.py
PWA_BROWSER=webkit python3 backend/tests/browser/families.py
python3 frontend/e2e/pwa_v1.py
PWA_BROWSER=webkit python3 frontend/e2e/pwa_v1.py
python3 -m unittest discover -s deploy/test -p 'test_*.py'
```

Новые Go/PG проверки: две семьи/два одноимённых ребёнка/две локальные карточки
одной личности, parent A/B UUID/list изоляция, trainer denial, CSRF/no-store,
пагинация, устаревшие версии, уникальность при конкурирующей привязке, гонка
редактирования семьи, unlink/revoke, reset→старый session401, staff+parent union,
глобальный reset только owner, старый trainer endpoint не захватывает parent,
архив клуба, живое снятие admin, общий reader удерживает связь до чтения,
25closed→13/12/52% с history20 и upcoming20, коррекция→12/13/48%, cancelled/open,
null rate и обязательные массивы. Отдельная миграционная проверка: настоящий0010
со staff-сессией/назначением/архивным membership→0011→rerun, сохранение данных
и FK-отказ для межклубной связи. Старые auth/access/attendance/training/migration
проверки сохраняются.

Браузерный сценарий `backend/tests/browser/families.py` использует реальный owner
UI login, затем fetch в настоящем браузере с cookie/CSRF к Go и отдельные SQL
assertions. Создаёт двух детей, родителя и чужую семью; проверяет чтение своего,
foreign404, запрет staff API, unlink, reset и revoke. Ответы не подменяются.
Родительский UI не объявляется готовым: он принадлежит #200. Сценарий добавлен
в существующий CI для Chromium/WebKit после production image build.

Фактический локальный результат:

- Go vet/build и полный go test -race ./... на PostgreSQL18.6 —PASS, без SKIP.
  Дополненные family/upgrade/contract проверки —PASS.
- Production frontend typecheck/build и Docker image —PASS.
- Новый family browser→API→PG сценарий —PASS Chromium и WebKit26.5.
- Deployment unit43 и claim14 —PASS.
- Полный PWA Chromium154/WebKit26.5 —PASS: реестр/формы/назначения/отзыв,
  четыре режима2/4/2/1, closed1/present2/absent3=40%, static-only cache,
  offline/lost response/save guards/two-build lifecycle; контраст4.88/5.43.

Первый новый browser harness запуск имел несовпадающий локальный порт18086/18089,
второй —невалидный synthetic phone. Исправлены только setup/fixture, assertions
и продуктовые проверки не ослаблены. Один одновременный запуск двух старых PWA
harness столкнулся с общим портом18081; движки запускаются последовательно.
Эти ошибки подготовки не объявляются продуктовым PASS. Полные повторные прогоны
обоих движков завершились exit0/PASS.

Non-root WebKit использует Debian13 библиотеки в приватном browser cache:
libgtk4/graphene/harfbuzz-icu/manette/hyphen/GLES/cairo-script/hidapi/lzo.
PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1 отключает только проверку системного
пакетного расположения; движок действительно запущен, все browser/API/SQL assertions
сохранены. CI использует стандартный playwright install --with-deps без этого флага.
Физические устройства/родительский frontend/приёмка сотрудников не проверялись
в backend issue и не объявляются готовыми. #188 остаётся отдельной задачей.

## Стабилизация обязательных CI fixtures

Исходный f64aacf: PR run39246544367 полностью PASS, push run39246534626 attempt1
FAIL в WebKit dashboard при полном переходе со страницы с незавершёнными запросами;
attempt2 FAIL в Chromium reports: ожидались4 присутствия, отображались3. Эти
failures сохранены в issue, не заменены успешным результатом парного прогона.

Обнаружена конкретная гонка reports fixture: проверка строки на «Присутствует»
срабатывала уже по option открытого редактора до PUT commit. Теперь тест ждёт
настоящий PUT200, закрытие correction-form, status-span и SQL present; только затем
возвращает наблюдающий отчёт на передний план, проверяет visibility и отправляет
attendance-changed. Прежние4/3/57.1% и строгие pageerror assertions сохранены.
Dashboard fixture перед полным goto дожидается networkidle предыдущей страницы,
включая запросы journal/options, чтобы переход не обрывал их на старом документе.

Это только два существующих e2e fixtures; frontend/src/UX/API client не меняются.
Область опубликована до изменения в #200:
https://github.com/NikishGum/JudeOSLite/issues/200#issuecomment-6266739093.
Локальные Chromium/WebKit прогоны обоих сценариев и новый CI фиксируются в issue.

После синхронизации локально PASS все четыре затронутых сценария:
Chromium154/WebKit26.5 dashboard (40%→60%→62.5%, роли/отзыв/late replies) и
Chromium154/WebKit26.5 reports (3/7→4/7,57.1%, настоящий PUT200/закрытый редактор/
SQL present перед refresh). Strict pageerror/числа/права/ошибки не ослаблены.

# Проверка RD-03 / #185

Дата: 14.10.2026, Europe/Minsk. Только синтетические локальные fixtures,
реальные Go API/PostgreSQL. Рабочие данные/HTTPS клуба не изменялись.

## Назначение и координация

Старт от main fa1884a2cd4fda59333b3f21fd39935e69d6a948, изолированный worktree
`/workspace/rd-03-sessions`, ветвь `codex/rd-03-sessions`.
#183 CLOSED/Done, PR #190 merge 56475b64ad29d9018dbce3afe86d40724bae5df6
содержится в main; CI38542258544 и Deploy38543282879 success. Последующая
docs-only main CI38549066479 также success. #166 CLOSED и в main.

Собственный claim: **StarMadeGalaxy / acquired=true**, request
`e6a1d80a2ff24f768eb1809951d501ff`,
[run38549704417](https://github.com/NikishGum/JudeOSLite/actions/runs/38549704417).
Единственное назначение/In progress подтверждены до кода и после возобновления.
Повторного claim/прямого назначения нет. #184 занята NikishGum, её app-файлы
не редактировались; scope и group-контракт подтверждены live комментарием и
опубликованной ветвью. Статусы завершения смотреть в live GitHub.

## Выполненные проверки

- `npm ci --cache /tmp/rd03-npm-cache` и `npm --prefix frontend run build` — PASS.
- Production `docker build -f deploy/Dockerfile -t judeoslite:rd03 .` — PASS
  с публичным proxy CA через предусмотренный build secret. Backend не изменён.
- `go vet ./... && TEST_DATABASE_URL=… go test -race ./...` — PASS,
  Go1.27.2/PostgreSQL18.6: schedule/sessions/attendance/corrections,
  access/integration training, миграции и прочие существующие suites.
- `PWA_TEST_IMAGE=judeoslite:rd03 python3 frontend/e2e/pwa_v1.py` — Chromium
  154.0.8037.57 PASS. UI→API→PG, четыре режима 2/4/2/1, три причины, сетевой
  сбой/потерянный ответ/двойная отправка, close defaults, коррекция и независимый
  ledger 2 present/3 absent = 40%; роль/revoke/expiry/сессия/PWA и оболочка.
- Тот же полный `pwa_v1.py` с `PWA_BROWSER=webkit` — WebKit26.5 PASS,
  включая четыре расписания, независимый ledger, сетевые и PWA-сценарии.
- `PWA_TEST_IMAGE=judeoslite:rd03 python3 frontend/e2e/redesign_training.py` —
  Chromium154.0.8037.57 PASS; WebKit26.5 с `PWA_BROWSER=webkit` PASS.
  День/неделя/включительный период, два филиала и одинаковые названия групп,
  чужой group ID, create/reschedule/replace/cancel, отмена остаётся в списке,
  direct journal/reload/back/forward, Escape/Tab/возврат фокуса,
  network abort без перехода, double tap один запрос, повторный вход,
  исправление предыдущего участника, точный остаток2 и два close_default,
  trainer admin API404, обязательная причина, конкурентный correction409,
  перечитывание и история двух сотрудников, неизменный снимок3.
- Новый disposable сценарий включён в CI для обоих движков. Каждый запуск
  создаёт уникальные контейнеры/сеть и удаляет их в finally; публичный порт
  18085, два таких запуска выполнять последовательно.
- 320/390/1280: document overflow отсутствует, видимые контролы ≥44px;
  дополнительно 320×480 кнопки отметки выше нижней навигации, панель причин
  получает фокус и закрывается Escape без записи. CSS/типографика и экраны
  проверены двумя пакетными раундами, изображения открыты и просмотрены.
- `git diff --check`, Python compile — PASS.

В Cloud WebKit установлен в `/tmp`, недостающие Debian-библиотеки размещены
в его приватном sys/lib. `PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1` отключает
только проверку системного package-manager; browser/UI/API/DB assertions
не отключались. Стандартный CI использует `playwright install --with-deps`
и не устанавливает этот флаг.

## Визуальные свидетельства

[Календарь390](redesign-evidence/rd03/rd185-chromium-calendar-390.png) ·
[Календарь1280](redesign-evidence/rd03/rd185-chromium-calendar-1280.png) ·
[Карточки390](redesign-evidence/rd03/rd185-chromium-attendance-390.png) ·
[Карточки1280](redesign-evidence/rd03/rd185-chromium-attendance-1280.png) ·
[Действия320×480](redesign-evidence/rd03/rd185-chromium-attendance-action-320.png) ·
[Причины320×480](redesign-evidence/rd03/rd185-chromium-reasons-320.png) ·
[Коррекция390](redesign-evidence/rd03/rd185-chromium-correction-390.png) ·
[Коррекция1280](redesign-evidence/rd03/rd185-chromium-correction-1280.png).

Полные снимки сделаны сверху документа; fixed нижняя навигация отображается
на границе первого viewport. Дополнительный viewport действий доказывает
доступность кнопок при рабочей прокрутке. Это эмуляция браузера, не устройство.

## Найденные проблемы и ограничения

Первый новый runner не показал вход из-за mode700/600 локальной собранной
статики, недоступной UID65532 контейнера. Исправлена только копия fixture:
директории755/файлы644. Повтор прошёл. Одновременный запуск двух новых
runner на18085 дал port-in-use; повтор выполнен последовательно. Первый WebKit
не запустился из-за отсутствующих библиотек/переопределения LD_LIBRARY_PATH
его wrapper; добавлены публичные библиотеки в приватный sys/lib, затем PASS.
Эти failures не скрываются и не объявляются продуктовыми дефектами.

Первый просмотр выявил перенос подписей «Неделя/Период» на320; исправлены
отступы/nowrap. В журнале уточнена прокрутка текущей карточки и интервалы
кнопок коррекции; второй Chromium проход с viewport assertions PASS.

Impeccable применён: контекст PRODUCT/DESIGN/референс прочитан напрямую,
потому что launcher отсутствует; new-work/craft-floor и React best practices
прочитаны. CLI context/detector не выполнялись. Первая попытка независимого
review прервалась по лимиту сервиса, после «продолжай» повторена.

Физические iPhone/Android, экранная клавиатура устройства, установка PWA,
пользовательская визуальная приёмка и новое HTTPS-предметное тестирование
NOT RUN здесь; относятся к RD-06. Backend/права/транзакционные правила не
изменялись. Ни синтетические fixtures, ни скриншоты не выдаются за рабочие
сведения клуба или пользовательский signoff.

Независимый Impeccable finish reviewer: **ship**, просмотрел11 Chromium
изображений и исходники, материальных замечаний не обнаружил. Его границы —
source/screenshot review; тесты приняты из протокола, native keyboard/screen
reader/computed contrast самостоятельно не измерялись. Неблокирующее замечание
о белом тексте выбранной disabled-строки состава исправлено явным тёмным фоном;
это механическая CSS-правка, не новая полная визуальная приёмка.

Независимый documenter: **ship**. Сверил контракт и шесть парных снимков с
исходниками и существующей дизайн-системой. Уточнены размеры текста действий
на ПК/телефоне; новых глобальных токенов или примитивов для DESIGN/sidecar нет.
Повторные browser-тесты в этом документирующем проходе не выполнялись.

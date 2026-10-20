# Проверки карточки доступа #215

20.10.2026. Основа main `dc3522f`, регистрация UX-STAFF `a9f6a3b` в main.
Зависимости151/152/184/198/200/201 закрыты, результаты и успешные проверки
подтверждены по live issue/PR и ancestry main. Других активных чатов/PR по215 нет.
StarMadeGalaxy, request `60b404f1bc344c199ef14d4863369034`,
[claim39437660096](https://github.com/NikishGum/JudeOSLite/actions/runs/39437660096):
acquired=true, единственный исполнитель/In progress. Ветка `codex/ux-215-staff-access`.

## Фактические проверки

- `npm ci --prefix frontend`, `npm run build --prefix frontend`: PASS, TypeScript
  и production Vite. Существующий warning chunk>500kB сохранён, порог не менялся.
  Отдельного frontend lint script в репозитории нет; lint PASS не заявляется.
- Go1.27.2: форматирование, `go vet ./...`, `go test -race ./...` на реальной
  PostgreSQL18.6, сборка server/migrate/create-owner: PASS, без database SKIP.
  Дополнительный `TestStaffDirectoryIntegration -race` после итогового расширения:
  parent-only/бывшие staff/mixed, локальная приватность и canManageAccount,
  point administrator assignment/revoke, duplicates/version/archive/owner/CSRF,
  atomic audit rollback/concurrent versions, согласованный version+roles snapshot
  при20 конкурентных изменениях, live role/session после middleware: PASS.
  Owner candidate lookup/буквальные wildcard/лимит30, перевод существующего
  parent-only в staff без дубля и без потери family_access: PASS в Go/PG.
  Дополнительная проверка legacy audit: parent-only после правки аккаунта с
  пустыми/null `clubIds` не становится сотрудником; бывший administrator с
  непустыми прежними `clubIds` остаётся доступен. `staff-history-final.log`: PASS
  после фактического прогона; итоговый CI проверяет изменённый predicate.
- Production Docker image: PASS с session CA secret mount, без изменения TLS trust
  и без включения session CA/секретов в слои.
- 14 тестов очереди/клиента: PASS. Состав JSON/штатные Blocked by согласованы;
  назначение напрямую не выполнялось.
- `frontend/e2e/staff_access.py`: реальный browser→Go→disposable PG, Chromium
  151.0.7922.34 и WebKit26.5: PASS в обоих движках, jsErrors=[].
  Новый сценарий включён в штатный CI обоих движков.

Сценарий проверяет: создание trainer/admin с первым назначением; выбор существующего
staff без нового аккаунта; role+club в одном назначении; выбранный адресат вне
фильтра; duplicate login без частичной записи; owner/admin/trainer/parent/mixed scope;
cancel/draft/focus; конфликт версии с явной загрузкой; login/password/disable с
прекращением реальных сессий; enable без оживления токена; снятие одной/последней
роли с сохранением личности и reload; network failure/retry, один PUT при double
submit, потерянный ответ после commit/независимый SQL; failed directory закрывает
действия; mobile последняя строка→начало карточки→возврат к фильтру/позиции/фокусу;
keyboard/Escape; поздний old-scope ответ после ухода; приватность staff audit.

Визуальный пакет: list/card/form,1440/1280/390/320 в каждом движке,
overflow≤viewport, цели≥44px, автоматический rendered text/placeholder contrast.
Синтетическое длинное имя и длинный список специально проверяют перенос/scroll.
Снимки/логи — [.impeccable/review/staff215](../.impeccable/review/staff215/).

## Первичные неполные попытки

`chromium-attempt1.log`: обнаружена продуктовая ошибка — любой409 ошибочно
считался конфликтом версии, занятый логин блокировал поля/отмену. Теперь только
`version_conflict` блокирует submit; отмена остаётся доступной, login_taken
позволяет исправить логин. Password policy в UI совпадает с12–128байт сервера.

`chromium-attempt2.log`: fixture сравнивал сохранённое имя с конечным пробелом;
сервер штатно trim. Fixture приведён к нормализованному имени, assertions сохранены.
`chromium-attempt3.log`: все UI-операции/размеры прошли, финальный privacy assertion
захватил также клубные события с разрешённым названием. SQL ограничен
`entity_type='staff_user'`, проверка отсутствия staff имени/логина/пароля сохранена.
После этого оба полных адресных сценария PASS; неполные попытки не названы PASS.

Локальный WebKit первоначально не запускался из-за отсутствующих host libraries.
Восемь официальных Debian13 пакетов из существующего
[манифеста](../.impeccable/review/ux216/webkit-dependencies.json) повторно загружены,
SHA256 проверены, распакованы в пользовательский каталог; отсутствующие .so
добавлены в пользовательский browser bundle sys/lib. Подтверждены настоящий
launch/DOM WebKit26.5. `PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1` пропускает
только ldconfig preflight, проверки приложения/движка сохранены. CI использует
штатный `playwright install --with-deps`. Неудачные bootstrap/preflight не PASS.

## Impeccable и границы

Контекст PRODUCT/DESIGN/design-system и прототип Б прочитаны напрямую:
launcher и detector отсутствуют, detector PASS не заявляется.
New-work/Operate/craft-floor и React best practices применены внутри текущего
мира PRIDE. [Surface contract](../.impeccable/surfaces/frontend-src-features-staff.md).
Независимый [finish review](../.impeccable/review/staff215/finish-review.md) →
[bounded verdict ship](../.impeccable/review/staff215/finish-verdict.md):
две исходные правки resolved — мобильное раскрытие фильтров с активным счётчиком
(первая нормальная строка полностью выше dock на390/320), отдельная подпись
«Филиал сотрудника». Для добавленного owner lookup отдельно исправлены подпись
«Существующий аккаунт» и полный переносимый name/login выбранного адресата вне
обрезаемого native select. Все четыре замечания проверены независимо и resolved;
это verdict по указанным исправлениям/форме, не новая приёмка всего продукта.
[Documentation check](../.impeccable/review/staff215/documentation-check.md) подтвердил
сохранение DESIGN.md/sidecar/токенов и соответствие контракта исходникам.

Финальный набор логов: `go-checks-final.log`, `staff-api-final.log`,
`frontend-build.log`, `docker-build-shipping.log` + `docker-build-history-final.log`
(после уточнения исторического predicate), `coordination.log`,
`chromium-candidates.log` (полный сценарий с parent lookup) +
`chromium-existing-verdict.log` (адресное подтверждение двух последних правок формы),
`webkit-confirmation.log` (полный сценарий на финальной форме). Все PASS.
Снимки `*-existing-*`, `*-list/card/form-*`, `*-filters-expanded/active-*` лежат
в корне evidence; первый пакет — `round1/`, исходная обрезка выбранного логина —
`candidate-before-copy/`. Хеши исходников/артефактов — `results.json`.
Итоговые commit/CI/main/Done фиксируются в live issue после проверки доставки.

`chromium-confirmation.log` остановился на синхронном чтении scrollY до выполнения
запланированного requestAnimationFrame. Проверка теперь дожидается реального фокуса
заголовка карточки, установленного в том же callback, затем по-прежнему требует
scrollY=0. Без retries/новых timeout. `chromium-confirmation2.log` PASS; последующие
полные Chromium/WebKit также подтверждают scroll/filter/focus. Дополнительные
снимки фильтров и lookup — подтверждение конкретных review findings, без нового
общего поиска дефектов/полировки.

Прежние staff-фрагменты `pwa_v1`, `redesign_registry`, `shadcn_runtime` адаптированы
к выбранной карточке: те же UI create/несколько ролей/версия/отзыв/disable/SQL/44px
assertions, без retries/увеличения timeout/ослабления прав и формул остальных разделов.
Проверки физических устройств, HTTPS/PWA-установки и пользовательская приёмка #188
в этой задаче не проводились и не объявляются PASS.

Локальный `docker-build-history.log` завершился ResourceExhausted: диск среды был
заполнен кэшем Docker. Это не PASS. Удалён только неиспользуемый build cache этой
задачи через `docker builder prune -f`; тестовая PG и исходники сохранены.
Повторная сборка зафиксирована отдельно в `docker-build-history-final.log`.
Первый branch Build39444474780 на a41aac8 отменён как superseded для проверки
уточнённого predicate; Coordination39444474765 завершён success.

Перед доставкой интегрирован main `af1aa91` (фоновое обновление без сброса экрана).
Изменения App/общих resource hooks и оба новых background_refresh сценария CI
сохранены. Конфликт только в prepend памяти разрешён сохранением обеих записей.
Совместный production image проверен отдельно в `docker-build-merged.log`;
полный штатный CI проверяет совмещённый SHA, включая staff и background refresh.

Main Build39445133862 на `af1aa91` упал на старом ожидании удаления отчёта
после transport error. Отдельный main `d41d969` уже согласовал reports/dashboard
и reports_v1 с сохранением последнего подтверждённого содержимого/DOM и явной
ошибкой. Этот коммит интегрирован без собственных правок данных тестов; проверки
401/403/смены scope не менялись. Branch Build39446274766 на `14dffda` отменён
как superseded, не PASS. На совмещённом production-коде оба полных staff сценария
прошли ещё раз: `merged-chromium.log`, `merged-webkit.log`, jsErrors=[].
Новый полный CI проверяет также актуальные background-refresh regression.

Branch Build39447286756 на `8734722`: Go/PG, frontend/image, оба новых staff
сценария и backend браузерные suites прошли; общий browser шаг затем FAILED
в старом `redesign_registry:282`: ожидался пустой локальный список после revoke A.
Fixture admin управляет A **и B**, и trainer сохраняет B; единый реестр должен
оставлять эту запись. Исправлены только expectations: та же личность в списке,
нет revoke A, есть revoke B; прежний независимый API требует ровно `[B]`, прежний
переход в журнал A проверяет отказ доступа. Без retries/новых timeout/правок API.
Ошибка и её CI URL не названы PASS. Полные PWA/regression выполняются в обоих
движках перед новым полным CI; окончательные run/SHA — в live issue.

Финальная интеграция main `d2ff446`/`e9dd2fb`: background-refresh измеряет scroll
после fonts/layout и явного положения (допуск только1px); preview запускается
из frontend. Сохранены все проверки DOM/focus/draft/access. В workflow разрешён
конфликт перестановки Playwright bootstrap: установка один раз в раннем шаге,
обе staff suites остаются в полном browser блоке. Product-код215 не менялся.
Полные локальные `pwa_v1` Chromium/WebKit и исправленный background_refresh
Chromium/WebKit — PASS; логи `pwa-final-*`, `background-final-*`. Повторно14
coordination tests — PASS (`coordination-final.log`). Окончательные main SHA/полный
CI/Done фиксируются в live issue после действительного завершения, без нового
коммита только ради статуса. До этого issue остаётся In progress.

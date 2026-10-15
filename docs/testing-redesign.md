# RD-06: сквозная проверка редизайна

15.10.2026 · [#188](https://github.com/NikishGum/JudeOSLite/issues/188).
**Проверка продолжается; это не разрешение рабочего запуска.** Физическая PWA,
HTTPS role smoke и пользовательская приёмка пока не выполнены. Issue остаётся открытым.

## Основа и воспроизведение

Проверенная программная основа — `d8be57452b3f079d7b02b2bd3a389cf8d0277adf`,
merge RD-05/PR #194. RD-06 меняет тесты и документы; production-компоненты, API,
схема, исходные PRIDE-ресурсы и Working Panel сохранены.

Передача из остановленного Cloud-чата проверена по
[комментарию](https://github.com/NikishGum/JudeOSLite/issues/188#issuecomment-6187046654).
Исходный [claim](https://github.com/NikishGum/JudeOSLite/actions/runs/38699613597)
успешен: `acquired=true`, StarMadeGalaxy, request
`fc02eb61696d4543b4e75cc757b604e3`. Повторного claim/назначения не было.
GitHub-коннектор подтвердил StarMadeGalaxy; пользователь явно разрешил его вместо
неавторизованного `gh api user`. Один исполнитель, live In progress, зависимости
#166/#183–#187 CLOSED/Done, открытых конкурирующих PR/работающего Cloud-чата не найдено.

Исходный main зелёный:
[Build](https://github.com/NikishGum/JudeOSLite/actions/runs/38632181369),
[Coordination](https://github.com/NikishGum/JudeOSLite/actions/runs/38632181415),
[Deploy](https://github.com/NikishGum/JudeOSLite/actions/runs/38633667079).
15.10 публичный `/release.json` HTTPS-стенда совпал с этим SHA;
`/api/readyz` вернул 200/no-store. Это только публичная проверка готовности.

`npm ci`, TypeScript и production Vite build прошли с Node 24.19.0.
Production Docker build с `APP_REVISION` исходного SHA прошёл.
Go 1.27.2: gofmt, vet, `go test -race -v ./...`, build трёх команд — PASS;
PostgreSQL 18.6, 44 основных теста, 15 test packages, пропусков нет.
Первый локальный Go-запуск опередил создание тестового DB-контейнера и упал
на DNS. После явного pg_isready повтор зелёный; первая ошибка сохранена.

Существующие сценарии выполняются **без подмен API** на production-образе и
отдельных disposable PostgreSQL/network/container для каждого запуска:

```sh
cd frontend
npm ci
npm run build
cd ..
docker build -f deploy/Dockerfile -t judeoslite:ci .
python3 -m pip install playwright==1.62.0
python3 -m playwright install chromium webkit
python3 frontend/e2e/pwa_v1.py
PWA_BROWSER=webkit python3 frontend/e2e/pwa_v1.py
python3 frontend/e2e/redesign_training.py
PWA_BROWSER=webkit python3 frontend/e2e/redesign_training.py
python3 frontend/e2e/redesign_reports.py
PWA_BROWSER=webkit python3 frontend/e2e/redesign_reports.py
python3 frontend/e2e/redesign_dashboard.py
PWA_BROWSER=webkit python3 frontend/e2e/redesign_dashboard.py
```

Использовать версию Node из `.nvmrc`. Для установленного Chrome/Edge передать
`CHROMIUM_PATH` к его исполняемому файлу и оставить `PWA_BROWSER=chromium`.
Это запуск настоящего браузерного бинарника в автоматическом режиме;
установку PWA он **не** подтверждает. `pwa_v1.py` уже вызывает shell/registry.

## Доставка и публичный HTTPS — срез 15.10, Europe/Minsk

[PR #195](https://github.com/NikishGum/JudeOSLite/pull/195) доставлен в main
`75078069f315a3303b546b0905a0a634f1c471da`.
[Build 38708997849](https://github.com/NikishGum/JudeOSLite/actions/runs/38708997849)
и [Deploy 38710232985, attempt 2](https://github.com/NikishGum/JudeOSLite/actions/runs/38710232985/attempts/2)
успешны на этом SHA. Первая попытка deploy остановлена root preflight до pull,
остановки приложения и миграций: VPS требовал перезагрузки. Пользователь
перезагрузил сервер, повторный preflight прошёл; повторён только неудачный job
с прежними SHA/digest/generation 131. Контроллер и ограничения доступа не менялись.
[Доказательства доставки](https://github.com/NikishGum/JudeOSLite/issues/188#issuecomment-6188706676).

В 12:19:49 локальный исполнитель отдельно получил `/release.json` с этим SHA
и `/api/readyz` 200/no-store. В 12:25
[Workspace выполнил дополнительный публичный HTTP smoke](https://github.com/NikishGum/JudeOSLite/issues/188#issuecomment-6188898932):
health/readiness 200, защищённые API без сессии 401/no-store, manifest/три PNG
с совпавшими размерами, SW и выдача SPA-маршрутов. TLS проверялся.
Это датированные публичные чтения; UI под ролями и физическая установка PWA
ими не подтверждены. После следующего изменения main нужно сверить новый SHA.

## Браузеры и реальные устройства

Mac17,9, macOS 27.0.1 (26A434), Docker 29.8.2, Playwright 1.62.0.
Версии ниже взяты из работающих браузеров, а не старого снимка установленного приложения.

| Среда | Сквозной PWA/shell/registry | Занятия | Отчёты | Главная | Физическая установка/relaunch |
| --- | --- | --- | --- | --- | --- |
| Chromium 151.0.7922.34 | PASS | PASS | PASS | PASS | NOT RUN |
| WebKit 26.5 на Mac | PASS | PASS | PASS | PASS | Не является физическим Safari |
| Google Chrome 155.0.8059.79 на Mac | PASS | Включены основные операции PWA | Включён 40% ledger | PASS | NOT RUN / доступ инструмента заблокирован |
| Microsoft Edge 155.0.4283.45 на Mac | PASS | Включены основные операции PWA | Включён 40% ledger | PASS | NOT RUN / доступ инструмента заблокирован |
| iPhone 13 / Safari | NOT RUN для RD-06 | NOT RUN | NOT RUN | NOT RUN | NOT RUN |
| Android / Chrome | DEFERRED / NOT RUN | DEFERRED | DEFERRED | DEFERRED | Явно отложено пользователем 15.10 |

Для iPhone Mirroring удалось подключиться и увидеть домашний экран, затем связь
прервалась при использовании телефона. iOS 26.6.2 сообщалась пользователем
в прежнем v1-проходе; версия и Safari в RD-06 заново не подтверждены.
Результаты QA164/v1 и прежнее отложение Android не переносятся в RD-06.
Отдельным новым решением 15.10 пользователь явно отложил Android/Chrome и его
физическую PWA на последующую проверку. Это DEFERRED / NOT RUN, не PASS;
Chrome/Edge/iPhone и приёмка сотрудников остаются обязательными.

## Предметный путь и независимые числа

1. Администратор: создание пяти отдельных карточек, контакт/дата, совпадающие
   ФИО и семейный контакт без слияния; сохранение/повторное открытие, архив/возврат.
   Отдельный registry fixture проверяет две группы, датированные включение,
   исключение и перевод, историю/состав, общий identity только владельцем.
2. Четыре UI-расписания: weekly=2, day_parity=4, date_list=2, once=1 занятие.
   Ожидаемые даты вычисляет Python независимо от API preview/generation.
   Отдельный SQL-сеанс на сегодня обеспечивает воспроизводимый журнал;
   он не выдаётся за занятие, созданное UI-расписанием.
3. Training fixture: once, перенос, замена фактического тренера, отмена с причиной,
   дневной/недельный/произвольный период, одинаковые имена в разных филиалах,
   прямой journal URL, reload/back/forward и возврат к выбранному периоду/UUID.
4. Тренер открывает frozen roster A–E. A present; B unknown; C illness;
   D excused; E close_default/unknown. До подтверждения открытия снимка нет.
   Панель причин/отмена/Escape, фокус, предыдущая карточка, сводка, double submit,
   закрытый/отменённый журнал без записи проверены.
5. Администратор исправляет C на present с отдельной причиной. SQL независимо
   подтверждает `present=2, absent=3, unknown=2, illness=0, excused=1`.
   Один closed, **2/(2+3)=40%** в отчёте владельца. История не подменяет текущие числа.
6. Независимый reports fixture: **3/7=42,9% → 4/7=57,1%**, пустой closed с «—»,
   planned/open/cancelled, архив/перевод, сохранённый timezone занятия, ID drilldown,
   история 50+8 с tied timestamps и новым событием между страницами.
7. Dashboard fixture: **2/5=40% → 3/5=60% → 5/8=62,5%**; два филиала с разницей
   timezone 24 часа, разные today, SQL-счётчики, frozen roster, реальные
   mark/close/correct/archive, точные переходы к группе/журналу/созданию карточки.

## Доступ, сеть, lifecycle и обновление

Проверены отдельные owner/admin/trainer и многоролевой сотрудник, несколько
назначений, чужие прямые API/UUID, live revoke/archive/expiry, logout/change user,
поздний ответ после смены scope/выхода, очистка DOM перед pagehide и свежее me
после настоящего возврата history. Native BFCache hit отдельно не измерен.
Несохранённые формы защищены при переходе/смене филиала/back/update; pending save
не размонтируется. События и history/storage не содержат предметных черновиков.

RD-06 дополнительно проверяет потерю транспорта **до commit и после commit**
для mark, close и correct. При потерянном ответе интерфейс не заявляет успех:
для закрытия — «Изменение не подтверждено», для коррекции — «Исправление не
подтверждено». SQL отличает отсутствие записи от уже выполненной транзакции;
reload читает фактический closed/исправленную отметку. Повтор stale-коррекции
получает конфликт и не создаёт второе событие corrected. Mark сохраняет
идемпотентный повтор. Это контролируемый transport fault, не физический авиарежим.
Управляемая service worker страница WebKit обошла первоначальный route-перехват
коррекции: запрос успешно дошёл, поэтому ожидаемой ошибки не было. Инъекция
close/correct перенесена на fetch delivery, с отдельными assertions одного
срабатывания и HTTP 200 до потери ответа. Настоящая транзакция/SQL сохранены;
первоначальный неуспешный прогон удержан в evidence.

Manifest/icons/start_url/scope/standalone проверены. Cache Storage содержит
только версионированную allowlist публичной статики: API и персональные HTML
ответы не кешируются. Две настоящие сборки A/B, waiting, очистка старого кеша,
save guard и сохранение другой вкладки проходят. Offline queue отсутствует.

## Адаптация и доступность

320/390/768/1280/1440: горизонтального overflow нет; основные контролы ≥44px,
поля и основные мобильные действия 16px. Dashboard CTA помещается над нижней
навигацией на 320×780/390×844; короткая панель причин проверена на 320×480.
Native Dialog: цикл Tab/Shift+Tab, Escape, возврат к клавиатурному инициатору,
inert фон, scroll lock, вложенная защита черновика. Reduced motion проверен.
Физические safe-area/клавиатура/поворот остаются NOT RUN для RD-06.

Добавлен измеритель видимого текста на реальных computed colors/backgrounds:
обычный ≥4,5:1, крупный ≥3:1. Минимумы измеренных образцов — 4,64 и 5,43.
Disabled-контролы исключены; изображения под текстом требуют отдельного замера.
Стрелка select занимает зарезервированный отступ вне текста. Это проверка
текстового контраста выбранных экранов, не полная сертификация WCAG.

Первые Mac/WebKit прогоны выявили различие нативного фокуса: pointer click
не фокусирует кнопку, обычный Tab пропускает кнопки при текущей настройке Cocoa.
Минимальная HTML-проба воспроизвела это вне PRIDE; Option+Tab фокусирует следующую
кнопку. Клавиатурные сценарии теперь сначала фокусируют инициатор и активируют
его Enter; native traversal отчёта использует Option+Tab только на Mac/WebKit.
Assertions фокуса/порядка/возврата сохранены. Код приложения не менялся.

Impeccable: прочитаны context проекта, Audit/Harden/craft floor; автоматический
context launcher не запустился из-за отсутствующего engine/недоступной записи.
Использованы PRODUCT/DESIGN/Working Panel и ручная проверка desktop/mobile
снимков. Автоматический детектор и полная пользовательская оценка не объявлены PASS.

## Уточнение CI-сценария реестра — 15.10

[Push CI на 3a0a92f](https://github.com/NikishGum/JudeOSLite/actions/runs/38716127275)
остановился на строгом `assert not errors`: WebKit сообщил access-control error
для GET истории групп участника. Парный PR run38716136572 и оба CI предыдущего
head2107707 прошли. Сценарий после создания сразу закрывал карточку, не проверяя
отдельно загружаемую историю; после archive/restore/identity save тоже переходил
дальше по готовности основной карточки. Это оставляло неоднозначность отмены
дочернего чтения при навигации; лог не доказывает причину всех сообщений WebKit.

`redesign_registry.py` теперь проверяет начальный период, строки списка и три
сохранённых периода после архива/возврата и правки личности перед уходом.
`pwa_v1.py` при необработанной ошибке дополнительно печатает URL/stack/последние
API events. Прежний список ошибок и все его assertions сохранены.
Production-код не менялся. После правки повторены только затронутые полные
PWA(shell/registry) сценарии: Chromium151.0.7922.34 и WebKit26.5 — PASS,
ledger по-прежнему 2/5=40%. Исходный CI log и хеши новых локальных логов сохранены
в results.json. Это автоматические disposable API/PG-проверки, без физической установки.

## Что ещё блокирует завершение

- Browser Use отклонил test.judopride.tech, затем повтор после разрешения
  пользователя снова вернул saved permission/managed policy deny. Запрет
  сохранялся и после сообщения «доступ включён». Отдельный запрос для loopback
  QA188 также отклонён. Обход через иной браузер/CDP/телефон не выполнялся.
- При явной обратной передаче из Workspace 15.10 исходный claim, аккаунт,
  единственный StarMadeGalaxy, OPEN/In progress и отсутствие параллельного
  PR/работающего Cloud-чата проверены заново; claim не запускался.
  После сообщения пользователя о снятии запрета повтор в 12:34 Europe/Minsk
  `createBrowserTab('chrome', 'https://test.judopride.tech')` снова отклонён:
  `A saved user permission setting blocks this action`; `origin approval gate
  did not grant access`; `A saved permission or managed policy denies access`.
  Чтение панели настроек через `getApp('Codex')` также запрещено для
  `com.openai.codex`. Пользователю передан штатный путь Settings → Computer Use
  → Manage у Chrome → Blocked websites; управляемое правило меняет администратор.
  Это технический блокер, а не отсутствие разрешения заказчика. Обхода нет.
- Последующие снимки настроек пользователя показывают другой UI: Agent permissions,
  `Browsing: Always allow` у `https://test.judopride.tech` и у Default;
  отдельного списка Blocked websites на этом экране нет. Панель Chrome пишет
  `Not installed`, однако `listBrowsers()` обнаруживает Chrome как `type=extension`.
  Обновление подключения инструмента и повтор того же штатного запроса также
  вернули saved permission/managed-policy deny (состояние записано в 12:51).
  Снимки не подтверждают наличие блокирующей записи; точный источник отказа
  не установлен. Пользователю предложен штатный перезапуск Chrome/Codex,
  а не повторное согласие или обход gate.
- Пользователь затем подтвердил полный перезапуск Chrome и Codex; повторный
  штатный запрос снова отклонён той же политикой. Следующий способ восстановления
  — `/feedback` с точным отказом, показанными разрешениями и ID локального чата;
  дополнительные подтверждения заказчика не требуются.
- Поэтому новый HTTPS QA188/role smoke и физическая установка/standalone/
  закрытие/relaunch/смена сотрудника в Chrome/Edge/iPhone не проверены.
- Android недоступен и отдельно явно отложен пользователем 15.10.
  Приёмка сотрудниками/визуальное решение пользователя не получены.

Порядок оставшихся действий — [протокол приёмки](acceptance-redesign.md).
Доказательства — [RD-06 evidence](redesign-evidence/rd06/README.md).

# RD-06: текущий release и оставшаяся приёмка —21.10.2026

Часовой пояс отчёта — Europe/Minsk. Проверенный main:
`36c35b2e961279133cd5cc354d7770fc6fe42fea`.
Отдельный worktree `/workspace/JudeOSLite-rd06-20261021`, ветвь
`codex/rd-06-acceptance-current`. Пользователь явно передал ранее начатую #188
в этот чат. Предыдущие четыре чата idle/notLoaded, последние turns completed;
конкурирующих открытых PR при старте нет. Прямые #166/#183–#187 CLOSED/Done,
их результаты в истории main. #220/#221 также CLOSED/Done и реализованы в main.

## Claim и точные проверки main

Исходный claim не повторялся: StarMadeGalaxy, request
`fc02eb61696d4543b4e75cc757b604e3`,
[run38699613597/job116304342880](https://github.com/NikishGum/JudeOSLite/actions/runs/38699613597/job/116304342880).
Заново прочитанный лог содержит `acquired:true` для #188/StarMadeGalaxy/request.
`gh api user --jq .login` и GitHub connector подтвердили StarMadeGalaxy;
live #188 OPEN/In progress, назначен только StarMadeGalaxy. Прямого назначения нет.

[Build39620402525](https://github.com/NikishGum/JudeOSLite/actions/runs/39620402525)
и [Deploy39623354372](https://github.com/NikishGum/JudeOSLite/actions/runs/39623354372)
завершились SUCCESS/attempt1 на указанном SHA. Прочитаны jobs/steps и полные
логи check/deploy, в Git сохранён только относящийся к задаче extract:
[ci-and-claim.json](ci-and-claim.json). Это проверка **свежих CI-прогонов текущего
main**, не новые локальные browser runs и не перенос старого PASS.

| Текущий CI | Реальный результат |
| --- | --- |
| Frontend build, фоновые обновления | PASS, Chromium/WebKit; временные ошибки и scope/privacy |
| Go formatting/vet/race/PG18.6/migrations/build | PASS; audit/reports также в полном suite |
| PWA обоих движков | PASS:2/4/2/1 расписания,40%, роли/отзыв/expiry/account swap/stale, mark/close/correct до/после commit, два builds/save guard/static-only cache |
| Занятия #216 | PASS обоих движков: отдельные карточки, недельная сетка/страница, snapshot, correction/history, прямой URL/back/reload и пять ширин |
| Отчёт #221 | PASS обоих движков:3/7→4/7, пустой closed, архив/перевод/timezone, период/возврат, ошибки/late/revoke; main dashboard также PASS40→60→62,5% |
| История #220 | UX-HISTORY PASS Chromium151.0.7922.34/WebKit26.5, jsErrors=[]; прежний RD-04 также проверяет paging/scope/revoke |
| Прочие интеграции | staff/notifications/billing/documents/families обоих движков PASS;22 synthetic frontend screen/state groups отдельно от backend-приёмки |
| Operations/Compose/TLS/restore | PASS; штатный VPS deploy подтвердил HTTPS readiness и release SHA |

Production runtime и существующие e2e assertions здесь не менялись; добавлен
отдельный opt-in HTTPS smoke существующего QA188 без предметных записей.
Полные suites не запускаются повторно без изменения/сбоя. Исходные неудачные
попытки #220/#221 сохранены в их [протоколах истории](../../../history-search-validation.md)
и [статистики](../../../statistics-overview-validation.md); текущий зелёный CI
не стирает эти failures. В частности, main95c0ca2 ранее остановился до history
browser из-за готовности временной PostgreSQL; исправление TCP preflight уже
в проверенном main36c35b2.

## Независимый публичный HTTPS —18:37 Europe/Minsk

TLS verification включена. [public-https.json](public-https.json) сохраняет
время, пути/статусы/cache-control/размеры/SHA256 и сравнение с локальным build:

- `/release.json`200/no-cache и точный main SHA до и после проверки;
- health/readiness200/JSON/no-store;
- auth/me, dashboard, reports filters/attendance, audit filters/events,
  notifications и portal/families без сессии401/JSON/no-store;
- `/history` и `/statistics`200/SPA; это выдача оболочки, не authenticated UI;
- все17 файлов `frontend/dist`, включая HTML shell, JS/CSS/fonts/icons,
  manifest и worker, побайтно совпали с HTTPS. Assets immutable;
  manifest standalone/start_url=/scope=/, реальные PNG-размеры сохранены.

Локально на Node24.19.0 выполнены `npm --prefix frontend ci` и
`npm --prefix frontend run build` (включает TypeScript). PASS, прежнее Vite
предупреждение о chunk>500kB сохранено; отдельного lint script нет.
Первоначальное диагностическое обращение к несуществующему `/api/history`
получило ожидаемую для неизвестного пути404; правильные audit endpoints затем
проверены отдельно. Это не ошибка существующего API и не PASS authenticated history.

## Impeccable и текущие экраны

Прочитаны Impeccable4.3.1/audit, DESIGN/design-system и контекст текущих экранов.
Launcher/context и detector отсутствуют — NOT RUN, применён direct context
fallback. UI не меняется. Один совместный просмотр существующих синтетических
desktop/mobile снимков #220/#221 выполнен; новыми снимками этого чата они не названы:

- [История desktop](../../../../.impeccable/review/history220/chromium-1280-default.png),
  [история WebKit mobile](../../../../.impeccable/review/history220/webkit-390-open.png).
  Все пять product source hashes в history220/source-hashes.json совпадают;
  e2e hash изменён только доставленным TCP preflight36c35b2.
- [Статистика desktop](../../../../.impeccable/review/statistics221/implementation-desktop.png),
  [статистика mobile](../../../../.impeccable/review/statistics221/implementation-mobile.png).
  `git diff 3686821 HEAD -- frontend/src/features/owner-dashboard
  frontend/src/features/club-workspace/Reports.tsx` пустой: утверждённая реализация
  статистики и её интеграция сохранены. Парные reference/implementation и контентные
  319/320/390/768/1280/окно1440 уже проверены в протоколе #221.

Измеримые a11y/responsive проверки текущего main прошли в CI; скриншоты помогают
передаче человеку, но не подтверждают его мнение, физическую клавиатуру телефона
или новый полный WCAG-аудит. Новых продуктовых блокеров из прочитанных результатов
и просмотренных снимков не установлено.

## Authenticated HTTPS —18:50 Europe/Minsk

Первоначально не было тестового входа; пользователь предоставил пароль в этом
чате. Он введён через скрытый TTY prompt, не записан в файлы/evidence/аргументы
команды. Callable локального Browser app `js` нет; используется headless Linux
Chromium151.0.7922.34 с проверкой TLS. Это технический HTTPS-проход, не управление
физическим браузером заказчика. [https-readonly.json](https-readonly.json) —
**PASS с первой попытки**,18:50:17–18:50:39, release36c35b2 проверен до/после:

- 26 API и35 browser requests, все API no-store; существующие QA188/admin/trainer,
  чужой филиал и owner-only APIs403/404 согласно матрице;
- исходный журнал A–E и независимый отчёт API/UI дают1closed/2present/3absent,
  unknown2/illness0/excused1/**40%**; ручные даты/«Применить»/сохранение диапазона
  после быстрого месяца, детализация проблемы и обратный переход, вкладка финансов;
- новая история находит единственную corrected15.10 по action+actor UUID+
  `#короткийID corrected`, total1 и точному ID записи; API owner/admin и UI
  администратора/подробности/reload подтверждены;
- тренер открывает закрытый контрольный журнал по canonical URL и после reload;
  admin/trainer прямой UI статистики не показывает отчёт владельца;
- actual worker Cache Storage содержит только versioned static `/assets/`;
  1280/390 у истории/статистики и390 у тренера без overflow, jsErrors=[];
- digest полного контрольного журнала до/после идентичен. Предметных POST/PUT/
  DELETE нет; только три login/logout. Все сессии завершены, me401 проверен.

[Desktop40%](https-summary-1280.png), [mobile40%](https-summary-390.png),
[панель поиска desktop](https-history-toolbar-1280.png),
[панель поиска mobile](https-history-toolbar-390.png) — новые реальные HTTPS
снимки синтетического QA188. Профили/имена сотрудников не включены в захват.
Новых фикстур/аккаунтов/занятий этот runner не создаёт.

Воспроизведение после проверки текущего release и `playwright install chromium`:

```sh
python3 frontend/e2e/redesign_https_readonly.py \
  --expected-sha 36c35b2e961279133cd5cc354d7770fc6fe42fea \
  --out /tmp/rd06-https
```

Runner требует скрытый пароль или защищённую QA188_PASSWORD, не запускается
автоматически в CI и не повторяет неудачный вход. Не ослабляет limiter/права.
Source SHA256 runner записан в JSON. Доставка документов/e2e не меняет
production inputs; [равенство исходному36c35b2](production-inputs.json) проверено
перед отправкой. [SHA256 evidence и runner](checksums.json) сохранены отдельно.

## Что пока не выполнено

Самостоятельные проходы трёх ролей, текущая визуальная оценка и новый физический
Chrome/Edge installed-PWA smoke не подтверждены. Сценарии и памятки обновлены:
[точный остаток](../../../acceptance-redesign.md#остаток-приёмки--21102026),
[памятки](../../../staff-guide-redesign.md), [отличия](../../../redesign-changes-v1.md).
Пользователь21.10 ответил о самостоятельной приёмке сотрудников: **«Ещё не проводили»**.
Контрольный QA188-журнал15.10/40% сохранён; предметные записи стенда не менялись.
Android и оставшиеся ручные iPhone сохраняют согласованный DEFERRED/NOT RUN,
нового согласия не запрашивается. #188 OPEN/In progress до оставшейся приёмки.

Доставка документов и повторный release-контроль фиксируются после commit/push/main.

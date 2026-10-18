# RD-07 — проверка переноса shadcn в рабочее приложение

18.10.2026, отдельный cloud worktree `/workspace/shadcn-runtime`, ветка
`codex/shadcn-runtime`, issue[#198](https://github.com/NikishGum/JudeOSLite/issues/198).
Исходный main78c1b69 + регистрацияa0ca509. Для a0ca509 фактически SUCCESS:
[Build39118168710](https://github.com/NikishGum/JudeOSLite/actions/runs/39118168710),
[Coordination39118168697](https://github.com/NikishGum/JudeOSLite/actions/runs/39118168697),
[Deploy39118791283](https://github.com/NikishGum/JudeOSLite/actions/runs/39118791283).
Это проверки исходной базы, не нового UI. Claim run39118216590 acquired=true;
единственный assigneeNikishGum/In progress перепроверен после реализации.

Эталон `17ecd57068cff17e4eaf055e0ac503b61cab0399` изучен в отдельном detached worktree;
в production перенесены компоненты/тема и адаптеры реального UI. Карта функций —
[shadcn-runtime.md](shadcn-runtime.md). Demo/registries/role/day controls и fake
settings не включены. Backend/shared API/lifecycle/SW/миграции не изменены.

## Среда и команды

Node24.19.0/npm11.9.0, Python3.12.14, Playwright1.62.0,
Chromium154.0.8037.57, WebKit26.5, PostgreSQL18.6-bookworm, Go1.27.2.
Все клубы/учётные записи синтетические в отдельных disposable Docker databases;
боевые данные и QA188 не использовались. Backend настоящий, browser requests идут
через настоящие API, assertions сверяют SQL/JSON. Docker image harness получает
текущий production dist, а не Vite development server.

```sh
npm --prefix frontend ci
npm --prefix frontend run build
python3 -m unittest discover -s .github/scripts -p 'test_claim*.py'
# В golang:1.27.2 с TEST_DATABASE_URL отдельного PostgreSQL:
go vet ./...
go test -race ./...
# Каждый сценарий выполняется отдельно в Chromium и WebKit:
python3 frontend/e2e/pwa_v1.py
python3 frontend/e2e/redesign_training.py
python3 frontend/e2e/redesign_reports.py
python3 frontend/e2e/redesign_dashboard.py
PWA_BROWSER=webkit python3 frontend/e2e/pwa_v1.py
PWA_BROWSER=webkit python3 frontend/e2e/redesign_training.py
PWA_BROWSER=webkit python3 frontend/e2e/redesign_reports.py
PWA_BROWSER=webkit python3 frontend/e2e/redesign_dashboard.py
docker build --secret id=proxy_ca,src="$CODEX_PROXY_CERT" -f deploy/Dockerfile -t judeoslite:rd198 .
```

Cloud WebKit dependencies переустановлены в private cache: старые symlinks в /tmp
после возобновления среды не работали. В cloud установлен
`PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1`: отключена только проверка списка
пакетов distro, реальный WebKit26.5 запускается со всеми runtime libraries.
CI использует штатное `playwright install --with-deps` без этого параметра.

## Фактически выполнено

| Проверка | Результат |
| --- | --- |
| TypeScript7 / Vite8 production build | PASS |
| Go vet + race, все пакеты и PostgreSQL/access/migration tests | PASS |
| Штатная очередь назначения,14 coordination tests | PASS |
| Chromium RD-03 | PASS: day/week/period, create/move/replace/cancel, group IDs/two clubs, journals/back/reload, save/network/correction/conflict/history |
| Chromium RD-04 | PASS:3/7=42.9%→4/7=57.1%, причины/архив/перевод/зоны, периоды, paging58, revoke и recovery |
| Chromium RD-05 | PASS:2/5=40%→3/5=60%→5/8=62.5%, реальные действия/счётчики, роли/разные даты/revoke/logout |
| Chromium PWA/RD-01/RD-02/RD-07 | PASS:4 режима2/4/2/1, реестр/группы/сотрудники/филиалы, query/back/черновики/сохранения, cache/lifecycle/update/privacy |
| WebKit PWA/RD-01/RD-02/RD-07 | PASS:тот же полный сценарий и исходные assertions |
| WebKit RD-03/RD-04/RD-05 | PASS:те же операции/права/ledger, ошибки/сеть/recovery и keyboard |
| Production Docker image | PASS:чистая production сборка Node24.19/Go1.27.2 |

## Снимки и доступность

[`../.impeccable/review/shadcn-runtime/`](../.impeccable/review/shadcn-runtime/)
содержит viewport screenshots всех9 разделов1280/900/390/320, participant detail/
editor, профиль desktop collapsed/mobile, More, календарь и hover/focus.
Это синтетические данные настоящей БД, не demo mode приложения. Дополнительные
role captures1280/390 лежат в `roles/`; stdout/stderr и SHA256 — `logs/` и `results.json`. У сохранённых логов удалены
только хвостовые пробелы/пустой EOF; записаны хеши исходного и сохранённого текста.
Автоматические проверки подтверждают no overflow, desktopSidebar248/76/mobileDOM0,
живые настройки, настоящее меню выхода,44px mobile targets, календарь/гражданскую
дату/query и возврат фокуса. Desktop исключения выбранного эталона: toggle36px,
стрелки/дата32px. Staff padding16v12h/Avatar44 и выравнивание чисел при переносе
подписей проверены геометрией. Контраст проверяется исходным rendered-text audit
без исключений или снижения порогов4.5/3; окончательные минимумы обоих движков: normal4.88:1, large5.43:1.

## Исходные ошибки и исправления

- Пропущенный TooltipProvider давал runtime error после login — добавлен provider,
  pageerror assertion сохранён.
- Ручная стрелка native select4px не измерялась contrast audit — восстановлены
  штатные известные5px и положения; сам audit не изменён.
- `transition-all` давал промежуточную mobile date высоту43.625px — оставлен
  только переход цветов; minimum44px сохранён.
- Присутствие `data-active=false` активировало все Sidebar строки в Tailwind4 —
  false атрибут убран, добавлена проверка единственного активного пункта.
- Специфичность базовых button styles красила roster и нарушала контраст —
  native baseline понижен через :where, сохранены семантические feature styles.
- Календарь получил44px дни, узкий viewport fit и правильные ghost/selected цвета.
- Fixtures обновлены под реальный profile menu, отсутствие Sidebar DOM на mobile,
  второй read dialog после сохранения editor и закрытие card перед навигацией.
  Responsive query делает атомарный DOM snapshot после фактического mount новой
  оболочки; исчезающие nth locators не пропускаются и размеры не округляются вниз.
  Все бизнес/SQL/rights assertions оставлены.
- Два отдельных поздних WebKit dashboard прогона дали pageerror «access control
  checks» для уходящих sessions/options requests, один training — internal error
  на reload. Assertions не фильтровались. Диагностический повтор с полным
  pageerror/stack/failedRequests и неизменным API прошёл; исходный main контроль
  тоже прошёл. Причина исходных сбоев не подтверждена, повтор не назван исправлением
  приложения. Исходные логи сохранены в evidence/logs; окончательный CI обязателен.
- После верхнего закрытия обновлён полный прямой/обратный Tab цикл в harness:
  каждый промежуточный контрол проверяется, новый крестик не пропускается.
- Первые WebKit попытки не дошли до приложения из-за missing runtime library;
  после установки выполнены настоящие повторные запуски. Это не application PASS.

Поздняя параллельная подготовка трёх disposable PG завершилась до API с
PostgreSQL not ready; это не провал assertion приложения. Убраны только неиспользуемые
тома собственных текущих тестов, старые/активные тома сохранены. Повторные четыре
сценария выполнены последовательно и прошли; причина исходной остановки контейнеров
отдельно не доказана. Автоматические retry/filter/пропуски не добавлены.

## Impeccable / границы результата

Применены context, new-work с закреплённым пользовательским эталоном, Operate,
craft-floor, два ограниченных визуальных прохода, один detector (результат `[]`),
независимый finish reviewer и documenter. Исходный review.md дал `fix` только
для недоступного сверху выхода длинных панелей; verdict.md оценил исправление
как resolved/`ship` без оставшихся пунктов. Это verdict по одному исправлению,
а не новая общая приёмка ролей/PWA. Верхний44px выход закреплён при прокрутке,
вызывает прежний guarded onCancel и блокируется busy; containment/restoration
сохранены, нижняя кнопка отмены остаётся.
Официальный shadcn skill применён к композиции Sidebar/DropdownMenu/Avatar/
Popover/Calendar/Field/ToggleGroup, semantic theme и source-reviewed компонентам.

Физические устройства, native installation/standalone, физическая сеть/экранная
клавиатура, screen reader и самостоятельная приёмка сотрудников — NOT RUN в этом
cloud прогоне. Native back/forward проверяет новую сессию; native BFCache попадание
не измерено. Старые PASS/DEFERRED #188 не присваиваются новому дизайну. Публичный
HTTPS нового UI не проверен до доставки проверенного PR в main. #198 остаётся
открытой до готовности результата в main; чужая #188 не закрывается.

# Проверка RD-04 · 14.10.2026

Задача [#186](https://github.com/NikishGum/JudeOSLite/issues/186).
[Контракт UI/read-only](redesign-reports.md).
Ветка `codex/rd-04-reports`, основа main a7ec960 после RD-02/RD-03, затем535bacc (только fixture/docs RD-03).
#183 CLOSED, её результат56475b6 и успешный CI38542258544/main проверены;
перед стартом #186 Todo/без исполнителя и другой работы. Claim StarMadeGalaxy,
request d8f2d43ac78845db8a9bef5210e2258c,
[acquired=true run38555111624](https://github.com/NikishGum/JudeOSLite/actions/runs/38555111624).
Назначение и In progress повторно проверены перед отправкой.

## Контроль browser → Go → PostgreSQL

`frontend/e2e/redesign_reports.py`: новая disposable БД PostgreSQL18.6,
loopback Go из production-образа, собранная frontend-статика. Только отдельные
синтетические fixtures, внешние данные/согласованный HTTPS-контроль v1 не изменяются.
Два филиала A/B, одинаковые имена групп, 4+3 участника. Исторический сеанс A
имеет sourceDate31января, старт00:30 1февраля2000 по сохранённому Europe/Minsk
(исторический UTC+02); браузер Pacific/Honolulu не меняет выборку.

| Период 2000-02-01 | Closed | Planned | Open | Present | Absent | Unknown | Illness | Excused | UI |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| A, включая пустой closed | 2 | 0 | 1 | 2 | 2 | 1 | 1 | 0 | 50,0% |
| B | 1 | 1 | 0 | 1 | 2 | 0 | 1 | 1 | 33,3% |
| Сеть | 3 | 1 | 1 | 3 | 4 | 1 | 2 | 1 | 42,9% |
| После UI illness→present в A | 3 | 1 | 1 | 4 | 3 | 1 | 1 | 1 | 57,1% |
| Только пустой closed | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | — |

Закрытие/отметки/отмена — реальные API, коррекция — реальный UI с проверкой
сохранённого audit. SQL создаёт синтетические исторические занятия и проверки
перевода/архива. Отменённое с отметками, open с present, planned и следующий день
не меняют знаменатель. Архив/перевод после снимка не перемещают прежние отметки.
Сумма unknown+illness+excused совпадает с absent до и после коррекции.

Проверены: быстрые Сегодня/Неделя/Месяц на воскресенье03.01.2027 (неделя28.12.2026),
отдельно от browser timezone; возврат сохранённого ручного периода; включительность,
неверный диапазон/нулевой знаменатель; совпадающие имена и выбор UUID из desktop
таблицы и mobile списка; независимость шапки/отчёта; переход в реальный календарь
с проверкой club/group/from/through; архивный филиал не предлагает рабочий переход.

Сбой reports скрывает показатели, retry сохраняет выбор. Задержан настоящий ответ
A, затем выбран B: старый ответ не возвращает A. Коррекция/attendance-changed
перечитывает сервер. Задержанный отчёт после отзыва owner не восстанавливает DOM.
Для admin аналогично проверены отозванные назначения и поздний ответ истории.
API admin reports403/trainer audit403, no-store, отсутствие PRIVATE payload/причин
в общей истории; logout очищает приватную область.

История:55 событий A+2 B+1 network на одну дату; первая страница50, следующая8,
без повторов. Новая более поздняя вставка после первой страницы не попадает
за её cursor. Филиал/действие/даты, отдельные network events, empty/error/retry,
поздний ответ прежнего фильтра, admin scope и revoke. Общий аудит не содержит
ни контактов, ни причины реальной поштучной коррекции. Tab/Shift+Tab быстрых дат,
все видимые controls≥44px и mobile input/button/select≥16px; overflow320/390/1280
отсутствует, JS pageerror отсутствует. Контраст использованных текстовых пар
по нормативным цветам (WCAG sRGB): красный/белый5,16:1, secondary/белый5,43:1,
secondary/серый5,06:1, зелёный/белый6,81:1, text/белый15,43:1; все≥4,5.
Это расчёт токенов, не измерение каждого пикселя или screen reader проверка.

## Выполненные команды

- `npm ci`, `npm run build` — PASS, TypeScript и production Vite.
- Production Docker build с доверенным CA secret — PASS.
- `PWA_TEST_IMAGE=judeoslite:rd04 python3 frontend/e2e/redesign_reports.py` —
  Chromium154.0.8037.57 PASS на первоначальном и окончательном коде.
- `PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1 PWA_TEST_IMAGE=judeoslite:rd04
  PWA_BROWSER=webkit python3 frontend/e2e/redesign_reports.py` — WebKit26.5 PASS.
  Использованы приватные Debian browser libs; стандартный CI не использует skip.
- `go vet ./... && go test -race ./...` в Go1.27.2 Docker с disposable PostgreSQL18.6
  — PASS, все пакеты, reports/audit, access/training integration и миграции.
- `python3 -m unittest discover -s deploy/test -p 'test_*.py'` —43 PASS.
- `PWA_TEST_IMAGE=judeoslite:rd04 python3 frontend/e2e/pwa_v1.py` — PASS.
- `PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1 PWA_TEST_IMAGE=judeoslite:rd04
  PWA_BROWSER=webkit python3 frontend/e2e/pwa_v1.py` — WebKit26.5 PASS.
  Оба полных сценария: четыре режима2/4/2/1, ledger40%, RD-01/RD-02,
  сессии/права/lifecycle, network/lost-response/retry, static-only cache,
  две сборки/ожидающее обновление/другая сохраняющая вкладка.
- Итоговый стандартный CI/main/deploy — ссылки в live issue/PR после push.
- `python3 -m py_compile frontend/e2e/redesign_reports.py`, `git diff --check` — PASS.

Новая регрессия включена в стандартный CI для Chromium/WebKit после существующих
PWA/RD-03 сценариев. API/SQL/access tests остаются существующими и не ослабляются.

## Визуальное evidence и Impeccable

Существующая Working Panel/PRIDE сохранена. Context launcher и detector отсутствуют
в этом runtime (`No such file or directory`, exit127); PRODUCT/DESIGN, прототип,
new-work/Operate/craft-floor и React best practices прочитаны напрямую.
Автоматический detector не выполнялся, его PASS не заявляется.

Проведены два пакетных визуальных прохода. Первый показал необходимость
прокручивать причины в desktop-таблице: незавершённые/отметки/причины сгруппированы
в подписанные ячейки без потери полей; даты на390 уплотнены в одну строку,
на320 оставлены вертикально. Финальный Chromium функциональный повтор PASS.

Финальные пары (синтетические данные):

- [Отчёт1280](redesign-evidence/rd04/rd186-chromium-report-1280.png),
  [390](redesign-evidence/rd04/rd186-chromium-report-390.png),
  [320](redesign-evidence/rd04/rd186-chromium-report-320.png).
- [История1280](redesign-evidence/rd04/rd186-chromium-history-1280.png),
  [390](redesign-evidence/rd04/rd186-chromium-history-390.png),
  [320](redesign-evidence/rd04/rd186-chromium-history-320.png).

Каждый снимок открыт и проверен. Full-page capture помещает fixed bottom-nav
на уровне нижнего края исходного viewport; это не положение панели внутри потока.
Реальное отсутствие document overflow и высоты контролов проверены браузером.

Независимый Impeccable finish reviewer: **ship**. Прочитал код/контракт/craft-floor
и открыл все шесть финальных Chromium screenshots. Материальных замечаний нет.
Его scope — код/изображения; собственные Go/WebKit/PWA/computed styles не запускал.
Независимый documenter: **ship** в пределах документационной сверки кода,
контракта/brief и шести screenshot. Явных неточностей нет; правок не потребовалось.
[Finish review](redesign-evidence/rd04/finish-review.md),
[documentation check](redesign-evidence/rd04/documentation-check.md).
Новых глобальных токенов/ресурсов/примитивов нет; DESIGN.md и исходный sidecar
не переписываются. Отсутствие sidecar/schema у существующего DESIGN — прежнее
состояние проекта, его ремонт не входит в #186.

## Ограничения и исходные неуспехи

Локальный WebKit сначала сообщил missing system libraries; публичные Debian
библиотеки извлечены в приватный browser sys/lib. Host validation остаётся отдельным
локальным ограничением; UI/API assertions не отключаются. Один ранний одновременный
старт нового WebKit runner пересёкся с Chromium на18086 и получил port-in-use;
повтор запускается последовательно. Это ошибки подготовки окружения, не PASS.

Первый полный Chromium PWA остановился в существующей RD-02 проверке
`not_to_contain_text('Synthetic однофамилец')` после переключения в B. Та же fixture
раньше явно создаёт общую личность во втором филиале: её имя там законно.
Отрицательная проверка могла пройти только в промежуточном пустом DOM.
Уточнён только тест: ждёт реальный ответ B, проверяет общий participantId и **другой
локальный membershipId**, видимость разрешённой строки и404 по membershipId из A
даже для owner в B; прежний admin foreign404 сохранён. Продуктовый код реестра
не меняется, исходный FAIL сохранён. Полный сценарий после поправки прошёл в Chromium154 и WebKit26.5.

Физические устройства, экранная клавиатура устройства, PWA installation/standalone,
новый предметный HTTPS-проход и пользовательская визуальная приёмка здесь NOT RUN;
это RD-06. Linux WebKit не выдаётся за физический iPhone. Контрольные screenshots
не являются рабочими данными клуба. Итоговые SHA/CI/main — live issue/PR.

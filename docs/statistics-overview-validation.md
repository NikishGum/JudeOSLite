# Статистика А / #221 — проверка переноса, 21.10.2026

Поручение: [#221](https://github.com/NikishGum/JudeOSLite/issues/221).
Основа реализации — main `28ef7b1`; регистрация из подготовленного `9e2a7a8`
доставлена отдельно, с сохранением UX-HISTORY и остальных новых записей.
Первый claim отказал из-за отсутствующей регистрации; после её доставки штатный
[run39585004352](https://github.com/NikishGum/JudeOSLite/actions/runs/39585004352)
подтвердил acquired=true, NikishGum и In progress. Отказ не обходился назначением.

## Контракт и backend

[Соответствие элементов API](statistics-overview-ux.md#соответствие-элементов-контрактам--21102026-221).
Основные total/clubs/groups, расчёт процента и finance summary сохранены.
Минимальное расширение attendance: `openSessions[]` — `id`, `clubId`, `groupId`,
`date`, `startsAt`, `timezone`. Это только навигационные метаданные; состав,
личные отметки и тренер в агрегат не добавлены. Список выбирается из того же
`per_session` CTE и того же SQL/MVCC-снимка, что и счётчики, по реальному `open`,
без cancelled/superseded, с включительными датами в сохранённом timezone.
Пустой список — `[]`. Старые потребители продолжают получать прежние поля.

Существующая страница занятия читает журнал через прежний проверенный API.
Обратный переход использует фиксированный `/statistics` и проверенные даты/UUID,
без произвольного return URL. Проблемы и журналы сохраняют период/филиал/группу;
возврат из подчинённой детализации восстанавливает обзор. Архивный филиал
сохраняет историю и счётчик, но не получает ссылку рабочего журнала.

## Парное визуальное сравнение

Эталон неизменяемый: `dfb9549d2be9d04a977ee7529cea93dd9147cdd9`,
`design-system/prototypes/statistics/index.html?idea=overview`.
Пары содержат одинаковые **синтетические** агрегаты; это сравнение оформления,
а не доказательство backend. Полевые данные в runtime поступают только из API.

| Снимок | Эталон | Реализация |
| --- | --- | --- |
| Desktop: отчёт 1216px, эталонное окно1280 | [reference](../.impeccable/review/statistics221/reference-desktop.png) | [implementation](../.impeccable/review/statistics221/implementation-desktop.png) |
| Mobile: отчёт358px, окно390 | [reference](../.impeccable/review/statistics221/reference-mobile.png) | [implementation](../.impeccable/review/statistics221/implementation-mobile.png) |

Ширина измеряется по DOM предметной области; sidebar не считается частью
отчёта. В парном mobile-снимке скрыта только фиксированная навигация оболочки
через screenshot style, чтобы она не закрывала сравниваемый контент. Обычные
[снимки оболочки](../.impeccable/review/statistics221/shell-1440.png) сохраняют её.
Выполнены контентные319/320/390/768/1280 и окно1440, без overflow, цели≥44px,
reduced-motion; длинные имена проверены отдельно без новых циклов полировки.

Необходимые интеграционные отличия: заголовок страницы и внешние отступы даёт
существующая оболочка, сохраняются вкладки посещаемости/финансов; на телефоне
добавлено раскрытие дополнительных показателей, чтобы не потерять план/отметки.
Продукт не содержит А/Б, демонстратора ошибок, фиксированного «сегодня»,
демоданных или прежнего объяснения отсутствия финансов.

Impeccable: применён new-work для переноса закреплённого направления, craft-floor,
прямое чтение контекста после отсутствующего launcher. Detector также отсутствует;
это не PASS детектора. Независимый finish reviewer проверил пары обоих движков и
оболочку; исправления его списка и итоговый вердикт —
[finish-review.md](../.impeccable/review/statistics221/finish-review.md).
Глобальные DESIGN.md/токены не меняются: это локальная поверхность А.

## Проверки

| Проверка | Результат и граница |
| --- | --- |
| npm build / TypeScript | PASS; отдельного lint-script в проекте нет |
| Go vet, full `go test -race ./...`, PostgreSQL18.6, Go build | PASS: [лог](../.impeccable/review/statistics221/go-vet-race.log) |
| reports integration с новым полем, parent/совмещённые роли/expiry | PASS: [лог](../.impeccable/review/statistics221/reports-roles-expiry.log) |
| redesign_reports Chromium151 / WebKit26.5, реальный Go/PG | PASS: [Chromium](../.impeccable/review/statistics221/reports-chromium.log), [WebKit](../.impeccable/review/statistics221/reports-webkit.log) |
| statistics_overview, синтетические API, оба движка | PASS: [геометрия](../.impeccable/review/statistics221/result.json), [WebKit](../.impeccable/review/statistics221/ui-webkit.log) |
| background_refresh, оба движка | PASS: polling/focus/ошибка/контекст/доступ, без remount |
| redesign_dashboard, оба движка | PASS: прежняя главная и общий hook |
| PWA Chromium/WebKit | PASS: [Chromium](../.impeccable/review/statistics221/pwa-chromium.log), [WebKit](../.impeccable/review/statistics221/pwa-webkit.log); физическая установка не проверялась |
| screenshot_redesign Chromium | PASS:22 группы состояний, синтетические ответы; включает финансовый UI |

Реальный контроль: 3/7=42,9%; после коррекции4/7=57,1%. Два филиала, одинаковые
имена групп, transfer/archive, closed empty, planned/open/cancelled/superseded,
нулевой знаменатель/период, причины, сохранённый timezone. Переход open→журнал→
статистика сохраняет реальные фильтры и число1. Проверены error/retry, поздний
ответ чужого контекста, отзыв owner/admin и no-store. Go отдельно проверяет
parent/admin+parent/trainer+parent/owner+parent, чужие ID/scope, отзыв/expiry между
middleware и SQL-чтением. История и серверный взвешенный итог сохраняются.

Новый UI harness дополнительно проверяет применение ручного периода без запроса
при ошибочном черновике, сохранение ручного диапазона при быстрых периодах,
фокус/DOM/раскрытия/сортировку при фоновых чтениях, 503 с сохранением данных и
403/404/410 с очисткой имён, достоверный отбор проблем, duplicate UUID и архив.

Первоначальные локальные попытки не названы PASS: отсутствовали регистрация и
библиотеки WebKit; затем старые проверки ожидали немедленного применения дат,
табличного drilldown и16px у компактных контролов. Harness приведён к явному
контракту #221: submit, раскрытые строки,14px быстрые кнопки/12px счётчик, при
сохранении44px целей и прежних предметных assert. Прежние failures не означали
разрешения ослабить расчёт или доступ.

WebKit локально запущен с официальными библиотеками Debian13 в пользовательском
каталоге. Обёртка установленного browser bundle сохранена с добавлением inherited
LD_LIBRARY_PATH вместо его потери. Предварительная проверка системного ldconfig
отключена только для локального запуска; реальный browser launch/DOM и сценарии
выполнены. CI использует штатный playwright --with-deps.

Воспроизведение UI: `npm --prefix frontend run build`, preview4173,
`python3 frontend/e2e/statistics_overview.py`; для пар — `STATS_REFERENCE` с URL
локально обслуживаемого immutable прототипа. Исторические `reports_v1.py` не
заменяют актуальный сценарий `redesign_reports.py`. PWA/устройства и приёмка
сотрудников #188 этой задачей не закрываются. iPay не подключён.

Проверка регистрации main28ef7b1 в CI завершилась WebKit internal error в
прежнем shell reload (run39584976311), до статистики; это не объявлено PASS.
Полные локальные PWA Chromium/WebKit на итоговой реализации прошли.

## Доставка в main и последующий контроль

Реализация `3686821`, [PR225](https://github.com/NikishGum/JudeOSLite/pull/225),
merge в main `95c0ca2`. Оба исходных CI (push39588288371 и PR39588333810)
полностью PASS. В main также доставлена совместимая история #220; её код и
предметные проверки сохраняются.

Повтор main39590723829 прошёл статистику в Chromium/WebKit, но остановился до
браузера `history_search.py`: мигратор пытался подключиться к временной БД до
готовности TCP. Сохранён [исходный failure](../.impeccable/review/statistics221/main-ci39590723829-failure.log).
`pg_isready` без host мог видеть временный Unix-socket сервер initdb. В harness
готовность теперь проверяется по127.0.0.1 и конкретной БД, с явным отказом при
исчерпании прежних30 попыток. Runtime/migrations и предметные assertions не
меняются. Адресный сценарий истории повторяется в обоих движках.

Deploy39591194140 отказал в выборе уже устаревшего main4bf7bd8 после появления
95c0ca2; проверка текущего SHA сохранена. Развёртывание допускается только после
успешного push-CI фактического main. Точный финальный CI/deploy — GitHub Actions
и итоговый отчёт чата; старые PASS не подменяют этот контроль.

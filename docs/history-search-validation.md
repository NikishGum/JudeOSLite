# UX-HISTORY/#220: протокол проверки

21.10.2026, Europe/Minsk. Область — компактная история, поиск/авторы/total,
существующие права и keyset. Отдельное дерево `codex/issue-220-history` от main
`a17d0cb`; регистрация `2654202` доставлена в main до product-кода.
Прямые RD-04/#186 и RD-08/#200 CLOSED; результаты и прежние успешные CI проверены.
#215 CLOSED/результат в main, конфликтующих активных чатов/PR не обнаружено.
Auth/staff/API-клиент/миграции/глобальные токены не меняются.

Штатный claim: StarMadeGalaxy, request`640bf1fdd546437aac81022b88495c65`,
[run39578429446](https://github.com/NikishGum/JudeOSLite/actions/runs/39578429446),
acquired=true, единственный исполнитель и live In progress подтверждены.

## Проверенные границы

- `go vet ./...`, полный `go test -race ./...` с реальной PostgreSQL18.6
  (disposable databases через TEST_DATABASE_URL) — PASS. Новые audit tests:
  поиск за первой страницей, AND/порядок/регистр/е-ё/#ID, literal `%`/`_`/backslash,
  единые подписи/неизвестные коды, точный actor UUID, одноимённые/disabled авторы,
  owner/admin/multi/trainer/parent-only/network/archive, чужие accounts/counts,
  курсор другого условия, пустая страница с ненулевым total, конкурентные вставки
  и согласованный items/total. Прежние live session/role/disable после middleware
  проверки сохранены. Публичная выдача без private payload и с no-store.
- Production TypeScript/Vite build — PASS. В репозитории нет отдельного lint script;
  lint не объявляется выполненным. Сохраняется предупреждение Vite о chunk>500KB.
- Background regression Chromium151.0.7922.34/WebKit26.5 — PASS: обе ширины,
  timer/focus, сохранение DOM/деталей/ввода/фокуса, transport503,401/403.
-14 coordination tests — PASS. `git diff --check` — PASS.
- Точный SQL statement проверен `EXPLAIN (ANALYZE,BUFFERS)` на236 синтетических
  событиях:4.504ms execution, matched70KB в памяти, без disk spill. Это малый
  контроль, не нагрузочный тест. [План](../.impeccable/review/history220/sql-plan.txt).
  Точный count требует проверки всех совпадений; индекс без измеренной причины
  не добавлялся, миграция не требуется.

## Исходные неуспешные попытки

Новая PG-фикстура первоначально пропустила обязательный club_id семьи; исправлена
только фикстура. Проверка нулевого результата для `_` была неверна: literal `_`
входит в `attendance_record`. Теперь тест проверяет57 буквальных совпадений и
нулевой результат для `__`; продуктовый поиск не менялся ради ожидания.

Ранние native browser probes делили один тестовый app; шестой вход законно
получил429. Лимитер не менялся. Штатный новый runner создаёт отдельную БД/app на
каждый запуск, с двумя входами. В первом Docker-прогоне локальный dist с mode600
не читался USER65532; runner теперь копирует временный dist755/644, как существующий
RD-04 harness. Product API/auth assertions, таймауты и retries не ослаблялись.

В Cloud отсутствовали WebKit runtime libraries. Debian trixie snapshot packages
скачаны штатным apt с проверкой метаданных и извлечены в пользовательский каталог,
ссылки на runtime .so добавлены в локальный Playwright bundle. WebKit26.5 реально
запущен. Локальный запуск использует PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1
(проверка системной установки пакетов), сами browser/API/DOM assertions сохранены.
Стандартный CI устанавливает --with-deps и не использует этот флаг.

Первый WebKit visual pass выявил переполнение390px: native select сохранял
в scrollWidth полное длинное выбранное название, хотя сам control rectangle
оставался в пределах колонки. Локальные overflow:hidden/text-overflow:ellipsis
ограничивают только поле select, сохраняя native popup/клавиатуру. Проверка
page scrollWidth сохранена без допусков. Исправлены также двойная рамка Input,
повтор внутреннего заголовка и высота многострочных chips; mobile Button16px
сохраняет прежнюю проверку RD-04. Это продуктовые исправления, не ослабление теста.

Impeccable: прочитаны context проекта, distill/craft-floor; launcher отсутствует,
применён direct context fallback. Выбранный вариант А/PRIDE сохранён, повторного
выбора концепции нет. Browser-снимки и измерения выполняются пакетно.
Физический macOS/native popup и реальные телефоны здесь не заявлены PASS;
проверяется WebKit engine. Отдельная приёмка/PWA/устройства #188 не закрываются.

Адресный production browser→Go→PG runner прошёл Chromium151.0.7922.34 и
WebKit26.5: default live Minsk/чужой timezone, поиск за первой50, AND/порядок/е-ё/#ID,
быстрый ввод300ms/Enter/Escape/подсветка, UUID/одноимённые/disabled авторы,
комбинированные условия/снятие с фокусом, неверные даты без запроса,
дозагрузка57 без дублей, пустой результат/очистка, фон/transport retry,
поздние ответы/отзыв. Все1280/980/390/320, фильтры открыты/закрыты, длинные
значения, цели≥44, select48/appearance:none, контраст и reduced-motion — PASS,
jsErrors=[]. [Снимки и измерения](../.impeccable/review/history220/).

Адаптированный RD-04 Chromium — PASS: прежний ledger/отчёт сохранён, дозагрузка
50+8 при новой вставке даёт «Загружено58 из59», refresh с начала, network/privacy/
late responses/live revoke. В первой WebKit попытке новый harness установил hold
до завершения предыдущего date request, заблокировав refresh; теперь ожидает
строки/готовую кнопку до установки hold. Assertion/timeouts/retries не ослаблены.
Адаптированный RD-04 WebKit26.5 также PASS; исходные неуспешные попытки сохранены.
После retry harness ожидает исчезновение alert (завершённый успешный ответ),
а не уже показанный пустой результат. Итоговые CI/main фиксируются при доставке.

Точная доставка/CI и live завершение публикуются в [#220](https://github.com/NikishGum/JudeOSLite/issues/220).
[Хеши production-исходников/метаданные](../.impeccable/review/history220/source-hashes.json)
и [хеши доказательств](../.impeccable/review/history220/checksums.json) связывают
последний пакет с кодом. После последней CSS-поправки отдельные shipping прогоны
Chromium/WebKit снова PASS, включая border0 внутреннего поиска/outer48, все
четыре ширины и те же функциональные assertions. Дальнейшей полировки нет.

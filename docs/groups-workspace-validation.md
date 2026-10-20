# Проверка #214 · 20.10.2026

Исходная основа: main9ca620d с Build39416135812 SUCCESS. Регистрацияd210a24
в main подтверждена ls-remote;14 queue unit tests PASS. Claim39426047727
acquired=true, единственный StarMadeGalaxy / In progress проверены до кода.
Изолированная ветвь `codex/ux-214-groups`, первый implementation SHAda51751.

## Фактически выполнено

- npm ci, typecheck и production build: PASS. Существующий warning bundle>500kB
  сохраняется, конфигурация/проверки не ослаблялись.
- Адресный Chromium151.0.7922.34 → Go1.27.2 → PostgreSQL18.6: проверены добавление
  нескольких с частичным отказом/одним запросом на участника при double tap,
  перевод/завершение на дату, граничный состав, weekly preview/change,
  сохранность отменённого переноса при новой версии серии, far move и автоматическая
  смена периода, once без правила, cancellation planned/network/retry,
  открытие настоящего snapshot и отмена с подтверждением.
- Финальный расширенный сценарий (SQL count сохранённых marks/snapshot, closed guards,
  day_parity/date_list, замена тренера, потеря ответа после commit/версия/права,
  состояние ошибки/retry, keyboard, размеры1440/1280/390/320): PASS.
  Лог: `.impeccable/review/groups214/targeted-pass.log`, JS errors=[],
  Chromium151.0.7922.34, Go1.27.2, PostgreSQL18.6.
- Настольные/мобильные снимки реального тестового администратора на синтетической БД:
  `.impeccable/review/groups214/{roster,schedule,rule-editor}-{1440,390}.png`,
  `list-390.png`. Первый scrolling capture был недостоверен (fixed skip-link);
  заменён снимками от document top. Локальная корректировка убрала повторный
  заголовок/создание группы с мобильной detail-страницы, сохранив их в списке.
- Независимый Impeccable finish review: FIX→SHIP по трём исправлениям F1–F3
  (фокус добавления, валидность future date-list picker, mobile unavailable recovery).
  Два отдельных отчёта/измерения в `.impeccable/review/groups214/`; craft-floor
  прочитан при review и root до исправлений, launcher/detector отсутствуют.
- Штатный Build and checks39428487194 на da51751: FAIL в прежнем selector
  `.group-detail` внутри redesign_registry.py:215. Проверки состава/границ дат/архива/
  переходов/ID сохранены, локальный fragment адаптирован к новой group-roster,
  contextual settings и реальному read групп при смене филиала. Без retries или
  увеличения timeout. Точный изменённый group-fragment отдельно выполнен на
  Chromium/Go/PostgreSQL,1280/390/320: PASS (`registry-group-fragment-pass.log`).
  Новый штатный CI финального SHA проверяется до закрытия; итоговые run/SHA — live #214.
  Workflow не менялся.

## Неполные попытки

Исходный harness ожидал слово «пересека», но API возвращает «На эти даты участник
уже состоит в группе»; assert уточнён до реального сообщения. Тест обнаружил
неправильное доступное имя нового select с вложенными options: исправлены explicit
label/htmlFor у новой группы перевода и фильтра состояния. Отдельная попытка
дошла до open cancellation, затем упала на ошибке fixture SQL `session_id` вместо
`training_session_id`; production SQL/миграции не менялись. После нескольких
свежих входов локальный login limiter вернул429/15 минут; лимит не отключался,
повторный login отложен до естественного конца окна. Неполные попытки не PASS
полного адресного сценария. Ещё один неполный harness-проход ожидал date-list
правило, оставшись в другой группе; перед assertion добавлен явный переход в
нужный groupId. Последующий полный адресный прогон PASS.

Локальная адресная проверка адаптированного fragment также обнаружила hover:
глобальное правило button:hover давало белый заголовок выбранной группы на светлой
поверхности (1,07:1). В локальном `.group-choice:hover` явно сохранён существующий
тёмный text token. Повтор тех же состояний, включая архивную группу, прошёл contrast
на1280/390/320. Общий CSS/токены не менялись. Это механическое исправление состояния,
не новый визуальный цикл; прочие пункты независимого verdict остаются в его scope.

Main CI39431171642 на1c1b8a8 прошёл обновлённый registry fragment,
RD-07 и RD-01 и остановился в pwa_v1.py:714 на старом имени кнопки
«Закрыть форму». В DOM новая «Закрыть форму группы» уже была disabled.
Исправлено только имя в прежнем to_be_disabled assertion. Отдельный реальный
held POST response check PASS: закрытие формы и навигация блокируются,
в SQL ровно одна группа, после release форма закрывается. Лог
`.impeccable/review/groups214/pending-response-pass.log`.
Штатный workflow/таймауты/проверки SW не менялись; финальный результат — live #214.

## Границы

Только локальные синтетические fixtures, без реальных персональных записей,
новой PWA/device/HTTPS/приёмки сотрудников. #188 остаётся отдельным открытым issue.
Новые runtime-файлы/CSS только в feature/groups; SessionForm/ScheduleEditor,
раздел sessions/навигация/API/backend/миграции/токены сохраняются побайтно.
Визуальная система сохранена; DESIGN.md/sidecar не требуют новой палитры/токенов.
Множественное добавление не атомарно; поимённый результат и повтор оставшихся явны.
Массовый перевод не реализован, сохранён атомарный одиночный перевод.

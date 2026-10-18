# PRIDE — перенос shadcn-дизайна в runtime

Поручение18.10.2026, RD-07/#198. База — актуальный main `78c1b69` (регистрация задачи `a0ca509`); эталон
`origin/codex/shadcn-app-template`/`17ecd57068cff17e4eaf055e0ac503b61cab0399`.
Прототип даёт целевой вид; штатная реализация даёт данные/операции/права.

## Карта сохранения функций

| Экран прототипа | Рабочая реализация | Сохраняемые действия |
| --- | --- | --- |
| Sidebar/шапка/mobile/profile | App/navigation, NavigationBoundary, Dialog, PwaStatus | Реальные me/club/role union, все URL/query/direct/back/forward, смена контекста/выход, draft/pending save, lifecycle/PWA |
| Главная трёх ролей | features/dashboard/Dashboard и panel adapters | Счётчики реального API, выбранный филиал/дата, group/session/journal/report drilldown и актуализация после команд |
| Участники | ParticipantRegistry/ParticipantEditor/MembershipHistory | Поиск/пагинация, полная identity/контакт/вступление/начальная группа, owner-only shared identity, версии/конфликты, архив/возврат/история |
| Группы | GroupManagement/MembershipHistory | Create/edit/archive, roster на дату, include/exclude/transfer, интервалы, group→sessions с ID |
| Занятия/календарь | SessionCalendar/SessionForm | Серверные date/timezone/группа/период, once/move/substitute/cancel, детали/версии/переходы |
| Правила расписания (нет полной формы в demo) | ScheduleManagement/ScheduleEditor | Все4 режима, preview/save, версии/генерация/история |
| Журналы тренера (demo только читает) | TrainerSessions/SessionJournal/AttendanceCards | Frozen roster, последовательные карточки/причины, cancel, сохранение перед переходом, close defaults, серверное восстановление |
| Журналы администратора/коррекции | AdminJournals/AdminJournal/RecordHistory | Чтение, закрытые correction поштучно/reason/version/conflict/history |
| Статистика | OwnerDashboard/report adapters | Все фильтры/быстрые и ручные периоды/архивные, closed/planned/open/причины/формула/zero, drilldown |
| Сотрудники | TrainerManagement/TrainerEditor + OwnerManagement mode=staff | Аккаунт/пароль/disable, несколько admin/trainer назначений, revoke, серверные ограничения |
| Филиалы | OwnerManagement mode=clubs | Create/edit/zone/address/archive с последствиями и историей |
| История | AuditHistory | Разрешённый envelope, фильтры/стабильная пагинация, без private payload |

Общие действия не заменяются demo callbacks. Registry showcase/role/state/day switch/
synthetic values не входят в приложение. Настройки профиля не могут писать имя без API:
меню использует фактический доступ, поддерживаемые настройки интерфейса и настоящий logout.
Backend/API-client/CSRF/date/attendance/lifecycle/SW остаются существующими контрактами.


## Перенесённая система

- Source-reviewed компоненты radix-nova/shadcn взяты из точного эталона `17ecd57`,
  а не из меняющегося registry. MIT notice — `frontend/src/components/SHADCN-LICENSE.md`.
  Impeccable4.5.2 установлен в cloud в `~/.agents/skills/impeccable`; официальный
  shadcn skill установлен из `shadcn-ui/ui` в `~/.agents/skills/shadcn`.
- Sidebar248/76px, шапка80px; ниже768px Sidebar отсутствует в DOM, шапка72px,
  три разрешённых приоритетных раздела и «Ещё» только с остальными разрешёнными.
  Приоритеты и union ролей остаются в существующем App/navigation.
- Реальные имя/назначения и logout находятся в меню профиля. Единственная новая
  настройка — фактическое сворачивание меню в текущей вкладке; она применяется
  сразу, без неподдерживаемой записи имени или фиктивной кнопки сохранения.
  Предпочтение не оставляет cookie/localStorage и сбрасывается при выходе.
- Button/Avatar/Sidebar/DropdownMenu/Popover/Calendar/Field/ToggleGroup — одна
  система с общей семантической темой. Существующие native формы/Dialog сохраняют
  обработчики, NavigationBoundary и блокировки сохранений, используют те же токены.
- Участник открывается по имени в side sheet; точка статуса имеет доступное имя.
  После сохранения редактор закрывается, карточка остаётся открытой. Её закрытие
  сохраняет штатную семантику URL/detail и защищённых переходов.
- Владелец выбирает «Сотрудники сети» или «Тренеры филиала»; администратор получает
  свой прежний список тренеров. Для сети роли и филиалы объединяются по реальному
  id из существующих owner-authorized `/api/staff` и `/api/trainer-accounts`.
  Общий аккаунт показан один раз; назначение тренера и admin меняется своими
  штатными командами. Padding строк16×12px, аватар44px с fallback.
- Стрелки/дата в одной группе; гражданская дата не переводится в timezone браузера.
  На телефоне стрелки44px, на desktop32px по выбранному компактному эталону;
  desktop Sidebar toggle36px. Календарь доступен лениво, дни44px.
- Новые client-only компоненты не меняют backend, API-клиент, расчёты, SW/cache
  или NavigationBoundary. Tailwind4.3 поддерживает действующий Vite8; React19/TS7
  сохранены. Общий vendor и календарь разбиты на отдельные chunks.

## Проверка и границы

Браузерный RD-07 сценарий включён в существующий PWA harness с настоящими Go/PG
и synthetic fixture. Проверяет все9 разделов1280/900/390/320, геометрию Sidebar,
profile/settings/logout, name→card→editor, календарь/URL/фокус, отсутствие overflow
и отсутствие sidebar cookie. Существующие RD-01–05 и PWA assertions продолжают
проверять операции/права/CSRF/lifecycle/сеть. Изменены селекторы действий меню,
ожидания закрытия именно редактора (карточка теперь отдельный dialog) и снимок
контролов при responsive mount; серверные проверки не ослаблены.

Физическая установка и действия сотрудников не выполняются cloud Playwright.
Прежняя приёмка #188 и её ограничения не закрываются этой задачей. Результаты
новых запусков и снимки фиксируются отдельно в `docs/shadcn-runtime-validation.md`.

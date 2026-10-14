# RD-01: контракт оболочки и навигации

Реализация [#183](https://github.com/NikishGum/JudeOSLite/issues/183), 14.10.2026.
Источник визуальных решений: [Working Panel](../design-system/prototypes/dashboard/DESIGN.md),
[preview](../design-system/prototypes/dashboard/preview.html), [PRIDE](../DESIGN.md).
Главная пока содержит настоящий контекст и переходы. Метрики/ролевые главные — RD-05;
новые предметные представления — RD-02–RD-04. Прототип неизменён и не входит в runtime.

## URL

`frontend/src/app/navigation.ts` экспортирует `Section`, `Route`, `sections`, `readRoute`,
`routePath` и `allowedSections`. Никакого router dependency.

- `/` — главная сети/назначений. `/<section>` — раздел без выбранного рабочего филиала.
- `/clubs/:clubId/<section>` — раздел с текущим филиалом.
- Старый `/clubs/:clubId` открывает главную этого филиала; прямой URL обслуживается Go SPA.
- `/clubs/:clubId/participants?detail=:membershipId` — настоящая карточка участника.
  Используется **club_memberships.id**, не глобальный participantId. Сервер проверяет scope.
  Открытие/закрытие карточки создаёт history entry; reload/back/forward восстанавливают чтение.
  Создание новой карточки — локальная форма; URL появляется после успешного сохранения.
- Поддерживаемые sections: `home`, `sessions`, `participants`, `groups`, `journals`,
  `statistics`, `staff`, `clubs`, `history`. Неизвестный раздел, неверный detail или недоступный
  филиал показывают объяснение и переход на главную без предметных запросов.
- В RD-01 detail поддержан только для `participants`. RD-02/RD-03 добавляют реальные
  адаптеры group/session/journal и параметры фильтров в этой модели; не создавать фиктивное
  открытие detail до готового адаптера. Group/date/period — фильтры, никогда не права.
- History содержит только URL и числовой `prideIndex`, без персональных сведений/CSRF/черновиков.
  Обычный клик использует pushState, модифицированный клик сохраняет поведение ссылки.
  Back/forward, прямой URL, reload проверены в Chromium/WebKit. Native BFCache hit не измерен.

## Карта всех функций v1

| Раздел | Подключённые существующие компоненты и операции |
| --- | --- |
| Главная | Реальное имя пользователя/филиала, назначения, ссылки; нет синтетических счётчиков |
| Занятия | SessionCalendar: период/группа, once, перенос/замена/отмена; ScheduleManagement для owner/admin: четыре режима, preview, версии/генерация/история |
| Участники | ParticipantRegistry, ParticipantEditor, MembershipHistory: поиск/карточка/создание/правка/архив/возврат/начальная группа/датированные интервалы |
| Группы | GroupManagement: создание/имя/архив, состав на дату, поиск участника, включение/исключение/перевод/история |
| Журналы | TrainerSessions/SessionJournal/AttendanceCards для live trainer; AdminJournals/AdminJournal/RecordHistory для owner/admin; открытие/причины/сводка/close/correction/история |
| Статистика | OwnerDashboard: существующий read-only отчёт со всеми фильтрами/полями/формулой |
| Сотрудники | TrainerManagement в выбранном филиале; OwnerManagement mode=staff для администраторов сети и нескольких назначений |
| Филиалы | OwnerManagement mode=clubs: создание/редактирование/архив филиала |
| История | AuditHistory: разрешённый сервером публичный envelope, фильтры/пагинация |

Филиальные операции требуют конкретного доступного филиала. OwnerManagement mode=staff
остаётся сетевым даже при выбранном рабочем филиале; назначения перечислены в его UI.
Отчёт сохраняет собственный явный «Филиал отчёта»/период: текущий рабочий филиал не меняет
область серверного отчёта. RD-04 согласует drilldown/filter adapters с этой оболочкой.

## Права, lifecycle и переходы

`/api/auth/me` остаётся источником назначений; `GET /api/auth/clubs/:id` подтверждает
выбранный филиал. Owner видит сеть, administrator — управление выбранным филиалом,
trainer — занятия/свой журнал. Роли объединяются, без переключателя профиля.
Для истории достаточно хотя бы одного текущего admin-назначения, её фактический scope
ограничивает сервер. Меню при отсутствии выбранного филиала объединяет назначения;
конкретный филиал использует только свои роли. Owner mobile: главная/занятия/статистика,
admin: главная/занятия/участники, trainer-only: главная/занятия/журналы; остальные — «Ещё».

Смена club/user/любого назначения (включая другой филиал и owner-флаг) размонтирует старую feature-область и отменяет её GET через
существующие AbortController. Данные показываются только при совпадении проверенного
club с актуальным me; новый scope не получает прежнюю карточку. Отзыв прав имеет приоритет
над черновиком. Session-ended/session-changed и pagehide очищают персональный DOM; pageshow
заново читает me. CSRF/API generation и серверные проверки оставлены. Пароли, payload и
черновики не помещаются в history/storage/межвкладочные сообщения.

NavigationBoundary отслеживает изменённые формы (включая controlled-поля без name) в памяти.
Ссылки, смена филиала, выход, detail переходы и обновление PWA требуют подтверждения потери
черновика. Поиск type=search не считается предметным черновиком. Browser back сначала
возвращает текущую history-позицию, затем предлагает отмену/переход; reload/уход документа
защищены beforeunload. Закрытие/сброс формы её существующими кнопками остаётся явным действием.
Пока pendingMutations > 0, навигация не размонтирует сохраняющий компонент; пользователь
видит предложение дождаться ответа. Не отменять отправленный POST/PUT при навигации.

PwaStatus виден на любом разделе и экране входа. Service worker добавляет публичный
build-time shell fallback для новых URL, **не кеширует навигационные ответы или API**.
Версионированная allowlist статики, две сборки/ожидающее обновление и сохранение другой
вкладки остаются прежними. Offline queue нет.

## Общие файлы для следующих задач

- `app/App.tsx`, `app/navigation.ts`, `app/app.css`, `app/PwaStatus.tsx`,
  `shared/ui/*`, `shared/api.ts`, `shared/app-lifecycle.ts`, `service-worker.js` — общая граница.
  Согласовать изменения до параллельной работы RD-02–RD-04; не переписывать lifecycle/router.
- `shared/ui/Dialog.tsx`: native modal, desktop справа до 480px, mobile снизу до 90dvh,
  safe-area, focus/Tab/Shift+Tab/Escape/возврат, inert фон и блокировка прокрутки.
  `busy` запрещает закрытие; cancel type=button. Attendance ActionSheet использует тот же Dialog.
- `shared/ui/NavigationIcon.tsx`: только используемые inline SVG; декоративные, aria-hidden.
- PRIDE source tokens/fonts/logo неизменны. Локальные рекомендации Working Panel:
  `--panel-success #24664e`, `--panel-success-bg #edf6f1`, `--panel-warning #86520e`,
  `--panel-warning-bg #fff5e5`, `--panel-line #e5e8ec`, sidebar224/208, gap24,
  body14 desktop/16 mobile, heading30/26, mobile breakpoint760. Эти значения не новый бренд.
- Loading/error/retry/empty и формы сохранены в реальных feature-компонентах;
  нет универсальных предметных заготовок. RD-02–RD-04 меняют свои feature-каталоги;
  RD-05 использует опубликованные ими read/navigation adapters.

[Проверка RD-01](redesign-shell-validation.md) · [План](development-plan-redesign.md).

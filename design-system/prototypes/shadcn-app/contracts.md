# Контракты приложения и граница шаблона

Проверено по исходникам main `78c1b69`, 16.10.2026. Документы базы знаний описывают решения на свою дату; итоговое состояние реализации сверено с React и Go.

## Прочитанные источники

`PRODUCT.md`, `DESIGN.md`, `design-system/README.md`, токены/локальные шрифты и Working Panel; `docs/collaboration.md`, live #188, knowledge-base README/memory/crm-context; `docs/redesign-navigation.md`; типы и реализации frontend app/navigation, shared/api и предметных features.

## Контракты, используемые непосредственно

- `src/data.ts` импортирует `Section`, `sections`, `allowedSections` из `frontend/src/app/navigation.ts`; не создаёт параллельную карту прав разделов.
- `Role` и данные занятий используют типы `Panel`/`Session` из `features/dashboard/types.ts`.
- Карточки участников проверяются TypeScript по `Card` из `ParticipantEditor.tsx`, группы по `Group` из `features/groups/types.ts`. TS paths разрешают React types из локального package, поэтому установка production frontend для шаблона не требуется. Все runtime-данные локальные.

## Что необходимо сохранить при будущем переносе

| Область | Реализованный контракт | Граница шаблона |
| --- | --- | --- |
| Сессия и права | `/api/auth/me` — user, clubs/roles, csrfToken; `/api/auth/clubs/:id` подтверждает контекст. Owner/admin/trainer, объединение назначений, scope на сервере | Переключатель роли явно демонстрационный; запрещено переносить его как авторизацию |
| URL | `/`, `/<section>`, `/clubs/:clubId/<section>`; `?detail=` для карточки/группы, `?group=` и предметные query adapters занятий/журналов/статистики | Изолированные hash-маршруты, без конфликтов с production URL; детали локальные, при reload закрываются |
| Участники | ID детали — `club_memberships.id`, не глобальный participantId; полный birthDate, локальные контакты, joinedOn, archived, version, canEditIdentity | Реальные типы; синтетические имена и email на `.invalid`; карточка только для чтения |
| Группы | Состав на дату, датированные интервалы, перевод/исключение/архив, сохранённая история | Текущая демонстрационная проекция; без изменения состава |
| Расписание/занятия | Четыре серверных режима, preview/save/версии; planned/open/closed/cancelled; перенос/замена/отмена; local date/timezone | Список одного дня, фильтр и чтение детали. Календарный registry не заменяет алгоритм генерации Go |
| Журнал | Frozen roster при открытии; последовательные отметки; absent: unknown/illness/excused; close атомарно превращает неотмеченных в absent/unknown | Статусы и пояснение; полноценный flow находится в AttendanceCards/SessionJournal/AdminJournal |
| Коррекции | После закрытия — owner/admin, по одной отметке, обязательная причина и история | Только объяснение, без фиктивного сохранения |
| Отчёт | closed, planned, open, present, absent, unknown, illness, excused, percentage; отменённые исключены; present/(present+absent); null → «—» | В каждом из двух синтетических closed журналов 3 present/1 absent unknown: итого 6/8=75%; без закрытых — «—» |
| API | same-origin credentials, no-store, CSRF на mutations, ApiError code/message, abort GET, защита от смены auth generation | API не вызывается. При интеграции сохранить существующий shared/api, не заменять fetch-копиями |
| Lifecycle/PWA | Очистка персонального DOM, me revalidation, dirty-form navigation guard, pending mutations; SW не кеширует API и персональные ответы; offline queue нет | Отдельный шаблон не регистрирует service worker и не претендует на эти гарантии |

Финансы/iPay, Excel, медицинские справки и родительский кабинет в меню не добавлены. Контекст отчёта в реальном приложении имеет собственные явные фильтры; это следует сохранить при интеграции. Шаблон показывает только текущий демосрез, не заменяет фильтры отчёта.

Перенос Sidebar в runtime — самостоятельная задача после согласования общей границы app/shared/API и статуса issue; не выполнять замену оболочки по одному виду прототипа.

# Проверка шаблона

16.10.2026, Europe/Minsk. Область — только `design-system/prototypes/shadcn-app`, база main `78c1b69`. Это не приёмка приложения и не закрытие #188.

## Сборка

- Чистый `npm ci` → TypeScript → Vite production build: PASS.
- `npm audit --audit-level=high`: 0 vulnerabilities после удаления ненужного дерева CLI; исходные utilities сохранены с MIT.
- В dist/index.html сохранён контракт `pride-shadcn-template`.
- Браузер отдельно проверял production preview, не только dev. В новом preview-tab console errors/warnings: `[]`. В процессе замены зависимостей dev HMR временно сообщал об ошибке перезагрузки; это не ошибка итогового production запуска.

## Браузер

Codex in-app browser. Desktop 1280×720/800, mobile 390×844. На desktop/mobile `scrollWidth === innerWidth`; один main, вычисленный шрифт Rubik. Viewport override после проверки сброшен.

| Сценарий | Результат |
| --- | --- |
| Все девять разделов owner и подборка registries | PASS: переходы и заголовки |
| Администратор | PASS: нет Статистики/Филиалов; доступны реестр, группы, сотрудники, история |
| Тренер | PASS: Главная/Занятия/Журналы; предметные занятия ограничены demo-назначением |
| Смена филиала + поиск «Мария» | PASS: одна карточка выбранного филиала |
| Карточка участника / закрытие | PASS: правильный контекст, русская подпись Закрыть |
| Группа → занятия | PASS: выбран филиал группы и поиск «Младшая группа»; один подходящий сеанс |
| Мобильный Sidebar | PASS: Sheet, навигация закрывает меню, русские подписи и кнопка закрытия |
| Загрузка | PASS: aria-busy и Skeleton |
| Ошибка → повтор | PASS: Alert, восстановление demo-среза |
| Пустота → показать данные | PASS: Empty, восстановление |
| Адаптивные таблицы | PASS: подписанные значения и действие внутри экрана телефона |
| Контекст/URL | Hash-маршруты изолированы; browser back/forward предусмотрены обработчиками, отдельный ручной проход не заявлен |

Статистика демонстрационная: 2 closed × (3 present + 1 absent unknown) = 6/8 = 75%. Для филиала без closed — «—». Отменённое занятие не даёт отметок в отчёте. Нет API, форм mutation, авторизации или PWA; эти операции не тестировались как часть шаблона.

## Доступность и оформление

- Имена и подписи Sidebar/Sheet локализованы; overlay содержит Title/Description. Один main; skip-link и фокус заголовка раздела.
- Основные кнопки 44px; выбор/ошибки/статусы имеют текст; prefers-reduced-motion отключает переходы.
- Проверка основных пар CSS-токенов по WCAG-формуле: текст/фон14.37:1, вторичный/серый5.06:1, вторичный/белый5.43:1, белый/красный5.16:1, success6.17:1, warning6.03:1, active-nav6.10:1. Это численная проверка выбранных пар, не полный аудит каждого пикселя.
- Impeccable detector запущен один раз: `[]`, режим **DEGRADED** (нет htmlparser2/css-select/css-tree/domutils); computed contrast detector не оценивал. Отсутствие findings не трактуется как полный аудит.

## Ограниченная визуальная проверка

Две визуальные итерации. Исходный finish-reviewer: `fix` — мобильная таблица сети, английские подписи, технический ID в карточке. Единая batch исправила их. Первые дополнительные скриншоты Sheet снялись до завершения enter-анимации; reviewer запросил `recapture`. Повторные снимки проверены визуально в стабильном состоянии. Последний verdict: **ship, все три material fixes resolved**. Это оценка трёх исправлений, не новая полная приёмка.

Доказательства: [desktop](evidence/desktop.png), [mobile](evidence/mobile.png), [карточка](evidence/detail.png), [меню](evidence/menu.png). Рабочие копии reviewer расположены в `.impeccable/review/shadcn-app/` и не входят в task-коммит. Шаблон документирован отдельным DESIGN.md; глобальный PRIDE не переопределён.

Физические браузеры/устройства, скринридер и реальные API в этой задаче не проверялись. Production frontend и backend не изменялись; существующие проектные E2E/Go tests не запускались, поскольку изолированный шаблон не входит в их runtime.

## Уточнение по снимкам заказчика — 16.10.2026

Этот раздел заменяет прежний результат про мобильный Sidebar: навигационного Sidebar на телефоне больше нет. Первые desktop/mobile/menu/detail captures выше — доказательства первоначального коммита `4245b9b`; текущие изменения показаны в новых файлах ниже.

- ProfileMenu: настройки открываются на desktop и mobile; изменение имени на «Анна Демо» сохраняется в памяти и обновляет меню; видимость демоинструментов меняется после «Сохранить настройки», восстанавливается тем же меню. PASS.
- «Выйти из демо» показывает отдельный экран и размонтирует Workspace; «Открыть демо» начинает чистое состояние с исходным именем/ролью/контекстом. Это локальный демовыход, не API logout. PASS.
- Карточки участника/сотрудника открываются кнопкой с именем. Отдельная кнопка «Карточка» и колонка больших статусов убраны; точка имеет title/aria-label и не кодирует состояние только цветом. PASS.
- Staff AvatarFallback: вычислено display:flex, 44×44px, инициалы центрированы. Селектор подписей больше не затрагивает span аватара. Одна строка личности со списком её синтетических назначений. PASS.
- Mobile 390×844: sidebar DOM 0, document scrollWidth 390. Основные вкладки owner: Главная/Занятия/Статистика; admin: Главная/Занятия/Участники; trainer: Главная/Занятия/Журналы. Остальные разрешённые разделы в нижнем Sheet «Ещё», выбор закрывает его. PASS.
- shadcn Calendar/DayPicker: открывается по дате, русский месяц/неделя, Europe/Minsk; выбор 17 октября показывает пустоту и выбранную дату, «Демодень» возвращает 16 октября. Календарь загружается отдельным chunk; сетка планирования/правила расписания не имитируются. PASS.
- TypeScript/production build PASS; npm audit 0. Detector в этом проходе выполнен один раз: 43 advisory (42 про размеры вне неполного frontmatter ramp, 1 про радиус), 0 nonadvisory. Это не полный аудит браузера/скринридера.

Текущие captures: [desktop participants](evidence/desktop-participants.png), [desktop staff](evidence/desktop-staff.png), [mobile participants](evidence/mobile-participants.png), [mobile staff](evidence/mobile-staff.png), [More](evidence/mobile-more.png), [calendar](evidence/mobile-calendar.png), [settings](evidence/profile-settings.png). Все 7 открыты независимым reviewer и признаны валидными; визуальных material fixes нет. Единственный fix — синхронизировать локальные DESIGN.md/sidecar с обновлённым UI. Это уточнение локального шаблона, не изменение/приёмка production приложения.

Итоговый Verdict Pass: **persistence fix resolved, ship**; относится к синхронизации двух локальных документов. UI review отдельно подтвердил отсутствие материальных дефектов в изменённом scope. Итоговый production browser: русская «Навигация календаря», console error/warn `[]`; временный viewport override сброшен. Сборка разделена: основной JS ~459kB, календарный chunk ~86kB, оба до порога предупреждения Vite.

## Компоновка по замечаниям заказчика — 17.10.2026

Область: профиль desktop/mobile, ось context bar, сотрудники, шапка расписания. Независимая layout assessment и два механических layout scan до/после выполнены раздельно; оба scan вернули `[]`, что не заменяет полный аудит доступности.

- До исправления реальный ProfileMenu Trigger имел 32px вместо 64px; Popover Trigger даты 32px рядом со стрелками 44px. Причина подтверждена DOM: `asChild` меняет `data-slot`, поэтому прежние button-only selectors не применялись.
- После исправления профиль 64px desktop, mobile 44×44px, collapsed sidebar 44×44px; аватар целиком внутри hover/open/focus. Только одна focus outline. Настольный menu width 256px, мобильный не выходит за viewport.
- Элементы context bar имеют общую вертикальную середину 39.5px: toggle 36px, separator 24px с align-self:center, branch selector 44px. PASS.
- Staff row открывает карточку по клику; имя/роль/филиалы идут плотной вертикальной группой, avatar 44px выровнен с первым смысловым блоком. Полные назначения проверены в открытой карточке Елены Пример. PASS.
- Дата/стрелки стоят одной группой: 32px desktop, 44px mobile; заголовок не повторяет дату. «Следующий день» дал 17 октября, «К демодню» вернул 16 октября. PASS.
- На 390px поиск/статус рядом; на 320px оба 256px по ширине контента, на отдельных строках. Document width/scrollWidth 320/320. Нижняя навигация и профиль доступны, горизонтального overflow нет.
- Production TypeScript/Vite build PASS. Source palettes/fonts/lifecycle/API неизменны. Проверка выполнена в in-app browser; физические устройства и скринридер не заявлены.

Новые снимки: [desktop schedule](evidence/layout-desktop-sessions.png), [desktop profile](evidence/layout-desktop-profile.png), [desktop staff](evidence/layout-desktop-staff.png), [mobile schedule](evidence/layout-mobile-sessions.png), [mobile profile](evidence/layout-mobile-profile.png), [mobile staff](evidence/layout-mobile-staff.png), [320px](evidence/layout-mobile-320.png). Переходные captures с закрывающимся Sheet были заменены стабильными до передачи reviewer. Локальные DESIGN.md/sidecar синхронизированы; глобальная дизайн-система не переопределялась.

Итог независимого finish-review: **ship**, материальных дефектов в указанном объёме нет. Проверены все семь стабильных снимков и согласованность локальной документации. Оценка относится к уточнению существующей компоновки PRIDE; полный поведенческий аудит и аудит доступности в неё не входят.

### Внутренние отступы сотрудника — уточнение 17.10.2026

Новый снимок заказчика выявил отсутствие горизонтального inset в hover и разный вертикальный padding первой/последней строки. У всех записей теперь одинаковый padding 16px 12px; исключения по положению в списке удалены. На desktop вычисленный отступ аватара от края строки 12px, обе строки имеют padding 16px 12px. На 320px ширина строки 256px, document width/scrollWidth 320/320; филиалы переносятся. Production build, JSON и diff checks PASS; browser error/warn `[]`. Layout scan до изменения и автоматический hook после изменения без deterministic findings; это не полный аудит. Независимый finish-review двух снимков: **resolved / clear / ship** для отступов и переносов. Снимки: [desktop](evidence/staff-insets-desktop.png), [320px](evidence/staff-insets-mobile.png).

# Подходящие registries shadcn

Исследовано **16.10.2026**, Europe/Minsk. Источники: [официальный каталог](https://ui.shadcn.com/docs/directory), [машиночитаемый индекс](https://ui.shadcn.com/r/registries.json), первичные документации и фактический `shadcn search/view`. Индекс — датированный срез; health не гарантирует совместимость конкретного компонента. Сокращённый срез записан в `registry-snapshot.json`.

| Приоритет | Registry / подтверждённые items | Где полезен PRIDE | Ограничения / решение |
| --- | --- | --- | --- |
| Основа | [@shadcn](https://ui.shadcn.com/docs/components/sidebar): `sidebar`, `sidebar-07`, `table`, `sheet`, `select`, `calendar`, `empty` | Оболочка, карточки, реестр, выбор контекста, состояния | Официальные primitives уже установлены. Собрана предметная композиция на Sidebar, а не чужой общий dashboard с выручкой |
| Первый кандидат | [@reui](https://reui.io/docs/components/radix/data-grid): `data-grid`, `event-calendar`, `c-calendar-23` | Расширенный реестр, календарь занятий, локализация, фильтры, история | Есть варианты Radix/Base и Tailwind 4. [Components/examples бесплатны, Pro blocks/templates платные](https://reui.io/docs). Не покупать и не ставить весь набор. Зависимости/контракт CRUD проверять отдельно |
| Альтернатива реестра | [@tablecn](https://table-cn.vercel.app): `data-table`, `filter-builder` | Поиск, сортировка, серверная пагинация, URL-фильтры | Текущий data-table использует TanStack Table v9, @querycn/table-react/filter-react, dnd-kit. Намного тяжелее простой Table. URL-адаптер согласовать с существующей навигацией; inline editing не обходит version/права/аудит |
| Дополнение календаря | [@kibo-ui](https://www.kibo-ui.com/components/calendar): `calendar`, `mini-calendar`, `combobox` | Сетка месяца, выбор близкой даты, поиск группы/тренера | [Бесплатный open-source registry](https://www.kibo-ui.com). Calendar — представление, не механизм четырёх правил расписания. API и локаль адаптировать к Europe/Minsk |
| Резерв | [@diceui](https://diceui.com): `combobox`, `combobox-demo`, `combobox-tags-demo` | Доступный поиск и выбор группы/тренера, дополнительные формы | CLI search подтверждён. В индексе health degraded: последние две dry-run проверки не прошли. Документация компонентов через web вернула 526; не считать установку проверенной |

В каталог также входит `@shadcnblocks`, но на дату проверки отмечен degraded (валидация items), а для текущего шаблона официального Sidebar достаточно. Декоративные/AI-chat/платёжные registries не решают задачи v1.

## Воспроизводимый поиск и установка

Из каталога шаблона (npm — его package runner):

```sh
npx shadcn@latest search @shadcn -q sidebar --limit 4
npx shadcn@latest search @reui -q data-grid --limit 3
npx shadcn@latest search @reui -q event-calendar --limit 3
npx shadcn@latest search @tablecn --limit 4
npx shadcn@latest search @kibo-ui -q calendar --limit 4
npx shadcn@latest search @diceui -q combobox --limit 3
```

Команды для последующего точечного добавления, **не выполнены**:

```sh
npx shadcn@latest add @reui/data-grid
npx shadcn@latest add @reui/event-calendar
npx shadcn@latest add @tablecn/data-table
npx shadcn@latest add @kibo-ui/calendar
npx shadcn@latest add @diceui/combobox
```

Имена/URL взяты из текущего индекса, а не придуманы. Публичные namespace доступны CLI без ручного добавления в components.json. В шаблоне оставлен официальный registry: сторонние пакеты не установлены. `view @tablecn/data-table` подтверждает описание и зависимости, но не является install/build проверкой.

Рекомендация: сейчас официальный shadcn; при усложнении реестра выбрать **один** из ReUI Data Grid / tablecn; для календаря сначала оценить ReUI Event Calendar. Не дублировать в runtime несколько таблиц/календарных систем. При переносе проверить русский текст, клавиатуру, телефон, server pagination, timezone и существующие права/версии. Рекомендации не являются утверждённым решением миграции.

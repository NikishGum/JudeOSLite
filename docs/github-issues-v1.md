# GitHub Issues: перенос плана v1

Проверено: 2026-10-11T20:53:33Z. Репозиторий: [NikishGum/JudeOSLite](https://github.com/NikishGum/JudeOSLite/issues).

Источник — [план реализации](development-plan-v1.md), [неизменяемая версия `6fb734e3e3713f713741175f7ac587bb9fbf7699`](https://github.com/NikishGum/JudeOSLite/blob/6fb734e3e3713f713741175f7ac587bb9fbf7699/docs/development-plan-v1.md). SHA-256 исходного файла: `a3c6b85ec542adf4ce0a71f68bd2c34a350ebd22fab47ded7318160c0da3b965`.

По поручению заказчика от 11.10.2026 перенесены **все 148 задач** из 8 разделов; каждая карточка стала отдельной issue. Названия, 12 полей каждой карточки, формулировки критериев/проверок, открытые условия и все прямые зависимости сохранены. Содержание плана не сокращалось и не переопределялось.

В тексте карточек изменены только адреса Markdown-ссылок: обязательные зависимости ведут на соответствующие issues; документы, передача результата и общий контекст — на исходную версию файлов в GitHub. Каждая issue содержит ссылку на свою исходную карточку и общие разделы порядка реализации, открытых условий, инженерных уточнений, правил исполнителя, покрытия/рисков и контрольных точек.

**358 прямых зависимостей** также записаны штатными отношениями GitHub `Blocked by` / `Blocking`. Связи перенесены из поля «Зависимости»; упоминания в «Контексте» и «Передаче дальше» не превращались в дополнительные блокеры. Прямые связи не заменялись транзитивными.

Issues создавались после своих предшественников, поэтому номера GitHub не обязаны совпадать с порядком карточек в документе. Ниже реестр приведён в исходном порядке плана. Стабильный ключ — ID плана. Исполнители не назначены, все задачи открыты; перенос не является началом разработки или выполнением согласований.

Статус выполнения вести в GitHub. Этот реестр фиксирует перенос и проверку на указанную дату; он не дублирует текущую доску статусов. Последующие изменения требований нужно согласовывать и отражать в документах и соответствующих issues.

## Проверка переноса

- Получены обратно из GitHub и проверены 148 заголовков и полные тела issues; текст всех 1 776 полей совпадает с исходными карточками при нормализации только адресов ссылок.
- Проверены все 358 прямых связей `Blocked by` и их обратное направление `Blocking`: нет пропусков, добавленных зависимостей или перепутанного направления.
- ID уникальны, исходный граф ацикличен; дублирующихся или отсутствующих задач нет.
- Ветвь первого рабочего сценария `QA-03` содержит 53 задачи вместе с самой проверкой. В таблицах она отмечена «да»; остальные 95 задач также перенесены.
- Исходный план оставлен без изменений. Это проверка переноса, а не приёмка приложения.

[Машиночитаемый реестр](github-issues-v1.json) содержит соответствия, исходные зависимости и SHA-256 каждой исходной карточки и созданного тела issue для последующей сверки. Хеши тел фиксируют состояние на момент переноса, а не запрещают последующее согласованное редактирование.

## Общий контекст, сохранённый в источнике

- [Порядок реализации и первый результат](development-plan-v1.md#delivery).
- [Открытые условия B01–B10](development-plan-v1.md#conditions).
- [Расхождения и инженерные уточнения](development-plan-v1.md#differences).
- [Общие правила исполнителя](development-plan-v1.md#context).
- [Матрицы требований, предложений, таблиц и рисков](development-plan-v1.md#coverage).
- [Практический порядок и контрольные точки](development-plan-v1.md#acceptance).

**Обязательный возврат к старым долгам и переходу к оплатам через CRM:** [DEC-08](https://github.com/NikishGum/JudeOSLite/issues/8) и её исходные зависимости сохранены; реальное подключение не разрешено самим созданием issues.

## E0 — 9 задач

| ID плана | Issue | Первый сценарий QA-03 | Прямые зависимости |
| --- | --- | --- | --- |
| DEC-01 | [#1 — Зафиксировать минимальный технический профиль](https://github.com/NikishGum/JudeOSLite/issues/1) | да | Нет |
| DEC-02 | [#2 — Описать контракт отметки и завершения](https://github.com/NikishGum/JudeOSLite/issues/2) | да | Нет |
| DEC-03 | [#3 — Уточнить пределы работы с общими аккаунтами](https://github.com/NikishGum/JudeOSLite/issues/3) | да | Нет |
| DEC-04 | [#4 — Описать контракт месячного начисления](https://github.com/NikishGum/JudeOSLite/issues/4) | — | Нет |
| DEC-05 | [#5 — Согласовать формулы обзора владельца](https://github.com/NikishGum/JudeOSLite/issues/5) | — | Нет |
| DEC-06 | [#6 — Согласовать маппинг реального Excel](https://github.com/NikishGum/JudeOSLite/issues/6) | — | Нет |
| DEC-07 | [#7 — Получить договорный комплект iPay](https://github.com/NikishGum/JudeOSLite/issues/7) | — | Нет |
| DEC-08 | [#8 — Вернуться к старым долгам перед платёжными тестами](https://github.com/NikishGum/JudeOSLite/issues/8) | — | [DEC-04](https://github.com/NikishGum/JudeOSLite/issues/4), [DEC-06](https://github.com/NikishGum/JudeOSLite/issues/6), [DEC-07](https://github.com/NikishGum/JudeOSLite/issues/7) |
| DEC-09 | [#9 — Согласовать обработку финансовых исключений](https://github.com/NikishGum/JudeOSLite/issues/9) | — | [DEC-04](https://github.com/NikishGum/JudeOSLite/issues/4) |

## E1 — 24 задачи

| ID плана | Issue | Первый сценарий QA-03 | Прямые зависимости |
| --- | --- | --- | --- |
| BASE-01 | [#10 — Запустить минимальный Go-сервис](https://github.com/NikishGum/JudeOSLite/issues/10) | да | [DEC-01](https://github.com/NikishGum/JudeOSLite/issues/1) |
| BASE-02 | [#11 — Собрать React-оболочку PRIDE](https://github.com/NikishGum/JudeOSLite/issues/11) | да | [DEC-01](https://github.com/NikishGum/JudeOSLite/issues/1) |
| BASE-03 | [#12 — Добавить последовательный runner миграций](https://github.com/NikishGum/JudeOSLite/issues/12) | да | [BASE-01](https://github.com/NikishGum/JudeOSLite/issues/10) |
| BASE-04 | [#13 — Подготовить изолированную тестовую PostgreSQL](https://github.com/NikishGum/JudeOSLite/issues/13) | да | [BASE-03](https://github.com/NikishGum/JudeOSLite/issues/12) |
| BASE-05 | [#14 — Определить единый JSON-контракт ошибок](https://github.com/NikishGum/JudeOSLite/issues/14) | да | [BASE-01](https://github.com/NikishGum/JudeOSLite/issues/10) |
| BASE-06 | [#15 — Добавить минимальный транзакционный аудит](https://github.com/NikishGum/JudeOSLite/issues/15) | да | [BASE-03](https://github.com/NikishGum/JudeOSLite/issues/12), [BASE-04](https://github.com/NikishGum/JudeOSLite/issues/13) |
| AUTH-01 | [#16 — Создать модель клубов и назначений сотрудников](https://github.com/NikishGum/JudeOSLite/issues/16) | да | [BASE-03](https://github.com/NikishGum/JudeOSLite/issues/12), [BASE-04](https://github.com/NikishGum/JudeOSLite/issues/13), [DEC-03](https://github.com/NikishGum/JudeOSLite/issues/3) |
| AUTH-02 | [#17 — Создать хранение серверных сессий](https://github.com/NikishGum/JudeOSLite/issues/17) | да | [AUTH-01](https://github.com/NikishGum/JudeOSLite/issues/16) |
| AUTH-03 | [#18 — Создать начального владельца безопасной командой](https://github.com/NikishGum/JudeOSLite/issues/18) | да | [AUTH-01](https://github.com/NikishGum/JudeOSLite/issues/16) |
| AUTH-04 | [#19 — Реализовать операцию входа](https://github.com/NikishGum/JudeOSLite/issues/19) | да | [AUTH-02](https://github.com/NikishGum/JudeOSLite/issues/17), [AUTH-03](https://github.com/NikishGum/JudeOSLite/issues/18), [BASE-05](https://github.com/NikishGum/JudeOSLite/issues/14) |
| AUTH-05 | [#20 — Реализовать операцию выхода](https://github.com/NikishGum/JudeOSLite/issues/20) | да | [AUTH-04](https://github.com/NikishGum/JudeOSLite/issues/19) |
| AUTH-06 | [#21 — Реализовать чтение текущего сотрудника](https://github.com/NikishGum/JudeOSLite/issues/21) | да | [AUTH-04](https://github.com/NikishGum/JudeOSLite/issues/19) |
| AUTH-07 | [#22 — Ограничить ресурсы клубом и группой на сервере](https://github.com/NikishGum/JudeOSLite/issues/22) | да | [AUTH-06](https://github.com/NikishGum/JudeOSLite/issues/21), [BASE-05](https://github.com/NikishGum/JudeOSLite/issues/14) |
| AUTH-08 | [#23 — Реализовать создание сотрудника](https://github.com/NikishGum/JudeOSLite/issues/23) | да | [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15), [DEC-03](https://github.com/NikishGum/JudeOSLite/issues/3) |
| AUTH-09 | [#24 — Реализовать выдачу клубного назначения](https://github.com/NikishGum/JudeOSLite/issues/24) | да | [AUTH-08](https://github.com/NikishGum/JudeOSLite/issues/23) |
| AUTH-10 | [#25 — Реализовать локальный отзыв назначения](https://github.com/NikishGum/JudeOSLite/issues/25) | — | [AUTH-09](https://github.com/NikishGum/JudeOSLite/issues/24), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15) |
| AUTH-11 | [#26 — Реализовать отключение общего аккаунта владельцем](https://github.com/NikishGum/JudeOSLite/issues/26) | — | [AUTH-10](https://github.com/NikishGum/JudeOSLite/issues/25), [DEC-03](https://github.com/NikishGum/JudeOSLite/issues/3) |
| AUTH-12 | [#27 — Сделать экран входа и выхода](https://github.com/NikishGum/JudeOSLite/issues/27) | да | [BASE-02](https://github.com/NikishGum/JudeOSLite/issues/11), [AUTH-04](https://github.com/NikishGum/JudeOSLite/issues/19), [AUTH-05](https://github.com/NikishGum/JudeOSLite/issues/20), [AUTH-06](https://github.com/NikishGum/JudeOSLite/issues/21) |
| AUTH-13 | [#28 — Сделать форму создания сотрудника](https://github.com/NikishGum/JudeOSLite/issues/28) | да | [AUTH-08](https://github.com/NikishGum/JudeOSLite/issues/23), [AUTH-12](https://github.com/NikishGum/JudeOSLite/issues/27) |
| AUTH-14 | [#29 — Сделать редактор клубных назначений владельца](https://github.com/NikishGum/JudeOSLite/issues/29) | — | [AUTH-09](https://github.com/NikishGum/JudeOSLite/issues/24), [AUTH-10](https://github.com/NikishGum/JudeOSLite/issues/25), [AUTH-11](https://github.com/NikishGum/JudeOSLite/issues/26), [AUTH-12](https://github.com/NikishGum/JudeOSLite/issues/27) |
| OPS-01 | [#30 — Подготовить Compose и HTTPS на VPS](https://github.com/NikishGum/JudeOSLite/issues/30) | да | [BASE-01](https://github.com/NikishGum/JudeOSLite/issues/10), [BASE-03](https://github.com/NikishGum/JudeOSLite/issues/12), [AUTH-04](https://github.com/NikishGum/JudeOSLite/issues/19), [BASE-02](https://github.com/NikishGum/JudeOSLite/issues/11) |
| CLUB-01 | [#31 — Реализовать создание клуба владельцем](https://github.com/NikishGum/JudeOSLite/issues/31) | да | [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15) |
| CLUB-02 | [#32 — Реализовать изменение метаданных клуба](https://github.com/NikishGum/JudeOSLite/issues/32) | да | [CLUB-01](https://github.com/NikishGum/JudeOSLite/issues/31) |
| CLUB-03 | [#33 — Сделать форму метаданных клуба владельца](https://github.com/NikishGum/JudeOSLite/issues/33) | да | [CLUB-01](https://github.com/NikishGum/JudeOSLite/issues/31), [CLUB-02](https://github.com/NikishGum/JudeOSLite/issues/32), [AUTH-12](https://github.com/NikishGum/JudeOSLite/issues/27) |

## E2 — 22 задачи

| ID плана | Issue | Первый сценарий QA-03 | Прямые зависимости |
| --- | --- | --- | --- |
| MEM-01 | [#34 — Создать таблицы идентичности и принадлежности клубу](https://github.com/NikishGum/JudeOSLite/issues/34) | да | [AUTH-01](https://github.com/NikishGum/JudeOSLite/issues/16), [BASE-04](https://github.com/NikishGum/JudeOSLite/issues/13) |
| GRP-01 | [#35 — Создать таблицы групп и датированного состава](https://github.com/NikishGum/JudeOSLite/issues/35) | да | [MEM-01](https://github.com/NikishGum/JudeOSLite/issues/34), [BASE-04](https://github.com/NikishGum/JudeOSLite/issues/13) |
| MEM-02 | [#36 — Реализовать поиск участников клуба](https://github.com/NikishGum/JudeOSLite/issues/36) | да | [MEM-01](https://github.com/NikishGum/JudeOSLite/issues/34), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22) |
| MEM-03 | [#37 — Реализовать чтение одной карточки](https://github.com/NikishGum/JudeOSLite/issues/37) | да | [MEM-02](https://github.com/NikishGum/JudeOSLite/issues/36) |
| MEM-04 | [#38 — Реализовать создание участника с начальной группой](https://github.com/NikishGum/JudeOSLite/issues/38) | да | [MEM-01](https://github.com/NikishGum/JudeOSLite/issues/34), [GRP-01](https://github.com/NikishGum/JudeOSLite/issues/35), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15) |
| MEM-05 | [#39 — Реализовать изменение карточки](https://github.com/NikishGum/JudeOSLite/issues/39) | — | [MEM-03](https://github.com/NikishGum/JudeOSLite/issues/37), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15), [DEC-03](https://github.com/NikishGum/JudeOSLite/issues/3) |
| MEM-06 | [#40 — Реализовать архивирование принадлежности](https://github.com/NikishGum/JudeOSLite/issues/40) | — | [MEM-05](https://github.com/NikishGum/JudeOSLite/issues/39) |
| MEM-07 | [#41 — Сделать список и чтение карточки участника](https://github.com/NikishGum/JudeOSLite/issues/41) | да | [MEM-02](https://github.com/NikishGum/JudeOSLite/issues/36), [MEM-03](https://github.com/NikishGum/JudeOSLite/issues/37), [AUTH-12](https://github.com/NikishGum/JudeOSLite/issues/27) |
| MEM-08 | [#42 — Сделать форму нового участника](https://github.com/NikishGum/JudeOSLite/issues/42) | да | [MEM-04](https://github.com/NikishGum/JudeOSLite/issues/38), [MEM-07](https://github.com/NikishGum/JudeOSLite/issues/41) |
| MEM-09 | [#43 — Сделать форму изменения карточки](https://github.com/NikishGum/JudeOSLite/issues/43) | — | [MEM-05](https://github.com/NikishGum/JudeOSLite/issues/39), [MEM-07](https://github.com/NikishGum/JudeOSLite/issues/41) |
| MEM-10 | [#44 — Сделать действие архивирования карточки](https://github.com/NikishGum/JudeOSLite/issues/44) | — | [MEM-06](https://github.com/NikishGum/JudeOSLite/issues/40), [MEM-07](https://github.com/NikishGum/JudeOSLite/issues/41) |
| GRP-02 | [#45 — Реализовать чтение групп и состава](https://github.com/NikishGum/JudeOSLite/issues/45) | да | [GRP-01](https://github.com/NikishGum/JudeOSLite/issues/35), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22) |
| GRP-03 | [#46 — Реализовать создание группы](https://github.com/NikishGum/JudeOSLite/issues/46) | да | [GRP-01](https://github.com/NikishGum/JudeOSLite/issues/35), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15) |
| GRP-04 | [#47 — Реализовать изменение названия группы](https://github.com/NikishGum/JudeOSLite/issues/47) | да | [GRP-03](https://github.com/NikishGum/JudeOSLite/issues/46) |
| GRP-05 | [#48 — Реализовать архивирование группы](https://github.com/NikishGum/JudeOSLite/issues/48) | — | [GRP-04](https://github.com/NikishGum/JudeOSLite/issues/47) |
| GRP-06 | [#49 — Реализовать назначение тренера группе](https://github.com/NikishGum/JudeOSLite/issues/49) | да | [GRP-02](https://github.com/NikishGum/JudeOSLite/issues/45), [AUTH-09](https://github.com/NikishGum/JudeOSLite/issues/24), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15) |
| MEM-11 | [#50 — Реализовать перевод участника с даты](https://github.com/NikishGum/JudeOSLite/issues/50) | — | [MEM-04](https://github.com/NikishGum/JudeOSLite/issues/38), [GRP-02](https://github.com/NikishGum/JudeOSLite/issues/45), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15) |
| GRP-07 | [#51 — Сделать форму группы и её названия](https://github.com/NikishGum/JudeOSLite/issues/51) | да | [GRP-02](https://github.com/NikishGum/JudeOSLite/issues/45), [GRP-03](https://github.com/NikishGum/JudeOSLite/issues/46), [GRP-04](https://github.com/NikishGum/JudeOSLite/issues/47), [AUTH-12](https://github.com/NikishGum/JudeOSLite/issues/27) |
| GRP-08 | [#52 — Сделать форму назначения тренера группе](https://github.com/NikishGum/JudeOSLite/issues/52) | да | [GRP-02](https://github.com/NikishGum/JudeOSLite/issues/45), [GRP-06](https://github.com/NikishGum/JudeOSLite/issues/49), [GRP-07](https://github.com/NikishGum/JudeOSLite/issues/51) |
| GRP-09 | [#53 — Сделать форму перевода участника](https://github.com/NikishGum/JudeOSLite/issues/53) | — | [MEM-11](https://github.com/NikishGum/JudeOSLite/issues/50), [GRP-02](https://github.com/NikishGum/JudeOSLite/issues/45), [MEM-07](https://github.com/NikishGum/JudeOSLite/issues/41) |
| GRP-10 | [#54 — Сделать действие архивирования группы](https://github.com/NikishGum/JudeOSLite/issues/54) | — | [GRP-05](https://github.com/NikishGum/JudeOSLite/issues/48), [GRP-07](https://github.com/NikishGum/JudeOSLite/issues/51) |
| UI-01 | [#55 — Добавить выбор текущего клуба](https://github.com/NikishGum/JudeOSLite/issues/55) | да | [AUTH-06](https://github.com/NikishGum/JudeOSLite/issues/21), [AUTH-12](https://github.com/NikishGum/JudeOSLite/issues/27) |

## E3 — 25 задач

| ID плана | Issue | Первый сценарий QA-03 | Прямые зависимости |
| --- | --- | --- | --- |
| TRAIN-01 | [#56 — Создать правила расписания и конкретные занятия](https://github.com/NikishGum/JudeOSLite/issues/56) | да | [GRP-01](https://github.com/NikishGum/JudeOSLite/issues/35), [BASE-04](https://github.com/NikishGum/JudeOSLite/issues/13) |
| TRAIN-02 | [#57 — Реализовать создание регулярного правила](https://github.com/NikishGum/JudeOSLite/issues/57) | да | [TRAIN-01](https://github.com/NikishGum/JudeOSLite/issues/56), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15), [GRP-06](https://github.com/NikishGum/JudeOSLite/issues/49) |
| TRAIN-03 | [#58 — Реализовать изменение расписания с даты](https://github.com/NikishGum/JudeOSLite/issues/58) | да | [TRAIN-02](https://github.com/NikishGum/JudeOSLite/issues/57) |
| TRAIN-04 | [#59 — Добавить повторяемую генерацию занятий](https://github.com/NikishGum/JudeOSLite/issues/59) | да | [TRAIN-01](https://github.com/NikishGum/JudeOSLite/issues/56), [TRAIN-02](https://github.com/NikishGum/JudeOSLite/issues/57) |
| TRAIN-05 | [#63 — Проверить генерацию после ручных исключений](https://github.com/NikishGum/JudeOSLite/issues/63) | — | [TRAIN-03](https://github.com/NikishGum/JudeOSLite/issues/58), [TRAIN-04](https://github.com/NikishGum/JudeOSLite/issues/59), [TRAIN-07](https://github.com/NikishGum/JudeOSLite/issues/61), [TRAIN-08](https://github.com/NikishGum/JudeOSLite/issues/62) |
| TRAIN-06 | [#60 — Сделать редактор регулярного расписания](https://github.com/NikishGum/JudeOSLite/issues/60) | да | [TRAIN-02](https://github.com/NikishGum/JudeOSLite/issues/57), [TRAIN-03](https://github.com/NikishGum/JudeOSLite/issues/58), [GRP-07](https://github.com/NikishGum/JudeOSLite/issues/51) |
| TRAIN-07 | [#61 — Реализовать отмену занятия](https://github.com/NikishGum/JudeOSLite/issues/61) | — | [TRAIN-04](https://github.com/NikishGum/JudeOSLite/issues/59), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15) |
| TRAIN-08 | [#62 — Реализовать замену фактического тренера](https://github.com/NikishGum/JudeOSLite/issues/62) | — | [TRAIN-04](https://github.com/NikishGum/JudeOSLite/issues/59), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15) |
| TRAIN-09 | [#66 — Сделать действие отмены занятия](https://github.com/NikishGum/JudeOSLite/issues/66) | — | [TRAIN-07](https://github.com/NikishGum/JudeOSLite/issues/61), [TRAIN-12](https://github.com/NikishGum/JudeOSLite/issues/65) |
| TRAIN-10 | [#67 — Сделать форму замены тренера](https://github.com/NikishGum/JudeOSLite/issues/67) | — | [TRAIN-08](https://github.com/NikishGum/JudeOSLite/issues/62), [TRAIN-12](https://github.com/NikishGum/JudeOSLite/issues/65) |
| TRAIN-11 | [#64 — Реализовать чтение сегодняшних занятий](https://github.com/NikishGum/JudeOSLite/issues/64) | да | [TRAIN-04](https://github.com/NikishGum/JudeOSLite/issues/59), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22) |
| TRAIN-12 | [#65 — Сделать экран сегодняшних занятий](https://github.com/NikishGum/JudeOSLite/issues/65) | да | [TRAIN-11](https://github.com/NikishGum/JudeOSLite/issues/64), [AUTH-12](https://github.com/NikishGum/JudeOSLite/issues/27), [UI-01](https://github.com/NikishGum/JudeOSLite/issues/55) |
| ATT-01 | [#68 — Создать снимок состава и причины отсутствия](https://github.com/NikishGum/JudeOSLite/issues/68) | да | [TRAIN-01](https://github.com/NikishGum/JudeOSLite/issues/56), [DEC-02](https://github.com/NikishGum/JudeOSLite/issues/2), [BASE-04](https://github.com/NikishGum/JudeOSLite/issues/13) |
| ATT-02 | [#69 — Реализовать открытие занятия со снимком](https://github.com/NikishGum/JudeOSLite/issues/69) | да | [ATT-01](https://github.com/NikishGum/JudeOSLite/issues/68), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15) |
| ATT-03 | [#70 — Реализовать одну текущую отметку](https://github.com/NikishGum/JudeOSLite/issues/70) | да | [ATT-02](https://github.com/NikishGum/JudeOSLite/issues/69), [DEC-02](https://github.com/NikishGum/JudeOSLite/issues/2) |
| ATT-04 | [#71 — Реализовать атомарное завершение журнала](https://github.com/NikishGum/JudeOSLite/issues/71) | да | [ATT-03](https://github.com/NikishGum/JudeOSLite/issues/70), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15) |
| ATT-05 | [#72 — Реализовать исправление закрытой отметки](https://github.com/NikishGum/JudeOSLite/issues/72) | — | [ATT-04](https://github.com/NikishGum/JudeOSLite/issues/71), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15) |
| ATT-06 | [#73 — Сделать последовательные карточки участников](https://github.com/NikishGum/JudeOSLite/issues/73) | да | [ATT-02](https://github.com/NikishGum/JudeOSLite/issues/69), [ATT-03](https://github.com/NikishGum/JudeOSLite/issues/70), [TRAIN-12](https://github.com/NikishGum/JudeOSLite/issues/65) |
| ATT-07 | [#74 — Сделать нижнюю панель причины отсутствия](https://github.com/NikishGum/JudeOSLite/issues/74) | да | [ATT-06](https://github.com/NikishGum/JudeOSLite/issues/73), [DEC-02](https://github.com/NikishGum/JudeOSLite/issues/2) |
| ATT-08 | [#75 — Сделать просмотр и завершение результатов](https://github.com/NikishGum/JudeOSLite/issues/75) | да | [ATT-04](https://github.com/NikishGum/JudeOSLite/issues/71), [ATT-06](https://github.com/NikishGum/JudeOSLite/issues/73), [ATT-07](https://github.com/NikishGum/JudeOSLite/issues/74) |
| ATT-09 | [#76 — Реализовать чтение журнала за период](https://github.com/NikishGum/JudeOSLite/issues/76) | — | [ATT-04](https://github.com/NikishGum/JudeOSLite/issues/71), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22) |
| ATT-10 | [#77 — Сделать журнал для ПК и телефона](https://github.com/NikishGum/JudeOSLite/issues/77) | — | [ATT-09](https://github.com/NikishGum/JudeOSLite/issues/76), [BASE-02](https://github.com/NikishGum/JudeOSLite/issues/11) |
| ATT-11 | [#78 — Сделать форму административной правки](https://github.com/NikishGum/JudeOSLite/issues/78) | — | [ATT-05](https://github.com/NikishGum/JudeOSLite/issues/72), [ATT-10](https://github.com/NikishGum/JudeOSLite/issues/77) |
| ATT-12 | [#79 — Проверить историю и гонку завершения](https://github.com/NikishGum/JudeOSLite/issues/79) | — | [ATT-05](https://github.com/NikishGum/JudeOSLite/issues/72), [TRAIN-05](https://github.com/NikishGum/JudeOSLite/issues/63), [MEM-11](https://github.com/NikishGum/JudeOSLite/issues/50) |
| QA-03 | [#80 — Принять первый рабочий сценарий тренера](https://github.com/NikishGum/JudeOSLite/issues/80) | да | [ATT-08](https://github.com/NikishGum/JudeOSLite/issues/75), [MEM-08](https://github.com/NikishGum/JudeOSLite/issues/42), [GRP-08](https://github.com/NikishGum/JudeOSLite/issues/52), [TRAIN-06](https://github.com/NikishGum/JudeOSLite/issues/60), [AUTH-13](https://github.com/NikishGum/JudeOSLite/issues/28), [CLUB-03](https://github.com/NikishGum/JudeOSLite/issues/33), [OPS-01](https://github.com/NikishGum/JudeOSLite/issues/30) |

## E4 — 17 задач

| ID плана | Issue | Первый сценарий QA-03 | Прямые зависимости |
| --- | --- | --- | --- |
| MED-01 | [#81 — Создать медицинские периоды с неизвестным окончанием](https://github.com/NikishGum/JudeOSLite/issues/81) | — | [MEM-01](https://github.com/NikishGum/JudeOSLite/issues/34), [BASE-04](https://github.com/NikishGum/JudeOSLite/issues/13) |
| MED-02 | [#82 — Реализовать регистрацию справки](https://github.com/NikishGum/JudeOSLite/issues/82) | — | [MED-01](https://github.com/NikishGum/JudeOSLite/issues/81), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15) |
| MED-03 | [#83 — Реализовать исправление срока справки](https://github.com/NikishGum/JudeOSLite/issues/83) | — | [MED-02](https://github.com/NikishGum/JudeOSLite/issues/82) |
| MED-04 | [#84 — Реализовать отзыв ошибочной справки](https://github.com/NikishGum/JudeOSLite/issues/84) | — | [MED-03](https://github.com/NikishGum/JudeOSLite/issues/83) |
| MED-05 | [#85 — Реализовать чтение справок участника](https://github.com/NikishGum/JudeOSLite/issues/85) | — | [MED-02](https://github.com/NikishGum/JudeOSLite/issues/82), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22) |
| MED-06 | [#86 — Сделать форму периода справки](https://github.com/NikishGum/JudeOSLite/issues/86) | — | [MED-02](https://github.com/NikishGum/JudeOSLite/issues/82), [MED-03](https://github.com/NikishGum/JudeOSLite/issues/83), [MED-05](https://github.com/NikishGum/JudeOSLite/issues/85), [MEM-07](https://github.com/NikishGum/JudeOSLite/issues/41) |
| MED-07 | [#87 — Показать справки в карточке и занятии](https://github.com/NikishGum/JudeOSLite/issues/87) | — | [MED-05](https://github.com/NikishGum/JudeOSLite/issues/85), [MED-04](https://github.com/NikishGum/JudeOSLite/issues/84), [ATT-06](https://github.com/NikishGum/JudeOSLite/issues/73) |
| IMPORT-01 | [#88 — Создать таблицы batch и проверенных строк](https://github.com/NikishGum/JudeOSLite/issues/88) | — | [MEM-01](https://github.com/NikishGum/JudeOSLite/issues/34), [GRP-01](https://github.com/NikishGum/JudeOSLite/issues/35), [BASE-04](https://github.com/NikishGum/JudeOSLite/issues/13) |
| IMPORT-02 | [#89 — Прочитать согласованный Excel в batch](https://github.com/NikishGum/JudeOSLite/issues/89) | — | [IMPORT-01](https://github.com/NikishGum/JudeOSLite/issues/88), [DEC-06](https://github.com/NikishGum/JudeOSLite/issues/6) |
| IMPORT-03 | [#90 — Нормализовать поля импортной карточки](https://github.com/NikishGum/JudeOSLite/issues/90) | — | [IMPORT-02](https://github.com/NikishGum/JudeOSLite/issues/89) |
| IMPORT-04 | [#91 — Проверить ошибки импортной строки](https://github.com/NikishGum/JudeOSLite/issues/91) | — | [IMPORT-03](https://github.com/NikishGum/JudeOSLite/issues/90) |
| IMPORT-05 | [#92 — Реализовать ручное сопоставление совпадений](https://github.com/NikishGum/JudeOSLite/issues/92) | — | [IMPORT-04](https://github.com/NikishGum/JudeOSLite/issues/91), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22), [MEM-02](https://github.com/NikishGum/JudeOSLite/issues/36) |
| IMPORT-06 | [#93 — Реализовать чтение предпросмотра batch](https://github.com/NikishGum/JudeOSLite/issues/93) | — | [IMPORT-05](https://github.com/NikishGum/JudeOSLite/issues/92) |
| IMPORT-07 | [#94 — Сделать экран предпросмотра и сопоставления](https://github.com/NikishGum/JudeOSLite/issues/94) | — | [IMPORT-06](https://github.com/NikishGum/JudeOSLite/issues/93), [IMPORT-05](https://github.com/NikishGum/JudeOSLite/issues/92), [AUTH-12](https://github.com/NikishGum/JudeOSLite/issues/27) |
| IMPORT-08 | [#95 — Реализовать атомарное применение batch](https://github.com/NikishGum/JudeOSLite/issues/95) | — | [IMPORT-06](https://github.com/NikishGum/JudeOSLite/issues/93), [MEM-04](https://github.com/NikishGum/JudeOSLite/issues/38), [MEM-11](https://github.com/NikishGum/JudeOSLite/issues/50), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15) |
| IMPORT-09 | [#96 — Сделать подтверждение и итог импорта](https://github.com/NikishGum/JudeOSLite/issues/96) | — | [IMPORT-08](https://github.com/NikishGum/JudeOSLite/issues/95), [IMPORT-07](https://github.com/NikishGum/JudeOSLite/issues/94) |
| IMPORT-10 | [#97 — Проверить повтор и совпадения импорта](https://github.com/NikishGum/JudeOSLite/issues/97) | — | [IMPORT-08](https://github.com/NikishGum/JudeOSLite/issues/95), [IMPORT-09](https://github.com/NikishGum/JudeOSLite/issues/96) |

## E5 — 18 задач

| ID плана | Issue | Первый сценарий QA-03 | Прямые зависимости |
| --- | --- | --- | --- |
| FEE-01 | [#98 — Создать интервалы цены и месячные начисления](https://github.com/NikishGum/JudeOSLite/issues/98) | — | [MEM-01](https://github.com/NikishGum/JudeOSLite/issues/34), [BASE-04](https://github.com/NikishGum/JudeOSLite/issues/13), [DEC-04](https://github.com/NikishGum/JudeOSLite/issues/4) |
| FEE-02 | [#99 — Создать таблицу финансовых корректировок](https://github.com/NikishGum/JudeOSLite/issues/99) | — | [FEE-01](https://github.com/NikishGum/JudeOSLite/issues/98), [MED-01](https://github.com/NikishGum/JudeOSLite/issues/81), [BASE-04](https://github.com/NikishGum/JudeOSLite/issues/13) |
| FEE-03 | [#100 — Реализовать изменение месячной цены](https://github.com/NikishGum/JudeOSLite/issues/100) | — | [FEE-01](https://github.com/NikishGum/JudeOSLite/issues/98), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15), [DEC-04](https://github.com/NikishGum/JudeOSLite/issues/4) |
| FEE-04 | [#101 — Реализовать остановку начислений с месяца](https://github.com/NikishGum/JudeOSLite/issues/101) | — | [FEE-03](https://github.com/NikishGum/JudeOSLite/issues/100) |
| FEE-05 | [#102 — Реализовать возобновление начислений](https://github.com/NikishGum/JudeOSLite/issues/102) | — | [FEE-04](https://github.com/NikishGum/JudeOSLite/issues/101) |
| FEE-06 | [#103 — Добавить повторяемую генерацию начисления месяца](https://github.com/NikishGum/JudeOSLite/issues/103) | — | [FEE-01](https://github.com/NikishGum/JudeOSLite/issues/98), [FEE-03](https://github.com/NikishGum/JudeOSLite/issues/100) |
| FEE-07 | [#104 — Реализовать ручную корректировку неоплаченного месяца](https://github.com/NikishGum/JudeOSLite/issues/104) | — | [FEE-02](https://github.com/NikishGum/JudeOSLite/issues/99), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22), [DEC-04](https://github.com/NikishGum/JudeOSLite/issues/4) |
| FEE-08 | [#112 — Реализовать чтение объяснимого остатка](https://github.com/NikishGum/JudeOSLite/issues/112) | — | [FEE-02](https://github.com/NikishGum/JudeOSLite/issues/99), [PAY-04](https://github.com/NikishGum/JudeOSLite/issues/111), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22) |
| FEE-09 | [#105 — Сделать форму цены с выбранного месяца](https://github.com/NikishGum/JudeOSLite/issues/105) | — | [FEE-03](https://github.com/NikishGum/JudeOSLite/issues/100), [MEM-07](https://github.com/NikishGum/JudeOSLite/issues/41) |
| FEE-10 | [#106 — Сделать управление паузой начислений](https://github.com/NikishGum/JudeOSLite/issues/106) | — | [FEE-04](https://github.com/NikishGum/JudeOSLite/issues/101), [FEE-05](https://github.com/NikishGum/JudeOSLite/issues/102), [FEE-09](https://github.com/NikishGum/JudeOSLite/issues/105) |
| FEE-11 | [#113 — Сделать форму ручной суммы месяца](https://github.com/NikishGum/JudeOSLite/issues/113) | — | [FEE-07](https://github.com/NikishGum/JudeOSLite/issues/104), [FEE-08](https://github.com/NikishGum/JudeOSLite/issues/112), [MEM-07](https://github.com/NikishGum/JudeOSLite/issues/41) |
| FEE-12 | [#114 — Сделать финансовую карточку участника](https://github.com/NikishGum/JudeOSLite/issues/114) | — | [FEE-08](https://github.com/NikishGum/JudeOSLite/issues/112), [MEM-07](https://github.com/NikishGum/JudeOSLite/issues/41) |
| FEE-13 | [#107 — Проверить месячную цену, паузу и повтор jobs](https://github.com/NikishGum/JudeOSLite/issues/107) | — | [FEE-05](https://github.com/NikishGum/JudeOSLite/issues/102), [FEE-06](https://github.com/NikishGum/JudeOSLite/issues/103) |
| FEE-14 | [#120 — Реализовать корректировку после зачёта оплаты](https://github.com/NikishGum/JudeOSLite/issues/120) | — | [FEE-07](https://github.com/NikishGum/JudeOSLite/issues/104), [PAY-04](https://github.com/NikishGum/JudeOSLite/issues/111), [PAY-09](https://github.com/NikishGum/JudeOSLite/issues/119), [DEC-09](https://github.com/NikishGum/JudeOSLite/issues/9) |
| PAY-01 | [#108 — Создать услуги и привязки счетов](https://github.com/NikishGum/JudeOSLite/issues/108) | — | [MEM-01](https://github.com/NikishGum/JudeOSLite/issues/34), [BASE-04](https://github.com/NikishGum/JudeOSLite/issues/13) |
| PAY-02 | [#109 — Создать попытки и сохранённые события](https://github.com/NikishGum/JudeOSLite/issues/109) | — | [PAY-01](https://github.com/NikishGum/JudeOSLite/issues/108), [BASE-04](https://github.com/NikishGum/JudeOSLite/issues/13), [FEE-01](https://github.com/NikishGum/JudeOSLite/issues/98) |
| PAY-03 | [#110 — Создать факты платежа и полного сторно](https://github.com/NikishGum/JudeOSLite/issues/110) | — | [PAY-02](https://github.com/NikishGum/JudeOSLite/issues/109), [BASE-04](https://github.com/NikishGum/JudeOSLite/issues/13) |
| PAY-04 | [#111 — Создать историю зачёта и обратных записей](https://github.com/NikishGum/JudeOSLite/issues/111) | — | [PAY-03](https://github.com/NikishGum/JudeOSLite/issues/110), [FEE-01](https://github.com/NikishGum/JudeOSLite/issues/98), [BASE-04](https://github.com/NikishGum/JudeOSLite/issues/13) |

## E6 — 24 задачи

| ID плана | Issue | Первый сценарий QA-03 | Прямые зависимости |
| --- | --- | --- | --- |
| PAY-05 | [#115 — Реализовать настройку услуги в режиме disabled/test](https://github.com/NikishGum/JudeOSLite/issues/115) | — | [PAY-01](https://github.com/NikishGum/JudeOSLite/issues/108), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15) |
| PAY-06 | [#116 — Реализовать привязку счёта участнику](https://github.com/NikishGum/JudeOSLite/issues/116) | — | [PAY-05](https://github.com/NikishGum/JudeOSLite/issues/115), [MEM-03](https://github.com/NikishGum/JudeOSLite/issues/37), [DEC-07](https://github.com/NikishGum/JudeOSLite/issues/7) |
| PAY-07 | [#117 — Реализовать подтверждение факта платежа](https://github.com/NikishGum/JudeOSLite/issues/117) | — | [PAY-03](https://github.com/NikishGum/JudeOSLite/issues/110), [BASE-06](https://github.com/NikishGum/JudeOSLite/issues/15) |
| PAY-08 | [#118 — Реализовать точный зачёт в целевой месяц](https://github.com/NikishGum/JudeOSLite/issues/118) | — | [PAY-04](https://github.com/NikishGum/JudeOSLite/issues/111), [PAY-07](https://github.com/NikishGum/JudeOSLite/issues/117), [DEC-09](https://github.com/NikishGum/JudeOSLite/issues/9) |
| PAY-09 | [#119 — Реализовать отмену прежнего зачёта](https://github.com/NikishGum/JudeOSLite/issues/119) | — | [PAY-04](https://github.com/NikishGum/JudeOSLite/issues/111) |
| PAY-10 | [#121 — Реализовать подтверждённое полное сторно](https://github.com/NikishGum/JudeOSLite/issues/121) | — | [PAY-03](https://github.com/NikishGum/JudeOSLite/issues/110), [PAY-09](https://github.com/NikishGum/JudeOSLite/issues/119), [DEC-07](https://github.com/NikishGum/JudeOSLite/issues/7) |
| IPAY-01 | [#122 — Реализовать договорный HTTP-транспорт](https://github.com/NikishGum/JudeOSLite/issues/122) | — | [BASE-01](https://github.com/NikishGum/JudeOSLite/issues/10), [BASE-05](https://github.com/NikishGum/JudeOSLite/issues/14), [PAY-05](https://github.com/NikishGum/JudeOSLite/issues/115), [DEC-07](https://github.com/NikishGum/JudeOSLite/issues/7) |
| IPAY-02 | [#123 — Реализовать кодировку и денежные поля XML](https://github.com/NikishGum/JudeOSLite/issues/123) | — | [IPAY-01](https://github.com/NikishGum/JudeOSLite/issues/122) |
| IPAY-03 | [#124 — Проверять договорную подпись до обработки](https://github.com/NikishGum/JudeOSLite/issues/124) | — | [IPAY-01](https://github.com/NikishGum/JudeOSLite/issues/122), [DEC-07](https://github.com/NikishGum/JudeOSLite/issues/7) |
| IPAY-04 | [#125 — Сохранять финансовое событие до подтверждения](https://github.com/NikishGum/JudeOSLite/issues/125) | — | [IPAY-02](https://github.com/NikishGum/JudeOSLite/issues/123), [IPAY-03](https://github.com/NikishGum/JudeOSLite/issues/124), [PAY-02](https://github.com/NikishGum/JudeOSLite/issues/109) |
| IPAY-05 | [#126 — Реализовать ServiceInfo для текущего месяца](https://github.com/NikishGum/JudeOSLite/issues/126) | — | [IPAY-03](https://github.com/NikishGum/JudeOSLite/issues/124), [PAY-06](https://github.com/NikishGum/JudeOSLite/issues/116), [FEE-08](https://github.com/NikishGum/JudeOSLite/issues/112), [DEC-08](https://github.com/NikishGum/JudeOSLite/issues/8), [DEC-09](https://github.com/NikishGum/JudeOSLite/issues/9) |
| IPAY-06 | [#127 — Реализовать TransactionStart со снимком цели](https://github.com/NikishGum/JudeOSLite/issues/127) | — | [IPAY-04](https://github.com/NikishGum/JudeOSLite/issues/125), [IPAY-05](https://github.com/NikishGum/JudeOSLite/issues/126) |
| IPAY-07 | [#128 — Реализовать TransactionResult](https://github.com/NikishGum/JudeOSLite/issues/128) | — | [IPAY-06](https://github.com/NikishGum/JudeOSLite/issues/127), [PAY-07](https://github.com/NikishGum/JudeOSLite/issues/117), [PAY-08](https://github.com/NikishGum/JudeOSLite/issues/118) |
| IPAY-08 | [#129 — Реализовать StornStart](https://github.com/NikishGum/JudeOSLite/issues/129) | — | [IPAY-07](https://github.com/NikishGum/JudeOSLite/issues/128), [PAY-10](https://github.com/NikishGum/JudeOSLite/issues/121) |
| IPAY-09 | [#130 — Реализовать StornResult](https://github.com/NikishGum/JudeOSLite/issues/130) | — | [IPAY-08](https://github.com/NikishGum/JudeOSLite/issues/129), [PAY-10](https://github.com/NikishGum/JudeOSLite/issues/121) |
| IPAY-10 | [#131 — Проверить ACK и сбой обработки](https://github.com/NikishGum/JudeOSLite/issues/131) | — | [IPAY-07](https://github.com/NikishGum/JudeOSLite/issues/128), [IPAY-09](https://github.com/NikishGum/JudeOSLite/issues/130) |
| IPAY-11 | [#132 — Проверить повторы начала и успешного результата](https://github.com/NikishGum/JudeOSLite/issues/132) | — | [IPAY-06](https://github.com/NikishGum/JudeOSLite/issues/127), [IPAY-07](https://github.com/NikishGum/JudeOSLite/issues/128) |
| IPAY-12 | [#133 — Проверить конфликтующий и неизвестный результат](https://github.com/NikishGum/JudeOSLite/issues/133) | — | [IPAY-07](https://github.com/NikishGum/JudeOSLite/issues/128) |
| IPAY-13 | [#134 — Проверить сторно и гонку с корректировкой](https://github.com/NikishGum/JudeOSLite/issues/134) | — | [IPAY-09](https://github.com/NikishGum/JudeOSLite/issues/130), [FEE-14](https://github.com/NikishGum/JudeOSLite/issues/120) |
| IPAY-14 | [#139 — Сделать экран настройки и состояния обмена](https://github.com/NikishGum/JudeOSLite/issues/139) | — | [PAY-05](https://github.com/NikishGum/JudeOSLite/issues/115), [PAY-06](https://github.com/NikishGum/JudeOSLite/issues/116), [IPAY-04](https://github.com/NikishGum/JudeOSLite/issues/125), [FEE-12](https://github.com/NikishGum/JudeOSLite/issues/114), [PAY-11](https://github.com/NikishGum/JudeOSLite/issues/138) |
| IPAY-15 | [#135 — Проверить интеграцию на договорном стенде](https://github.com/NikishGum/JudeOSLite/issues/135) | — | [DEC-08](https://github.com/NikishGum/JudeOSLite/issues/8), [DEC-09](https://github.com/NikishGum/JudeOSLite/issues/9), [IPAY-10](https://github.com/NikishGum/JudeOSLite/issues/131), [IPAY-11](https://github.com/NikishGum/JudeOSLite/issues/132), [IPAY-12](https://github.com/NikishGum/JudeOSLite/issues/133), [IPAY-13](https://github.com/NikishGum/JudeOSLite/issues/134) |
| IPAY-16 | [#136 — Реализовать сверку с подтверждённым реестром](https://github.com/NikishGum/JudeOSLite/issues/136) | — | [DEC-07](https://github.com/NikishGum/JudeOSLite/issues/7), [DEC-08](https://github.com/NikishGum/JudeOSLite/issues/8), [IPAY-07](https://github.com/NikishGum/JudeOSLite/issues/128) |
| IPAY-17 | [#137 — Проверить восстановление обмена после простоя](https://github.com/NikishGum/JudeOSLite/issues/137) | — | [IPAY-16](https://github.com/NikishGum/JudeOSLite/issues/136), [IPAY-15](https://github.com/NikishGum/JudeOSLite/issues/135) |
| PAY-11 | [#138 — Реализовать чтение состояния обмена и проблем](https://github.com/NikishGum/JudeOSLite/issues/138) | — | [PAY-05](https://github.com/NikishGum/JudeOSLite/issues/115), [IPAY-04](https://github.com/NikishGum/JudeOSLite/issues/125), [PAY-07](https://github.com/NikishGum/JudeOSLite/issues/117), [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22) |

## E7 — 9 задач

| ID плана | Issue | Первый сценарий QA-03 | Прямые зависимости |
| --- | --- | --- | --- |
| OWNER-01 | [#140 — Реализовать согласованные агрегаты владельца](https://github.com/NikishGum/JudeOSLite/issues/140) | — | [DEC-05](https://github.com/NikishGum/JudeOSLite/issues/5), [ATT-09](https://github.com/NikishGum/JudeOSLite/issues/76), [FEE-08](https://github.com/NikishGum/JudeOSLite/issues/112), [PAY-07](https://github.com/NikishGum/JudeOSLite/issues/117), [PAY-10](https://github.com/NikishGum/JudeOSLite/issues/121) |
| OWNER-02 | [#141 — Сделать обзор владельца](https://github.com/NikishGum/JudeOSLite/issues/141) | — | [OWNER-01](https://github.com/NikishGum/JudeOSLite/issues/140), [UI-01](https://github.com/NikishGum/JudeOSLite/issues/55) |
| PWA-01 | [#142 — Добавить manifest и обновление статической оболочки](https://github.com/NikishGum/JudeOSLite/issues/142) | — | [BASE-02](https://github.com/NikishGum/JudeOSLite/issues/11), [AUTH-12](https://github.com/NikishGum/JudeOSLite/issues/27) |
| QA-01 | [#143 — Проверить роли на согласованных устройствах](https://github.com/NikishGum/JudeOSLite/issues/143) | — | [ATT-08](https://github.com/NikishGum/JudeOSLite/issues/75), [ATT-11](https://github.com/NikishGum/JudeOSLite/issues/78), [MED-07](https://github.com/NikishGum/JudeOSLite/issues/87), [IMPORT-09](https://github.com/NikishGum/JudeOSLite/issues/96), [FEE-12](https://github.com/NikishGum/JudeOSLite/issues/114), [OWNER-02](https://github.com/NikishGum/JudeOSLite/issues/141), [PWA-01](https://github.com/NikishGum/JudeOSLite/issues/142), [AUTH-14](https://github.com/NikishGum/JudeOSLite/issues/29) |
| QA-02 | [#144 — Проверить межклубную изоляцию и финансовый запрет](https://github.com/NikishGum/JudeOSLite/issues/144) | — | [AUTH-07](https://github.com/NikishGum/JudeOSLite/issues/22), [MEM-03](https://github.com/NikishGum/JudeOSLite/issues/37), [MED-05](https://github.com/NikishGum/JudeOSLite/issues/85), [FEE-08](https://github.com/NikishGum/JudeOSLite/issues/112), [IMPORT-06](https://github.com/NikishGum/JudeOSLite/issues/93), [OWNER-01](https://github.com/NikishGum/JudeOSLite/issues/140), [PAY-11](https://github.com/NikishGum/JudeOSLite/issues/138) |
| OPS-02 | [#145 — Настроить проверяемое резервное копирование](https://github.com/NikishGum/JudeOSLite/issues/145) | — | [OPS-01](https://github.com/NikishGum/JudeOSLite/issues/30) |
| OPS-03 | [#146 — Проверить восстановление и обновление версии](https://github.com/NikishGum/JudeOSLite/issues/146) | — | [OPS-02](https://github.com/NikishGum/JudeOSLite/issues/145), [BASE-03](https://github.com/NikishGum/JudeOSLite/issues/12) |
| CUT-01 | [#147 — Провести пробный импорт и сверку начальных остатков](https://github.com/NikishGum/JudeOSLite/issues/147) | — | [DEC-06](https://github.com/NikishGum/JudeOSLite/issues/6), [DEC-08](https://github.com/NikishGum/JudeOSLite/issues/8), [IMPORT-10](https://github.com/NikishGum/JudeOSLite/issues/97), [FEE-13](https://github.com/NikishGum/JudeOSLite/issues/107), [IPAY-16](https://github.com/NikishGum/JudeOSLite/issues/136) |
| CUT-02 | [#148 — Зафиксировать готовность рабочего перехода](https://github.com/NikishGum/JudeOSLite/issues/148) | — | [CUT-01](https://github.com/NikishGum/JudeOSLite/issues/147), [IPAY-17](https://github.com/NikishGum/JudeOSLite/issues/137), [QA-01](https://github.com/NikishGum/JudeOSLite/issues/143), [QA-02](https://github.com/NikishGum/JudeOSLite/issues/144), [OPS-03](https://github.com/NikishGum/JudeOSLite/issues/146) |

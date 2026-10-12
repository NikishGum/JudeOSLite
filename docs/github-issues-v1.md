# GitHub issues первой версии

Проверено 12.10.2026: **18 открытых issues, 27 прямых зависимостей; все Todo, без исполнителей.** Старые #1–#148 удалены по прямому поручению заказчика, не закрыты как выполненные.

Источник полного переноса — [план](development-plan-v1.md) и [JSON карточек](implementation-tasks-v1.json) в коммите [4b650de](https://github.com/NikishGum/JudeOSLite/tree/4b650dea4b0d6f811e9d92e775e004b3199d8bc1). Полные тексты новых issues прочитаны обратно и сверены без сокращения полей. Зависимости отражены в текстах и штатных Blocked by / Blocking, обе стороны проверены. [Машиночитаемый реестр и хеши](github-issues-v1.json).

Прежний расширенный комплект сохранён в [истории](archive/README.md). Номера GitHub после удаления не переиспользуются: V1-01 — #149, а не прежний #1/DEC-01.

## Задачи

| ID | Issue и результат | Прямые зависимости |
| --- | --- | --- |
| V1-01 | [#149 — Рабочая основа Go + React + PostgreSQL и проверки сборки](https://github.com/NikishGum/JudeOSLite/issues/149) | нет |
| V1-02 | [#150 — Вход сотрудников, выход и серверная проверка доступа](https://github.com/NikishGum/JudeOSLite/issues/150) | [#149](https://github.com/NikishGum/JudeOSLite/issues/149) |
| V1-03 | [#151 — Филиалы и назначения администраторов для владельца](https://github.com/NikishGum/JudeOSLite/issues/151) | [#150](https://github.com/NikishGum/JudeOSLite/issues/150) |
| V1-04 | [#152 — Создание и управление тренерами своего филиала](https://github.com/NikishGum/JudeOSLite/issues/152) | [#151](https://github.com/NikishGum/JudeOSLite/issues/151) |
| V1-05 | [#153 — Ручной реестр участников и карточки филиала](https://github.com/NikishGum/JudeOSLite/issues/153) | [#151](https://github.com/NikishGum/JudeOSLite/issues/151) |
| V1-06 | [#154 — Группы и датированная история состава](https://github.com/NikishGum/JudeOSLite/issues/154) | [#152](https://github.com/NikishGum/JudeOSLite/issues/152), [#153](https://github.com/NikishGum/JudeOSLite/issues/153) |
| V1-07 | [#155 — Расписание: дни недели, чётность дат, календарные даты и разовое занятие](https://github.com/NikishGum/JudeOSLite/issues/155) | [#154](https://github.com/NikishGum/JudeOSLite/issues/154) |
| V1-08 | [#156 — Календарь занятий, отмена и замена тренера администратором](https://github.com/NikishGum/JudeOSLite/issues/156) | [#155](https://github.com/NikishGum/JudeOSLite/issues/155) |
| V1-09 | [#157 — Журнал занятия: снимок состава, отметки и атомарное завершение](https://github.com/NikishGum/JudeOSLite/issues/157) | [#156](https://github.com/NikishGum/JudeOSLite/issues/156) |
| V1-10 | [#158 — Мобильные карточки посещаемости тренера с кнопками и выбором причины](https://github.com/NikishGum/JudeOSLite/issues/158) | [#157](https://github.com/NikishGum/JudeOSLite/issues/157) |
| V1-11 | [#159 — Журнал администратора и поштучное исправление закрытых отметок](https://github.com/NikishGum/JudeOSLite/issues/159) | [#157](https://github.com/NikishGum/JudeOSLite/issues/157) |
| V1-12 | [#160 — Статистика владельца по филиалам, группам и периоду](https://github.com/NikishGum/JudeOSLite/issues/160) | [#159](https://github.com/NikishGum/JudeOSLite/issues/159) |
| V1-13 | [#161 — Адаптация всех ролей и установка PWA](https://github.com/NikishGum/JudeOSLite/issues/161) | [#158](https://github.com/NikishGum/JudeOSLite/issues/158), [#159](https://github.com/NikishGum/JudeOSLite/issues/159), [#160](https://github.com/NikishGum/JudeOSLite/issues/160) |
| V1-14 | [#162 — Проверка изоляции филиалов и полной матрицы ролей](https://github.com/NikishGum/JudeOSLite/issues/162) | [#158](https://github.com/NikishGum/JudeOSLite/issues/158), [#159](https://github.com/NikishGum/JudeOSLite/issues/159), [#160](https://github.com/NikishGum/JudeOSLite/issues/160) |
| V1-15 | [#163 — Проверка расписания, исторических данных и конкурентных отметок](https://github.com/NikishGum/JudeOSLite/issues/163) | [#159](https://github.com/NikishGum/JudeOSLite/issues/159), [#160](https://github.com/NikishGum/JudeOSLite/issues/160) |
| V1-16 | [#164 — Сквозные проверки браузеров и мобильной PWA](https://github.com/NikishGum/JudeOSLite/issues/164) | [#161](https://github.com/NikishGum/JudeOSLite/issues/161), [#165](https://github.com/NikishGum/JudeOSLite/issues/165) |
| V1-17 | [#165 — Развёртывание на VPS, HTTPS и восстановление резервной копии](https://github.com/NikishGum/JudeOSLite/issues/165) | [#161](https://github.com/NikishGum/JudeOSLite/issues/161) |
| V1-18 | [#166 — Приёмка первой версии и запуск с ручным вводом данных](https://github.com/NikishGum/JudeOSLite/issues/166) | [#162](https://github.com/NikishGum/JudeOSLite/issues/162), [#163](https://github.com/NikishGum/JudeOSLite/issues/163), [#164](https://github.com/NikishGum/JudeOSLite/issues/164), [#165](https://github.com/NikishGum/JudeOSLite/issues/165) |

Порядок определяется зависимостями, а не только номером: реальная PWA-проверка #164 требует HTTPS-развёртывания #165. Отдельных текущих issues для финансов/iPay, Excel или справок нет.

## Работа вдвоём

Человек назначает **одного** исполнителя; метка In progress появляется автоматически после назначения. Агент не назначает себя и не забирает Todo. До кода проверяет live исполнителя/метку, отсутствие работы коллеги и результаты зависимостей в main. Несколько исполнителей — конфликт, не начало совместной записи в один issue.

Открытие без исполнителя / снятие назначения даёт Todo, закрытие — Done. In review и ревью коллеги необязательны. Закрывать только готовое в main, не по одному push ветви. [Подробные правила](collaboration.md), [промпт исполнителя](task-agent-prompt.md).

## Проверка автоматизации

Workflow issue-status.yml опубликован и активен. Все 18 запусков при создании issues завершились успешно; [реальная проверка восстановления Todo](https://github.com/NikishGum/JudeOSLite/actions/runs/38226762759) также успешна. Назначение/снятие/закрытие/переоткрытие, сохранение сторонних меток и отказ при конфликте исполнителей проверены локально на сценариях. Для проверки не назначали исполнителя, не начинали реализацию и не создавали дополнительных issues.

Сведения о статусах здесь — срез проверки; актуальную работу всегда читать в GitHub.

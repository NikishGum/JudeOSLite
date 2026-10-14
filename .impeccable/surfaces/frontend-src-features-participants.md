---
version: 1
slug: "frontend-src-features-participants"
primary_target: "frontend/src/features/participants"
related_targets: ["frontend/src/features/groups","frontend/src/features/staff","frontend/src/features/clubs"]
---

# RD-02 — предметное рабочее пространство
Mode: Operate. Scope: participants, groups, staff, clubs; #184 only.
Audience: владелец сети и администратор конкретного доступного филиала. Работа: найти человека, проверить контакт/дату, сохранить карточку, изменить датированный состав или назначения.
## Direction contract
THESIS: контекст филиала → читаемый список → одна предметная операция; сохраняем все поля v1, убираем вложенные рамки и смешение редактируемых контекстов.
OWN-WORLD: выбранный Working Panel/PRIDE: локальный Rubik, белая поверхность на холодном фоне, тонкие разделители, красное основное действие, статусы с текстом. Общие токены неизменны.
STORY: сотрудник видит scope и статус, открывает полную карточку, правит в панели с защищённым фокусом; ошибка сохраняет форму, конфликт требует явного перечитывания. История и прежние снимки остаются.
FIRST VIEWPORT: заголовок и главное действие, компактный поиск/состояние, на ПК строки с выровненными полями; на телефоне последовательные карточки и кнопки ≥44px. Редактор справа на ПК, снизу на телефоне через RD-01 Dialog.
FORM: pinned Working Panel, code-first extension; seed не запрашивается — композиция и визуальный мир заданы заказчиком, новой identity нет.
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
Constraints: реальные API, CSRF/server scope/multi assignments/lifecycle/PWA/draft and pending-save guards; никаких demo roles/data. Group→sessions URL carries existing ID; RD-03 applies calendar filter.

---
version: 1
slug: "frontend-src-app-app-tsx"
primary_target: "frontend/src/app/App.tsx"
related_targets: ["frontend/src/features","frontend/src/shared/ui"]
---

# PRIDE CLUB OS — макеты19.10.2026
Mode: Operate. Прямой визуальный источник — три приложенных макета пользователя.
## Direction contract
THESIS: рабочий день спортивного клуба: обзор → нужный человек/занятие → подтверждённое действие. Макеты задают узнаваемую структуру; все числа и состояния поступают из API.
OWN-WORLD: светлая холодная основа, белые панели16px, Rubik, насыщенный красный активного меню и главных действий, тонкие разделители, локальная оригинальная эмблема. shadcn/Radix — общая основа контролов.
STORY: владелец видит сеть, администратор закреплённые филиалы, тренер только назначения без выбора филиала, родитель только свою семью. Посещения, платежи и документы связаны с ребёнком, ошибки и последствия действий понятны заранее.
FIRST VIEWPORT:248px Sidebar/76px collapsed с логотипом и группами разделов;80px шапка с контекстом, уведомлениями и профилем справа; приветствие/подзаголовок, четыре реальные метрики, обзор и ближайшие тренировки в двух колонках. Телефон:72px контекст/профиль,16px поля, карточки в одну колонку, нижняя навигация.
FORM: пользователь закрепил композицию и язык приложенными макетами; seed user-screenshots-20261019, code-led. Анимация Sidebar220ms раскрывает целые подписи без сжатия/переноса; выбранные состояния меняются сразу. reduced-motion без движения.
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
Constraints: all real APIs/roles/multi assignments/routes/query/history/CSRF/dates/PWA/lifecycle/dirty/pending saves remain. Do not ship demo data/day/registry controls. Existing acceptance188 stays separate.

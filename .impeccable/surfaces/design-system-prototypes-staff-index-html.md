---
version: 1
slug: "design-system-prototypes-staff-index-html"
primary_target: "design-system/prototypes/staff/index.html"
related_targets: ["design-system/prototypes/staff/staff.css","design-system/prototypes/staff/staff.js"]
---

# Сотрудники — локальные UX-концепции
Mode: Operate. Scope: independent HTML/CSS/JS prototype, no live application changes.
Audience: владелец сети; задача — создать сотрудника, выдать роль/филиал, сменить пароль, снять назначение или отключить весь аккаунт.
World: inherited PRIDE/Working Panel, global visual authority unchanged. Code-led, two compositions explicitly requested, no identity selection.
Structure: one roster only; trainers and administrators are combinable roles. User explicitly rejected separate staff/trainers tabs on 20.10.2026. Role+branch filters match one assignment.
A: table with role/branch mapping, access state and focused account dialog.
B: roster with selected account beside it; mobile detail replaces roster, back preserves filters. Access actions precede individual assignments.
Constraints: synthetic data in memory, no API requests or backend changes. Native dialog, focus restoration, >=44px controls. Assignment form combining trainer/admin is an unapproved UX proposal; production permissions must be preserved in a separate implementation issue.
Open: user composition choice and further feedback. This prototype is not product acceptance.

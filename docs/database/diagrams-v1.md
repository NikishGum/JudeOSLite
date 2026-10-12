# Диаграммы минимальной v1

Версия 2.0, 12.10.2026. Предложение; соответствует [схеме из 12 таблиц](schema-v1.md). Будущие финансы/справки/импорт здесь не изображены, предыдущая версия сохранена в [истории](../archive/README.md).

```mermaid
erDiagram
  clubs ||--o{ staff_club_roles : access
  staff_users ||--o{ staff_club_roles : roles
  staff_users ||--o{ staff_sessions : login
  participants ||--o{ club_memberships : identity
  clubs ||--o{ club_memberships : local_card
  clubs ||--o{ training_groups : groups
  training_groups ||--o{ group_memberships : dated_roster
  club_memberships ||--o{ group_memberships : participant
  training_groups ||--o{ schedule_rules : schedule
  staff_users ||--o{ schedule_rules : trainer
  schedule_rules o|--o{ training_sessions : generate
  training_groups ||--o{ training_sessions : session
  staff_users ||--o{ training_sessions : actual_trainer
  training_sessions ||--o{ attendance_records : frozen_roster
  club_memberships ||--o{ attendance_records : local_participant
  group_memberships ||--o{ attendance_records : roster_source
  clubs ||--o{ audit_events : scoped_history
  staff_users ||--o{ audit_events : actor
```

Составные клубные FK и ограничения подробнее в схеме; ER не заменяет авторизацию и транзакционные проверки.

```mermaid
stateDiagram-v2
  [*] --> planned
  planned --> open: тренер открывает, снимок на дату
  planned --> cancelled: администратор отменяет
  open --> cancelled: отмена с предупреждением, история сохранена
  open --> closed: остатки absent/unknown + закрытие атомарно
  open --> open: отметка или исправление тренера
  closed --> closed: администратор правит одну отметку с причиной
```

```mermaid
flowchart TD
  Card[Карточка участника] --> Present[Присутствует]
  Card --> Absent[Отсутствует]
  Absent --> Sheet[Нижняя панель: неизвестна / болезнь / уважительная]
  Sheet --> Save[Сохранение на сервере]
  Present --> Save
  Sheet -->|Отмена без записи| Card
  Save -->|Успех| Next[Следующая карточка]
  Save -->|Ошибка| Retry[Остаться и предложить повтор]
  Next --> Finish[Явное завершение допустимо с неотмеченными]
  Finish --> Close[Остатки absent/unknown и closed одной транзакцией]
  Close --> Stats[Статистика владельца по закрытым занятиям]
```

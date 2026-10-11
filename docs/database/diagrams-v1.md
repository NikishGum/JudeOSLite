# Диаграммы JudeOSLite v1

Версия: 1.0, 11.10.2026. Статус: визуальное представление предложенной [схемы БД](schema-v1.md) и [архитектуры](../architecture-v1.md). Диаграммы Mermaid можно читать непосредственно в Markdown на GitHub и в поддерживающих Mermaid редакторах. Они не заменяют полный список полей, составных FK и транзакционных проверок.

## 1. Обзор областей

```mermaid
flowchart TB
    Access[Сотрудники и права] --> Club[Клуб: отдельный зал]
    Club --> Membership[Участник в клубе]
    Person[Единая идентичность участника] --> Membership
    Membership --> Group[История групп]
    Group --> Lessons[Расписание и конкретные занятия]
    Lessons --> Attendance[Снимок состава и отметки]
    Membership --> Medical[Допуск и периоды болезни]
    Membership --> Terms[История месячного взноса]
    Terms --> Charges[Начисления и корректировки]
    IPay[iPay: услуга и лицевой счёт] --> Attempts[Попытки и входящие события]
    Attempts --> Payments[Подтверждённые платежи и сторно]
    Payments --> Allocation[Зачёт и обратные записи]
    Allocation --> Charges
    Excel[Импорт: предпросмотр и подтверждение] --> Membership
    Audit[Аудит значимых изменений] -.-> Membership
    Audit -.-> Attendance
    Audit -.-> Charges
```

Финансы ограничены принадлежностью участника клубу. Общая идентичность не делает финансовые/медицинские данные доступными администратору другого зала.

## 2. Доступ и реестр — T01–T09

```mermaid
erDiagram
    staff_users ||--o{ staff_sessions : has
    staff_users ||--o{ staff_club_roles : assigned
    clubs ||--o{ staff_club_roles : scopes
    participants ||--o{ club_memberships : belongs
    clubs ||--o{ club_memberships : contains
    clubs ||--o{ training_groups : contains
    training_groups ||--o{ group_trainers : assigned
    staff_users ||--o{ group_trainers : coaches
    training_groups ||--o{ group_memberships : includes
    club_memberships ||--o{ group_memberships : history
    clubs {
        uuid id PK
        text name
        text timezone
    }
    staff_users {
        uuid id PK
        text login UK
        boolean is_owner
    }
    staff_sessions {
        uuid id PK
        uuid staff_user_id FK
        bytea token_hash UK
        timestamptz expires_at
    }
    staff_club_roles {
        uuid id PK
        uuid staff_user_id FK
        uuid club_id FK
        text role
    }
    participants {
        uuid id PK
        text full_name
        date birth_date
        smallint birth_year
    }
    club_memberships {
        uuid id PK
        uuid participant_id FK
        uuid club_id FK
        text contact_value
        text status
    }
    training_groups {
        uuid id PK
        uuid club_id FK
        text name
    }
    group_trainers {
        uuid id PK
        uuid group_id FK
        uuid staff_user_id FK
        date valid_from
        date valid_until
    }
    group_memberships {
        uuid id PK
        uuid group_id FK
        uuid club_membership_id FK
        date valid_from
        date valid_until
    }
```

is_owner — общий доступ; роль administrator/trainer назначается отдельно клубу. Групповые периоды хранят историю, а не дублируют participants. Возможность нескольких групп в модели не утверждает гостевые сценарии.

## 3. Занятия и справки — T10–T13

```mermaid
erDiagram
    training_groups ||--o{ schedule_rules : planned_by
    schedule_rules o|--o{ training_sessions : generates
    training_groups ||--o{ training_sessions : hosts
    staff_users ||--o{ training_sessions : actual_trainer
    training_sessions ||--o{ attendance_records : roster
    group_memberships ||--o{ attendance_records : source
    club_memberships ||--o{ attendance_records : participant
    club_memberships ||--o{ medical_records : evidence
    schedule_rules {
        uuid id PK
        uuid group_id FK
        uuid trainer_id FK
        uuid series_key
        smallint weekday
        time starts_local
        date valid_from
        date valid_until
    }
    training_sessions {
        uuid id PK
        uuid group_id FK
        uuid schedule_rule_id FK
        uuid series_key
        uuid trainer_id FK
        date occurrence_on
        timestamptz starts_at
        text status
    }
    attendance_records {
        uuid id PK
        uuid session_id FK
        uuid club_membership_id FK
        uuid group_membership_id FK
        text status
        integer version
    }
    medical_records {
        uuid id PK
        uuid club_membership_id FK
        text kind
        date starts_on
        date ends_on
    }
```

schedule_rule_id и series_key могут отсутствовать только для согласованного исторического импорта. series_key сохраняет связь версий регулярного пункта: повторная генерация учитывает уже созданное, отменённое или перенесённое занятие. Состав фиксируется при открытии, а не пересчитывается по сегодняшней группе. Допуск и болезнь — виды medical_records; болезнь не уменьшает начисление автоматически.

## 4. Финансы и iPay — T14–T23

```mermaid
erDiagram
    club_memberships ||--o{ fee_terms : priced
    club_memberships ||--o{ charges : owes
    fee_terms o|--o{ charges : source
    charges ||--o{ charge_adjustments : adjusted
    medical_records o|--o{ charge_adjustments : basis
    clubs ||--o{ payment_services : services
    payment_services ||--o{ payment_accounts : accounts
    club_memberships ||--o{ payment_accounts : recipient
    payment_accounts ||--o{ payment_attempts : starts
    payment_services ||--o{ ipay_events : receives
    payment_attempts o|--o{ ipay_events : correlates
    payment_attempts ||--o| payments : confirmed
    ipay_events ||--o| payments : confirms
    payments ||--o| payment_reversals : reversed
    ipay_events ||--o| payment_reversals : confirms
    payments ||--o{ payment_allocations : distributes
    charges ||--o{ payment_allocations : credited
    payment_reversals o|--o{ payment_allocations : undoes
    payment_allocations o|--o{ payment_allocations : reverses
    fee_terms {
        uuid id PK
        uuid club_membership_id FK
        date effective_from_month
        date effective_until_month
        bigint amount_minor
    }
    charges {
        uuid id PK
        uuid club_membership_id FK
        uuid source_fee_term_id FK
        date billing_month
        bigint base_amount_minor
    }
    charge_adjustments {
        uuid id PK
        uuid charge_id FK
        uuid medical_record_id FK
        bigint delta_minor
        text request_key
    }
    payment_services {
        uuid id PK
        uuid club_id FK
        text connection_key
        text service_no
        text mode
    }
    payment_accounts {
        uuid id PK
        uuid service_id FK
        uuid club_membership_id FK
        string personal_account
    }
    payment_attempts {
        uuid id PK
        uuid payment_account_id FK
        string provider_transaction_id
        string provider_trx_id
        bigint amount_minor
        text status
        text storn_status
    }
    ipay_events {
        uuid id PK
        uuid service_id FK
        uuid attempt_id FK
        text request_type
        text semantic_fingerprint
        text processing_state
    }
    payments {
        uuid id PK
        uuid attempt_id FK
        uuid confirmation_event_id FK
        bigint amount_minor
    }
    payment_reversals {
        uuid id PK
        uuid payment_id FK
        uuid attempt_id FK
        uuid confirmation_event_id FK
        bigint amount_minor
    }
    payment_allocations {
        uuid id PK
        uuid payment_id FK
        uuid charge_id FK
        uuid reverses_allocation_id FK
        uuid payment_reversal_id FK
        bigint amount_minor
        text entry_key UK
    }
```

Диаграмма показывает смысл связи, а [schema-v1.md](schema-v1.md#integrity) задаёт составные FK по клубу/получателю/попытке. Один reversal на payment — предложенное полное сторно v1; частичные операции ждут B07. Отрицательная allocation ссылается на положительную, не заменяет её.

## 5. Импорт и аудит — T24–T26

```mermaid
erDiagram
    clubs ||--o{ import_batches : destination
    staff_users ||--o{ import_batches : uploads
    import_batches ||--o{ import_rows : previews
    club_memberships o|--o{ import_rows : resolves
    training_groups o|--o{ import_rows : maps
    clubs o|--o{ audit_events : scope
    staff_users o|--o{ audit_events : actor
    import_batches {
        uuid id PK
        uuid club_id FK
        uuid uploaded_by FK
        text source_digest
        text state
        text apply_request_key
    }
    import_rows {
        uuid id PK
        uuid batch_id FK
        text source_locator
        uuid resolved_membership_id FK
        uuid resolved_group_id FK
        text state
    }
    audit_events {
        uuid id PK
        uuid club_id FK
        uuid actor_id FK
        text entity_type
        uuid entity_id
        text action
    }
```

NULL club_id аудита — только сетевое действие; NULL actor_id — системная операция/провайдер. entity_id — полиморфный указатель аудита, не универсальная связь бизнес-таблиц. Персональные данные импорта остаются в закрытой БД, не в репозитории.

## 6. Подтверждение платежа

```mermaid
sequenceDiagram
    participant Parent as Родитель в ЕРИП
    participant IPay as iPay
    participant CRM as Go / адаптер
    participant DB as PostgreSQL
    Parent->>IPay: Услуга и лицевой счёт
    IPay->>CRM: ServiceInfo
    CRM->>DB: Проверить привязку, прочитать начисления
    CRM-->>IPay: Допустимая сумма по политике B03
    Parent->>IPay: Подтверждение суммы
    IPay->>CRM: TransactionStart (ID, сумма)
    CRM->>DB: Проверить доверие, сохранить attempt и event
    DB-->>CRM: COMMIT
    CRM-->>IPay: Стабильный ServiceProvider_TrxId
    IPay->>CRM: TransactionResult
    CRM->>DB: Сохранить event и один payment, зачёт по B03
    DB-->>CRM: COMMIT
    CRM-->>IPay: Договорный ACK без XML Error
    IPay->>CRM: Повтор того же результата
    CRM->>DB: Найти ранее обработанный результат
    CRM-->>IPay: Ответ без повторного финансового эффекта
```

Это успешная согласованная цепочка, не утверждение о настроенном подключении. Подпись проверяется до доступа к данным/изменений; на схеме действие объединено в обработку CRM. Неуспех/конфликт/неизвестная попытка имеют отдельные сценарии [архитектуры](../architecture-v1.md). Сумма хранится на старте, а не извлекается из результата.

## 7. Состояния занятия

```mermaid
stateDiagram-v2
    [*] --> planned: Генерация из расписания
    planned --> open: Открыть и зафиксировать состав
    open --> closed: Закрыть по решению B04
    planned --> cancelled: Отмена администратором
    open --> cancelled: Отмена с историей
    closed --> closed: Правка прошлой отметки администратором
```

Состояния и закрытие — инженерное предложение. Возврат закрытого занятия в open, отмена уже закрытого занятия и автопроставление отсутствия не считаются разрешёнными без B04. Отмена/замена не удаляет занятие и его audit.

## 8. Согласованность комплекта

T01–T26 соответствуют реестру схемы. Поля на диаграмме — ключевые, nullable/индексы/все общие поля читать в описании. Любое изменение связи или имени таблицы обновляется одновременно здесь, в схеме и в архитектуре; финальный план ссылается на нужную область, а не на одну нечитабельную общую ERD.

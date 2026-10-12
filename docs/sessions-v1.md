# Календарь и исключения занятий V1-08

12.10.2026, [#156](https://github.com/NikishGum/JudeOSLite/issues/156). Владелец/администратор видит список по группе и включительному периоду, фактического тренера, локальные дату/время/зону и статус. Trainer видит только собственные фактические занятия; обновление календаря показывает отмены и замены. После замены прежний тренер не получает сеанс, даже если остаётся тренером исходного правила.

## Данные и состояния

Переиспользованы `training_sessions`, календарный расчёт и advisory transaction lock V1-07. 0001–0007 не меняются; 0008 добавляет trigger защиты истории существующих занятий. Разовое имеет `schedule_rule_id=NULL`, не создаёт правило или состав.

- Отмена `planned/open → cancelled` сохраняет ID, rule/starts_on/время/тренера/снимок и существующие отметки. Причина: trim, 1–1000 символов, без управляющих символов. Open требует отдельного предупреждения и подтверждения актуальной версии; устанавливается closed_at, если его ещё нет. Отмена не вызывает заполнение unmarked → absent, ничего не удаляет. Cancelled исключается из будущей статистики; это не проведённое closed занятие. Closed отменять нельзя.
- Замена для planned/open проверяет действующего тренера своего филиала и группу, ставит manual override. Для open меняется только фактическое назначение: время/снимок/авторы существующих отметок сохраняются. Выбор того же trainer с актуальной версией безопасен, без фиктивной правки/аудита.
- Перенос — дополнительная команда только для ещё не начавшегося planned без замороженного состава. Новые дата/время будущие; длительность 1–1440 минут, исходная зона сохраняется. Open/frozen/closed переносить нельзя, чтобы сохранить дату снимка и историю. Общий календарь V1-07 отвергает неоднозначное/nonexistent DST время.

**Исходный starts_on при переносе остаётся датой вхождения серии**, rule/club/group/timezone также неизменны. Новый календарь фильтрует фактическую локальную дату `starts_at AT TIME ZONE timezone`: date в ответе — фактическая дата, sourceDate — исходный starts_on. Старый `/schedule/sessions` сохраняет контракт V1-07 по starts_on. UI показывает «Перенесено с …» и дату окончания при переходе полуночи.

Генератор никогда не обновляет записи; manual/cancelled/open/closed/frozen сохраняются при генерации и датированной смене правила. Исходная дата серии резервируется даже при переносе. Retired обычные планы (`superseded_at != NULL`) сохраняют ID, не входят в рабочий календарь и не редактируются. Cancelled продолжает занимать живой слот группы.

DB-trigger запрещает смену исходных ID/club/group/rule/starts_on/timezone/created_at, изменение retired/cancelled, переписывание статуса/тренера/времени closed, времени open/frozen, снятие уже установленного roster_frozen_at/manual override и возврат open в planned. Upgrade не меняет прежние данные.

## API и транзакции

`B = /api/clubs/{clubID}/sessions`.

| Запрос | Контракт |
| --- | --- |
| GET B?from=YYYY-MM-DD&until=YYYY-MM-DD&groupId=… | Session[] за `[from,until)`, максимум 366 дней, groupId необязателен, без retired |
| GET B/groups | Group[] включая архив для истории; owner/admin — свой филиал, trainer — группы своих правил/фактических сеансов |
| POST B | `{groupId,trainerId,date,startsTime,durationMinutes}` → 201 Session без правила |
| POST B/{id}/cancel | `{version,reason,confirmOpen?:boolean}` → 200 Session, confirmOpen=true обязателен для open |
| POST B/{id}/replace-trainer | `{version,trainerId}` → 200 Session |
| POST B/{id}/reschedule | `{version,date,startsTime,durationMinutes}` → 200 Session |

Session: id,clubId,groupId,groupName,ruleId,trainerId,trainerName,date,sourceDate,timezone,startsAt,endsAt,status,manualOverride,rosterFrozen,closedAt,cancelledAt,cancellationReason,supersededAt,version. Имена актуальные; авторство — по историческим staff ID. Selector новых тренеров — прежний `/trainers?available=true`.

Все endpoints используют auth.Require/RequireClub, Origin/CSRF и no-store. Owner/administrator пишут, trainer читает только свои фактические занятия даже при фильтре группы. Чужой/несуществующий/неверный club/group/session/trainer одинаково 404. Запись повторяет `auth.LockClubAdministrator` внутри транзакции: actor/session/administrator share locks → `schedule.Lock` → `clubs.RequireActive`. Создание использует `groups.ReadGroup(active=true)` и `auth.RequireTrainer`. Команды блокируют session FOR UPDATE и проверяют version/status/superseded_at; замена/перенос держат guards действующих group/trainer до commit. Отмена существующего занятия допустима при архивной группе/отозванном тренере, но не в архивном филиале. Общий lock берётся до session, совместимо с генерацией/сменой версии правила.

400 invalid_input/неизвестные или несовместимые поля; 409 version_conflict, session_state_conflict, open_confirmation_required, session_duplicate, club_archived/group_archived; безопасный 503 not_ready. Повтор старой версии возвращает явный 409. Audit training_session created/cancel/replace-trainer/reschedule атомарен: actor/club/ID, before/after trainer, исходная дата/времена/статус/версии, признаки снимка/записанной причины. Без ФИО, логинов, контактов, свободного текста причины и секретов; причина хранится в самом занятии. Отказ аудита откатывает запись/version/timestamps.

Формы сохраняют ввод при ошибке, не сообщают ложного успеха, блокируют двойной submit. Конфликт требует явной загрузки актуальных занятий со сбросом формы. Чтения отменяются; поздний результат мутации игнорируется после смены сотрудника/филиала, ключи компонентов различаются.

## Передача V1-09 и статистике

V1-09 фиксирует snapshot один раз, использует совместимый порядок locks и session FOR UPDATE/version. На каждом открытии/чтении/записи проверять текущие actor/session/role и **фактический trainer_id session**, а не тренера группы/rule. Запись только в open без superseded_at; cancelled/closed недоступны тренеру. Отмена open сохраняет snapshot/marks, никогда не вызывает unmarked → absent. Существующие marked_by/исторические авторы не заменяются вместе с trainer_id. V1-11 исправляет отметки closed с причиной, не переписывает тренера/время.

Статистика учитывает проведённые closed, отдельно незавершённые, исключает cancelled/retired независимо от сохранившихся marks. Период календаря — фактическая локальная дата. Состав при первом открытии фиксировать на **фактическую дату**: у перенесённого planned она может отличаться от исходного starts_on серии.

## Проверки и границы

Go gofmt/vet/build, полный `TEST_DATABASE_URL=… go test -race ./...` на PostgreSQL 18.6: clean/upgrade/rerun/checksum/concurrent migrations, прежние suites. Новые HTTP/DB: once без rule, дубли/чужие IDs/роли/CSRF/даты/время/период, отмены planned/open/confirm/version, сохранение snapshot/authors/unmarked consumer fixture, closed/frozen/retired DB/API guards, manual move/sourceDate/полночь, фактическая видимость тренера, regeneration/dated edit, concurrent command/create/generation, duplicate move, rollback каждой команды/создания при сбое аудита, lifecycle/revoke actor после middleware/revoke и disable target.

`frontend/e2e/sessions_v1.py` — только disposable loopback `http://127.0.0.1:8080`, совпадающий APP_ORIGIN/AUTH_INSECURE_LOCAL=true, собранная статика и синтетический owner / synthetic-browser-password (интерактивный create-owner). Нужны Python Playwright/Chromium, контейнер PostgreSQL sessions156-db, пользователь test и БД browser156; SESSIONS_TEST_DB_CONTAINER принимает только sessions156-* контейнер. Script создаёт уникальные synthetic club/group/trainers, fixture open/closed/snapshot/marks только в disposable БД. Из корня: `python3 frontend/e2e/sessions_v1.py`. Проверены create retry/double submit, replace и refresh двух trainer UI, stale form/сохранённый ввод, planned/open cancel/предупреждение, snapshot/authors, closed UI guards, перенос/sourceDate/дата окончания, reload/смена филиала; 320/390/1280 без overflow/JS-ошибок, кнопки ≥44 px. Снимки `/tmp/sessions156-{320,390,1280}.png`. Поле причины связано через label/htmlFor: проверка stale формы выявила неоднозначное имя вложенного textarea, исправлено.

Production Docker/Compose, финальные SHA/CI — в отчёте issue. Полный журнал V1-09, реальные iPhone/Android, HTTPS/VPS/PWA и пользовательская приёмка остаются следующим задачам; synthetic consumer не выдаётся за реализованную посещаемость.

# Журнал занятия V1-09

12.10.2026, [#157](https://github.com/NikishGum/JudeOSLite/issues/157). Реализованы список собственных фактических занятий на сегодня/выбранную дату, снимок состава, отдельные отметки и атомарное завершение. Интерфейс пока список с формой на строку; последовательные мобильные карточки и панель причин — V1-10, журнал администратора/коррекции — [готовый V1-11](corrections-v1.md).

## Снимок и состояния

Миграция 0009 добавляет `attendance_records`, прежние миграции не меняются. Составные FK связывают запись с сеансом, группой, локальной карточкой **club_memberships.id** и конкретным периодом `group_memberships.id` одного филиала. UUID записи стабилен, UNIQUE(session,card); удаление/смена ключей запрещены.

Первое открытие planned одной транзакцией выбирает периоды `[valid_from,valid_until)` на **фактическую локальную дату starts_at** в сохранённой зоне занятия (не sourceDate перенесённого вхождения), вставляет NULL-отметки, выставляет open/roster_frozen_at и аудит. Одна SQL-выборка видит согласованный состав до или после конкурентного перевода. Архивные карточки с действующим периодом входят в исторический состав. Последующий перевод/архив/новая запись состава его не пересобирают; имена и признак архива показываются актуальные. Контакты/дата рождения тренеру не выдаются.

Инженерное правило доступности: открыть можно с локального календарного дня занятия, включая более ранний planned; будущий день отвергается. Архив группы не препятствует работе уже назначенного исторического занятия, архив филиала запрещает запись. Повтор open для open/closed возвращает существующий журнал без новой записи/аудита и не открывает closed заново. Cancelled/retired открыть нельзя. Пустой снимок также фиксируется roster_frozen_at и безопасно завершается.

Отметка: NULL до записи; present без absence_reason либо absent с unknown/illness/excused. Записаны marked_by/marked_at/origin и version. Ручная отметка — manual; завершение остатка — close_default. Тренер может менять одну отметку только в open. Повтор того же эффективного значения в open не меняет автора, время, версии и аудит, в том числе при старой версии повторяемой команды. Другая правка со старой версией даёт 409; после закрытия даже идентичная mark-команда запрещена.

Close заполняет все NULL как absent/unknown/close_default, пишет историю каждой изменённой отметки и переводит занятие в closed одной транзакцией. Повтор close для closed возвращает журнал без изменений; ошибка любого аудита откатывает **все** отметки, историю, версии и статус. Версия занятия меняется при открытии/значимой отметке/закрытии; версия записи меняется только при значимой правке этой записи. Старая версия close не закрывает журнал после новой отметки: пользователь перечитывает и подтверждает актуальную сводку.

Отмена из V1-08 сохраняет snapshot, NULL и ручные marks/авторов; не вызывает close-default. Замена меняет только actual trainer_id, прежние marked_by остаются. Cancelled исключать из статистики независимо от сохранившихся marks. Closed пустой журнал учитывать как проведённое занятие.

## API и права

`B=/api/clubs/{clubID}/trainer-sessions`, `J=B/{sessionID}/journal`.

| Запрос | Ввод / результат |
| --- | --- |
| GET B?date=YYYY-MM-DD | Session[]; дата необязательна, сегодня в clubs.timezone; только actual trainer_id текущего сотрудника, включая planned/open/closed/cancelled, без retired |
| GET J | `{session: Session, records: Record[]}`; planned без снимка даёт пустой records |
| POST J/open | `{version}` → Journal |
| PUT J/records/{recordID} | `{version,sessionVersion,state,absenceReason}` → Journal; present/null или absent/unknown\|illness\|excused |
| POST J/close | `{version}` → Journal |

Session сохраняет контракт V1-08. Record: id,membershipId,sourceGroupMembershipId,fullName,archived,state,absenceReason,markedBy,markedAt,origin,version. Nullable поля честно NULL до первой отметки; пустые списки `[]`. Ответ — серверное подтверждение всей текущей сводки, без optimistic save/офлайн-очереди.

Каждый запрос повторяет текущий enabled actor, live session и trainer-роль своего филиала **внутри транзакции**, затем schedule advisory lock; запись дополнительно блокирует active club. Journal берёт session FOR UPDATE и проверяет actual trainer_id/superseded_at/status. Owner/admin без роли и фактического назначения тренера не получают этот endpoint; их отдельный доступ появляется в V1-11. Чужой session/record/club и отсутствующий ID одинаково 404, no-store/Origin/CSRF действуют для всех запросов. Составная FK — целостность, а не замена прав.

Порядок locks: actor → live session → actor trainer-role → schedule → active club (при записи) → session. Совместим с V1-07/V1-08; общий schedule lock исключает пересечение с генерацией/исключениями, строка занятия сериализует mark/close/cancel/replace. Прямой SQL insert/update attendance также блокирует сеанс через trigger. INSERT возможен только до freeze в planned, UPDATE только open, ключи/DELETE запрещены. V1-11 расширяет guard/check новой миграцией 0010 для поштучной correction, прежние миграции сохранены.

400 invalid_input (строгие поля, даты/версии/сочетание причины); 409 version_conflict/journal_state_conflict/session_not_available/club_archived; 404 not_found; безопасный 503 not_ready. Гонка mark/close или cancel/close даёт одному запросу успех, другому 409; после reload существует однозначный open/closed/cancelled и никаких частичных автопропусков.

История — существующие audit_events, без новой очереди/таблицы event sourcing. training_session journal_opened/journal_closed; attendance_record marked сохраняет old/new state,absenceReason,markedBy,markedAt,origin,version,sessionId, actor и created_at события. Имена, контакты и секреты не пишутся в payload. Общая история V1-20 показывает русские подписи новых действий/типа; raw before/after тренеру не выдаются этим endpoint.

## Интерфейс и проверки

После выбора филиала trainer видит «Мои занятия и журнал», локальную дату/Сегодня/обновление, статус и время. Первый переход в planned показывает явное подтверждение открытия. В open список участников позволяет выбрать/сохранить отдельную отметку и изменить её; завершение показывает число неотмеченных, последствия и отдельное подтверждение. Closed доступен для чтения; cancellation/replacement проверяются повторно сервером. При ошибке нет ложного сохранения; ввод сохраняется, double submit блокируется; 409 требует явного reload со сбросом ввода. 404/401 очищают недоступные данные, смена филиала/аккаунта и unmount игнорируют поздние ответы. API возвращает текущую сводку при повторной загрузке.

Проверены Go vet/build и полный `TEST_DATABASE_URL=… go test -race ./...` на PostgreSQL 18.6: повтор open/mark/close, пустой snapshot, локальный день/фактическая дата переноса, границы интервалов, перевод/архив после открытия, все три причины/исправления, stale version, изоляция/чужие ID, replacement/authors, live отзыв роли/сессии после middleware, архив клуба, SQL FK/guards/privacy. 12 конкурентных open + mark/close и 8 cancel/close; rollback открытия/отметки и всего завершения при отказе финального аудита. Прежние suites и миграционный clean/upgrade/rerun/checksum/concurrent apply прошли.

Браузерная регрессия: `python3 frontend/e2e/attendance_v1.py`. Требуются disposable контейнер `attendance157-db` (или ATTENDANCE_TEST_DB_CONTAINER с префиксом attendance157-), PostgreSQL пользователь test и БД browser157, сервер со собранной статикой на http://127.0.0.1:8080 с тем же APP_ORIGIN/AUTH_INSECURE_LOCAL=true, bootstrap synthetic owner / synthetic-browser-password. Только синтетические данные; скрипт создаёт уникальные клуб/участников/тренеров через API и разовые сеансы на сегодня напрямую в **тестовой** БД. Python Playwright/Chromium, CHROMIUM_PATH по умолчанию /usr/bin/chromium.

Chromium 154.0.8037.57: UI → API → PostgreSQL → UI, open/double submit, network failure/retry с сохранённой причиной, поштучная правка/409/reload, warning/cancel completion/close/autodefaults/reload, настоящая cancellation snapshot, replacement с запретом старому тренеру, logout. 320/390/1280 без overflow/JS errors, видимые кнопки ≥44 px, снимки /tmp/attendance157-{320,390,1280}.png. Реальные устройства/PWA/пользовательская приёмка остаются следующими задачами. Production Docker build и Compose clean startup/rerun/restart/readiness recovery/непубличная БД прошли. Перезапуск PostgreSQL сохранил настоящие snapshot/marks/history. SHA/CI/delivery — в отчёте issue.

## Расширение V1-11

0010 расширяет guard/value CHECK только для closed correction с отдельной change_reason и новой версией записи; snapshot/closed status/trainer запрет сохранены. Admin API и поштучная история — [corrections-v1.md](corrections-v1.md). Trainer API продолжает возвращать текущие значения, включая origin=correction, без текста объяснения. Отчёт V1-12 должен читать актуальные attendance_records закрытых сеансов.

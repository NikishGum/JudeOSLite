# Журналы филиала и коррекции V1-11

12.10.2026, [#159](https://github.com/NikishGum/JudeOSLite/issues/159). Владелец в выбранном филиале и текущий administrator читают занятия по группе/дате, снимок состава, отметки/причины и историю отдельной отметки. После закрытия исправляют одну запись с обязательным объяснением. Давность не ограничена; архив группы/карточки и перевод не меняют снимок. Архив филиала разрешает чтение истории, запрещает запись.

## Контракт

`B=/api/clubs/{clubID}/journals`, `J=B/{sessionID}`.

| Запрос | Результат / ввод |
| --- | --- |
| GET B?date=YYYY-MM-DD&groupId=UUID | Session[]; date обязательна, groupId необязателен. Фактическая локальная дата занятия в его сохранённой timezone; архивные группы доступны, retired планы исключены |
| GET J | `{session,records}`; planned/open/closed/cancelled доступны для чтения, planned до открытия возвращает пустой снимок |
| PUT J/records/{recordID} | `{version,state,absenceReason,changeReason}` → Journal. Только closed, одна запись; present/null либо absent/unknown\|illness\|excused; changeReason после TrimSpace 1–1000 Unicode-символов |
| GET J/records/{recordID}/history | Event[] в порядке created_at,id: время/автор/действие, старое и новое состояние/absenceReason/markedBy/markedAt/origin/version/changeReason; объяснение события отдельно |

Session и базовые Record — [V1-09](attendance-v1.md). Admin Record дополнительно возвращает текущую changeReason, origin может быть correction. История выбирается только для существующей записи **этого снимка и филиала**; JSON before/after преобразуется в явный список полей, произвольный payload не выдаётся. Имена сотрудника и участника актуальные, исторических снимков имён нет. Контакты/рождение/логины/секреты не выдаются. Объяснение коррекции доступно только owner/admin через этот API; trainer API не выдаёт changeReason. Общая история V1-20 показывает только безопасный envelope, включая русскую подпись corrected.

400 invalid_input (версии, строгие поля/даты, пустая/длинная/недопустимая причина); 404 not_found (чужой/несуществующий club/group/session/record или нет administrator-права); 409 version_conflict/journal_state_conflict/club_archived; безопасный 503 not_ready. Origin/CSRF и no-store действуют для всех запросов.

Ожидаемая **версия записи** проверяется всегда, включая совпадающее значение. Другая запись журнала не создаёт ложный конфликт. Значимая правка обновляет только эту attendance_record: state,absence_reason,marked_by/at,origin=correction,change_reason,version/updated_at. Present очищает absence_reason, предыдущие значения остаются в истории. Совпадающее актуальное значение — no-op без новой версии/истории и без замены прежнего объяснения; устаревшая команда даёт 409 даже при совпадении значения. При потерянном ответе изменённой команды повтор со старой версией требует перечитать журнал.

## Транзакции и миграция

0010 добавляет attendance_records.change_reason, расширяет value CHECK и guard_attendance. Старые миграции не меняются. INSERT после freeze, DELETE/смена snapshot ключей, manual/close_default в closed и correction в open/cancelled запрещены. Closed correction требует origin, новую версию и непустую причину. Тренерский endpoint сохраняет запрет closed-записи.

Lock order: current enabled actor/live session → administrator-роль (owner проходит как владелец) → общий schedule advisory lock → active club при записи → session FOR UPDATE. Все права повторно проверяются внутри транзакции, включая отзыв после auth middleware. Совместимо с journal open/mark/close/cancel/replace; фактическая trainer-роль не даёт администраторских прав.

Старое/новое состояние и обязательное объяснение пишутся в существующий audit_events(action=corrected,change_reason) **в той же транзакции**, что и запись. Ошибка аудита откатывает версию/отметку/автора/объяснение целиком. Session status/closed_at/version и snapshot сохраняются. Для V1-12 статистика читает актуальные attendance_records закрытых занятий; audit — история, не источник текущих сумм. Cancelled/retired исключать, пустой closed учитывать как занятие.

## Интерфейс и воспроизведение

«Журналы филиала»: дата, все/конкретная группа, статус/тренер, явный переход к снимку. Только одна форма коррекции открыта одновременно; обязательное объяснение отличается от причины отсутствия. История показывает автора/время/старое→новое/происхождение/объяснение. Сетевой сбой сохраняет ввод, double submit блокируется, 409 блокирует повтор до явного «Перечитать журнал и сбросить ввод». 401/404 очищают недоступные записи; смена филиала/аккаунта/unmount игнорируют поздние ответы. Права в интерфейсе не заменяют серверные проверки.

Проверки backend: `cd backend && TEST_DATABASE_URL=postgres://test:test-only@127.0.0.1:5432/test?sslmode=disable go test -race ./...`, `go vet ./...`, `go build ./cmd/server ./cmd/migrate ./cmd/create-owner`. Disposable PostgreSQL 18.6, только синтетические данные. Проверены давняя дата 2000 года, все причины/смена типа/present clearing, исторический архив/перевод, отдельные версии/no-op/stale same, owner/admin/multiple roles/trainer/foreign IDs, строгие поля/CSRF, no-store/privacy, live отзыв роли/сессии/disable после middleware, rollback аудита, 8 одновременных исправлений и SQL guards. Миграции: clean/upgrade/rerun/checksum/concurrent runner и отдельный 0009→0010 с настоящим closed snapshot.

Браузерная регрессия: `python3 frontend/e2e/corrections_v1.py`. Disposable контейнер corrections159-db (или CORRECTIONS_TEST_DB_CONTAINER с префиксом corrections159-), пользователь test и БД browser159. Сервер со собранной статикой на http://127.0.0.1:8080, APP_ORIGIN тот же, AUTH_INSECURE_LOCAL=true; bootstrap owner / synthetic-browser-password. Python Playwright и Chromium (CHROMIUM_PATH, по умолчанию /usr/bin/chromium). Скрипт создаёт уникальные клубы/группы/участников/тренера/admin через API, исторический сеанс — SQL только в тестовой БД.

UI→API→PostgreSQL→UI: группа/дата/архив, история прежнего trainer, обязательная причина, network failure/retry/сохранённый ввод, double submit, owner concurrent correction/409/reload, смена типа/причины, история обоих авторов, reload, изоляция/trainer запрет/owner club switch/live revoke/logout. 320/390/1280 без overflow/JS errors, видимые кнопки ≥44 px; снимки /tmp/corrections159-{320,390,1280}.png. Реальные устройства/PWA/приёмка остаются соответствующим задачам. SHA/CI/main — в отчёте issue.

Production Docker build и Compose clean startup/rerun/restart/readiness recovery/непубличная БД прошли. Отдельный restart disposable PostgreSQL сохранил полные fingerprints snapshot/current corrections, sessions и audit_events.

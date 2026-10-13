# Эксплуатация и восстановление V1-17

Обновлено 13.10.2026, [#165](https://github.com/NikishGum/JudeOSLite/issues/165).
Переиспользуются Compose/Caddy, digest-deploy, один migration runner,
server flock и restricted SSH из [#167](operations-test.md). Второго pipeline нет.

## Согласованная политика

Источник — [обновлённое issue и итоговый протокол владельца](https://github.com/NikishGum/JudeOSLite/issues/165#issuecomment-6140917093):

- VPS `187.7.69.230`, HTTPS `https://test.judopride.tech`; root-команды выполняет владелец.
- Копии исключительно локальные: `/srv/judeos-test/backups`, root-only.
- Ежедневно **03:00 Europe/Minsk**; хранить **7 последних завершённых** копий
  и дополнительно архивы, на которые ссылаются current/previous/operation.
- Незавершённые, неизвестные и повреждённые архивы сохранять.
- S3/SFTP/restic/offsite не используются и не являются критерием закрытия #165.

Прежняя offsite-политика от 12.10.2026 заменена этим прямым решением владельца.
Потеря VPS/диска означает потерю и данных, и локальных копий. Secrets, контакты,
пароли и dump не передавать в чат/Git. У Cloud нет настроенного SSH/VPN к VPS.

## Подтверждённая серверная приёмка

13.10.2026 владелец установил reviewed root admin.py/operations.py из main
`1b480de32d0edf8263ef9dcaccbcef676808d300`, сверил SHA-256, получил preflight PASS.
Автоматический deploy этого main 38295830684 успешен; Cloud независимо проверил
HTTPS readyz/SHA и HTTP→HTTPS 308 с включённой проверкой сертификата.

Результаты владельца из итогового протокола:

- HTTPS CRM: absent/illness → закрытый журнал → correction present с причиной;
  история/разные авторы и отчёт 1 closed / 1 present / 0 absent = 100% подтверждены.
  50% с двумя участниками были примером для Cloud, а не дополнительным критерием.
- Свежий backup `20261013T073342-d469e4ce` восстановлен в отдельную
  `restorecheck_10f99ca0b181`. Migration history, точное содержимое каждой строки
  и uploads совпали с manifest. Непустые schedule_rules=1, training_sessions=7,
  attendance_records=1, audit_events=21; 10 migrations.
- Restart-check приложения/БД сохранил все строки/файлы и подтвердил HTTPS/SHA.
  Затем удалена только проверочная БД; live DB не заменялась.
- Внешний probe 5432 с компьютера владельца завершился TCP timeout; порт
  недоступен из этой сети. Preflight проверил отсутствие опубликованных app/db ports.
- Firefox на настоящем HTTPS: secure context, активный/управляющий SW,
  11 cache entries только same-origin /assets без query/API. Обе cookie
  __Host-pride_session и __Host-pride_csrf — Secure/HttpOnly/SameSite=Strict/Path=/,
  HostOnly. Значения не публикуются.

Физическая установка iPhone/Android и матрица Chrome/Edge остаются #164.
Эти серверные PASS не повторяются без причины.

## Пользовательская приёмка и ручной запуск V1-18

Техническая зависимость #164 закрыта по согласованной Mac/iPhone матрице;
Android/Chrome отложен решением заказчика 13.10.2026, DEFERRED / NOT RUN.
Актуальные границы и SHA физических проверок — [testing-v1.md](testing-v1.md).
Серверный restore и QA164 не заменяют приёмку рабочих ролей/данных заказчиком.

Набор, роли, результаты и решение заказчика — [acceptance-v1.md](acceptance-v1.md).
Выбраны два вымышленных филиала F1/F2; отдельный контроль с аккаунтами/вводом
фактически проверен исполнителем на HTTPS. Последнее решение заказчика 13.10.2026:
«Нет, просто придумай вымышленное, и всё». #166 завершается этим согласованным
контролем; реальные филиалы/назначения/данные, обучение сотрудников и рабочий запуск
отложены. Личного просмотра интерфейса заказчиком и понимания памятки сотрудниками
не подтверждено. Дата/ответственный реального запуска не назначены.

При последующем рабочем запуске ввод данных и паролей только через защищённую CRM,
без выгрузки в Git/issue/чат. Передать [короткую памятку](staff-guide-v1.md), проверить
понимание ролей и завершения журнала, подтвердить рабочие назначения/состав и
дату начала ручного учёта. Готовность технического стенда не заменяет эти действия.

Перед сессией сверить HTTPS readyz/release SHA, записать устройства/роли и локальные
даты. При обновлении стенда во время приёмки записать оба SHA и повторить затронутые
шаги. Рабочие журналы не исправлять ради контрольных процентов; согласованные
контрольные операции выполнять отдельно. Их архив не исключает закрытые занятия
из исторического общего отчёта. SQL seed/reset/удаление QA164 для приёмки не нужны.

После запуска контролировать backup по Result/новой завершённой копии, а не одному
enabled timer, включая первый календарный запуск 14.10.2026 03:00 Europe/Minsk.
Существующая local-only политика и принятый риск потери VPS/диска сохраняются.

### Приёмка локального service/timer — 13.10.2026

[Владелец передал вывод VPS](https://github.com/NikishGum/JudeOSLite/issues/165#issuecomment-6142229067)
после установки reviewed main `3a8d56a615cdf3d7eda433a49938602c8ff293da`:

- SHA-256 четырёх source-файлов и установленных admin.py/operations.py — OK;
  systemd verify и local preflight — PASS.
- Настоящий `systemctl start judeos-backup.service`: Result=success,
  ExecMainStatus=0; inactive/dead после завершения oneshot нормально.
- local-backup.phase=ready, новая копия `20261013T090815-6f48fc14`;
  завершение 13.10.2026 09:08:24 UTC.
- Retention: keep_last=7, completed=7, removed=12, preserved_unknown=0.
  Среди сохранённых — referenced previous `20261012T191202-a8bc16dd` и
  current `20261013T082808-099bf4c2`. Неизвестных архивов в этом запуске не было;
  их сохранение и дополнительные старые references проверены policy/integration тестами.
- Timer enabled/active, OnCalendar `*-*-* 03:00:00 Europe/Minsk`, Persistent=yes,
  AccuracyUSec=1s, RandomizedDelayUSec=0. Следующий запуск — 14.10.2026
  00:00 UTC = 03:00 Europe/Minsk. Фактический service проверен ручным запуском;
  первый календарный запуск на момент приёмки ещё не наступил.

Current/accepted/operation ready соответствовали SHA 3a8d56a, generation=75.
Main CI [38368620120](https://github.com/NikishGum/JudeOSLite/actions/runs/38368620120)
и Deploy test [38369260362](https://github.com/NikishGum/JudeOSLite/actions/runs/38369260362)
успешны. Все согласованные серверные критерии #165 подтверждены.

## Развёртывание, диагностика и rollback

`deploy/test/compose.yml`: Caddy 80/443, Go с React, PostgreSQL 18.6, одноразовый
migrate. App/db не публикуют host ports; db/app имеют healthchecks, постоянные
volumes сохраняют данные/сертификаты. APP_ORIGIN HTTPS, insecure-local выключен.
DB пароль/pgpass находятся вне Git/образа в root-only `/srv/judeos-test-config`.
App работает UID 65532, read-only, без capabilities; логи ограничены ротацией.

Обновление: commit → успешный push-CI текущего main → Deploy test → GHCR digest →
server lock → stop app → backup → один migrate → start → HTTPS readiness/точный SHA.
Есть короткое окно обслуживания, без seed/reset. Старый/PR/failed CI отвергается;
admission watermark сохраняется при ошибке/rollback. Миграции имеют advisory lock
и checksum. При migration failure не запускать старый бинарник и не восстанавливать
live DB автоматически: проверить стадию/worker и повторить исправленный digest
или доставить новый проверенный commit.

```sh
python3 -I /usr/local/lib/judeos-deploy/admin.py status
python3 -I /usr/local/lib/judeos-deploy/admin.py preflight
systemctl list-units --all 'judeos-test-deploy-*' --no-pager
systemctl show judeos-backup.service --no-pager --property=ActiveState,SubState,Result,ExecMainStatus
systemctl list-timers judeos-backup.timer --no-pager
df -h /srv
curl --fail --silent --show-error https://test.judopride.tech/api/readyz
curl --fail --silent --show-error https://test.judopride.tech/release.json
```

Status `local-backup` показывает последнюю стадию, archive ID, время и retention:
keep_last/completed/archives/referenced/removed/preserved_unknown. Phase ready
означает завершённую копию, восстановленную HTTPS readiness и успешную очистку;
phase failed и ненулевой service Result требуют диагностики. Enabled timer сам
по себе не доказывает выполненный backup. Технические команды скрывают SQL/Docker
stderr; raw logs, Config.Env, env/secret файлы и строки CRM в чат не отправлять.

Rollback допускается только на previous verified image с **точно одинаковой**
migration history/checksum и подтверждённой совместимостью данных:

```sh
python3 -I /usr/local/lib/judeos-deploy/admin.py rollback --confirm-compatible
```

## Копия, retention и восстановление

```sh
python3 -I /usr/local/lib/judeos-deploy/admin.py backup
python3 -I /usr/local/lib/judeos-deploy/admin.py backup-local
python3 -I /usr/local/lib/judeos-deploy/admin.py restore-check YYYYMMDDTHHMMSS-xxxxxxxx
```

`backup` создаёт ручную копию без очистки. `backup-local` — тот же maintenance_backup
и затем согласованная retention; именно его вызывает service. Общий deploy.lock
держится от проверки verified current release до завершения cleanup. Требуются
history=current.history, phase ready/manual-rollback и работающий app. App коротко
останавливается для согласованной пары DB/uploads, в finally возвращается на ту же
версию с TLS readiness. При failed deploy или уже остановленном app команда не
запускает его сама. При backup/readiness failure cleanup не выполняется.

Архив root-only: custom-format database.dump, uploads.tar, metadata.json.
Backup проверяет pg_restore --list, SHA-256 архивов и неизменность данных до/после
копирования. Manifest содержит число и SHA-256 канонических отсортированных строк
каждой public/pride таблицы и структуру/хеши файлов. Временные COPY-файлы приватны
и удаляются, личные строки не печатаются. Предмиграционный backup использует ту же
функцию после stop app; сам deploy не запускает новую retention.

Очистка распознаёт только штатное имя YYYYMMDDTHHMMSS-xxxxxxxx с корректной датой,
root-owned каталог и ровно три обычных файла без ссылок, известную metadata
V1-19/V1-17, совпадающие SHA-256 dump/uploads, custom-dump magic и tar format.
Порядок — время записи завершённой
metadata, затем имя для одинакового времени; это учитывает несколько копий в секунду.
Хранит семь последних завершённых **всех типов** (manual/deploy/scheduled) плюс
все current/previous/operation references, даже если они старше. Повреждённые,
неполные, неизвестные форматы/файлы, дополнительные файлы/каталоги и ссылки не
попадают в семь и не удаляются. Неверная release reference запрещает cleanup.
Поэтому суммарно каталогов может быть больше семи; следить за диском и разбирать
неизвестные архивы вручную root. Во время backup/retention не менять архивы/SQL
параллельными root-командами. Перед удалением кандидат проверяется повторно.

Restore сначала проверяет checksums, создаёт случайную новую restorecheck_<12 hex>
и применяет pg_restore одной транзакцией. Сверяет history и каждую строку, включая
сотрудников/роли/состав/расписание/занятия/frozen snapshot/отметки/аудит. Uploads
распаковывает только в отдельный private temporary каталог; absolute paths/../links
и специальные файлы запрещены. Live DB/uploads не заменяются. Старые V1-19 архивы
без content manifest восстановимы, но требуют ручной предметной проверки.
При ошибке отдельная БД остаётся для root-диагностики. После успеха явно удалить
только её (live имя/wildcard отвергаются):

```sh
python3 -I /usr/local/lib/judeos-deploy/admin.py drop-restore-check restorecheck_XXXXXXXXXXXX
```

Восстановление повторять при существенном изменении схемы/backup-процедуры
и по договорённости эксплуатации. Наличие Docker image digest для recovery
проверять заранее: metadata не сохраняет образ. При реальном переключении на
восстановленную БД согласовать потерю более новых данных, подготовить DB credentials,
отозвать прежние staff_sessions до открытия доступа, проверить migrate/HTTPS/вход/
роли/отчёт/PWA. Автоматического стирания live DB нет. Без уцелевшей локальной копии
при потере VPS/диска эта политика не обеспечивает disaster recovery.

## Установка service/timer владельцем

Root на VPS использует чистый `/root/JudeOSLite` и конкретный reviewed main commit
из отчёта исполнителя. Сначала получить/сверить checkout; не использовать force/reset.
Временно приостановить новые Deploy test в Actions и остановить timer;
дождаться уже работающих и ожидающих worker/service. Установка ниже держит тот же
flock, что deploy/backup. Проверка отсутствия worker важна: Python-процесс мог
прочитать старый модуль ещё до ожидания lock. После установки/preflight вернуть
Deploy test в прежний автоматический режим.
Не менять sudoers/controller/SSH/Secrets/volumes. Прежний offsite.py, если остался
на сервере, не вызывается новым service; внешние хранилища не настраиваются.

```sh
cd /root/JudeOSLite
test -z "$(git status --porcelain)"
git fetch origin main
git merge --ff-only REVIEWED_MAIN_SHA
git rev-parse HEAD
sha256sum deploy/test/admin.py deploy/test/operations.py
if systemctl cat judeos-backup.timer >/dev/null 2>&1; then
  systemctl stop judeos-backup.timer
fi
test -z "$(systemctl list-units --state=active,activating,deactivating,reloading --no-legend 'judeos-backup.service' 'judeos-test-deploy-*')"
flock /srv/judeos-test/state/deploy.lock bash -euo pipefail <<'INSTALL'
test -z "$(systemctl list-units --state=active,activating,deactivating,reloading --no-legend 'judeos-backup.service' 'judeos-test-deploy-*')"
install -o root -g root -m 0755 deploy/test/admin.py /usr/local/lib/judeos-deploy/admin.py.new
install -o root -g root -m 0644 deploy/test/operations.py /usr/local/lib/judeos-deploy/operations.py.new
mv -T /usr/local/lib/judeos-deploy/operations.py.new /usr/local/lib/judeos-deploy/operations.py
mv -T /usr/local/lib/judeos-deploy/admin.py.new /usr/local/lib/judeos-deploy/admin.py
install -o root -g root -m 0644 deploy/test/judeos-backup.service /etc/systemd/system/judeos-backup.service
install -o root -g root -m 0644 deploy/test/judeos-backup.timer /etc/systemd/system/judeos-backup.timer
systemd-analyze verify /etc/systemd/system/judeos-backup.service /etc/systemd/system/judeos-backup.timer
systemctl daemon-reload
python3 -I /usr/local/lib/judeos-deploy/admin.py preflight
INSTALL
sha256sum /usr/local/lib/judeos-deploy/admin.py /usr/local/lib/judeos-deploy/operations.py
```

После установки проверить **настоящий systemd service**, затем включить таймер:

```sh
systemctl start judeos-backup.service
systemctl show judeos-backup.service --no-pager --property=ActiveState,SubState,Result,ExecMainStatus
python3 -I /usr/local/lib/judeos-deploy/admin.py status
systemctl enable --now judeos-backup.timer
systemctl is-enabled judeos-backup.timer
systemctl is-active judeos-backup.timer
systemctl list-timers judeos-backup.timer --no-pager
systemctl show judeos-backup.timer --no-pager --property=TimersCalendar,NextElapseUSecRealtime,Persistent,AccuracyUSec,RandomizedDelayUSec
```

Для закрытия #165 владелец передаёт: installed hashes, preflight PASS,
service Result=success/ExecMainStatus=0, local-backup.phase=ready с **новым**
archive ID/completed_at и retention (7 последних или меньше, если пока накоплено
меньше семи, плюс referenced; unknown сохранены), timer enabled/active и следующий
запуск 03:00 Europe/Minsk (00:00 UTC). Не передавать содержимое dump/metadata/Secrets.
Service oneshot после успеха обычно inactive/dead — это нормально; exit/Result и
новая копия важнее ActiveState. Первый start создаёт реальную копию сразу, не ждёт
ближайшего 03:00. Persistent запускает пропущенный backup после возвращения VPS;
при работающем deploy service ждёт общий lock. AccuracySec=1s, случайной задержки нет.
Во время ExecStart oneshot обычно activating: это занятый worker, даже если
systemctl is-active не вернул 0. Установка проверяет эти состояния и до, и после
получения lock; если появился ожидающий worker, установка отказывает до записи файлов.

## Проверки реализации

Из корня с готовым judeoslite:ci, Docker/Compose и Python:

```sh
python3 -m unittest discover -s deploy/test -p 'test_*.py' -v
python3 deploy/test/integration_check.py
```

Тесты покрывают сохранение трёх release references, семь завершённых и legacy
архивы, одинаковую секунду, incomplete/unknown/corrupt/extra files/links, отказ
очистки после backup/readiness failure и общий CLI deploy lock. Disposable PG18.6/
Caddy TLS сценарий проверяет настоящую регулярную локальную копию, cleanup реальных
архивов с references/unknown/incomplete и точный restore этой новой копии.
Service/timer синтаксис и Europe/Minsk calendar проверяются в Cloud; запуск systemd
на VPS остаётся подтверждением владельца. Restic/remote credentials не требуются.
Финальные CI/commits/серверный service результат — в live #165.

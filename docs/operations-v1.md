# Эксплуатация и восстановление V1-17

12.10.2026, [#165](https://github.com/NikishGum/JudeOSLite/issues/165).
Переиспользуются Compose/Caddy, digest-deploy, один migration runner,
server flock и restricted SSH из [#167](operations-test.md). Второго pipeline нет.

## Подтверждённое и ожидаемое

#161/#167 закрыты; их код и успешные проверки находятся в main `f8a95ea`.
Из Cloud проверены публичные TLS (certificate verification включён),
`https://test.judopride.tech/api/readyz` → `ok`, `/release.json` → полный
`f8a95eae0d6efef196bdcc890316f7c2e477c308`, manifest, sw.js с no-cache,
HTTP → HTTPS (308). CI main 38285072512 и Deploy test 38285528621 успешны.

**Для V1-17 ещё требуются ответы владельца:** использовать ли существующий
VPS `187.7.69.230` / `test.judopride.tech` как рабочий стенд первой версии,
кто выполняет root-команды, какое независимое offsite-хранилище и retention.
В #167 серверные операции выполнял владелец; у Cloud-чата нет настроенного
SSH/VPN-доступа. Приватные ключи, пароли, контакты и dump в чат/Git не присылать.
Публичная readiness не доказывает HTTPS-вход, серверный рестарт или restore.

Предложение политики, **пока не согласованное**: ежедневно около 03:00
Europe/Minsk (задержка до 5 минут), 7 дневных / 4 недельных / 3 месячных snapshots
offsite и 7 последних завершённых локальных архивов. Отдельные серверы/S3 bucket
в другом аккаунте/месте отказа предпочтительнее диска или snapshot того же VPS.
Пример конфигурации и таймер не устанавливаются/не включаются автоматически.
При отсутствии результатов VPS/offsite #165 остаётся открытой.

## Проверки в Cloud

12.10.2026: production Docker build (TypeScript/Vite/Go), 40 policy/recovery
tests, actionlint 1.7.12, Python compilation, bash syntax, git diff check;
Compose clean/rerun/restart/readiness recovery/непубличная БД — passed.
Календарь предлагаемого таймера разобран systemd-analyze с явной Europe/Minsk;
сервис/таймер на VPS ещё не установлен/не проверен.
В CI выявлена и исправлена гонка PWA setup: pg_isready теперь ждёт TCP,
который использует migrate из соседнего контейнера, а не Unix socket временного
init-сервера официального PostgreSQL image. Это изменение только теста.

`deploy/test/integration_check.py` запускает только случайный disposable Compose:
PostgreSQL 18.6, Caddy local CA с проверкой сертификата, hidden owner bootstrap,
HTTPS login/Secure/HttpOnly/SameSite cookies. Создаёт synthetic trainer,
2 participants/cards/intervals, weekly rule, 22 sessions и настоящий frozen
journal: illness → close/default unknown → correction present, отчёт 1/1=50%.
Новый restart-check, повтор deploy/migrate, локальный dump/restore и restic
encrypted **local** repository upload/check/forget/download/restore совпали
по каждой строке/файлу, включая 2 marks/13 audit events/10 migrations.
Application rollback/watermark проверены; disposable volumes удалены.

Публичный test.judopride.tech: Chromium с проверкой TLS, login screen,
secure context, действующий SW, ноль installability/JS errors, 11 cache entries
только /assets, widths 320/390/1280 без overflow. Вход реального сотрудника,
физические устройства и серверный/offsite restore этим не проверяются.

Воспроизведение (готовый образ judeoslite:ci, Docker/Compose, Python, restic):

```sh
python3 -m unittest discover -s deploy/test -p 'test_*.py' -v
bash deploy/check.sh judeoslite-ci-165
python3 deploy/test/integration_check.py
```

Если restic не в PATH, задать RESTIC_TEST_BINARY абсолютным путём к проверенному
бинарнику. На CI устанавливается пакет restic из ОС. Integration не читает
серверные secrets/каталоги и не использует рабочие DB/volumes. Финальные CI/SHA,
доставка в main и реальная приёмка публикуются в live #165.

## Развёртывание и диагностика

Рабочая конфигурация — `deploy/test/compose.yml`: Caddy 80/443, один Go с React,
PostgreSQL 18.6, migrate как одноразовая операция. App/db не публикуют host ports;
db/app имеют healthchecks, данные/сертификаты — постоянные volumes. Новый деплой
не выполняет seed/reset. APP_ORIGIN HTTPS, insecure-local выключен. DB пароль и
pgpass монтируются файлами из root-only `/srv/judeos-test-config`, вне Git/образа.
App работает UID 65532, read-only, без capabilities. Логи ограничены ротацией.

Обновление: commit → успешный push-CI текущего main → существующий Deploy test →
GHCR digest → server lock → остановка app → backup → один migrate → запуск →
TLS readiness и точный SHA. Есть короткое окно обслуживания. Старый/PR/failed CI
отвергается; admission watermark сохраняется даже при ошибке/ручном rollback.
Миграции дополнительно защищены advisory lock и checksum.

Root-диагностика на VPS:

```sh
python3 -I /usr/local/lib/judeos-deploy/admin.py status
python3 -I /usr/local/lib/judeos-deploy/admin.py preflight
systemctl list-units --all 'judeos-test-deploy-*' --no-pager
systemctl show judeos-backup.service --property=ActiveState,SubState,Result,ExecMainStatus
systemctl list-timers judeos-backup.timer --no-pager
df -h /srv
curl --fail --silent --show-error https://test.judopride.tech/api/readyz
curl --fail --silent --show-error https://test.judopride.tech/release.json
```

В status добавлены `offsite` и `offsite-restore`: стадия, archive/snapshot ID,
время, имя отдельной восстановленной БД; там нет credentials или строк CRM.
Технические команды скрывают Docker/SQL/restic stderr: он может содержать
секреты/контакты. Не отправлять raw logs, inspect Config.Env, config/env-файлы
или SQL-строки в issue/чат. Диагностику содержимого делать только локально root.
Ошибка backup/timer видна через ненулевой exit/Result; проверять время последнего
`offsite.phase=ready`, а не только состояние timer. `check` проверяет индексы,
фактическое чтение данных доказывает отдельный `restore-check`.

После migration failure не стартовать старый бинарник и не восстанавливать live
БД автоматически: выяснить стадию/worker, исправить и повторить тот же digest или
доставить новый проверенный commit. Rollback — только previous verified image,
точно одинаковая migration history/checksum и подтверждённая совместимость данных:

```sh
python3 -I /usr/local/lib/judeos-deploy/admin.py rollback --confirm-compatible
```

## Установка обновлённых операций

Владелец получает проверенный commit main на VPS в `/root/JudeOSLite`.
При обновлении root-owned операций сначала приостановить новые Actions deploy,
остановить backup timer и дождаться уже работающих worker/service. Остановка timer
сама не завершает активный service. Снять только временную паузу после preflight.

```sh
cd /root/JudeOSLite
git status --short
git rev-parse HEAD
systemctl stop judeos-backup.timer  # если таймер уже был установлен
bash deploy/test/bootstrap.sh
python3 -I /usr/local/lib/judeos-deploy/admin.py preflight
```

Bootstrap сохраняет существующие secrets, volumes, controller.py, sudoers и ключи;
обновляет admin.py/operations.py, staging controller и offsite.py. Протокол
controller в V1-17 не меняется, повторная активация/SSH reload не нужны.
Новых sudo-грант, Docker-группы или shell для judeos-deploy нет.

## Локальная копия и предметное восстановление

```sh
python3 -I /usr/local/lib/judeos-deploy/admin.py backup
python3 -I /usr/local/lib/judeos-deploy/admin.py restore-check YYYYMMDDTHHMMSS-xxxxxxxx
```

Ручной backup теперь требует verified current release, совпадение migration
history, phase ready/manual-rollback и работающий app. В том же server lock
останавливает app, сохраняет копию и в finally запускает эту же версию с TLS
readiness. При прерванном/неуспешном deploy или уже остановленном app не запускает
его сам. Предмиграционный backup использует ту же функцию после stop app.
Не делать параллельных ручных SQL-записей/изменений uploads во время backup.

Архив root-only: custom-format `database.dump`, `uploads.tar`, `metadata.json`.
Metadata содержит SHA-256 архивов, migration history и для каждой таблицы
public/pride число и SHA-256 всех канонических отсортированных JSON-строк, а для
uploads — структуру/хеши файлов. Личные данные не печатаются; временные COPY-файлы
приватные и удаляются. Содержимое до/после dump должно совпадать; при изменении
backup отказывает без завершённой metadata. Неполные архивы не отправляются
offsite и автоматически не удаляются. Следить за диском и разбирать их root.

Restore сначала проверяет checksum, создаёт случайную **новую**
`restorecheck_<12 hex>` и применяет pg_restore одной транзакцией. Сравнивает
history и точное содержимое каждой таблицы: сотрудники/сессии/роли, участники,
карточки, группы/интервалы, правила/занятия, frozen snapshot/отметки и аудит.
Одного совпадения количества строк недостаточно. Uploads распаковываются в
отдельный private temporary каталог и сверяются; абсолютные пути, `..`,
symlink/hardlink и специальные файлы запрещены. Live DB/uploads не заменяются.
При успехе выводятся только имя БД, PASS и counts таблиц; отдельно проверить,
что нужные предметные таблицы **не пусты**. Старые V1-19 архивы без manifest
можно восстановить, но автоматическая предметная проверка откажет; они требуют
ручной сверки. Для V1-17 взять новую копию.

После проверки удалить только отдельную БД явной командой (live имя отвергается):

```sh
python3 -I /usr/local/lib/judeos-deploy/admin.py drop-restore-check restorecheck_XXXXXXXXXXXX
```

При ошибке restore отдельная БД остаётся для root-диагностики. Перечень таких БД
посмотреть локально через PostgreSQL, не удалять по wildcard. Эта команда не
отключает активные подключения принудительно и не удаляет volumes.

## Зашифрованное offsite-хранилище и расписание

После согласования хранилища установить restic из доверенного пакета ОС:
`apt-get update && apt-get install -y restic`. В Cloud проверяется restic 0.19.1;
использованы базовые команды backup/check/forget/restore.

Root создаёт config 0600 на основе `deploy/test/offsite.example.json` и локально
редактирует endpoint/bucket, credentials, retention и local_keep. Это пример
S3, не готовый bucket и не подтверждение подключения. Не включать сетевой
backend без согласования. `offsite.py` допускает restic S3 HTTPS, SFTP и другие
поддерживаемые удалённые backend; shell не используется, credential env ограничен.
Для SFTP root заранее настраивает ключ/known_hosts по доверенному fingerprint;
отключать host-key/TLS verification нельзя.

```sh
install -o root -g root -m 0600 deploy/test/offsite.example.json /srv/judeos-test-config/offsite.json
editor /srv/judeos-test-config/offsite.json
python3 - <<'PY'
import getpass, os
password = getpass.getpass('Restic repository password from the owner password manager: ')
if len(password) < 16 or password != getpass.getpass('Repeat password: '):
    raise SystemExit('Password not saved.')
fd = os.open('/srv/judeos-test-config/restic-password', os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, 'w') as stream:
    stream.write(password + '\n')
PY
python3 -I /usr/local/lib/judeos-deploy/offsite.py init
python3 -I /usr/local/lib/judeos-deploy/offsite.py backup
python3 -I /usr/local/lib/judeos-deploy/offsite.py restore-check FULL_64_HEX_SNAPSHOT_ID
```

Пароль репозитория и credentials должны быть сохранены также в отдельном
защищённом хранилище владельца: файлы только на потерянном VPS не позволяют
восстановиться. Не класть пароль рядом с зашифрованным repository. Серверные DB
secret files отдельно сохранять зашифрованно у владельца; GitHub/GHCR/SSH tokens
не добавлять в DB backup. Init выполняется явно только для нового repository;
для существующего использовать `check`, не переинициализировать его.

Offsite backup держит общий deploy lock; app останавливается лишь на создание
согласованного локального архива и возвращается до сетевой передачи. Restic
шифрует содержимое, сохраняет snapshot host=judeos-test/tag=judeos-v1,
проверяет repository и только затем применяет согласованный forget/prune.
Retention группируется **host,tags**: меняющийся путь архива не создаёт вечную
группу на каждую копию. Локальная очистка только после успеха remote-команд,
сохраняет архивы current/previous/operation даже сверх local_keep. Сбой upload
или check не запускает retention; незавершённые/неизвестные архивы не стираются.

Restore принимает полный immutable snapshot ID, сверяет host/tag, скачивает
архив в отдельный private каталог и выполняет тот же настоящий предметный
restore-check. Изолированная БД остаётся для проверки, staging files удаляются.
В state offsite-restore фиксируется snapshot/БД/время успеха. Проверка локального
repository в CI не доказывает доступ к настоящему offsite месту.

Только после первого успешного upload → download → restore и утверждения
расписания root устанавливает reviewed service/timer (пример 03:00 Minsk):

```sh
install -o root -g root -m 0644 deploy/test/judeos-backup.service /etc/systemd/system/judeos-backup.service
install -o root -g root -m 0644 deploy/test/judeos-backup.timer.example /etc/systemd/system/judeos-backup.timer
systemd-analyze verify /etc/systemd/system/judeos-backup.service /etc/systemd/system/judeos-backup.timer
systemctl daemon-reload
systemctl enable --now judeos-backup.timer
systemctl list-timers judeos-backup.timer --no-pager
```

Persistent запускает пропущенную копию после возвращения сервера; timezone
задаётся явно, до 5 минут jitter. Второй ручной/таймерный backup сериализуется с
deploy/restore/rollback. Проверить хотя бы один реальный запуск **через service**,
Result=success и новый ready snapshot; enabled timer сам по себе не доказывает
выполненную копию. Восстановление повторять после существенного изменения
схемы/backup-процедуры и периодически по договорённости эксплуатации.

## Приёмка владельцем и восстановление после потери VPS

На согласованном HTTPS стенде создать синтетический филиал/тренера/двух участников
и группу, правило weekly и отдельное занятие, открыть журнал, отметить/закрыть,
исправить одну закрытую отметку с причиной и сверить отчёт владельца. Пароли/ФИО
реальных людей не передавать. Проверить вход и Secure/HttpOnly/SameSite=Strict
cookie, manifest/SW/static-only cache и текущую сессию после запуска PWA.
Матрица физических iPhone/Android/Chrome/Edge и установки остаётся #164.

Снять backup/content counts, перезапустить **app и БД** без удаления volumes:

```sh
python3 -I /usr/local/lib/judeos-deploy/admin.py restart-check
```

Команда держит общий lock, требует verified release, останавливает app, снимает
manifest, перезапускает db/ждёт health, возвращает app и сверяет каждую строку/файл.
В случае ошибки БД не восстанавливается автоматически; `restart-check` в status
показывает последнюю стадию. Проверять в согласованное окно без ручных записей.
Переход месяца может законно добавить новый горизонт генерации: несовпадение
в этом случае требует разбора и повторной проверки, а не обещания PASS.

Дождаться HTTPS readiness:
дождаться HTTPS readiness: сеансы не дублируются, snapshot/marks/audit/отчёт
сохраняются. Выполнить локальный и затем offsite restore свежей копии; PASS должен
покрывать непустые staff/group_memberships/schedule_rules/training_sessions/
attendance_records/audit_events. Проверить непубличность 5432 извне; Compose
отсутствие host ports и внешний сетевой probe — разные свидетельства.
Передавать только SHA/run IDs, PASS/агрегаты, snapshot/restore DB IDs, результаты
service/timer/cookies/портов, без секретов или содержимого CRM.

Disaster recovery: остановить новые deploy/backup, получить пароль репозитория
из независимого хранилища; на подготовленном втором VPS/отдельной БД скачать
выбранный snapshot, проверить manifest/data и доступный совместимый image digest.
Dump содержит хеши staff-паролей/серверные сессии: после реального переключения
отозвать прежние staff_sessions до открытия внешнего доступа. Создать новые
серверные DB credentials и настроить secret files для новой роли/БД. Переключение
проверенной конфигурации/volume и возможную потерю более новых записей согласовать
с владельцем; автоматического стирания/замены live-БД нет. Пройти migrate/HTTPS,
вход/три роли/отчёт/PWA и новый offsite backup. Наличие старого digest в registry
проверить заранее; backup metadata сама не сохраняет Docker image.

# Ранний тестовый стенд: ручная подготовка и эксплуатация

12.10.2026, [V1-19 / #167](https://github.com/NikishGum/JudeOSLite/issues/167).
Агент готовит PR и проверки в Cloud. Владелец выполняет все команды VPS и
заполняет GitHub Secrets самостоятельно. **Реальный деплой ещё не проверен.**
V1-17 (#165) после V1-13 проверяет полную CRM, вход/PWA, регулярный offsite
backup и восстановление предметных данных; этот ранний pipeline переиспользуется.

## Окончательные параметры

GitHub → Settings → Environments → **test**. Разрешить deployment только ветви
`main`. Для автоматического деплоя environment не должен требовать ручного
одобрения каждого запуска. GitHub Secrets заполняет владелец, значения в чат
не присылаются.

| Environment Variable | Значение |
| --- | --- |
| `DEPLOY_HOST` | `187.7.69.230` |
| `DEPLOY_PORT` | `22` |
| `DEPLOY_USER` | `judeos-deploy` |
| `TEST_DOMAIN` | `test.judopride.tech` |
| `DEPLOY_PATH` | `/srv/judeos-test` |

| Environment Secret | Назначение |
| --- | --- |
| `DEPLOY_SSH_KEY` | Отдельный приватный Ed25519-ключ Actions без passphrase |
| `DEPLOY_KNOWN_HOSTS` | Доверенная строка `187.7.69.230 ssh-ed25519 …`, полученная из публичного ключа VPS через административный канал |

`GITHUB_TOKEN` предоставляет GitHub автоматически: публикация `packages: write`,
чтение репозитория; SSH job имеет только `contents: read`, `actions: read`.
Не добавлять GHCR PAT или пароль БД в Environment. Образ:
`ghcr.io/nikishgum/judeoslite-test:<полный SHA>`; VPS использует только `@sha256:…`.
Пути фиксированы в root-owned контроллере; изменение Variables без новой
проверенной серверной конфигурации отвергается.

## Подтверждённые исходные сведения, ещё не готовность

srv2042768.hstgr.cloud (ID 2042768), Ubuntu 24.04.5 LTS x86_64, 1 CPU / 4 GB RAM /
50 GB, Docker 29.8.2, Compose plugin 5.6.0. Bootstrap проверяет существующие Docker
и `docker compose`, не переустанавливает их. SSH root:22 для владельца.
`judeos-deploy`: uid 999, gid 987, `/var/lib/judeos-deploy`, `/bin/sh`, без дополнительных
групп. Подтверждено пользователем: root:root 0755 контроллер, root:root 0700
`/srv/judeos-test` и `/srv/judeos-test-config`, одна sudo-гранта только контроллеру,
существующий ключ с `restrict` и forced command.

Hash прежнего контроллера:
`9019773bf7e360b0ec34e85fa005d36554356948f656f60473b6e9f5358be952`.
Его функции `trusted`, `capture`, `trusted_tree`, `preflight`, `apply`, `main` известны;
исходник/протокол не проверены. Новый протокол **не объявляется совместимым**
с прежним: нужна явная ручная активация после изучения кода и сохранения копии.
Доступ к Docker и неограниченный sudo пользователю не предоставляются.

DNS A `test.judopride.tech → 187.7.69.230`, TTL 300 сообщён владельцем.
TLS не проверен. Старое приложение удалено, каталоги сохранены; прежние secret
файлы не читаются/не импортируются. Новые volumes имеют суффикс `v1-19` и не
подхватывают неизвестную старую БД. После первого запуска они сохраняются.
Остановленный Hostinger Traefik с `unless-stopped` может вернуть конфликт 80/443.
UFW/Hostinger firewall не настроены, ОС просит reboot, пригодный backup неизвестен.
VPS оплачен до **7 ноября 2026**, автопродление выключено; проверить продление.

## Bootstrap и точная команда

Владелец помещает проверенный checkout PR в `/root/JudeOSLite` на VPS. После
merge использовать конкретный проверенный commit main, а не произвольный
скачанный скрипт. Если репозиторий private, настроить чтение своим обычным
административным способом; не помещать токен в URL или shell history.

```bash
cd /root/JudeOSLite
git status --short
git rev-parse HEAD
bash /root/JudeOSLite/deploy/test/bootstrap.sh
```

Скрипт `deploy/test/bootstrap.sh` запускается **root на VPS**, без аргументов.
Он проверяет безопасное владение каталогами, копирует Compose/Caddy и root-only
операции, создаёт каталоги state/backups/uploads и новые DB-secret файлы.
Он **сохраняет действующий controller.py, sudoers, authorized_keys, firewall и
старые secret файлы**. Повторный bootstrap сохраняет новые секреты и volumes;
обновление установленного кода во время деплоя запрещено — сначала дождаться
завершения worker и остановить новые запуски Actions.

После ручного изучения `controller.v1-19.py`, `admin.py`, `operations.py`:

```bash
python3 -I /usr/local/lib/judeos-deploy/admin.py activate-controller \
  --expected-old-sha256 9019773bf7e360b0ec34e85fa005d36554356948f656f60473b6e9f5358be952
```

Активация откажет, если прежний файл изменился, и сохранит root-only архив.
**Не менять sudoers на ALL, не добавлять пользователя в docker/sudo.**
Старый forced command может использовать прежний протокол; Actions использует
новый отдельный ключ и фиксированную команду ниже. Остальные старые ключи
владелец отдельно проверяет/удаляет при ненадобности; bootstrap их не удаляет.

Уточнение по диагностике владельца: старый ключ вызывает root-owned
`/usr/local/lib/judeos-deploy/ssh-entry.py` (0755), с hash
`85a9dd8c570d315e563b0b83e71caf698916388efa0320193e17e58946f21af3`.
Preflight сохраняет эту существующую точку входа только при точном совпадении
пути/hash и безопасном владении; допускается прямой вызов либо абсолютный
`/usr/bin/python3` с необязательным `-I`, без иных аргументов. Это не проверка
совместимости старого протокола с новым контроллером. Новый Actions-ключ обязан
по-прежнему вызывать контроллер напрямую. Порядок `restrict`/`command` не влияет
на проверку; опции, повторно включающие forwarding/PTY/user-rc, запрещены.
Сообщение об отказе указывает номер строки, не раскрывая ключ или команду.

При обновлении только этой проверки из проверенного checkout достаточно
атомарно заменить operations.py, когда worker не выполняется:

```bash
install -o root -g root -m 0644 /root/JudeOSLite/deploy/test/operations.py /usr/local/lib/judeos-deploy/operations.py.new
mv -T /usr/local/lib/judeos-deploy/operations.py.new /usr/local/lib/judeos-deploy/operations.py
python3 -I /usr/local/lib/judeos-deploy/admin.py preflight
```

Повторная активация контроллера, переустановка ключей и повторный ввод секретов
для этой правки не требуются.

## Публичный SSH-ключ Actions и host key

Создать отдельную пару ключей в доверенной административной среде владельца.
Например, root на VPS (файлы с `umask 077`, без печати приватного ключа):

```bash
umask 077
ssh-keygen -t ed25519 -N '' -C github-actions-judeos-test -f /root/judeos-actions-test
python3 -I /usr/local/lib/judeos-deploy/admin.py install-key /root/judeos-actions-test.pub
```

Installer добавит только публичный ключ с:
`restrict,command="/usr/bin/sudo -n /usr/local/lib/judeos-deploy/controller.py"`.
Он делает home/.ssh root:root 0755 и authorized_keys root:root 0644,
чтобы sshd мог читать публичный файл от UID deploy-user, а deploy-user не мог
снять ограничения заменой файла/родителя. Другие ключи сохраняются.
`restrict` включает запрет PTY, forwarding и user rc; отдельный `no-pty` не нужен.
Владелец заносит приватный файл в Environment Secret `DEPLOY_SSH_KEY` через
доверенный интерфейс/CLI GitHub. Если `gh` уже настроен у владельца:

```bash
gh secret set DEPLOY_SSH_KEY --repo NikishGum/JudeOSLite --env test < /root/judeos-actions-test
```

Получить **публичный** host key из VPS по уже доверенному административному
каналу, а не слепо доверять `ssh-keyscan` из CI:

```bash
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
awk '{print "187.7.69.230 " $1 " " $2}' /etc/ssh/ssh_host_ed25519_key.pub > /root/judeos-known-hosts
gh secret set DEPLOY_KNOWN_HOSTS --repo NikishGum/JudeOSLite --env test < /root/judeos-known-hosts
```

Если `gh` на VPS отсутствует, использовать доверенный интерфейс GitHub и файл;
не устанавливать его только ради этого шага. Приватный ключ не отправлять в чат,
issue или Git. После сохранения в Secrets и резервном хранилище владельца удалить
временный приватный файл с VPS. При ротации установить новый restricted public
key, заменить Secret, проверить деплой, затем убрать только прежний ключ.
При смене host key независимо подтвердить новый fingerprint, затем обновить Secret.

## Серверные секреты и безопасное заполнение

| Файл/доступ | Заполнение |
| --- | --- |
| `/srv/judeos-test-config/db-password-v1-19` | Bootstrap генерирует криптографически случайный пароль на VPS, root:root 0600 |
| `/srv/judeos-test-config/pgpass-v1-19` | Автоматически из того же пароля; root:root 0444 внутри root-only 0700 каталога, в app/migrate монтируется только этот файл |
| `/srv/judeos-test-config/registry/config.json` | При private GHCR root вводит отдельный PAT `read:packages` скрытым prompt `registry-login`; файл 0600, каталог 0700 |
| Пароль первого владельца | Скрытый двойной prompt `admin.py create-owner`; в БД только Argon2-хеш, отдельного env-файла нет |

V1-02 уже появилась в main во время подготовки PR. Пароль первого владельца
вводится после первого деплоя только скрытым терминальным prompt команды ниже;
на сервере хранится его Argon2-хеш в БД. Отдельного статического session secret
текущая реализация не требует. Старые `api-secrets.env`,
`bootstrap-secrets.env`, `migration-secrets.env`, `test.env`, `db-password`
с прежней установки не являются проверенной конфигурацией.

Для private GHCR создать отдельный read-only token аккаунта, которому доступен
этот package, с `read:packages` и нужной организационной SSO-авторизацией.
Не использовать токен публикации/`write:packages` на VPS. Логин:

```bash
python3 -I /usr/local/lib/judeos-deploy/admin.py registry-login
```

Токен вводится только скрытым prompt и передаётся `docker login --password-stdin`;
значение не появляется в argv/history/logs. При public package логин не нужен.
Для ротации registry token повторить login, проверить pull нового digest и
отозвать прежний токен. DB password нельзя просто заменить в файлах после init:
он не меняет пароль существующего PostgreSQL role. Процедура ротации:
maintenance/backup → смена role через root-only административный DB-сеанс без
логирования SQL → согласованное обновление обоих файлов → пересоздание app/migrate
→ readiness. Перед фактической ротацией реализовать/проверить её на копии БД;
никакого автоматического reset БД для ротации.

## Firewall, Traefik и reboot — вручную владельцем

Не отключать управляемый Traefik из агента. Владелец в Hostinger отключает его
автоматическое управление и решает, нужен ли контейнер. Preflight отказывает,
пока любой контейнер с именем traefik остаётся: одного `stop` недостаточно.
Если подтверждено, что он больше не нужен, владелец явно удаляет **этот**
остановленный контейнер; его volumes по умолчанию не удалять.

Без вывода секретов:

```bash
docker ps -a --format '{{.Names}} {{.Status}} {{.Ports}}'
docker inspect traefik-sjo7-traefik-1 --format '{{.State.Status}} {{.HostConfig.RestartPolicy.Name}}'
ss -lnt '( sport = :22 or sport = :80 or sport = :443 )'
getent ahostsv4 test.judopride.tech
df -h /srv
test ! -e /var/run/reboot-required || echo REBOOT_REQUIRED
```

В панели Hostinger разрешить inbound TCP **22, 80, 443**; сохранить доступ root
с административного адреса. При ограничении SSH только админским IP учесть, что
GitHub hosted runner имеет меняющиеся адреса: без отдельного решения такой
allowlist заблокирует Actions. UDP 443 не нужен для этого Caddyfile.

В существующей root-сессии сначала разрешить SSH, затем HTTP/HTTPS:

```bash
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw default deny incoming
ufw default allow outgoing
ufw enable
ufw status numbered
```

Проверить **второй** административный SSH-сеанс до закрытия первого.
После планового reboot вновь проверить доступ, Docker, DNS и порты.
Docker published ports могут обходить UFW; Compose публикует только Caddy
80/443, app/db/migrate вообще не имеют host ports. Внешнюю недоступность 5432
владелец дополнительно проверяет с другого узла; не считать UFW единственной защитой.

## Команда проверки готовности VPS

```bash
python3 -I /usr/local/lib/judeos-deploy/admin.py preflight
```

Проверяет root-owned tree/config/secret consistency, активированный контроллер,
restricted key, ровно одну sudo-гранту, отсутствие дополнительных групп,
Docker/Compose/config, закрытые DB/app ports, uploads permissions, DNS, активный
UFW, необходимость reboot и конфликт Traefik/портов. Успех — локальная подготовка;
доступ GHCR, firewall Hostinger, реальный SSH из Actions, сертификат/HTTPS и PWA
подтверждаются отдельными реальными проверками владельца.

## Порядок первого деплоя

1. Проверить код PR. Подготовить checkout на VPS, bootstrap, GHCR login (private),
   активировать контроллер и установить новый public key.
2. Устранить Traefik, настроить оба firewall, проверить второй SSH-сеанс,
   выполнить reboot и повторить preflight. Сохранить доступ root для диагностики.
3. Создать Environment `test` с branch policy main, заполнить пять Variables
   и два Secrets. Включить Actions и доступ `GITHUB_TOKEN` к публикации GHCR.
4. После готовности владельца merge PR в main. До готовности Environment/VPS
   оставлять PR открытым: merge активирует автоматический pipeline.
5. Дождаться успешного **Build and checks**, event push, main. **Deploy test**
   сам проверит run/repository/head SHA, соберёт и проверит точный commit,
   опубликует SHA-tag/digest и повторно сверит актуальный main перед SSH.
6. Сервер резервирует CI generation, получает digest, проверяет revision-label,
   Caddy и Compose, запускает db, останавливает app для согласованного backup,
   делает dump/uploads backup, выполняет один migrate, запускает app/Caddy,
   проверяет HTTPS `/api/readyz` и `/release.json` с ожидаемым SHA.
7. Владелец проверяет стенд и сообщает **только** SHA/статусы/ошибки без секретов:

```bash
curl --fail --silent --show-error https://test.judopride.tech/api/readyz
curl --fail --silent --show-error https://test.judopride.tech/release.json
python3 -I /usr/local/lib/judeos-deploy/admin.py status
python3 -I /usr/local/lib/judeos-deploy/admin.py backup
```

После первого успешного deploy создать первого владельца из интерактивной
root-сессии VPS (пароль 12–128 байт вводится дважды скрыто, не через argv/env/file):

```bash
python3 -I /usr/local/lib/judeos-deploy/admin.py create-owner --login owner --name 'Владелец сети'
```

Повторный запуск откажет, если владелец уже есть. Это явная bootstrap-операция
V1-02, а не seed/reset при деплое. Проверить вход и защищённые cookie в браузере
на HTTPS. Compose задаёт `APP_ORIGIN=https://test.judopride.tech`,
`AUTH_INSECURE_LOCAL=false`; значения неизменны для этого стенда.
После создания владельца сделать новый backup и проверить restore этой копии
в отдельную БД: она уже должна содержать созданного владельца.

Первый запуск создаёт новую БД; он не импортирует старые данные и не запускает seed.
Повторный deploy/restart сохраняет volumes, uploads и сертификаты Caddy.
Один CPU/4 GB: сборка только на GitHub runner, на VPS только pull/migrate/start.
Обновление даёт короткий maintenance window; zero-downtime здесь не обещается.

## Ручной деплой и устаревшие запуски

Actions → **Deploy test → Run workflow → main → ci_run_id**: числовой ID успешного
push-run **Build and checks** для **текущего** SHA main (из URL Actions).
Ручной запуск не обходит CI, branch/repo check или watermark. PR/fork/failed CI
отвергаются; старый run при изменившемся main отвергается до SSH.
Actions concurrency: один выполняющийся workflow, `cancel-in-progress: false`.
GitHub может заменять ожидающий pending-run более свежим; выполняющийся деплой
не отменяется. Отмена pending-run до SSH не запускает миграции.
Server flock охватывает backup/migrate/start/restore/rollback. Watermark
записывается до изменений и сохраняется даже при ошибке. Повтор возможен для
того же generation+SHA+digest; конфликтующий digest того же CI запрещён.
Если SHA-tag уже опубликован, повторная сборка должна переиспользовать его digest
(см. workflow). Не удалять watermark ради старого CI. Для возврата к старому коду
предпочтителен новый revert-commit с успешным CI.

SSH-запрос не переносит compose/scripts и не даёт shell. systemd worker работает
независимо от SSH/Actions, `TimeoutStartSec=0` не обрывает миграцию. При timeout
Actions сначала проверить status/unit; не отменять worker и не запускать restore.
Миграции Go имеют собственный timeout и transaction/advisory lock.

## Backup, проверка restore и recovery

```bash
python3 -I /usr/local/lib/judeos-deploy/admin.py backup
python3 -I /usr/local/lib/judeos-deploy/admin.py restore-check YYYYMMDDTHHMMSS-xxxxxxxx
```

`backup` сохраняет custom-format dump, uploads.tar и metadata/checksum в
`/srv/judeos-test/backups/<имя>`, root-only. Dump проверяется `pg_restore --list`; для dump и uploads сохраняются SHA-256.
Архивы обрабатываются потоком, не целиком в RAM VPS.
Автоматический предмиграционный backup делается с остановленным app; ручной
backup вне maintenance имеет транзакционно согласованную БД, но DB/files могут
меняться независимо, поэтому для согласованной пары владелец останавливает записи.
`restore-check` проверяет checksum и восстанавливает **новую отдельную** БД
`restorecheck_<случайный suffix>` одной транзакцией, сравнивает history; live DB
не затрагивается. При ошибке отдельная БД остаётся для root-диагностики.
Проверить содержимое нужных таблиц/количество строк и извлечь uploads.tar в
**отдельный root-only каталог**, не поверх live uploads. После проверки явно
удалить только отдельную restorecheck-БД. Фундамент содержит лишь schema pride;
проверка пользователей/занятий/отметок/аудита выполняется в V1-17 после их реализации.

Это пока локальные backups без автоматической очистки/таймера. Скопировать
проверенный backup и серверные secret файлы в отдельное зашифрованное хранилище
владельца, проверить чтение копии. Не считать Hostinger weekly schedule пригодной
копией при 0 snapshots. До накопления данных согласовать offsite/retention и
регулярный timer в V1-17; проверять `df -h /srv` перед релизом. В Cloud runtime
restore проверяется в disposable PostgreSQL, не на реальном VPS.

Для реального disaster recovery: запретить новые deploy, дождаться worker,
сохранить свежий backup текущего состояния, развернуть backup **в отдельную БД**,
проверить данные и совместимый image, затем явно переключить проверенную
конфигурацию/volume под root и пройти readiness. У этой операции нет команды
для автоматического стирания live-БД; согласовать возможную потерю новых данных.

## Ошибки и откат приложения

```bash
python3 -I /usr/local/lib/judeos-deploy/admin.py status
systemctl list-units --all 'judeos-test-deploy-*' --no-pager
systemctl show judeos-test-deploy-N --property=ActiveState,SubState,Result,ExecMainStatus
```

Не выполнять `docker compose config` без `--quiet` на старой secret-конфигурации,
не печатать `docker inspect ...Config.Env`, env/secret files или необработанные
логи в чат. Наш worker выдаёт только стадии и общие ошибки, не Docker/psql stderr.
`operation.json` показывает последнюю стадию и backup. После ошибки миграции
app может остаться остановленным; **нет автоматического** старта старого бинарника
или восстановления БД. Исправить причину и повторить **тот же digest**, либо
доставить новый проверенный commit. Проверить Docker disk usage и доступ GHCR.

Если владелец подтвердил совместимость кода с текущей схемой:

```bash
python3 -I /usr/local/lib/judeos-deploy/admin.py rollback --confirm-compatible
```

Команда допускает только предыдущий успешно проверенный image и **точное
совпадение migration history/checksums**, затем readiness/SHA. Разница истории
запрещает rollback даже при предположительно обратной совместимости: нужен
исправленный commit. Watermark не уменьшается. Совпадение history необходимо,
но совместимость семантики данных дополнительно подтверждает владелец.

## Непроведённая приёмка

Владелец должен подтвердить preflight, Secrets/SSH/GHCR, HTTPS/certificate,
правильный SHA, сохранение данных при повторном deploy, отказ failed/stale CI,
серверную блокировку, реальный backup/restore и совместимый rollback. Вход V1-02
уже есть в main: реальный HTTPS-вход проверяет владелец после create-owner.
Установка PWA пока не реализована; её приёмка остаётся после V1-13. Ни issue #167,
ни V1-17 не закрывать по одному PR или Cloud-прогону.

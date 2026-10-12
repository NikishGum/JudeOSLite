# Проверки V1-19 / #167

12.10.2026. Агент работал только в Cloud: к VPS не подключался, серверные
secret файлы/ключи владельца не читал. Результаты VPS ниже сообщил владелец.

## Первый успешный реальный деплой

[Deploy test 38250186366, attempt 2](https://github.com/NikishGum/JudeOSLite/actions/runs/38250186366)
завершился success для main `90e09c322c13c854a51f816d11b1bb7e21084fa1`.
Владелец отдельно проверил публичный HTTPS без отключения TLS verification:
`/api/readyz` вернул `{"status":"ok"}`, `/release.json` — этот же полный SHA.
Таким образом, первый Actions SSH → GHCR pull → предмиграционный backup →
миграции → запуск → HTTPS-проверка завершён. Владелец ранее подтвердил
активацию исправленного controller и local preflight; sudoers не расширялись.

Повторный deploy с проверкой сохранения данных, реальный restore в отдельную
БД и безопасный rollback ещё не подтверждены. PR #169 с исправлениями SSH,
GitHub API retry и transient timeout пока открыт; issue #167 остаётся открытой
до результата в main и остальных критериев. PWA/полная CRM/offsite recovery
по-прежнему относятся к V1-17 после V1-13.

## Проверки агента в Cloud

- 29 Python tests: selection только успешного push-CI текущего main, отказ
  PR/fork/failed/old CI, ограниченный digest/SHA payload, root ownership/symlinks,
  запуск worker через systemd без привязки к SSH, настоящий flock двух workers,
  порядок backup/migration/readiness, reservation при ошибке миграции,
  запрет rollback при изменённой истории, отказ restore в live-БД, сохранение
  прежних secret файлов, readable/nonwritable restricted authorized_keys,
  Compose без DB/app ports, build/socket/inline DB password.
- `actionlint v1.7.12`: deploy-test.yml и изменённый build.yml — passed.
- `bash -n deploy/test/bootstrap.sh`, Python compile, `git diff --check` — passed.
- Production Docker build из актуального main с V1-02 + PR: frontend typecheck /
  production build, server/migrate/create-owner binaries, nonroot runtime,
  revision label и release.json — passed. BuildKit proxy CA передан как secret.
- Базовый deploy/check.sh: PostgreSQL 18.6, clean startup, health/readiness,
  503 при остановке DB и recovery, повтор migrate, restart и port checks — passed.
- Настоящий disposable Compose из тестового шаблона: Caddy 2.10.2 с локальным
  CA и проверкой TLS certificate; HTTPS readiness/SHA; скрытый терминальный
  bootstrap владельца V1-02, HTTPS login и Secure/HttpOnly/SameSite cookies;
  два deploy с сохранением DB-row и upload-file; custom-format dump/checksum,
  restore в отдельную БД с проверкой row/history и uploads archive; application
  rollback при совпадающей истории; watermark после rollback не уменьшается — passed.
  Все test volumes/network удалены только в случайном disposable project.

У интеграционного теста заменены только VPS-specific paths/project/preflight
и локальный image pull. Root ownership/sudo/systemd, Hostinger firewall, DNS/ACME,
SSH из Actions, реальный GHCR publish/private pull и публичный TLS были
подтверждены первым deploy выше. Сохранение данных и PWA ещё требуют проверки
владельцем. Ошибку миграции
проверяет unit test; реальный серверный failure/recovery drill ещё не выполнен.

При первоначальной подготовке Environment `test` не удалось создать из Cloud:
GitHub API подключённого аккаунта вернул HTTP 403
`Resource not accessible by integration` на create/update environment. Настройка
Environment, Variables и Secrets описана в operations-test.md; затем владелец
самостоятельно создал и настроил Environment. Ни одно secret значение не
отправлялось в GitHub из Cloud.

Issue остаётся open/In progress до результата в main и реальной приёмки.
V1-17 сохраняет ответственность за полную CRM/PWA, регулярный offsite backup
с согласованным retention и восстановление предметных данных после V1-13.

## История диагностики до успешного деплоя

Уточнение по выводу владельца: preflight остановился на старом restricted ключе
с forced command через ssh-entry.py, тогда как новый Actions-ключ указывал
на controller.py напрямую. Исправлена проверка промежуточного обработчика:
только подтверждённый путь/root ownership/SHA-256, без произвольных аргументов,
и отдельный canonical Actions-ключ всё ещё обязателен. Дополнительно исправлен
разбор порядка опций, quoted spaces/commas; проверены отказ изменённому wrapper,
shell/аргументам, подделке restrict и повторному включению forwarding. Пять новых
regression tests прошли в Cloud. Владельцу остаётся обновить operations.py и
повторить реальный preflight; его успех пока не подтверждён.

Следующая диагностика владельца подтвердила успешный прежний local preflight,
но также действующий Match User ForceCommand через ssh-entry.py. Preflight
не учитывал приоритет серверного правила, поэтому давал ложное заключение о
готовности SSH-протокола. Новый preflight проверяет эффективный ForceCommand,
pubkeyauthentication, AuthorizedKeysCommand и реальный источник authorized_keys.
Root-only repair-ssh — отдельная ручная операция владельца. Четыре новых tests
проверяют policy mismatch, scoped replacement, сохранение root-политики,
отказ неизвестным scopes/командам, restore при syntax/root-policy/reload failures
и reload уже правильного файла без rewrite. 23 Cloud tests прошли. Реальные
repair/reload и первый успешный deploy ещё не подтверждены владельцем.

Владелец затем подтвердил успешные `repair-ssh` (backup сохранён, root policy
не изменена) и local preflight. Следующая попытка Deploy test остановилась до SSH:
GitHub API `/actions/runs/...` вернул HTTP 502 при повторной проверке CI.
Для HTTP 500/502/503/504 и ошибок соединения добавлены максимум четыре попытки
с паузами 2/4/8 секунд. Auth/другие 4xx, неверный JSON и нарушения policy
не обходятся. Шесть regression tests проверяют recovery, bounded failure,
отсутствие секретов в сообщениях retry, немедленный отказ permanent errors
и запрет выдачи deploy outputs, если main изменился во время retry.
Всего 29 Cloud tests прошли; реальный первый деплой по-прежнему не подтверждён.

Следующие повторы владельца дошли до нового контроллера, но state оставался
пустым. Журнал worker поколения 18 показал `code=killed, status=15/TERM`;
контрольный `systemd-run ... --property=TimeoutStartSec=0 /usr/bin/true` тоже
вернул 1. В systemd v255 unit-file parser преобразует zero timeout в infinity,
но transient D-Bus property parser использует `parse_sec` без этого преобразования.
Поэтому контроллер исправлен на явный `TimeoutStartSec=infinity`; существующий
worker-launch test проверяет этот параметр и отсутствие нулевого timeout.
Источник: `src/core/load-fragment.c` и `src/shared/bus-unit-util.c` systemd v255.
29 Cloud tests прошли. Владелец подтвердил: та же контрольная transient unit с
infinity завершилась `success`, `code=exited/status=0`, `systemd_check_exit=0`.
Это подтверждает исправление причины раннего timeout, но ещё не проверяет
реальный worker с GHCR/миграциями/HTTPS. Обновление controller на VPS и первый
успешный деплой пока не подтверждены.

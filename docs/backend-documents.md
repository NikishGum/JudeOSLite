# Приватные документы RD-11 / #203

Источник — [live issue](https://github.com/NikishGum/JudeOSLite/issues/203) и
[контракт](backend-contracts/README.md#документы--203). Backend-only: существующие
frontend, auth helpers и billing сохраняются. Claim StarMadeGalaxy,
request `0e8fdd96e70d423eaee814e26e532f9a`,
[run39257914661](https://github.com/NikishGum/JudeOSLite/actions/runs/39257914661),
`acquired=true`. Начальная основа main `30577ff`; #201/PR205 уже main.

## Хранение и HTTP

`0013_member_documents.sql` добавляет одну таблицу с bytea и составным FK
`(membership_id,club_id)`. Размер вычисляется БД. Индекс по карточке/created_at/id
поддерживает ограниченный список. Существующие записи/идентичности/посещения
не переписываются. Общий migrator требует непрерывные номера: перед штатной
доставкой0013 нужна настоящая0012 из #202; фиктивная миграция не добавляется.

Реализованы четыре URL контракта: GET/POST коллекции, GET binary `/download`,
POST `/delete` с обязательной положительной version. Metadata содержит все поля
MemberDocument; `expiresOn=null` допустим, поле обязательно в upload. Списки
`q/page/limit` → `{items,total,hasMore}`, порядок createdAt DESC/id DESC, пустой
массив `[]`. Поиск — буквальная подстрока имени, без wildcard-семантики.

Ограничения сервера: файл1–5242880 байт, JSON до7340032 байт; карточка до20 файлов
и26214400 байт суммарно. Превышение файла/тела413 `file_too_large`; квоты409
`version_conflict` с русским объяснением обоих лимитов. Блокировка membership
FOR UPDATE сериализует quota-check/insert/delete. Удаление физически освобождает
место. Архив карточки сохраняет файлы; архив филиала запрещает запись и закрывает
parent-доступ, оставляет owner/admin чтение по существующему правилу семей.

Строгий base64 без whitespace, magic bytes PDF/PNG/JPEG, совпадение расширения
`.pdf/.png/.jpg/.jpeg`. MIME вычисляет сервер: клиентское имя не определяет тип.
Пустые/HTML/SVG/неизвестные файлы, неверный base64, путь/CRLF/control/bidi в имени,
неверная дата, неизвестные поля и лишний JSON отвергаются400. Название и имя
trim, предел200 символов; filename дополнительно255 UTF-8 байт.

Файлы не попадают в static/uploads, JSON metadata, localStorage, audit или
публичную ссылку. Download отдельный binary response с вычисленным Content-Type,
Content-Length, `Content-Disposition: attachment` и безопасным filename/filename*,
`X-Content-Type-Options: nosniff`, `Cache-Control: no-store`. Все ошибки модуля,
401/CSRF и HEAD сохраняют no-store/nosniff; HEAD не пишет тело.

## Права и транзакции

Все операции используют прежнюю cookie-сессию и CSRF. Каждое чтение повторяет
`families.LockMemberReader` внутри транзакции: живой user/session, owner либо
текущий administrator, либо явная family_children + family_access именно этой
карточки. Trainer assignment/club discovery не дают права. Сохранённая ссылка
после unlink/revoke/reset/disable не заменяет повторную проверку. Чужой child,
club/document UUID и недоступная роль возвращают одинаковый404.

Upload/delete повторяют `auth.LockClubAdministrator`, active club и membership
в транзакции. Родитель читает, но не пишет; независимые смешанные роли сохраняют
предусмотренные права. Устаревшая версия409, пропущенная/нулевая400; повтор удаления
уже отсутствующего файла404. Предметное изменение и audit атомарны; ошибка audit
откатывает INSERT/DELETE. Нет отдельного внешнего хранилища, orphan-файлов или
неатомарной файловой очистки.

Audit entity_type=`member_document`, action=`member_document_uploaded/deleted`;
в envelope IDs, after_state только membershipId/version/mediaType/size. kind,
название, filename, срок, содержимое и медицинские сведения не сохраняются в
истории. #204 может использовать атомарные audit events для своих уведомлений.

## Проверки и доставка

Команды окончательной проверки и результаты будут дополнены после совместной
стыковки с #202. Новые suites:

- `backend/internal/documents/*_test.go`: форматы/base64/тело/имена/даты/границы;
  настоящие PG/HTTP роли/IDOR/CSRF, binary bytes/headers, пагинация, expiry/revoke/
  disable/unlink после middleware, quota-races/delete-race и audit rollback.
- `backend/migrations/documents_upgrade_test.go`:0012→0013/rerun, сохранение старой
  личности/сессии/архивной карточки и bytea, FK/size/path/media constraints.
- `backend/tests/browser/documents.py`: существующий frontend→Go→PG без подмен;
  три UI upload/download/SQL/reload, parent mobile read-only, version/CSRF/IDOR,
  UI delete/audit privacy и live unlink по сохранённому URL.

Исходный main CI39256443467 attempt1 FAIL: старый Chromium `shadcn_runtime.py:105`,
Page.screenshot calendar timeout30000 после fonts loaded. Этот результат не
связан с новым модулем и не считается устранённым без отдельной проверки.
Общие main.go/build.yml согласуются в #202; auth/frontend/billing не изменяются.
Реальные устройства, пользовательская приёмка и работа с персональными данными
стенда не входят в этот синтетический backend-контроль; #188 сохраняет свой статус.

Дополнение исходного main CI: attempt2 также FAIL в старом
`redesign_training.py:91`, ожидание profile-menu после входа; снимок дерева остаётся
на login. Предшествующие family Chromium/WebKit и оба PWA PASS. Это другой сбой,
чем screenshot attempt1; повторный запуск сделал другой чат, #203 его не запускала.


Предварительный контроль19.10 (до слияния #202):
`go vet ./...`, `go test -race ./...` с TEST_DATABASE_URL на PostgreSQL18.6 и
`go build ./cmd/server ./cmd/migrate ./cmd/create-owner` PASS. Для этого взята
точная опубликованная0012 из PR207/head `2b8298439923c22f0765df1f4d92d95c251fe1d0`
как временная копия для QA; её код не включается в коммиты #203. Штатный
DocumentsUpgradeFrom0012/rerun и прежние migration tests PASS. Ранее прямое
применение SQL0011→0013 через внешний тестовый overlay также PASS после поправки
fixture: для истёкшей сессии created_at нужно сдвигать в прошлое вместе с expires_at,
иначе нарушается существующий SQL CHECK. Product auth/schema не менялись.
Deployment unit43 и claim14 PASS; frontend production build и Docker build PASS.
Это ещё не финальная browser/серверная стыковка или доставка main.

Предварительная совместная QA-сборка (PR207 `2b82984` + documents `543155f`):
полный `go vet ./...`/`go test -race ./...` с настоящим PostgreSQL18.6, включая
billing и documents, PASS. Дополнительно проверена известная дата2028-02-29 и
неизвестный expiresOn через реальный HTTP/PG. Изолированный QA-server собран с
внешним compiler overlay: к опубликованному main.go добавлены только import
и documents.Handler; stock migrations.Check/Apply не ослаблены. Общие tracked
main.go/build.yml обеих рабочих ветвей не изменялись.

`PWA_TEST_IMAGE=judeoslite:rd203-qa python3 backend/tests/browser/documents.py`
и его `PWA_BROWSER=webkit` вариант PASS в Chromium154.0.8037.57/WebKit26.5,
owner1280 и parent390×844, без подмен HTTP. Byte-for-byte download проверен также
прямым SQL encode(content,'hex'); list/reload/version/delete/audit/CSRF/IDOR/unlink
подтверждены. В обоих случаях pageerror отсутствуют. Первый Chromium FAIL нового
harness: label «Файл» включает help text, exact-селектор его не находил. Исправлен
селектор, assertions не ослаблялись; оба следующих полных прохода exit0.

Локальный WebKit использует проверенные Debian13 runtime libraries в private
browser cache и PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1 только для проверки
системных пакетов non-root окружения. Движок и все assertions настоящие; штатный
CI использует playwright install --with-deps и не требует этого флага.

Фактический пример201 из синтетического WebKit QA (его БД удалена):

```json
{"id":"3dde0d79-8a82-4a04-996b-b17c075a9f6e","membershipId":"9b0b3f7d-510b-41b9-81c2-0aedf7a4ec3a","name":"Synthetic Document 0","kind":"medical","filename":"proof.pdf","mediaType":"application/pdf","size":42,"expiresOn":null,"createdAt":"2026-10-19T13:40:00.433271Z","version":1}
```

| Граница потока QA | Результат | Доказательство |
| --- | --- | --- |
| UI → POST | PASS | Три upload forms, настоящий201, без pageerror |
| API → PostgreSQL | PASS | bytea совпадает с независимым hex исходного файла |
| PostgreSQL → download | PASS | attachment и байты PDF/PNG/JPEG в browser download |
| Response → UI | PASS | list/reload, read-only parent390, UI delete и SQL |
| Живые права | PASS | Сохранённый URL после unlink404; HTTP/PG revoke/disable/session tests |

Это предварительная QA-стыковка; окончательные branch/main CI и HTTPS фиксируются
после штатного подключения в рабочей ветви и завершения окна доставки #202.

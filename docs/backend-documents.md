# Приватные документы RD-11 / #203

Источник — [issue203](https://github.com/NikishGum/JudeOSLite/issues/203) и
[контракт](backend-contracts/README.md#документы--203). Claim StarMadeGalaxy,
request `0e8fdd96e70d423eaee814e26e532f9a`,
[run39257914661](https://github.com/NikishGum/JudeOSLite/actions/runs/39257914661),
acquired=true. Старт main30577ff; итоговая стыковка от mainf55b086/PR207.
SHA, CI, доставка и Done фиксируются в live issue; локальный PASS не заменяет main.

## Хранение и HTTP

Одна таблица `0013_member_documents.sql`: bytea, вычисляемый БД size,
составной FK `(membership_id,club_id)`, индекс membership/created_at/id.
Существующие личности, сессии и посещения не переписываются.0013 следует за
настоящей0012 из #202. Требование непрерывности migrator сохранено без заглушек.

Четыре URL контракта: GET/POST коллекции, GET binary `/download`, POST `/delete`
с положительной version. MemberDocument содержит все обязательные поля;
`expiresOn=null` допустим, само поле обязательно в upload. Списки q/page/limit
возвращают `{items,total,hasMore}`, пустой массив `[]`, порядок createdAt DESC/id
DESC. q — буквальная подстрока названия, limit1–100, page>=0, ограниченный offset.

Инженерные лимиты сервера:

- Файл1–5242880 байт (5МиБ), JSON до7340032 байт (7МиБ).
- Карточка:20 файлов, всего26214400 байт (25МиБ).
- Превышение файла/тела413 `file_too_large`; квоты409 `version_conflict` с русским
  объяснением обоих лимитов. UI показывает message существующего ApiError.

Membership FOR UPDATE сериализует quota-check/insert/delete. Физическое удаление
освобождает место. Архив карточки сохраняет документы; архив филиала запрещает
запись и parent-чтение, оставляет owner/admin чтение по существующему правилу семей.

Строгий base64 без whitespace, PDF/PNG/JPEG по magic bytes, совпадение расширения
`.pdf/.png/.jpg/.jpeg`. MIME вычисляет сервер. Пустые/HTML/SVG/неизвестные файлы,
неверный base64, путь/CRLF/control/bidi в имени, неверная дата, неизвестные поля
и лишний JSON отвергаются400. Название/filename trim, до200 символов; filename
дополнительно до255 UTF-8 байт. Срок — настоящая ISO-дата1–9999 года либо null.

Файлы не попадают в static/uploads, JSON metadata, localStorage, audit или публичный
URL. Binary download отдельно от JSON: вычисленный Content-Type/Content-Length,
`Content-Disposition: attachment` с безопасным filename/UTF-8 filename*,
`X-Content-Type-Options: nosniff`, `Cache-Control: no-store`. Ошибки/401/CSRF и HEAD
сохраняют no-store/nosniff; HEAD download не пишет тело.

## Права и атомарность

Прежние cookie-сессии, Origin и CSRF. Каждое чтение вызывает
`families.LockMemberReader` в транзакции: живой user/session, owner, текущий
administrator либо явные family_children + family_access именно этой карточки.
Trainer assignment/club discovery не дают доступа. Сохранённая ссылка не обходит
unlink/revoke/reset/disable. Чужие club/child/document UUID и недоступная роль —404.

Upload/delete повторяют `auth.LockClubAdministrator`, active club и membership
в транзакции. Родитель читает; независимые смешанные роли сохраняют свои права.
Устаревшая версия409, отсутствующая/нулевая400, повтор удаления отсутствующего
файла404. Запись/физическое удаление и audit коммитятся вместе; отказ audit
откатывает и INSERT, и DELETE. Внешнего хранилища и orphan-файлов нет.

Audit entity_type=`member_document`, action=`member_document_uploaded/deleted`;
в envelope IDs, after_state только membershipId/version/mediaType/size. Название,
kind, filename, срок, содержимое и медицинские сведения не пишутся в историю.
#204 может использовать атомарные audit events для своих уведомлений.

## Проверки

На Go1.27.2/PostgreSQL18.6 выполнены:

```sh
cd backend
# TEST_DATABASE_URL указывает на отдельный локальный synthetic PostgreSQL.
gofmt -l .
go vet ./...
go test -race ./...
go build ./cmd/server ./cmd/migrate ./cmd/create-owner
```

Полный suite включает прежние auth/access/training/attendance и новые docs:
401/403/404/CSRF/IDOR, no-store/nosniff, byte-for-byte PDF/PNG/JPEG, HEAD,
пагинация, известная/неизвестная дата, границы JSON/base64/magic/имени/размера,
quota-race8 uploads, delete race, stale version, audit rollback/privacy.
Отзыв family link/access/session, disable и снятие administrator проверяются
также после middleware, перед предметной SQL-операцией.

`DocumentsUpgradeFrom0012` проверяет upgrade/rerun, неизменность прежней личности,
сессии/архивной карточки и bytea, FK/size/path/media constraints. Прежние migration
suites сохранены. Deployment unit43, claim14, frontend production build и
production Docker image проходят проверки.

```sh
PWA_TEST_IMAGE=judeoslite:rd203 python3 backend/tests/browser/documents.py
PWA_BROWSER=webkit PWA_TEST_IMAGE=judeoslite:rd203 python3 backend/tests/browser/documents.py
PWA_TEST_IMAGE=judeoslite:rd203 python3 frontend/e2e/pwa_v1.py
PWA_BROWSER=webkit PWA_TEST_IMAGE=judeoslite:rd203 python3 frontend/e2e/pwa_v1.py
```

Новый harness использует закреплённый Playwright Chromium по умолчанию, явный
CHROMIUM_PATH сохранён. Реальный frontend→Go→PG, без подмены запросов:

| Граница | Проверка |
| --- | --- |
| UI → POST | Три upload forms PDF/PNG/JPEG, настоящий201, без pageerror |
| API → PostgreSQL | bytea совпадает с независимым hex исходного файла |
| PostgreSQL → download | attachment/filename и байты всех трёх файлов |
| Response → UI | list/reload, owner1280, read-only parent390×844, UI delete и SQL |
| Живые права | Сохранённый URL после unlink404; HTTP/PG revoke/disable/session |

До штатного подключения совместный QA PR207/2b82984 + documents/543155f прошёл
полный Go/race/PG, docs Chromium154.0.8037.57/WebKit26.5 и полный прежний PWA
обоих движков: четыре режима2/4/2/1, независимые40%, причины/коррекции/роли/
network/lost response/retry, static-only cache и две сборки. Для предварительного
QA только import/Handler применялись внешним compiler overlay; stock Apply/Check
не ослаблялись. После merge207 эти две строки и две CI-команды подключены штатно;
billing/auth/frontend сохранены. Окончательные результаты delivery source — live203.

Пример201 из синтетического WebKit QA (его БД удалена):

```json
{"id":"3dde0d79-8a82-4a04-996b-b17c075a9f6e","membershipId":"9b0b3f7d-510b-41b9-81c2-0aedf7a4ec3a","name":"Synthetic Document 0","kind":"medical","filename":"proof.pdf","mediaType":"application/pdf","size":42,"expiresOn":null,"createdAt":"2026-10-19T13:40:00.433271Z","version":1}
```

## Исходные failures и границы

Первый новый Chromium harness FAIL: exact label «Файл» не учитывал help text;
исправлен selector. Первичный PWA setup FAIL: в QA worktree отсутствовал
node_modules, подключено уже установленное точное lockfile окружение.
Fixture истёкшей сессии сначала нарушал SQL CHECK: вместе с expires_at нужно
сдвинуть created_at в прошлое. После поправок полные проверки PASS; assertions
и product auth не ослаблялись.

Исходный main30577ff CI39256443467 attempt1 FAIL на calendar screenshot timeout,
attempt2 — на WebKit training login. Причины и отдельные улучшения старого harness
из main описаны в [CI-протоколе](ci-browser-checks.md). Эти исходные failures не
заменяются новыми PASS и не объявляются доказанно исправленными самим #203.

Локальный non-root WebKit использует проверенные Debian13 libraries в private
browser cache; PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1 пропускает только
проверку пакетов хоста. CI использует штатный install --with-deps. Реальные
устройства, физическая установка, приёмка сотрудников #188 и персональные данные
стенда здесь не проверялись/не менялись.

### Стабилизация focus-refresh проверки

Push CI39264328305 attempt1/d57b1ae прошёл весь настоящий browser→Go→PG блок и
остановился в старом synthetic screenshot_redesign.py:151: reads counter после
focus/networkidle не увеличился. Парный PR CI39264454781 того же d57b1ae прошёл
полностью. Исходный synthetic harness локально воспроизвёл тот же FAIL.

Исправлена только подготовка и ожидание в frontend/e2e/screenshot_redesign.py:
страница foreground/visible, затем явное ожидание нового GET200 выбранной семьи,
после него прежние assertions роста counter и сохранённого parent.draft. Нынешний
networkidle не доказывал нового чтения. Таймауты/retry не добавлялись, прежние
проверки не снимались. Внешняя точная копия нового harness прошла все22 screen/state
groups в Chromium151/WebKit26.5 до изменения tracked файла. Это synthetic
frontend-контроль, не замена реальных серверных тестов документов.

Область и точный patch опубликованы в #202 до изменения (комментарии6269218303,
6269277257,6269359277); product frontend/src, auth и billing не менялись. Перед
применением #200/#202 CLOSED, единственный открытый PR —#209, новых параллельных
правок этого файла не заявлено. Окно доставки #202 завершено с mainf55b086,
Build39263741407 attempt2/Deploy39267207286 SUCCESS и точным HTTPS release SHA.

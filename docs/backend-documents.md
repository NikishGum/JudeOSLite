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
trim, предел200 символов; filename дополнительно255 UTF-8 байт. Это проверка
допустимого формата, а не антивирус или санитарная обработка содержимого PDF.

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

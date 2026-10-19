# Контракты для фронтенда PRIDE —19.10.2026

**Разделение работ подтверждено заказчиком:** #200 — фронтенд в текущем чате;
#201–#204 — отсутствующий бэкенд, который реализует второй разработчик.
Серверный код и исполняемые миграции не входят в #200. Эти контракты являются
точкой стыковки: изменение формата согласовать через issue до изменения клиента.
Предложение SQL в `rd08-schema-proposal.sql` — проект, не готовая миграция:
разработчик проверяет ограничения, права, гонки, согласованность и тесты.

Действующий стек Go/React/PostgreSQL сохраняется. Нет нового сервиса,
параллельной системы аутентификации, отдельного фреймворка или нового хранилища.

## Порядок и границы

| Issue | Старт | Файлы/владение |
| --- | --- | --- |
| RD-09/#201 | Сразу после #198 (уже main) | `backend/internal/families/`, миграция0011; согласованные auth/me и auth/clubs |
| RD-10/#202 | После #201 | `backend/internal/billing/`, миграция0012 |
| RD-11/#203 | После #201 | `backend/internal/documents/`, миграция0013 |
| RD-12/#204 | После #201/#202/#203 | `backend/internal/notifications/`, миграция0014, события интегрированных модулей |

Общие backend/cmd/server/main.go и auth меняет backend-разработчик. Frontend,
tokens, шапка/навигация и компоненты принадлежат #200. Общие API-контракты меняются
согласованно. #202 и #203 могут выполняться параллельно после #201 в разных
каталогах с фиксированными номерами миграций. #188 не переиспользовать.
Занимать через `python3 scripts/claim_issue.py НОМЕР`, прямое назначение запрещено.

## Общий HTTP-контракт

- Существующая cookie-сессия/CSRF, Origin, no-store, отзыв доступа и серверные
  проверки сохраняются; данные авторизации не передаются в localStorage.
- UUID — строки. День — `YYYY-MM-DD`, timestamps — RFC3339. BYN — целое число
  копеек (`amountMinor`); JS-клиент не вычисляет деньги через float.
- Списки: `?q=&page=0&limit=25`, limit1–100; ответ `{items:[],total:0,hasMore:false}`.
  Стабильный порядок имя/id или createdAt DESC/id; списки ограничены/индексированы.
- Мутации JSON, `Content-Type: application/json`; POST создание возвращает201,
  остальные200. `version` обязателен при изменении;409 не перезаписывает чужую правку.
- Ошибки существующего формата `{error:{code,message}}`:400 invalid_input,
  401 unauthorized,403 forbidden/csrf_failed,404 not_found,409 version_conflict,
  413 file_too_large,503 not_ready. Чужой UUID не раскрывает существование записи.
- Все записи проверяют живую сессию/назначение в той же транзакции; версия,
  аудит, событие уведомления и предметная запись коммитятся атомарно.
- Недоступный новый модуль:501/503 с `code: module_unavailable`. До реализации
  текущий fallback404 также распознаётся фронтендом на коллекции как неподключённый
  модуль. Нет демоданных, fake save или локального журналирования оплат в production.
- iPay по последнему уточнению **не подключать сейчас**. Только интерфейс будущей
  онлайн-оплаты. Нет вызовов шлюза, подписи «оплачено» по возврату браузера или QR
  с выдуманным номером. Работающие ручные оплаты/счета остаются отдельным сценарием.

## Семья и кабинет родителя — #201

Использовать существующую аутентификацию staff_users/staff_sessions для аккаунта
человека; роль родителя получается из явной family_access, а не staff_club_roles.
`GET /api/auth/me` добавляет `user.isParent:boolean`; отсутствующее поле =false
для старого клиента. `clubs[].roles` может содержать `parent` вместе с прежними
ролями; родитель получает метаданные только филиалов своей семьи. Ни один старый
staff API не должен разрешать доступ по одному факту присутствия клуба в me.

Типы (поля обязательны, отсутствие связи — `null`, массив — `[]`):

```ts
type MemberSummary = { id:string; clubId:string; fullName:string; birthDate:string;
  groupName:string; trainerName:string; archived:boolean };
type ParentAccount = { id:string; login:string; displayName:string };
type Family = { id:string; clubId:string; name:string; contactName:string;
  phone:string; email:string; version:number; children:MemberSummary[];
  accounts:ParentAccount[] };
type FamilyInput = { name:string; contactName:string; phone:string; email:string;
  membershipIds:string[]; version:number };
```

| Метод | URL | Ответ/тело |
| --- | --- | --- |
| GET | `/api/clubs/{clubID}/families` | List<Family>, только owner/admin |
| POST | тот же | FamilyInput version0 → Family |
| GET/PUT | `/api/clubs/{clubID}/families/{familyID}` | Family / FamilyInput → Family |
| POST | `.../{familyID}/accounts` | `{login,displayName,password,version}` → Family |
| POST | `.../{familyID}/accounts/{userID}/reset-password` | `{password,version}` → Family |
| POST | `.../{familyID}/accounts/{userID}/revoke` | `{version}` → Family |
| GET | `/api/portal/families` | List<Family>, только явно связанные с текущим аккаунтом |
| GET | `/api/clubs/{clubID}/member-profiles/{membershipID}` | MemberProfile, owner/admin либо родитель именно этой карточки |

FamilyInput задаёт полный состав. Все membershipIds должны принадлежать тому же
филиалу; один membership не может быть в двух семьях. Перенос в другую семью
явный, с конфликтом409; не объединять по телефону/фамилии. Удаление связи немедленно
отзывает все будущие чтения ребёнка, файлов, оплат и уведомлений. Снимки посещений
не изменяются. Пароль12–128 байт, действующий Argon2id, ограниченная параллельность
хеширования, уникальность логина; в ответах/аудите/логах нет пароля или хеша.
Reset-password отзывает сессии; revoke убирает только family_access, сохраняя
сотруднику его независимые роли. Только текущий owner/admin семьи управляет доступом.

```ts
type Visit = { id:string; sessionId:string; date:string; time:string; groupName:string;
  state:'present'|'absent'|null; absenceReason:'unknown'|'illness'|'excused'|null };
type MemberProfile = MemberSummary & { joinedOn:string; contactName:string;
  contactValue:string; familyId:string|null; familyName:string|null;
  groups:{id:string;name:string;trainerName:string;from:string;until:string|null}[];
  attendance:{present:number;absent:number;rate:number|null;history:Visit[]};
  upcoming:{id:string;date:string;time:string;groupName:string;trainerName:string}[] };
```

History/upcoming — последние/ближайшие20; процент только closed, cancelled
исключены; null при нуле. `groupName/trainerName` пустые при отсутствии группы.
Страница тренера использует уже доступные группы/занятия, не открывает семейные
контакты или документы. Карточка ребёнка не даёт trainer доступ к этому API.

## Платежи, тарифы, абонементы — #202

```ts
type Plan = {id:string;clubId:string;name:string;amountMinor:number;visits:number;
  days:number;archived:boolean;version:number};
type Payment = {id:string;clubId:string;membershipId:string;fullName:string;
  description:string;amountMinor:number;currency:'BYN';
  status:'pending'|'paid'|'void';method:'cash'|'card'|'transfer'|'erip'|null;
  paidOn:string|null;createdAt:string;version:number;voidReason:string|null};
type Subscription = {id:string;membershipId:string;paymentId:string;planName:string;
  visits:number;used:number;validFrom:string;validUntil:string;
  status:'pending'|'active'|'expired'|'exhausted'|'cancelled'};
```

| Метод | URL | Тело/ответ |
| --- | --- | --- |
| GET/POST | `/api/clubs/{clubID}/billing/plans` | List<Plan> / `{name,amountMinor,visits,days,version:0}`→Plan |
| PUT | `.../plans/{planID}` | те же поля+archived+version → Plan |
| GET | `/api/clubs/{clubID}/payments` | List<Payment>, фильтры q/status/from/until/membershipId |
| POST | тот же | PaymentInput → Payment |
| POST | `.../payments/{paymentID}/confirm` | `{version,method,paidOn}`→Payment |
| POST | `.../payments/{paymentID}/void` | `{version,reason}`→Payment |
| GET | `/api/clubs/{clubID}/subscriptions?membershipId=` | List<Subscription> |
| GET | `/api/clubs/{clubID}/billing/summary?from=&until=` | `{paidMinor,pendingMinor,paidCount,pendingCount,currency:'BYN',byMonth:{month,paidMinor}[]}` |
| GET | `/api/portal/payments` | List<Payment>, только текущая семья |
| GET | `/api/portal/subscriptions` | List<Subscription>, только текущая семья |

```ts
type PaymentInput = {requestId:string;membershipId:string;description:string;
  amountMinor:number;method:'cash'|'card'|'transfer'|'erip'|null;
  paidOn:string|null;planId:string|null;validFrom:string|null};
```

method/paidOn обаnull → выставленный счёт pending; иначе ручная подтверждённая
оплата. `requestId` UUID идемпотентности: повтор одинакового тела возвращает ту же
запись, другое тело с тем же ключом409. Сумма1–999999999 копеек; partial/prepayment
не реализуются. Неполный месяц корректируется сотрудником явно, без импорта старых
долгов. При planId сумма должна равняться цене действующего тарифа. В абонемент
копируются название/visits/days; изменение плана не меняет историю. Подтверждение
pending/paid и аннулирование с причиной идут через version/транзакцию/audit.

`validUntil` — исключающая дата конца; использованы только present закрытых
журналов в интервале. Отменённые не расходуют занятия. Пересекающиеся действующие
абонементы одного membership запрещены409, чтобы одно посещение не списывалось
дважды. Коррекция посещения пересчитывает остаток; отрицательное значение не
скрывать (used может превышать visits, UI показывает остаток0).

Owner/admin пишут в своём филиале. Родитель читает только свои записи через portal;
тренер не видит финансовый журнал. Финансовая summary только owner/admin.
Существующая attendance-статистика не меняется. iPay подключается отдельным
будущим issue после договора/подписи/test setup, не входит в этот backend issue.

## Документы — #203

```ts
type MemberDocument = {id:string;membershipId:string;name:string;
  kind:'medical'|'contract'|'consent'|'other';filename:string;mediaType:string;
  size:number;expiresOn:string|null;createdAt:string;version:number};
```

| Метод | URL | Тело/ответ |
| --- | --- | --- |
| GET | `/api/clubs/{clubID}/participants/{membershipID}/documents` | List<MemberDocument> |
| POST | тот же | `{name,kind,filename,contentBase64,expiresOn}`→MemberDocument |
| GET | `.../documents/{documentID}/download` | приватный binary attachment |
| POST | `.../documents/{documentID}/delete` | `{version}`→`{status:'ok'}` |

До5MiB после decode, PDF/JPEG/PNG по magic bytes; JSON-тело максимум7MiB,
base64 malformed/несовпадение MIME/пустой файл400. Без HTML/SVG, публичных URL и
сервинга из static. Имя без путей/CRLF, Content-Disposition attachment с безопасным
filename*, nosniff/no-store. Лимит на количество/общий объём задаёт сервер и
возвращает понятную ошибку. В БД bytea допустим для малого клуба, решение о другом
хранилище согласовать заранее. Download отдельно проверяет живую сессию и связь.
Owner/admin upload/delete; родитель только list/download своих детей; trainer403/404.
Аудит только идентификаторы/версия/тип/размер, без содержимого/медицинских сведений.

## Уведомления — #204

```ts
type Notification = {id:string;kind:'family'|'payment'|'member_document'|'training_session';
  title:string;body:string;href:string;read:boolean;createdAt:string};
```

| Метод | URL | Тело/ответ |
| --- | --- | --- |
| GET | `/api/notifications?status=all\|unread&page=0&limit=25` | List<Notification> + `unreadCount:number` |
| POST | `/api/notifications/{id}/read` | `{}`→`{status:'ok'}` (идемпотентно) |
| POST | `/api/notifications/read-all` | `{}`→`{status:'ok'}` |

Только свои сообщения; при отзыве family/club/session прав прежние сообщения
больше не доступны и не входят в unreadCount. Родителю — его семья/оплаты/файлы,
тренеру — назначенные занятия, admin — события его филиала, owner — сеть.
Изменения расписания/замена/отмена создают событие только после успешного commit.
Уникальность получатель+event исключает дубли; собственные действия можно
исключать. Без push/email/SMS или регистрации внешних сервисов. href только
внутренний разрешённый путь, клиент не доверяет внешним ссылкам из ответа.

## Приёмка и доставка

Каждый backend issue: live claim/один исполнитель; миграции upgrade+rerun,
реальный PostgreSQL/race/ветки доступа; HTTP401/403/404/CSRF/IDOR; оптимистические
конфликты и конкурентная запись; пагинация; no-store и отзыв сессии/назначения;
примеры JSON соответствуют контракту. Код проверяется существующими auth/PWA/
attendance тестами. Новые assertions не заменяют прежние. В issue приложить
коммит/PR, команды, фактический результат и ограничения. Закрывать после main.

Синтетические ответы для фронтенд-проверок разрешены только в тестовом harness;
они не доказывают готовность сервера. После доставки backend выполнить отдельную
сквозную проверку browser→API→PostgreSQL, без подмены запросов.

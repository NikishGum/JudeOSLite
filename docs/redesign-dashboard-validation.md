# Проверка RD-05 · 14.10.2026

Задача [#187](https://github.com/NikishGum/JudeOSLite/issues/187),
[контракт](redesign-dashboard.md), ветка `codex/rd-05-dashboard` от
`893d213c842021d256b60dc4ca3f3b3c32459e6e` (RD-04 / PR #193).
После явной передачи задачи восстановлен переносимый незакоммиченный патч:
`git apply --check` успешен, затем обычный `git apply`; повторного claim не было.
Live #187: один StarMadeGalaxy, In progress; предыдущий чат idle, открытых PR нет.
Сохранённый claim: request `9dd9ff8579884f3a8dfd0726f97f30c8`,
[acquired=true run38582696362](https://github.com/NikishGum/JudeOSLite/actions/runs/38582696362).
Зависимости #184/#185/#186 повторно проверены CLOSED, результаты в main;
CI38553603917/eabae80, CI38556322715/535bacc, CI38558940347/893d213 — success.
Актуальный origin/main при восстановлении совпал с базой.

## Browser → Go → PostgreSQL

`frontend/e2e/redesign_dashboard.py` использует настоящий production Go,
свежую production-статику и отдельную PostgreSQL18.6. Синтетические fixtures
создаются в disposable контейнерах на loopback; данные реальных клубов не меняются.
Браузер Pacific/Honolulu; филиалы A Pacific/Kiritimati и B Pacific/Honolulu
имеют разные локальные даты с разницей24ч. Группы имеют одинаковые названия,
но разные UUID. У A три участника, у B два; open прошлого дня A приоритетнее planned.

| Контроль | Closed | Present | Absent | Серверный процент / UI |
| --- | ---: | ---: | ---: | --- |
| Два исходных закрытых журнала | 2 | 2 | 3 | 40% / 40,0% |
| Реальная коррекция absent→present | 2 | 3 | 2 | 60% / 60,0% |
| Отметка и закрытие прежнего open A | 3 | 5 | 3 | 62,5% / 62,5% |
| Только пустой closed | 1 | 0 | 0 | null / — |

Начальные полные счётчики сети: participants5, groups2, today5, open2;
администратор A:3/1/3/1. Trainer-only не получает participants, историю и отчёт.
Отдельные SQL-запросы контролируют локальные карточки, open и ledger закрытых
отметок. Создание через настоящую форму увеличивает participants A до4;
roster ранее открытого журнала остаётся3. После архива счётчик снова3,
исторический процент62,5% не меняется.

Проверены реальные переходы по club/group/session IDs, reload/back,
клавиатурный focus/Enter и видимый outline; создание участника через detail=new;
отметка/закрытие/коррекция/архив и обновление главной. Отчёт использует прежний
RD-04 SQL-процент, без второго расчёта на клиенте. Независимые сбои dashboard,
reports и audit скрывают старые значения только своего блока; retry восстанавливает
его без фиктивных нулей. Общий аудит не содержит контактов/причин коррекции.

Multi-role administrator+trainer получает действие отметок только для actual
назначения. После отзыва administrator с задержанным dashboard-ответом остаётся
trainer-home без manager-счётчиков/истории. Полный отзыв trainer очищает главную;
нет назначений — явное пустое состояние. Поздний отчёт после logout не возвращает
приватный DOM. Чужой филиал404, trainer reports/audit403, ответы no-store.

## PostgreSQL/API и сборки

`backend/internal/dashboard/http_test.go` проверяет реальную БД:111 локальных
карточек (больше страницы API), полные счётчики, роли/приоритет/чужой scope,
отзыв после middleware, disable/revoke сессии, архив филиала/группы,
замороженный roster и отсутствие приватных полей. Фиксированные исторические
даты с Europe/Minsk/UTC проверяют границу дня. После смены зоны филиала
его today/branchDate меняются, сохранённые date/timezone занятия — нет.
Конкурентная реальная отметка после начала read snapshot не попадает частично
в первый ответ; следующий запрос видит новое значение.

Повторные проверки в новом окружении, после последнего UI-исправления:

- `npm ci --cache /tmp/rd05-npm-cache && npm run build` — PASS,
  Node24.19.0, TypeScript и production Vite.
- Go1.27.2: `test -z "$(gofmt -l .)"`, `go vet ./...`,
  `TEST_DATABASE_URL=… go test -race ./...`,
  `go build ./cmd/server ./cmd/migrate ./cmd/create-owner` — PASS.
  Все пакеты, access/training integration и миграции выполнились с PostgreSQL18.6,
  а не были пропущены без TEST_DATABASE_URL. [Лог](redesign-evidence/rd05/go-checks.log).
- `BUILDX_CONFIG=/tmp/rd05-buildx docker build --secret
  id=proxy_ca,src=/etc/ssl/certs/ca-certificates.crt -f deploy/Dockerfile
  -t judeoslite:rd05 .` — PASS. Итоговый локальный образ
  `sha256:377f553f8340b874f889f36363b090eee9e1298d8d1b8c137111fe65fde3265a`.
  Прокси и CA сохранены, TLS-проверка не отключалась.
- `python3 -m unittest discover -s deploy/test -p 'test_*.py' -v` —43 PASS.

Команды новых браузерных прогонов:

```sh
PWA_TEST_IMAGE=judeoslite:rd05 python3 frontend/e2e/redesign_dashboard.py
PWA_TEST_IMAGE=judeoslite:rd05 PWA_BROWSER=webkit \
  PLAYWRIGHT_BROWSERS_PATH=/tmp/rd05-browsers \
  PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1 \
  python3 frontend/e2e/redesign_dashboard.py
```

Оба exit0: [Chromium154.0.8037.57](redesign-evidence/rd05/dashboard-chromium.log)
и [WebKit26.5](redesign-evidence/rd05/dashboard-webkit.log).
В локальном Debian13 отсутствовали некоторые системные browser libs;
публичные Debian-пакеты извлечены в private browser sys/lib. Skip отключает
только host dependency detector, не UI/API assertions. Стандартный CI использует
`playwright install --with-deps chromium webkit`, без этого skip.

## Регрессии и доставка

Все шесть существующих регрессий выполнены последовательно, общий runner exit0:

| Сценарий | Chromium154 | WebKit26.5 | Контроль |
| --- | --- | --- | --- |
| `frontend/e2e/pwa_v1.py` | [PASS](redesign-evidence/rd05/pwa_v1-chromium.log) | [PASS](redesign-evidence/rd05/pwa_v1-webkit.log) | RD-01/RD-02; четыре расписания2/4/2/1; ledger40%; права/сессия/сеть; static-only cache; две сборки/ожидающее обновление/сохраняющая вкладка |
| `frontend/e2e/redesign_training.py` | [PASS](redesign-evidence/rd05/redesign_training-chromium.log) | [PASS](redesign-evidence/rd05/redesign_training-webkit.log) | Дни/неделя/период; IDs/два филиала; перенос/замена/отмена; журнал/back/reload; double tap/сеть; correction conflict/history |
| `frontend/e2e/redesign_reports.py` | [PASS](redesign-evidence/rd05/redesign_reports-chromium.log) | [PASS](redesign-evidence/rd05/redesign_reports-webkit.log) | Ledger3/7→4/7; empty/архив/timezone; быстрые даты/ручной период; audit50+8; ошибка/retry/stale/revoke |

Для каждого сценария: `PWA_TEST_IMAGE=judeoslite:rd05 PWA_BROWSER=chromium|webkit
python3 frontend/e2e/ИМЯ.py`; для WebKit те же локальные browser path/private libs
и host validation skip, что описаны выше. Существующие assertions не менялись.
Все проверки прошли с окончательным продуктовым кодом, без повторных UI-правок.
Build workflow включает новый dashboard-сценарий после существующих регрессий
в обоих движках. Ссылки PR, exact SHA CI, main CI и deploy фиксируются в live #187
после публикации; issue остаётся открытым до результата в main и выполнения критериев.

## Визуальная проверка и история исправлений

Все три роли сняты в Chromium и WebKit на1440/1280/768/390/320:
30 окончательных изображений в [evidence](redesign-evidence/rd05/).
Высоты viewport:900 для desktop/tablet,844 для390,780 для320.
Проверены отсутствие горизонтального document overflow, высота действий≥44px,
основная кнопка manager/trainer выше фактической `.bottom-navigation` с зазором≥4px,
реальный DOM/keyboard порядок day/open → groups/history на компактной ширине.
Full-page screenshot рисует fixed navigation на краю исходного viewport;
она не является частью потока внутри карточки.

Исходный переданный verdict был **fix**. Материальные замечания:
CTA320 перекрыта nav; короткая колонка unfinished растянута на768;
визуальная перестановка не совпадала с DOM. Последние два исправлены перед
передачей через align-items:start и реальную compact DOM-композицию.
Тогда Chromium проходил, но WebKit падал: кнопка y672,28/h48, nav y705.
[Исходный WebKit FAIL](redesign-evidence/rd05/pre-handoff-webkit-fail.log)
сохранён как отрицательный результат, не как одобрение.

В этой сессии CTA поставлена перед вторичной строкой trainer, а дата/время,
группа и статус остались до действия. Полный текст и прогресс сохранены;
проверка фактических bounding boxes не ослаблена, force click не используется.
Оба окончательных сценария прошли. Дополнительной косметической полировки не было.
Impeccable launcher/detector отсутствует (exit127); контекст PRODUCT/DESIGN,
Working Panel, adapt/polish/craft-floor и React best practices прочитаны напрямую.
Detector PASS не заявляется; глобальный DESIGN/sidecar не ремонтировался.

Независимый read-only [finish reviewer](redesign-evidence/rd05/finish-review.md)
дал **ship**: прочитал код/контракты/live issue, лично открыл все30 свежих
снимков и сверил оба полных PASS-лога. Новых P0/P1/P2 нет. Он не приписывает
себе запуск тестов и не выдаёт визуальный review за физическую PWA-проверку.

Независимая [документационная сверка](redesign-evidence/rd05/documentation-check.md)
— **PASS**: контракт, протокол, KB/memory, исходники, CI,30 снимков и фактические
логи согласованы; блокирующих неточностей нет. Перед коммитом также прошли
`python3 -m py_compile frontend/e2e/redesign_dashboard.py`, `git diff --check`
и проверка локальных ссылок документов задачи.

## Границы результата

Потерянный результат прежних дополнительных Go-проверок не засчитан: полный
vet/race/PostgreSQL/build выполнен заново и записан. Старый Docker PASS до UI-правок
также заменён окончательной сборкой. Тестовые данные синтетические, локальные;
скриншоты не содержат реальных реестров или рабочих реквизитов.
Физические iPhone/Android, установка и standalone PWA на устройствах,
новая пользовательская визуальная приёмка — **NOT RUN** здесь, дальнейший RD-06.
Linux WebKit/Chromium и CI не подменяют эти проверки.

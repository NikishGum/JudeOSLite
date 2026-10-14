# RD-02: проверка предметных экранов

14.10.2026, Cloud Linux Debian13, Node24.19.0/Playwright1.62.0.
Отдельный worktree codex/rd-02-registry от main56475b6; claim NikishGum/acquired=true,
[run38547156787](https://github.com/NikishGum/JudeOSLite/actions/runs/38547156787),
request5f23319590ed49ffb1e37b7cf93457dc, единственный исполнитель/In progress.
#183 CLOSED/Done, PR190/result в main, CI38542258544/Deploy38543282879 success.
Другого исполнителя/чата #184 перед стартом не найдено. Изменения app/App.tsx/navigation.ts
зафиксированы в issue до правок; shared/API/токены/миграции неизменны.

## Реальные проверки

Production frontend → Go → PostgreSQL18.6 в disposable Docker. Только синтетические
fixtures двух филиалов; реальные записи клуба не изменялись. Новая
`frontend/e2e/redesign_registry.py` вызывается existing CI PWA harness обоих движков.
Данные и контейнеры удаляются harness после проверки.

- UI создаёт филиалы с адресом/zone, администратора с двумя назначениями, тренера,
  группы и две отдельные карточки с одинаковыми ФИО/семейным контактом.
- Все поля реестра/начальная группа, server validation с сохранением формы,
  local-vs-shared identity, owner versioned identity edit, поиск контакта,
  карточка, архив/возврат, отмена без записи. Shared identity fixture создаётся
  disposable SQL для проверки существующего owner-only правила нескольких филиалов.
- Конфликты participant/staff/interval: independent API меняет запись до stale UI
  submit; ошибка сохраняет введённое, перечитывание явное.
- Transfer/exclude/add сохраняют три интервала; выбранные даты до/после перевода
  дают независимо 2/1/1. Group archive не удаляет прежний roster; отмена архива не записывает.
- Реальные group detail/reload/back/close и group UUID в sessions URL.
  Фильтр календаря — RD-03, не объявляется выполненным.
- Multi trainer assignment, сохранённое второе назначение после отзыва первого,
  trainer-only и foreign membership ID404. Общая идентичность/контакт/роль остаются серверными.
- Keyboard focus/Tab/Shift+Tab, native Dialog, draft-close/Escape/club/back и nested
  guard с возвратом фокуса/scroll unlock; формы 16px, цели ≥44px, document overflow320.
- Прежняя полная регрессия RD-01/PWA: четыре режима2/4/2/1, один POST double tap,
  журнал/три причины/close/correction, ledger2present/3absent=40%, cancelled NULL,
  revoke/expiry/logout/swap/pagehide/persisted/native back/fresh me,
  две сборки/waiting/save guard/другая сохраняющая вкладка/static-only cache.

Команды:

```sh
npm ci --prefix frontend
npm run build --prefix frontend
docker build --secret id=proxy_ca,src="$CODEX_PROXY_CERT" -f deploy/Dockerfile -t judeoslite:ci .
python3 frontend/e2e/pwa_v1.py
PWA_BROWSER=webkit python3 frontend/e2e/pwa_v1.py
# disposable PG, Go1.27.2: TEST_DATABASE_URL=... go vet ./... && go test -race ./...
python3 -m py_compile frontend/e2e/redesign_registry.py frontend/e2e/redesign_shell.py frontend/e2e/pwa_v1.py
git diff --check
```

Go vet/race/все реальные PostgreSQL API/access/training/migration tests PASS.
Frontend build/typecheck и Docker build PASS. Первый полный Chromium154.0.8037.57
проход RD-02+RD-01+PWA exit0. Окончательная сборка после визуальной поправки: Chromium154.0.8037.57 и
WebKit26.5, весь RD-02+RD-01+PWA сценарий exit0/PASS. Локальный non-root WebKit
использовал Debian runtime libraries в приватном browser cache и
PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1 только для проверки системных пакетов;
UI/API/DB assertions не выключались. CI использует стандартный install --with-deps.

## Находки и Impeccable

Первый Chromium FAIL: fixture ожидала disabled identity у карточки одного филиала,
хотя v1 разрешает её локальное редактирование. Исправлена fixture общей identity двух
филиалов; сервер/права не менялись. Следующий FAIL: тест ожидал «Запись уже изменена»,
participants API пишет «Карточка уже изменена». Assertion проверяет общий смысл
«уже изменена» плюс сохранённое значение, версия/409 не ослаблены.
Визуальная первая проверка: повторный заголовок филиалов, основное действие ниже
второй шапки; убраны дубли, create поднят к основному heading. Полный снимок документа
при open modal показывал фон за пределами viewport; modal evidence теперь viewport,
list/card — full-page from top. Это исправление capture, не изменение backdrop/lifecycle.

Impeccable4.5.2 установлен в cloud в /home/agent/.agents/skills/impeccable;
SKILL.md/context/new-work/Operate/craft-floor/audit прочитаны. Выбранный пользователем
Working Panel фиксирован, новое направление не выбирается. Один detector pass на
изменённых feature targets вернул `[]`, новых dependencies нет. Шрифты/эмблема
унаследованы, новых shipping raster assets нет. Независимый finish-review/documenter
и итоговое свидетельство добавляются после их фактического выполнения.

Native BFCache hit, физическая PWA installation/iPhone/Android/реальная клавиатура
и новая пользовательская визуальная приёмка NOT RUN в RD-02; это область RD-06.
Статусный workflow не поддерживает RD-title, как отмечено в #183: после main/проверок/
закрытия метка Done проверяется и синхронизируется отдельно без изменения назначений.

## Доставка

Итоговые проверенные commit/PR/main/CI и закрытие — в live [#184](https://github.com/NikishGum/JudeOSLite/issues/184).
[Контракт](redesign-registry.md). RD-03–RD-06 здесь не реализованы.

## Скриншоты окончательной Chromium-сборки

Синтетические данные. Полная матрица10 поверхностей×3 ширины создаётся в /tmp/rd184-chromium;
в Git сохранены пары ключевых экранов. Modal — viewport с внутренней прокруткой,
списки/карточка — full-page от верха; full-page нижняя навигация остаётся фиксированной
на своей экранной позиции, это не наложение элементов вне прокрутки.

| Экран | ПК1280 | Телефон390 |
| --- | --- | --- |
| Реестр | [ПК](redesign-evidence/rd02/participants-list-1280.png) | [Телефон](redesign-evidence/rd02/participants-list-390.png) |
| Карточка | [ПК](redesign-evidence/rd02/participant-card-1280.png) | [Телефон](redesign-evidence/rd02/participant-card-390.png) |
| Форма участника | [ПК](redesign-evidence/rd02/participant-form-1280.png) | [Телефон](redesign-evidence/rd02/participant-form-390.png) |
| Группы/состав | [ПК](redesign-evidence/rd02/groups-list-1280.png) | [Телефон](redesign-evidence/rd02/groups-list-390.png) |
| Сотрудники/назначения | [ПК](redesign-evidence/rd02/staff-list-1280.png) | [Телефон](redesign-evidence/rd02/staff-list-390.png) |
| Филиалы | [ПК](redesign-evidence/rd02/clubs-list-1280.png) | [Телефон](redesign-evidence/rd02/clubs-list-390.png) |

## Независимый финиш Impeccable

[Finish review](../.impeccable/review/rd02/finish-review.md): **ship** по всему пакету
RD-02 — persistence/fidelity/ceiling/material_fixes/keep присутствуют, все30 captures
валидны, нет material fixes; inherited seed f62fcfae подтверждён prototype, новая
концепция не выбиралась. Это independent visual/source review, тесты reviewer
не запускал повторно. Новый системный дизайн не вводится.

[Documenter check](../.impeccable/review/rd02/documentation-check.md) завершён:
ordinary extension соответствует incumbent PRIDE, новых глобальных tokens/identity нет.
Pre-existing prototype-era wording PRODUCT.md и отсутствие .impeccable/design.json
зафиксированы, не исправлены побочно. После ship продуктовый UI не менялся.
Дополненный финальный Chromium также проверил UI global disable/re-enable администратора,
снятие одного назначения с сохранением другого и отмену/подтверждение архива филиала;
три исторических интервала остались в PostgreSQL. Полный сценарий exit0.

Параллельная #185 занята StarMadeGalaxy после нашей фиксации общих файлов;
контракт Route.group и владение app/navigation опубликованы также
[в #185](https://github.com/NikishGum/JudeOSLite/issues/185#issuecomment-6166323309).
Main fa1884a добавляет только правило Impeccable/память, оба дополнения сохраняются при интеграции.

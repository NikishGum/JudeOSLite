# RD-01: проверка оболочки

14.10.2026, исполнитель NikishGum, отдельный worktree/ветка `codex/rd-01-shell` от
main b80412f. #166 CLOSED/Done, итоговый 72460d8 в main; CI38444427222 и
Deploy38445430689 success. Main b80412f CI38535381353 success.
Штатная зависимость #183 — только #166; других чатов/исполнителей #183 перед стартом нет.
Claim acquired=true, request ce84f36e4c1c4351b700ded7bff80b26,
[run38535824077](https://github.com/NikishGum/JudeOSLite/actions/runs/38535824077),
единственный NikishGum/In progress. Прямого назначения не было.

## Среда и воспроизводимость

Cloud Linux Debian13, Node24.19.0, Playwright1.62.0, Chromium154.0.8037.57,
WebKit26.5, настоящий production build → Go → отдельный PostgreSQL18.6.
Синтетические disposable fixtures; реальные реестры/учётные записи клуба не изменялись.
Скрипты сами удаляют только свои контейнеры/БД. Проверка backend SPA использует реальный Go,
защищённый запрещённый API возвращает JSON/404, shell не заменяет API.

```sh
npm ci --prefix frontend
npm run build --prefix frontend
# В Cloud также --secret id=proxy_ca,src="$CODEX_PROXY_CERT"
docker build -f deploy/Dockerfile -t judeoslite:ci .
python3 frontend/e2e/pwa_v1.py
PWA_BROWSER=webkit python3 frontend/e2e/pwa_v1.py
```

Новые проверки `frontend/e2e/redesign_shell.py` вызываются существующей CI-регрессией обоих
движков. DOM ожидания переведены с прежнего стека/hash links на выбор реального раздела;
утверждения о DB, правах, количестве запросов, таймаутах, privacy и вычислениях сохранены.
Локальному non-root WebKit потребовались Debian runtime-библиотеки в приватном browser cache
и PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1 только для системного package check;
браузер реально запускается. Стандартный CI использует install --with-deps без этого флага.

## Проверенные сценарии

- Отдельные owner/admin/trainer и multi-role admin+trainer в двух филиалах;
  union разделов/оба журнала, owner/admin/trainer primary menu, «Ещё», правильные текущие роли.
- Все девять разделов/существующие функции, прямой club URL и participant membership detail,
  reload/back/forward, закрытие карточки, неизвестный раздел/чужой филиал.
- Черновик не теряется при отмене ссылки, смены филиала и browser back;
  подтверждённый back убирает форму, forward не восстанавливает отвергнутый черновик.
  Beforeunload предотвращён; Escape и возврат focus, native dialog/Tab/Shift+Tab,
  background overflow, reduced-motion. Никакого обхода серверных scope.
- Отзыв admin с сохранением trainer убирает реестр/пункты и записи первого филиала;
  второй admin-филиал работает, прямой API первого запрещён. Старые запросы не возвращают DOM.
- 320/390/768/1280/1440: screenshot matrix owner/admin/trainer/multi/login и предметных форм,
  отсутствие document overflow, цели ≥44px, mobile fields/actions16px.
  На 320×480 h3 и подтверждение открытия находятся над bottom-navigation до явного открытия:
  status planned/records=0. Нативные устройства/экранная клавиатура не измерены в RD-01.
- Прежняя полная регрессия: UI group/trainer/пять участников/четыре режима, даты 2/4/2/1,
  один запрос на double-submit; presence/три причины/cancel/close defaults/correction,
  независимо 1 closed/2 present/3 absent, unknown2/illness0/excused1 = 40%.
- Offline до отправки, ошибки до commit, потеря настоящего ответа после DB commit,
  retry без повторного аудита; cancelled сохраняет NULL, live revoke/expiry/logout/account swap.
- Manifest/icons/standalone signals, публичная оболочка offline/new URL,
  static-only CacheStorage, pagehide cleanup/persisted pageshow/fresh me и реальный native back.
  Native BFCache hit не измерен, физическая установка NOT RUN.
- Две настоящие сборки: waiting/update запрещён во время save, переход также заблокирован;
  новая версия не обрывает pending-save другой вкладки, кеш/роли остаются корректны.

## Сохранённые находки

Первый Chromium прогон: кнопка открытия журнала частично вне viewport на 320×480.
Причина: focus/scroll до завершения загрузки журнала и отсутствие нижнего scroll-padding.
Исправлены загрузочный момент focus и компактные отступы/scroll-padding; добавлено сравнение
геометрии кнопки с верхом нижней навигации. Следующий прогон выявил offline reload /statistics:
старый SW navigation regex допускал только /, /login и /clubs/:id. Добавлены новые публичные
маршруты с прежним исключением API и build-time fallback. Первый WebKit прогон также остановился при выборе раздела в фоновой вкладке владельца: immediate is_visible выбрал мобильный путь до доступной навигации. Навигационный helper теперь сначала выводит вкладку на передний план и ждёт существующий authenticated UI с прежним сроком ожидания; причина исходной недоступности не объявляется доказанно исправленной продуктовым кодом. Исходные FAIL сохранены здесь;
последующие реальные прогоны проверяют исправления, ожидания не ослаблялись.

## Фактический итог локальной проверки

14.10.2026, финальные запуски начались в 11:53:59 и 11:54:42 Europe/Minsk.
Chromium154.0.8037.57 и WebKit26.5: exit0, весь описанный сценарий PASS;
JS errors отсутствуют. Frontend build/typecheck, production Docker build,
Python compile, ссылки новых документов и git diff --check PASS.
Исходные PRIDE-токены/прототип/backend/миграции/deploy не изменены.

Скриншоты синтетических аккаунтов Chromium, текущая финальная сборка:

| Роль | ПК | Телефон |
| --- | --- | --- |
| Владелец | [1280](redesign-evidence/owner-1280.png) | [390](redesign-evidence/owner-390.png) |
| Администратор | [1280](redesign-evidence/admin-1280.png) | [390](redesign-evidence/admin-390.png) |
| Тренер | [1280](redesign-evidence/trainer-1280.png) | [390](redesign-evidence/trainer-390.png) |

[Подтверждение журнала 320×480](redesign-evidence/journal-confirm-320.png) ·
[Меню/keyboard 320×480](redesign-evidence/more-320.png).
Полная матрица, формы и WebKit screenshots создаются скриптом в /tmp;
в Git сохраняется этот компактный набор без реальных персональных данных.

## Результат доставки

Итоговые команды/CI/PR/main и статус записываются в итоговом live отчёте
[#183](https://github.com/NikishGum/JudeOSLite/issues/183). Контракт —
[redesign-navigation.md](redesign-navigation.md). Это проверка оболочки RD-01;
RD-02–RD-06, визуальная пользовательская приёмка и реальные устройства не объявлены выполненными.

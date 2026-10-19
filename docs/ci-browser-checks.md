# Воспроизводимость браузерных проверок — 19.10.2026

Проверка `main` `30577ff` после PR #206 выявила два разных сбоя:

- [Первый запуск](https://github.com/NikishGum/JudeOSLite/actions/runs/39256443467/attempts/1):
  `shadcn_runtime.py:105`, `Page.screenshot` открытого календаря превысил 30 секунд
  после сообщения `fonts loaded`. Причина остановки захвата не установлена.
- [Диагностический повтор того же SHA](https://github.com/NikishGum/JudeOSLite/actions/runs/39256443467/attempts/2):
  календарь и полные Chromium/WebKit PWA прошли; WebKit training остановился
  на входе тренера. В aria snapshot логин пустой, пароль заполнен, форма входа
  остаётся открытой. Это отдельный сбой подготовки формы, не повтор таймаута снимка.

Локально исходный полный PWA проходит с Chromium Playwright 151.0.7922.34 и
WebKit 26.5 на настоящих Go/PostgreSQL fixtures. В Chromium дополнительно
записаны состояние фокуса/видимости и отсутствие анимаций перед тремя снимками
календаря 1280/390/320. Диагностика была в `/tmp`, приложение не менялось.
Эти результаты не устанавливают причину исходного зависания screenshot.

## Изменение harness

Четыре текущих UI suites (`pwa_v1`, `redesign_training`, `redesign_reports`,
`redesign_dashboard`) используют браузер установленной версии Playwright по
умолчанию. Автоматический выбор `/usr/bin/chromium` убран: в повторном CI он
незаметно подставлял 155.0.8059.0 вместо скачанного 151.0.7922.34. Для явной
проверки другого браузера сохраняется `CHROMIUM_PATH`.

Три отдельных redesign suites теперь используют существующий `observe_api` /
`wait_for_api_idle` до ввода и после входа, как основной PWA harness. Перед
нажатием «Войти» проверяется фактическое значение логина. Начальное чтение сессии
завершается до заполнения управляемых React-полей. API, auth и UI приложения
не изменяются; прежние SQL, роли, сетевые сбои, pageerror и геометрические
assertions сохраняются. Retry и увеличение таймаутов не добавлены.

Для проверки нужны production dist/image, Playwright 1.62.0 и его браузеры:

```sh
python3 -m playwright install --with-deps chromium webkit
python3 frontend/e2e/pwa_v1.py
PWA_BROWSER=webkit python3 frontend/e2e/pwa_v1.py
python3 frontend/e2e/redesign_training.py
PWA_BROWSER=webkit python3 frontend/e2e/redesign_training.py
python3 frontend/e2e/redesign_reports.py
PWA_BROWSER=webkit python3 frontend/e2e/redesign_reports.py
python3 frontend/e2e/redesign_dashboard.py
PWA_BROWSER=webkit python3 frontend/e2e/redesign_dashboard.py
```

Локально PASS: полный PWA с Chromium151/WebKit26.5; training, reports и dashboard
с Chromium151, Chromium154 и WebKit26.5. Сохранены контрольные значения: четыре
расписания2/4/2/1, PWA40%, reports3/7→4/7, dashboard2/5→3/5→5/8.
Сценарии используют настоящий production Go/PostgreSQL18.6; `py_compile` и
`git diff --check` также PASS. Итог удалённого Build проверяется отдельно по SHA
ветви/PR, локальный PASS не подменяет GitHub Actions.

В cloud WebKit runtime libraries распакованы в private browser cache из Debian13
packages. Локальный `PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1` пропускает только
проверку пакетов хоста; реальный WebKit выполняет исходные assertions. В GitHub
Actions остаётся штатный `install --with-deps`. Физические устройства и #188
этой проверкой не принимаются. Backend #202 и реальный iPay не реализуются.

# Уточнение терминов — 19.10.2026

Scope: название семейного раздела и подписи участника в оплатах. Mode Operate;
исходный PRIDE/Rubik/shadcn сохраняется, CSS/токены/маршруты/права не меняются.
Применены Impeccable clarify, craft-floor и polish. Launcher context отсутствует
(exit127); прочитаны PRODUCT.md, DESIGN.md и текущая surface brief напрямую.

## Проверка интерфейса

Один полный batched browser pass, Chromium151 и WebKit26.5, роли owner/parent.
Ширины1280/900/640/390/320;640 соответствует эффективной CSS-ширине ПК1280 при200%.
Это не отдельная физическая проверка browser zoom. Синтетические API fixture
существующего harness, production dist; готовность backend ими не доказывается.

PASS: полное имя раздела в h1/Sidebar/More/bottom navigation, видимый и возвращённый
keyboard focus, минимальные цели44px с существующими desktop исключениями,
rendered contrast4.5/3 без исключений, отсутствие document overflow; новая
подпись/поиск/placeholder/column header участника в платеже, роли owner/parent,
отсутствие неожиданных pageerror.

Снимки и лог локальной проверки: /tmp/families-terminology-evidence/ и
/tmp/families-terminology-browser-confirmed.log. Визуально просмотрены desktop
семьи1280, мобильное меню320, родитель320 и форма оплаты320. Новых растровых
ресурсов в продукте нет. Visual verdict: ship, дефектов изменённых подписей нет.

Первый внешний verification script ошибочно ожидал desktop th после viewport320,
где существующий responsive CSS скрывает thead. Исправлена только подготовка
этой проверки: перед desktop header viewport1280; проверка видимости сохранена.

Production build PASS. Настоящий UI→Go→PostgreSQL18.6 billing harness PASS в
Chromium154/WebKit26.5: тариф, счёт, confirm/void, UUID повторы, версии, коррекции,
семейная изоляция/отзыв и CSRF/no-store. Исходный полный PWA остановился в
неизменённом redesign_registry.py:64 на кнопке назначения администратора;
проверка повторена отдельно, исходный лог /tmp/families-terminology-pwa-chromium.log
сохранён. Уточнение имён не объявляется исправлением неизвестной причины.

Backend документов #203, уведомления #204, реальные устройства и приёмка #188
в эту узкую визуальную проверку не входят. Доставка/удалённый CI проверяются
отдельно по коммиту/PR.

Итог полного локального PWA: один повтор Chromium151 PASS, WebKit26.5 PASS;
исходные40% и четыре расписания2/4/2/1 сохранены, новые названия проверены
существующими navigation/keyboard assertions. Причина первого staff fixture
сбоя не установлена; retries и ослабление assertions не добавлялись.

Общая однострочная замена payment selector заранее опубликована в #203:
https://github.com/NikishGum/JudeOSLite/issues/203#issuecomment-6269635338
Она не пересекается с focus patch PR209: git merge-file завершился clean,
полученный текст совпадает с независимо проверенной внешней композицией.
Все22 synthetic contract groups этой композиции PASS Chromium151/WebKit26.5.

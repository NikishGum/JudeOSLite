# RD-05 / #187 — независимая сверка CI test-only fix

14.10.2026. Reviewer `/root/finish_review`. **Verdict: ship для узкой правки сценария**; новых замечаний нет. Предыдущий UI/backend ship остаётся в силе, повторный visual pass не проводился и не требуется для этой правки.

Прочитаны текущий diff `frontend/e2e/redesign_dashboard.py`, относящийся rendering `AdminJournal`/`ParticipantRegistry`/`MembershipHistory`, CI failure logs `/tmp/rd05-logs/ci-push-failed.log` и `/tmp/rd05-logs/ci-pr-failed.log`.

Push run 38627562890 на исходном SHA70d1461 остановился на participant counter: ожидалось3, получено4. PR run38627604735 дошёл до финального `assert not errors` и сохранил WebKit access-control messages для pending journal/participant groups/list. Исходный сценарий действительно сразу выполнял reload/goto после появления контейнера/клика; он не дожидался всех проверяемых read responses и подтверждения archive. Это допускает гонку навигации с запросами. Логи сами по себе не доказывают универсальную причину всех сообщений WebKit.

Изменение устраняет именно эту неоднозначность:

- Три реальные journal records ожидаются до reload и после него. После reload count3 — более предметное условие, чем наличие пустой оболочки `.admin-journal`; records в этом компоненте не являются скрытым альтернативным представлением.
- После create ожидаются четыре строки реестра и одна строка истории группы перед уходом. История при создании карточки монтируется с пустым initial state и показывает строки после ответа; это ожидание не может пройти по старому DOM другой карточки.
- Archive ожидает ответ конкретного POST200, текст успешного сохранения, обновлённый список из трёх карточек и историю группы, затем независимо проверяет SQL count3. Только после этого открывается главная с прежним UI counter3.

Не добавлены force click, фиксированные sleep, увеличение таймаутов, retry всего сценария или фильтрация ошибок. Все прежние CTA geometry, DOM/keyboard, counters, ledger, rights и финальный pageerror assertions сохранены. Product/backend/CSS в этом delta не меняются.

Прочитаны фактические `/tmp/rd05-logs/dashboard-synchronized-chromium.log` и `dashboard-synchronized-webkit.log`: полные Chromium154.0.8037.57 и WebKit26.5 сценарии PASS. `git diff --check` PASS. Локальное ограничение WebKit прежнее: host package validation отключена для приватных runtime libraries; assertions выполнялись. Reviewer не присваивает этим результатам статус ещё не выполненного exact-SHA/main CI.

Правку можно коммитить и проверять штатным CI. Сохранить исходные failures в validation-протоколе и дождаться зелёного CI нового точного SHA перед доставкой.

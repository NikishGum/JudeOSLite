# Проверки раннего деплоя

Из корня репозитория в Cloud/CI:

```bash
python3 -m unittest discover -s deploy/test -p 'test_*.py' -v
bash -n deploy/test/bootstrap.sh
docker build --build-arg APP_REVISION=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa -f deploy/Dockerfile -t judeoslite:ci .
python3 deploy/test/integration_check.py
```

В управляемом Cloud передать build secret `proxy_ca` по инструкции runtime.
Интеграционный скрипт создаёт только случайный disposable Compose project,
отдельные volumes/network и временный каталог. Он использует настоящий
PostgreSQL, migrate, app и Caddy с локальным CA, проверяя сертификат через
доверенную публичную root.crt без отключения TLS verification.
Public DNS/ACME, VPS ownership/sudo/systemd/firewall/SSH/GHCR и PWA этим не проверяются.
Ручная подготовка владельцем: [operations-test.md](../../docs/operations-test.md).

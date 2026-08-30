# video-offers

Бэкенд «предложки видео для стримеров»: зрители отправляют ссылки, стример управляет очередью.

## Быстрый старт

```bash
cp .env.example .env
docker compose up -d db
make migrate-up
make run
```

API слушает `:8080` (см. `HTTP_ADDR` в `.env`).

Проверка:

```bash
curl -s localhost:8080/healthz
```

## Команды

| Команда | Описание |
|---|---|
| `make run` | Запуск API |
| `make build` | Сборка бинарника в `bin/api` |
| `make test` | Unit-тесты (`-short`, без Docker) |
| `make test-integration` | Unit + интеграционные тесты (нужен Docker) |
| `make migrate-up` | Накатить миграции |
| `make migrate-down` | Откатить одну миграцию |
| `make lint` | golangci-lint |

## Стек

Go 1.26, Fiber v2, PostgreSQL 16, pgx, goose, JWT + refresh tokens.

Спецификация API и доменная модель — в [AGENTS.md](AGENTS.md).  
Примеры curl — в [docs/api-examples.md](docs/api-examples.md).  
План MVP — в [docs/mvp-plan.md](docs/mvp-plan.md).

## Структура

```
cmd/api/           точка входа
internal/config/   env-конфиг
internal/domain/   типы и ошибки
internal/repo/     SQL-репозитории
internal/service/  бизнес-логика
internal/transport/http/  HTTP handlers
migrations/        goose SQL
```

## Переменные окружения

См. [.env.example](.env.example). Обязательные:

- `DATABASE_URL` — Postgres DSN
- `JWT_SECRET` — секрет для access JWT (в prod ≥ 32 байт)

Rate limit (по IP):

- `RATE_LIMIT_AUTH_MAX` / `RATE_LIMIT_AUTH_WINDOW` — `/auth/*`
- `RATE_LIMIT_OFFER_MAX` / `RATE_LIMIT_OFFER_WINDOW` — создание офферов

## Smoke-тест MVP

```bash
# 1. Register + login streamer и viewer
# 2. PATCH /me role=streamer
# 3. POST /streamers/:username/offers
# 4. GET /me/offers, PATCH status=watched
# 5. GET /me/sent
```

Полный чеклист — в [docs/mvp-plan.md](docs/mvp-plan.md#smoke-чеклист-mvp).

## Тесты

```bash
make test                 # быстро, без testcontainers
make test-integration     # с Postgres в Docker
```

Интеграционный тест `TestIntegration_MVPFlow` прогоняет полный сценарий MVP через Fiber `app.Test`.

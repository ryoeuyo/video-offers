# video-offers

Предложка видео для стримеров: зрители кидают ссылки, стример ведёт очередь. Сервис **не хостит видео** — хранятся только ссылка, заголовок, превью и статус.

## Что нужно

- Go 1.26+
- Node.js 20+ (для фронтенда)
- Docker + Docker Compose (PostgreSQL 16)
- `make`

## Локальная развёртка

### 1. Клонировать репозиторий

```bash
git clone https://github.com/ryoeuyo/video-offers.git
cd video-offers
```

### 2. Конфиг

```bash
cp .env.example .env
```

Минимум в `.env`:

| Переменная | Назначение |
|---|---|
| `DATABASE_URL` | DSN Postgres (совпадает с `docker-compose.yml`) |
| `JWT_SECRET` | секрет access JWT; в проде ≥ 32 байт |
| `HTTP_ADDR` | адрес API, по умолчанию `:8080` |
| `CORS_ORIGINS` | origin фронта, для dev: `http://localhost:5173` |

В продакшене задайте свой `JWT_SECRET`:

```bash
openssl rand -base64 48
```

### 3. База данных

```bash
docker compose up -d db
```

Дождитесь healthcheck (`pg_isready`), затем миграции:

```bash
make migrate-up
```

Если `go tool goose` недоступен:

```bash
go get -tool github.com/pressly/goose/v3/cmd/goose
make migrate-up
```

Проверка: `make migrate-status`.

### 4. API

```bash
make run
```

Проверка:

```bash
curl -s localhost:8080/healthz
```

Ожидается JSON со статусом API и БД.

### 5. Фронтенд

В **втором** терминале:

```bash
cd web && npm install
cd ..
make web
```

Откройте http://localhost:5173 — Vite проксирует `/api` на `:8080`.

Первый сценарий: регистрация → Настройки → «Стать стримером» → отправка видео на `/s/<username>`.

## Production (кратко)

1. Поднять Postgres 16, накатить `make migrate-up` с продовым `DATABASE_URL`.
2. Собрать API: `make build` → `bin/api`. Запускать с `APP_ENV=prod`, длинным `JWT_SECRET`, `LOG_LEVEL=info`.
3. Собрать фронт: `make web-build` → раздать `web/dist` (nginx/Caddy). `CORS_ORIGINS` должен содержать origin этого фронта.
4. Не коммитить `.env`. Rate limit: `RATE_LIMIT_*` в `.env.example`.

Обратного прокси: API на `/api` и `/healthz`, SPA — на `/`.

## Команды

| Команда | Описание |
|---|---|
| `make db-up` | Postgres в Docker |
| `make migrate-up` | Накатить миграции (goose) |
| `make run` | API (`go run ./cmd/api`) |
| `make web` | Vite `:5173` |
| `make web-build` | Production-сборка фронта |
| `make build` | Бинарник `bin/api` |
| `make test` | Unit-тесты (`-short`, без Docker) |
| `make test-integration` | Unit + интеграционные тесты (Docker) |
| `make lint` | golangci-lint |

## Стек

- **API:** Go, Fiber v2, PostgreSQL 16, pgx, goose, JWT + refresh, argon2id
- **Web:** React 19, TypeScript, Vite

Спецификация API — [AGENTS.md](AGENTS.md).  
Curl-примеры — [docs/api-examples.md](docs/api-examples.md).  
План MVP — [docs/mvp-plan.md](docs/mvp-plan.md).  
Twitch (backlog) — [docs/twitch-linking.md](docs/twitch-linking.md).

## Структура

```
cmd/api/                    точка входа
internal/config/            env-конфиг
internal/domain/            типы и ошибки
internal/repo/              SQL-репозитории
internal/service/           бизнес-логика
internal/transport/http/    HTTP handlers
migrations/                 goose SQL
web/                        React (Vite)
```

## Тесты

```bash
make test                 # быстро, без testcontainers
make test-integration     # с Postgres в Docker
```

Интеграционный тест `TestIntegration_MVPFlow` прогоняет полный сценарий MVP через Fiber `app.Test`.

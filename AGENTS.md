# video-offers

Бэкенд сервиса «предложка видео для стримеров». Зрители кидают ссылки на видео стримеру,
стример заходит в свою предложку и просматривает очередь.

Сервис **не хостит видео**. Храним только: ссылку, заголовок, превью (URL картинки), провайдера
и метаданные предложения. Проигрывание — переходом по ссылке на источник.

## Стек

- Go 1.26, [Fiber v2](https://docs.gofiber.io/) — HTTP
- PostgreSQL 16, драйвер `github.com/jackc/pgx/v5` + `pgxpool`
- Миграции — `goose` (`migrations/`, SQL-файлы, никакого auto-migrate на старте)
- Без ORM. Чистый SQL в слое репозиториев.
- Валидация — `go-playground/validator/v10`
- Логи — `log/slog` (JSON в проде, text в dev)
- Конфиг — переменные окружения через `caarlos0/env` + `.env` в dev

Module path: `github.com/ruslan/video-offers` (поправить, когда появится реальный репозиторий).

## Структура

```
cmd/api/main.go          точка входа: конфиг → пул БД → сервисы → роутер → graceful shutdown
internal/config          парсинг env, дефолты, валидация конфига
internal/domain          доменные типы и ошибки (User, Offer, Video, ErrNotFound ...). Без зависимостей.
internal/repo            доступ к БД: интерфейсы + pgx-реализации, SQL-запросы
internal/service         бизнес-логика: auth, offers, videos (резолв метаданных)
internal/transport/http  handler'ы Fiber, middleware, DTO запросов/ответов, роутинг
internal/pkg/...         мелкие утилиты (jwt, hash, pagination)
migrations               goose SQL-миграции
```

Направление зависимостей строго вниз: `transport → service → repo → domain`.
`service` принимает интерфейсы репозиториев, объявленные **в пакете service** (consumer-side interfaces),
а не в `repo`. Handler'ы не знают про SQL, репозитории не знают про HTTP.

## Доменная модель

**users** — `id (uuid)`, `email`, `username` (уникальный, он же slug предложки), `password_hash`,
`role` (`viewer` | `streamer`), `display_name`, `avatar_url`, `created_at`, `updated_at`.
Роль меняется самим пользователем в настройках (стать стримером). Отдельной сущности «стример» нет.

**streamer_settings** — `user_id (pk, fk users)`, `accepting_offers (bool)`, `min_account_age`,
`allow_anonymous`, `created_at`, `updated_at`. Создаётся при переключении роли в `streamer`.

**offers** — `id (uuid)`, `streamer_id (fk users)`, `sender_id (fk users, nullable)`,
`url`, `normalized_url`, `provider` (`youtube` | `twitch` | `vk` | `other`), `external_id`,
`title`, `thumbnail_url`, `duration_seconds (nullable)`, `comment (nullable, ≤500)`,
`status` (`pending` | `watched` | `skipped` | `rejected`), `watched_at (nullable)`, `created_at`.

Переходы статуса: `pending → watched | skipped | rejected`. Обратно — нельзя. Стример владеет статусом,
отправитель не может редактировать оффер после создания (только удалить свой `pending`).

Дедуп: уникальный частичный индекс на `(streamer_id, normalized_url) WHERE status = 'pending'` —
одно и то же видео нельзя предложить дважды, пока оно висит в очереди. Возвращаем 409.

`normalized_url` — канонический вид ссылки (без utm-меток, `youtu.be` → `youtube.com/watch?v=`,
без таймкодов), считается в сервисном слое.

## Резолв метаданных

При создании оффера сервер сам достаёт `title` / `thumbnail_url` / `duration`:
oEmbed-эндпоинты провайдеров (YouTube, Twitch, VK), с таймаутом 3s. Клиентским данным не доверяем.
Если провайдер не ответил — оффер создаётся с `title = ""`, фронт покажет голую ссылку;
резолв не блокирует создание. Реализация — интерфейс `VideoResolver` в `internal/service`,
чтобы в тестах подменялся фейком.

Ссылки на неизвестные домены принимаем как `provider = other`, но только `http`/`https`,
без приватных IP (SSRF).

## API

Префикс `/api/v1`. JSON везде. Аутентификация — `Authorization: Bearer <access_token>`.

| Метод | Путь | Доступ | Описание |
|---|---|---|---|
| POST | `/auth/register` | публично | регистрация |
| POST | `/auth/login` | публично | выдаёт access + refresh |
| POST | `/auth/refresh` | публично | обновление пары токенов |
| POST | `/auth/logout` | auth | отзыв refresh-токена |
| GET | `/me` | auth | текущий профиль |
| PATCH | `/me` | auth | display_name, avatar, роль |
| GET | `/me/settings` | streamer | настройки предложки |
| PATCH | `/me/settings` | streamer | вкл/выкл приём офферов |
| GET | `/streamers` | публично | список/поиск стримеров, пагинация |
| GET | `/streamers/:username` | публично | публичный профиль + статус приёма |
| POST | `/streamers/:username/offers` | auth | предложить видео |
| GET | `/me/offers` | streamer | своя очередь: `?status=&limit=&cursor=` |
| PATCH | `/me/offers/:id` | streamer | смена статуса |
| DELETE | `/me/offers/:id` | streamer | удалить оффер |
| GET | `/me/sent` | auth | что я отправил, со статусами |
| DELETE | `/me/sent/:id` | auth | отозвать свой `pending` оффер |
| GET | `/healthz` | публично | liveness + пинг БД |

### Соглашения

- Пагинация — **cursor-based** (`created_at, id`), ответ `{ "items": [...], "next_cursor": "..." }`.
  Никакого `offset` — очередь активно пополняется.
- Тела запросов/ответов — отдельные DTO в `transport/http`, доменные структуры наружу не отдаём.
- JSON-поля — `snake_case`. Время — RFC3339 UTC.
- Ошибка всегда одной формой:
  ```json
  { "error": { "code": "offer_duplicate", "message": "видео уже в очереди", "details": {} } }
  ```
  `code` — стабильный machine-readable снейк-кейс, `message` — для человека.
- Коды: 400 валидация, 401 нет/протух токен, 403 не своя сущность или не та роль,
  404 нет сущности, 409 конфликт (дубль, приём выключен), 422 невалидная ссылка, 429 rate limit.
- Доменные ошибки (`domain.ErrNotFound`, `domain.ErrConflict`, ...) маппятся в HTTP **в одном месте** —
  централизованный `ErrorHandler` Fiber. В handler'ах никаких `c.Status(404)` вручную.

### Auth

- Пароли — `argon2id` (`internal/pkg/hash`).
- Access JWT — 15 минут, в теле `sub`, `role`. Refresh — 30 дней, opaque-токен, хэш лежит в
  таблице `refresh_tokens` (ротация при каждом refresh, старый помечается использованным).
- Middleware: `RequireAuth` кладёт `userID` и `role` в locals; `RequireStreamer` проверяет роль.
- Rate limit на `/auth/*` и создание офферов (`fiber/middleware/limiter`).

## Команды

```bash
make run            # go run ./cmd/api
make migrate-up     # goose up
make migrate-down   # goose down (одна миграция)
make migrate-new n=add_offers   # новая миграция
make test           # go test ./...
make lint           # golangci-lint run
```

БД для разработки поднимается через `docker compose up -d db`.

## Конвенции кода

- Ошибки оборачиваем: `fmt.Errorf("create offer: %w", err)`. Слой repo превращает
  `pgx.ErrNoRows` и коды нарушения ограничений в доменные ошибки.
- `context.Context` первым аргументом во всём, что ходит в БД или в сеть.
- Никаких глобальных переменных и синглтонов: зависимости передаются через конструкторы `New*`.
- Тесты: сервисный слой — юнит-тесты с фейковыми репозиториями; репозитории и handler'ы —
  интеграционные на `testcontainers-go` с настоящим Postgres. Моки пишем руками, без mockery.
- Именование SQL: таблицы во множественном числе, snake_case, `id` — uuid v7 (генерируем в Go).

## Что осознанно отложено

OAuth через Twitch, вебсокеты/SSE для живой очереди, модерация и блок-листы отправителей,
платные приоритетные офферы, антиспам поверх rate limit. Не проектировать под это заранее,
но и не мешать: очередь читается по `streamer_id + status + created_at`.

# Задача: привязка Twitch и ограничения отправки офферов

**Статус:** backlog (post-MVP)  
**Приоритет:** высокий  
**Зависит от:** MVP auth, streamer settings, create offer  
**Связано:** [AGENTS.md](../AGENTS.md) — «OAuth через Twitch», `min_account_age`, `allow_anonymous`; UI и PATCH настроек gate — [offer-sender-gates.md](./offer-sender-gates.md)

---

## Цель

1. Любой пользователь может **привязать Twitch** к аккаунту OfferBox.
2. **Стример обязан** иметь привязанный Twitch, чтобы принимать офферы.
3. Стример настраивает, **кто может отправлять ему видео**, по данным Twitch:
   - есть ли follow на канал стримера;
   - как давно пользователь подписан (follow-time);
   - есть ли платная подписка (sub).

---

## User stories

| Как | Хочу | Чтобы |
|---|---|---|
| Зритель | привязать Twitch к профилю | стример мог проверить follow/sub при отправке видео |
| Стример | обязательно привязать Twitch | зрители видели мой канал и я мог включить проверки |
| Стример | включить «только фолловеры» | офферы не слали случайные люди |
| Стример | задать мин. follow-time (например 3 мес.) | отсечь свежие аккаунты / фоллов-bait |
| Стример | разрешить офферы только субам | награждать подписчиков |
| Зритель без Twitch | понять, почему не могу отправить | привязать аккаунт или выполнить условия |

---

## Функциональные требования

### Привязка Twitch (все роли)

- OAuth 2.0 Authorization Code через [Twitch Developer Console](https://dev.twitch.tv/console).
- Scopes пользователя (минимум):
  - `user:read:email` — опционально, если нужен email match (скорее не нужен)
  - **`user:read:follows`** — проверка, на кого подписан отправитель
- Scopes стримера (дополнительно при роли `streamer` или при включении sub-gate):
  - **`channel:read:subscriptions`** — проверка, подписан ли user X на канал стримера
- Один Twitch-аккаунт → один OfferBox-аккаунт (unique `twitch_user_id`).
- Unlink: viewer может отвязать; **streamer с `accepting_offers = true` — нельзя** без отключения приёма.
- UI: кнопка «Привязать Twitch» / статус «Привязан как @login».

### Обязательность для стримера

- Переключение в `role = streamer` **разрешено** без Twitch (чтобы не ломать onboarding).
- `accepting_offers = true` **запрещено**, пока Twitch не привязан → `409` `twitch_required`.
- Публичный профиль стримера показывает `twitch_login` (если привязан).

### Настройки gate (стример)

Расширение `streamer_settings`:

| Поле | Тип | Описание |
|---|---|---|
| `require_twitch_sender` | bool | Отправитель должен иметь привязанный Twitch (default `false` → позже `true`?) |
| `require_follow` | bool | Отправитель должен быть фолловером канала стримера |
| `min_follow_age_seconds` | int | Мин. время с момента follow (0 = не проверять) |
| `require_subscription` | bool | Нужна активная paid sub на канале стримера |

Правила:

- `min_follow_age_seconds > 0` ⇒ неявно включает `require_follow`.
- `require_subscription = true` ⇒ проверка sub через API стримера (нужен его OAuth с `channel:read:subscriptions`).
- Несколько условий — **AND** (все включённые должны выполняться).
- При нарушении — `403` с кодами:
  - `twitch_not_linked` — у отправителя нет привязки
  - `twitch_not_following`
  - `twitch_follow_too_new`
  - `twitch_subscription_required`

### Проверка при создании оффера

В `OfferService.Create`, после проверки `accepting_offers`:

1. Стример имеет привязанный Twitch (иначе приём уже выключен или 409 на settings).
2. Если gate включены — у отправителя есть запись `user_twitch_links`.
3. Запрос к Twitch API (от имени **отправителя** для follow, **стримера** для sub):
   - Follow: `GET https://api.twitch.tv/helix/channels/followers?broadcaster_id={streamer_twitch_id}&user_id={sender_twitch_id}`
   - Sub: `GET https://api.twitch.tv/helix/subscriptions/user?broadcaster_id=...&user_id=...` (token стримера)
4. Сравнить `followed_at` с `now - min_follow_age_seconds`.
5. Кешировать результат проверки на короткое время (например 60–300s), ключ `(streamer_id, sender_id, rules_hash)`.

При недоступности Twitch API — **fail closed** (`503` `twitch_unavailable`), не пропускать оффер молча.

---

## Модель данных (черновик)

### Миграция `00002_twitch.sql`

```sql
CREATE TABLE user_twitch_links (
    user_id           uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    twitch_user_id    text        NOT NULL,
    twitch_login      text        NOT NULL,
    twitch_display_name text      NOT NULL DEFAULT '',
    access_token_enc  bytea       NOT NULL,  -- или text + app-level encryption
    refresh_token_enc bytea       NOT NULL,
    token_expires_at  timestamptz NOT NULL,
    scopes            text[]      NOT NULL DEFAULT '{}',
    linked_at         timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX user_twitch_links_twitch_user_id_key ON user_twitch_links (twitch_user_id);

ALTER TABLE streamer_settings
    ADD COLUMN require_twitch_sender   boolean NOT NULL DEFAULT false,
    ADD COLUMN require_follow          boolean NOT NULL DEFAULT false,
    ADD COLUMN min_follow_age_seconds  integer NOT NULL DEFAULT 0,
    ADD COLUMN require_subscription    boolean NOT NULL DEFAULT false;
```

Токены Twitch — **шифровать at rest** (ключ `TWITCH_TOKEN_ENC_KEY` или reuse JWT_SECRET в dev только).

---

## API (новое / изменения)

| Метод | Путь | Доступ | Описание |
|---|---|---|---|
| GET | `/auth/twitch/connect` | auth | Redirect на Twitch OAuth (`state` + PKCE) |
| GET | `/auth/twitch/callback` | публично | Callback, сохранение link, redirect на фронт |
| GET | `/me/twitch` | auth | `{ linked, login, display_name, linked_at }` |
| DELETE | `/me/twitch` | auth | Отвязка (с проверкой роли/accepting_offers) |
| PATCH | `/me/settings` | streamer | + поля gate (см. таблицу выше) |

Изменения существующих:

- `POST /streamers/:username/offers` — gate checks, новые error codes.
- `GET /streamers/:username` — опционально `twitch_login`, флаги gate (без секретов).
- `PATCH /me/settings` `{ accepting_offers: true }` — 409 если Twitch не привязан.

---

## Twitch API (справка)

| Проверка | Endpoint | Чей token |
|---|---|---|
| Follow exists + `followed_at` | `GET /helix/channels/followers` | App или user token отправителя |
| Subscription | `GET /helix/subscriptions/user` | **User token стримера** + scope `channel:read:subscriptions` |

Rate limits Twitch — учитывать в кеше; batch не нужен на MVP этой фичи.

---

## Фронтенд

- **Настройки:** блок «Twitch» (привязать / отвязать / статус).
- **Стример:** секция «Кто может предлагать видео» — чекбоксы follow / sub / slider или input follow-time (дни/месяцы).
- **Страница оффера:** если 403 с `twitch_not_linked` — CTA «Привязать Twitch».
- **Стать стримером:** подсказка «Для приёма офферов нужен Twitch».

---

## Фазы реализации

### Фаза A — OAuth link (2–3 дня)

- [x] Twitch app, env: `TWITCH_CLIENT_ID`, `TWITCH_CLIENT_SECRET`, `TWITCH_REDIRECT_URI`
- [x] `user_twitch_links` repo + encrypt/decrypt tokens
- [x] Connect / callback / GET / DELETE `/me/twitch`
- [x] Unit + integration tests (mock Twitch HTTP)
- [x] Фронт: привязка в настройках

**Критерий:** viewer привязывает Twitch, видит login в профиле.

### Фаза B — Обязательность для стримера (0.5–1 день)

- [x] Block `accepting_offers=true` без link
- [x] Показ `twitch_login` на публичном профиле
- [x] Тесты + UI предупреждения

**Критерий:** стример без Twitch не может включить приём.

### Фаза C — Follow gate (2–3 дня)

- [x] Поля settings + PATCH
- [x] `TwitchClient` interface: `IsFollower`, cache
- [x] Enforcement в `Create`
- [x] Error codes + фронт сообщения

**Критерий:** non-follower получает 403; follower — 201.

### Фаза D — Follow-time + subscription (2–3 дня)

- [x] `min_follow_age_seconds` enforcement
- [x] Re-auth стримера для `channel:read:subscriptions` при включении sub-gate
- [x] Sub check + тесты

**Критерий:** свежий follow и non-sub отклоняются по правилам.

---

## Тесты (обязательно)

| Слой | Что |
|---|---|
| `service` | gate logic, fail closed, streamer без twitch |
| `service/twitch` | parse token refresh, follow/sub responses (httptest) |
| `repo` | unique twitch_user_id, unlink |
| `transport/http` | OAuth callback, create offer 403/503 |
| `web` | ручной smoke: link → settings → offer |

---

## Env

```env
TWITCH_CLIENT_ID=
TWITCH_CLIENT_SECRET=
TWITCH_REDIRECT_URI=http://localhost:8080/api/v1/auth/twitch/callback
TWITCH_TOKEN_ENC_KEY=   # 32 bytes, prod
FRONTEND_OAUTH_SUCCESS_URL=http://localhost:5173/settings
```

---

## Открытые вопросы

1. **Кеш follow/sub:** 60s vs 5min — баланс UX и rate limit.
2. **Gift sub / Prime:** считать Prime за sub или только paid tier?
3. **Стример сменил Twitch:** перепривязка, что с очередью и старыми проверками.
4. **Mod/VIP без sub:** отдельные флаги позже или out of scope.
5. **Анонимные офферы:** при Twitch-gate `allow_anonymous` всегда false — зафиксировать в спеке.

---

## Оценка

| Фаза | Дни |
|---|---|
| A OAuth | 2–3 |
| B Streamer required | 0.5–1 |
| C Follow | 2–3 |
| D Follow-time + sub | 2–3 |
| **Итого** | **~7–10 дней** |

---

## Definition of Done

- [ ] Любой user может link/unlink Twitch (viewer).
- [ ] Streamer не включает приём без Twitch.
- [ ] Streamer настраивает follow / follow-time / sub через `/me/settings`.
- [ ] Create offer проверяет правила; понятные error codes.
- [ ] Токены зашифрованы; refresh работает.
- [ ] `make test` зелёный; smoke в [docs/api-examples.md](./api-examples.md) дополнен.

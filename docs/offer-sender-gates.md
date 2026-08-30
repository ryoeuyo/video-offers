# Задача: кто может кидать предложки

**Статус:** done  
**Приоритет:** высокий  
**Зависит от:** MVP create offer, [привязка Twitch](./twitch-linking.md) (фазы A/B готовы)  
**Связано:** `streamer_settings`, `PATCH /me/settings`, `POST /streamers/:username/offers`

---

## Цель

Стример в настройках задаёт, **кто может отправлять ему видео**. Правила проверяются при `POST /streamers/:username/offers`. Зритель на странице стримера заранее видит условия и понятную ошибку, если не подходит.

Не проектируем блок-листы, VIP/mod и анонимные офферы — только правила по аккаунту отправителя.

---

## User stories

| Как | Хочу | Чтобы |
|---|---|---|
| Стример | включить «только с привязанным Twitch» | случайные аккаунты не засоряли очередь |
| Стример | включить «только фолловеры» | офферы слали люди с канала |
| Стример | задать мин. возраст follow (дни) | отсечь свежие фолловы |
| Стример | разрешить офферы только субам | награждать подписчиков |
| Стример | задать мин. возраст аккаунта OfferBox | отсечь однодневные регистрации |
| Зритель | видеть правила до отправки | не гадать, почему форма отклоняет |

---

## Текущее состояние

Колонки в БД уже есть, логики и UI нет.

| Слой | Что есть | Чего нет |
|---|---|---|
| `migrations/00001` | `allow_anonymous`, `min_account_age_seconds` | enforcement |
| `migrations/00002` | `require_twitch_sender`, `require_follow`, `min_follow_age_seconds`, `require_subscription` | чтение/запись в repo |
| `domain.StreamerSettings` | `AcceptingOffers`, `AllowAnonymous`, `MinAccountAge` | поля Twitch-gate |
| `repo/streamer_settings.go` | SELECT/UPDATE без Twitch-колонок | |
| `PATCH /me/settings` | только `accepting_offers` | остальные поля |
| `OfferService.Create` | только `accepting_offers` | все gates |
| Settings UI | чекбокс «принимать офферы» | секция «кто может предлагать» |
| Публичный профиль | `accepting_offers`, `twitch_login` | флаги правил |

`allow_anonymous` **не трогаем**: все офферы только от авторизованных (`POST` под Bearer). Поле остаётся в БД/ответе как есть.

---

## Настройки (стример)

Расширение `streamer_settings` / `PATCH /me/settings`:

| Поле | Тип | Default | Описание |
|---|---|---|---|
| `min_account_age_seconds` | int ≥ 0 | `0` | Мин. возраст аккаунта отправителя на OfferBox (`users.created_at`). `0` — не проверять |
| `require_twitch_sender` | bool | `false` | У отправителя должна быть запись в `user_twitch_links` |
| `require_follow` | bool | `false` | Отправитель фолловит канал стримера |
| `min_follow_age_seconds` | int ≥ 0 | `0` | Мин. время с `followed_at`. `0` — не проверять давность |
| `require_subscription` | bool | `false` | Активная платная sub на канал стримера |

### Инварианты (AND всех включённых)

1. `min_follow_age_seconds > 0` ⇒ неявно `require_follow = true`.
2. `require_follow` или `require_subscription` или `min_follow_age_seconds > 0` ⇒ неявно `require_twitch_sender = true`.
3. Любой Twitch-gate (`require_twitch_sender` / follow / sub) **запрещён**, если у стримера нет привязки Twitch → `409` `twitch_required` (как при `accepting_offers=true`).
4. `require_subscription = true` без scope `channel:read:subscriptions` у стримера → `409` `twitch_scope_required` (стример должен перепривязать Twitch). Пока scope нет — в UI disabled + подсказка.
5. Выключить приём офферов можно всегда; выключить отдельный gate — всегда.

UI может принимать возраст follow в **днях**, API хранит секунды.

---

## Проверка при создании оффера

В `OfferService.Create`, сразу после `accepting_offers`:

1. **Возраст аккаунта.** Если `min_account_age_seconds > 0` и `now - sender.created_at < N` → `403` `account_too_new`.
2. Если ни один Twitch-gate не включён — дальше как сейчас.
3. Если gate включены — у отправителя есть `user_twitch_links`, иначе `403` `twitch_not_linked`.
4. Follow / follow-time / sub — как в [twitch-linking.md](./twitch-linking.md) (Helix, кеш, fail closed `503` `twitch_unavailable`).

Порядок дешёвых проверок сначала (БД), сеть Twitch — в конце.

Коды ошибок:

| code | HTTP | Когда |
|---|---|---|
| `account_too_new` | 403 | аккаунт OfferBox младше порога |
| `twitch_not_linked` | 403 | нет привязки Twitch у отправителя |
| `twitch_not_following` | 403 | не фолловит канал |
| `twitch_follow_too_new` | 403 | follow моложе порога |
| `twitch_subscription_required` | 403 | нет sub |
| `twitch_unavailable` | 503 | Helix недоступен (fail closed) |

---

## API

### `GET` / `PATCH /api/v1/me/settings`

Ответ и тело PATCH (все поля опциональны в PATCH):

```json
{
  "accepting_offers": true,
  "allow_anonymous": false,
  "min_account_age_seconds": 86400,
  "require_twitch_sender": true,
  "require_follow": true,
  "min_follow_age_seconds": 2592000,
  "require_subscription": false
}
```

Валидация: `min_*_seconds ≥ 0`; неизвестные поля игнорировать.

### `GET /api/v1/streamers/:username`

Добавить в публичный ответ (без секретов), чтобы фронт показал правила:

```json
{
  "offer_rules": {
    "min_account_age_seconds": 86400,
    "require_twitch_sender": true,
    "require_follow": true,
    "min_follow_age_seconds": 2592000,
    "require_subscription": false
  }
}
```

Нули/false можно опускать (`omitempty`), но проще всегда отдавать полный объект.

---

## Фронтенд

### Настройки стримера — секция «Кто может предлагать видео»

Только при `role = streamer`. Disabled, пока Twitch не привязан (кроме возраста аккаунта OfferBox).

- чекбокс «Только с привязанным Twitch»
- чекбокс «Только фолловеры канала»
- поле «Минимальный возраст follow» (дни, `0` = без ограничения)
- чекбокс «Только подписчики» + подсказка про re-auth, если нет scope
- поле «Минимальный возраст аккаунта OfferBox» (дни)

Сохраняется тем же `PATCH /me/settings`, что и приём офферов.

### Страница стримера (`/s/:username`)

Если офферы принимаются — короткий список условий над формой. На `403` с кодами выше — сообщение + CTA «Привязать Twitch» для `twitch_not_linked`.

---

## Фазы реализации

### Фаза 1 — Модель и PATCH (1–2 дня)

- [x] Поля Twitch-gate в `domain.StreamerSettings`
- [x] `repo/streamer_settings.go` — SELECT/INSERT/UPDATE всех колонок
- [x] `UpdateSettingsInput` + валидация инвариантов
- [x] DTO GET/PATCH, `offer_rules` на публичном профиле
- [x] Unit-тесты: implicit flags, `twitch_required`, `twitch_scope_required`

**Критерий:** стример сохраняет правила, `GET /me/settings` и публичный профиль их возвращают. Create ещё не фильтрует.

### Фаза 2 — Возраст аккаунта OfferBox (0.5 дня)

- [x] `OfferService.Create` проверяет `users.created_at`
- [x] Тесты: молодой аккаунт 403, старый 201
- [x] UI поле в настройках + текст ошибки на странице оффера

**Критерий:** порог работает без Twitch API.

### Фаза 3 — Twitch gates (см. фазы C/D в twitch-linking.md)

- [x] `TwitchClient`: `IsFollower`, `FollowedAt`, `IsSubscriber` + кеш
- [x] Enforcement в `Create`
- [x] UI чекбоксы + дни follow + sub
- [x] Сообщения на странице оффера

**Критерий:** non-follower / свежий follow / non-sub получают 403; подходящий зритель — 201.

---

## Тесты (обязательно)

| Слой | Что |
|---|---|
| `service` | инварианты PATCH; `account_too_new`; каждый Twitch-код; fail closed |
| `repo` | round-trip новых колонок |
| `transport/http` | PATCH settings 200/409; create offer 403/503 |
| `web` | smoke: сохранить правила → чужой аккаунт видит отказ |

---

## Вне скоупа

- Блок-лист / allow-list конкретных username
- Отдельные флаги для mod / VIP
- Анонимные офферы (`allow_anonymous`)
- Платная приоритетная очередь

---

## Definition of Done

- [x] Стример сохраняет набор правил через `/me/settings`.
- [x] Create offer отклоняет отправителя по правилам с стабильными `code`.
- [x] Публичный профиль и страница оффера показывают условия.
- [x] Twitch-gate нельзя включить без привязки стримера.
- [x] `make test` зелёный; примеры в [api-examples.md](./api-examples.md) дополнены.

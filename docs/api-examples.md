# API examples

Базовый URL: `http://localhost:8080/api/v1`

Все тела — JSON. Время в ответах — RFC3339 UTC.

## Health

```bash
curl -s localhost:8080/healthz
```

## Auth

```bash
# Register
curl -s -X POST localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","username":"alice","password":"password1"}'

# Login
curl -s -X POST localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password1"}'

# Refresh
curl -s -X POST localhost:8080/api/v1/auth/refresh \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"<refresh_token>"}'

# Logout
curl -s -X POST localhost:8080/api/v1/auth/logout \
  -H "Authorization: Bearer <access_token>" \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"<refresh_token>"}'
```

## Profile

```bash
# Current user
curl -s localhost:8080/api/v1/me \
  -H "Authorization: Bearer <access_token>"

# Update profile / become streamer
curl -s -X PATCH localhost:8080/api/v1/me \
  -H "Authorization: Bearer <access_token>" \
  -H 'Content-Type: application/json' \
  -d '{"display_name":"Alice","role":"streamer"}'
```

## Streamer settings

```bash
curl -s localhost:8080/api/v1/me/settings \
  -H "Authorization: Bearer <access_token>"

curl -s -X PATCH localhost:8080/api/v1/me/settings \
  -H "Authorization: Bearer <access_token>" \
  -H 'Content-Type: application/json' \
  -d '{"accepting_offers":false}'
```

## Public streamers

```bash
# List (optional ?q=prefix&limit=20&cursor=...)
curl -s 'localhost:8080/api/v1/streamers?limit=20'

# Profile
curl -s localhost:8080/api/v1/streamers/alice
```

## Offers

```bash
# Create offer (viewer → streamer)
curl -s -X POST localhost:8080/api/v1/streamers/alice/offers \
  -H "Authorization: Bearer <access_token>" \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://www.youtube.com/watch?v=dQw4w9WgXcQ","comment":"must watch"}'

# Streamer queue
curl -s 'localhost:8080/api/v1/me/offers?status=pending&limit=20' \
  -H "Authorization: Bearer <streamer_access_token>"

# Change status
curl -s -X PATCH localhost:8080/api/v1/me/offers/<offer_id> \
  -H "Authorization: Bearer <streamer_access_token>" \
  -H 'Content-Type: application/json' \
  -d '{"status":"watched"}'

# Delete offer (streamer)
curl -s -X DELETE localhost:8080/api/v1/me/offers/<offer_id> \
  -H "Authorization: Bearer <streamer_access_token>"

# Sent offers (viewer)
curl -s 'localhost:8080/api/v1/me/sent?limit=20' \
  -H "Authorization: Bearer <access_token>"

# Revoke pending (viewer)
curl -s -X DELETE localhost:8080/api/v1/me/sent/<offer_id> \
  -H "Authorization: Bearer <access_token>"
```

## Error format

```json
{
  "error": {
    "code": "offer_duplicate",
    "message": "видео уже в очереди",
    "details": {}
  }
}
```

Common codes: `invalid_input`, `unauthorized`, `forbidden`, `not_found`, `conflict`, `offer_duplicate`, `offers_disabled`, `rate_limited`.

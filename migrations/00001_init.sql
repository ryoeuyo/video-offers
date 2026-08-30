-- +goose Up
-- +goose StatementBegin
CREATE TYPE user_role AS ENUM ('viewer', 'streamer');
CREATE TYPE offer_status AS ENUM ('pending', 'watched', 'skipped', 'rejected');
CREATE TYPE video_provider AS ENUM ('youtube', 'twitch', 'vk', 'other');

CREATE TABLE users (
    id            uuid PRIMARY KEY,
    email         text        NOT NULL,
    username      text        NOT NULL,
    password_hash text        NOT NULL,
    role          user_role   NOT NULL DEFAULT 'viewer',
    display_name  text        NOT NULL DEFAULT '',
    avatar_url    text        NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- Регистр не должен создавать двух разных XQC.
CREATE UNIQUE INDEX users_email_key ON users (lower(email));
CREATE UNIQUE INDEX users_username_key ON users (lower(username));

CREATE TABLE streamer_settings (
    user_id          uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    accepting_offers boolean     NOT NULL DEFAULT true,
    allow_anonymous  boolean     NOT NULL DEFAULT false,
    -- Секунды, а не interval: pgx мапит его в pgtype.Interval, а нам нужен time.Duration.
    min_account_age_seconds integer NOT NULL DEFAULT 0,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id         uuid PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash text        NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens (user_id);

CREATE TABLE offers (
    id               uuid PRIMARY KEY,
    streamer_id      uuid           NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- Отправитель может удалить аккаунт, оффер в очереди при этом остаётся.
    sender_id        uuid           REFERENCES users (id) ON DELETE SET NULL,
    url              text           NOT NULL,
    normalized_url   text           NOT NULL,
    provider         video_provider NOT NULL DEFAULT 'other',
    external_id      text           NOT NULL DEFAULT '',
    title            text           NOT NULL DEFAULT '',
    thumbnail_url    text           NOT NULL DEFAULT '',
    duration_seconds integer,
    comment          text           NOT NULL DEFAULT '',
    status           offer_status   NOT NULL DEFAULT 'pending',
    watched_at       timestamptz,
    created_at       timestamptz    NOT NULL DEFAULT now(),

    CONSTRAINT offers_comment_len CHECK (char_length(comment) <= 500),
    CONSTRAINT offers_watched_at_pending CHECK (status <> 'pending' OR watched_at IS NULL)
);

-- Одно и то же видео нельзя предложить дважды, пока предыдущее висит в очереди.
CREATE UNIQUE INDEX offers_pending_dedup_key
    ON offers (streamer_id, normalized_url)
    WHERE status = 'pending';

-- Основной запрос: очередь стримера с фильтром по статусу, курсор по (created_at, id).
CREATE INDEX offers_queue_idx ON offers (streamer_id, status, created_at DESC, id DESC);

-- Лента «что я отправил».
CREATE INDEX offers_sender_idx ON offers (sender_id, created_at DESC, id DESC)
    WHERE sender_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE offers;
DROP TABLE refresh_tokens;
DROP TABLE streamer_settings;
DROP TABLE users;
DROP TYPE video_provider;
DROP TYPE offer_status;
DROP TYPE user_role;
-- +goose StatementEnd

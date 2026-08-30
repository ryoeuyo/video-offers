-- +goose Up
-- +goose StatementBegin
CREATE TABLE user_twitch_links (
    user_id             uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    twitch_user_id      text        NOT NULL,
    twitch_login        text        NOT NULL,
    twitch_display_name text        NOT NULL DEFAULT '',
    access_token_enc    bytea       NOT NULL,
    refresh_token_enc   bytea       NOT NULL,
    token_expires_at    timestamptz NOT NULL,
    scopes              text[]      NOT NULL DEFAULT '{}',
    linked_at           timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX user_twitch_links_twitch_user_id_key ON user_twitch_links (twitch_user_id);

ALTER TABLE streamer_settings
    ADD COLUMN require_twitch_sender  boolean NOT NULL DEFAULT false,
    ADD COLUMN require_follow         boolean NOT NULL DEFAULT false,
    ADD COLUMN min_follow_age_seconds integer NOT NULL DEFAULT 0,
    ADD COLUMN require_subscription   boolean NOT NULL DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE streamer_settings
    DROP COLUMN IF EXISTS require_subscription,
    DROP COLUMN IF EXISTS min_follow_age_seconds,
    DROP COLUMN IF EXISTS require_follow,
    DROP COLUMN IF EXISTS require_twitch_sender;

DROP TABLE user_twitch_links;
-- +goose StatementEnd

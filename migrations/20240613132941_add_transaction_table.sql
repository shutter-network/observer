-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS encrypted_tx
(
    id                  BIGSERIAL PRIMARY KEY,
    tx                  BYTEA                                  NOT NULL,
    identity_preimage   BYTEA UNIQUE                           NOT NULL,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

CREATE TABLE IF NOT EXISTS decryption_data
(
    id                  BIGSERIAL PRIMARY KEY,
    "key"               BYTEA                                  NOT NULL,
    slot                BIGINT                                 NOT NULL,
    block_hash          BYTEA,
    identity_preimage   BYTEA UNIQUE                           NOT NULL,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

CREATE TABLE IF NOT EXISTS key_share
(
    id                  BIGSERIAL PRIMARY KEY,
    key_share           BYTEA                                  NOT NULL,
    slot                BIGINT                                 NOT NULL,
    identity_preimage   BYTEA UNIQUE                           NOT NULL,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE encrypted_tx;
DROP TABLE decryption_data;
DROP TABLE key_share;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS transaction
(
    id                  BIGSERIAL PRIMARY KEY,
    encrypted_tx        BYTEA,
    decryption_key      BYTEA,
    slot                BIGINT,
    block_hash          BYTEA,
    identity_preimage   BYTEA UNIQUE                           NOT NULL,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE transaction;
-- +goose StatementEnd

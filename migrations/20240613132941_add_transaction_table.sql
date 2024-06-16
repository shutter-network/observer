-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS transaction
(
    id                  BIGSERIAL PRIMARY KEY,
    encrypted_tx        BYTEA,
    decryption_key      BYTEA,
    slot                BIGINT,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE transaction;

DROP TYPE transaction_v1;
-- +goose StatementEnd

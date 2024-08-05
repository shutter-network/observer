-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS validator_registry
(
    id                  SERIAL PRIMARY KEY,
    version             BIGINT NOT NULL,
    chain_id            BIGINT NOT NULL,
    sender              BYTEA  NOT NULL,
    validator_index     BIGINT NOT NULL,
    nonce               BIGINT NOT NULL,
    is_registeration    BOOLEAN NOT NULL,
    event_block_number        BIGINT UNIQUE NOT NULL,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE validator_registry;
-- +goose StatementEnd

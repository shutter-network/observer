-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS validator_registration_message
(
    id                          SERIAL PRIMARY KEY,
    version                     BIGINT NOT NULL,
    chain_id                    BIGINT NOT NULL,
    validator_index             BIGINT NOT NULL,
    nonce                       BIGINT NOT NULL,
    is_registeration            BOOLEAN NOT NULL,
    signature                   BYTEA NOT NULL,
    event_block_number          BIGINT NOT NULL,
    created_at                  TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    updated_at                  TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL
);

CREATE TABLE validator_registry_events_synced_until(
    enforce_one_row bool PRIMARY KEY DEFAULT true,
    block_number bigint NOT NULL CHECK (block_number >= 0)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE validator_registration_message;
-- +goose StatementEnd

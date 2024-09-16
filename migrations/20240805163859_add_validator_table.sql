-- +goose Up
-- +goose StatementBegin
CREATE TYPE validator_registration_validity AS ENUM 
(
    'valid', 
    'invalid message',
    'invalid signature'
);

CREATE TABLE IF NOT EXISTS validator_registration_message
(
    id                          BIGSERIAL PRIMARY KEY,
    version                     BIGINT,
    chain_id                    BIGINT,
    validator_registry_address  BYTEA,
    validator_index             BIGINT,
    nonce                       BIGINT,
    is_registeration            BOOLEAN,
    signature                   BYTEA NOT NULL,
    event_block_number          BIGINT NOT NULL,
    event_tx_index              BIGINT NOT NULL,
    event_log_index             BIGINT NOT NULL,
    validity                    validator_registration_validity NOT NULL,
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
DROP TYPE validator_registration_validity CASCADE;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS decrypted_tx
(
    slot                BIGINT,
    tx_index            BIGINT,
    tx_hash             BYTEA UNIQUE NOT NULL,
    tx_status           tx_status_val NOT NULL,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    PRIMARY KEY (slot, tx_index)
);

CREATE TYPE tx_status_val AS ENUM 
(
    'included', 
    'not included',
    'unable to decrypt'
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE decrypted_tx;
DROP TYPE tx_status_val;
-- +goose StatementEnd

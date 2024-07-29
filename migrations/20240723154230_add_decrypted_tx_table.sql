-- +goose Up
-- +goose StatementBegin

CREATE TYPE tx_status_val AS ENUM 
(
    'included', 
    'not included',
    'unable to decrypt'
);


CREATE TABLE IF NOT EXISTS decrypted_tx
(
    slot                BIGINT,
    tx_index            BIGINT,
    tx_hash             BYTEA NOT NULL,
    tx_status           tx_status_val NOT NULL,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    PRIMARY KEY (slot, tx_index)
);

ALTER TABLE block
ADD COLUMN slot BIGINT DEFAULT 0;

ALTER TABLE block
ALTER COLUMN slot SET NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TYPE tx_status_val;
DROP TABLE decrypted_tx;
-- +goose StatementEnd

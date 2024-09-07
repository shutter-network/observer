-- +goose Up
-- +goose StatementBegin

CREATE TYPE tx_status_val AS ENUM 
(
    'included', 
    'not included'
);


CREATE TABLE IF NOT EXISTS decrypted_tx
(
    id                                  SERIAL PRIMARY KEY,
    slot                                BIGINT NOT NULL,
    tx_index                            BIGINT NOT NULL,
    tx_hash                             BYTEA NOT NULL,
    tx_status                           tx_status_val NOT NULL,
    decryption_key_id                   BIGINT NOT NULL,
    transaction_submitted_event_id      BIGINT NOT NULL,
    created_at                          TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    updated_at                          TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    UNIQUE (slot, tx_index),
    FOREIGN KEY (decryption_key_id) REFERENCES decryption_key (id),
    FOREIGN KEY (transaction_submitted_event_id) REFERENCES transaction_submitted_event (id)
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

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS encrypted_tx
(
    tx_index                    BIGINT                                 NOT NULL,
    eon                         BIGINT                                 NOT NULL,
    tx                          BYTEA                                  NOT NULL,
    identity_preimage           BYTEA                                  NOT NULL,
    created_at                  TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at                  TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    PRIMARY KEY (tx_index, eon)
);

CREATE TABLE IF NOT EXISTS decryption_data
(
    eon                          BIGINT                                 NOT NULL,
    identity_preimage            BYTEA                                  NOT NULL,     
    decryption_key               BYTEA                                  NOT NULL,
    slot                         BIGINT                                 NOT NULL,
    block_hash                   BYTEA,
    created_at                   TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at                   TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    PRIMARY KEY (eon, identity_preimage)
);

CREATE TABLE IF NOT EXISTS decryption_key_share
(
    eon                         BIGINT                                  NOT NULL,
    identity_preimage           BYTEA                                   NOT NULL,
    keyper_index                BIGINT                                  NOT NULL,
    decryption_key_share        BYTEA                                   NOT NULL,
    slot                        BIGINT                                  NOT NULL,
    created_at                  TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    updated_at                  TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    PRIMARY KEY (eon, identity_preimage, keyper_index)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE encrypted_tx;
DROP TABLE decryption_data;
DROP TABLE decryption_key_share;
-- +goose StatementEnd

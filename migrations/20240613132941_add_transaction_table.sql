-- +goose Up
-- +goose StatementBegin
CREATE TABLE decryption_keys_message (
    slot            BIGINT PRIMARY KEY,
    instance_id     BIGINT NOT NULL,
    eon             BIGINT NOT NULL,
    tx_pointer      BIGINT NOT NULL,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL
);

CREATE TABLE decryption_key (
    id                          BIGSERIAL PRIMARY KEY,
    eon                         BIGINT,
    identity_preimage           BYTEA,
    key                         BYTEA NOT NULL,
    created_at                  TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    updated_at                  TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    UNIQUE (eon, identity_preimage)
);

CREATE TABLE decryption_keys_message_decryption_key (
    decryption_keys_message_slot            BIGINT,
    key_index                               BIGINT,
    decryption_key_id                       BIGINT,
    created_at                              TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    updated_at                              TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    PRIMARY KEY (decryption_keys_message_slot, decryption_key_id, key_index),
    FOREIGN KEY (decryption_keys_message_slot) REFERENCES decryption_keys_message (slot),
    FOREIGN KEY (decryption_key_id) REFERENCES decryption_key (id)
);

CREATE TABLE transaction_submitted_event (
    id                                  BIGSERIAL PRIMARY KEY,
    event_block_hash                    BYTEA NOT NULL,
    event_block_number                  BIGINT NOT NULL,
    event_tx_index                      BIGINT NOT NULL,
    event_log_index                     BIGINT NOT NULL,
    eon                                 BIGINT NOT NULL,
    tx_index                            BIGINT NOT NULL,
    identity_prefix                     BYTEA  NOT NULL,
    sender                              BYTEA  NOT NULL,
    encrypted_transaction               BYTEA  NOT NULL,
    created_at                          TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    updated_at                          TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    UNIQUE (event_block_hash, event_block_number, event_tx_index, event_log_index)
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

CREATE TABLE IF NOT EXISTS block
(
    block_hash          BYTEA PRIMARY KEY,
    block_number        BIGINT UNIQUE NOT NULL,
    block_timestamp     BIGINT UNIQUE NOT NULL,
    tx_hash             BYTEA UNIQUE NOT NULL,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE decryption_keys_message;
DROP TABLE decryption_key;
DROP TABLE decryption_keys_message_decryption_key;
DROP TABLE transaction_submitted_event;
DROP TABLE decryption_key_share;
DROP TABLE decryption_key_share;
DROP TABLE block;
-- +goose StatementEnd

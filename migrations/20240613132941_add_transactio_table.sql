-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS transaction
(
    id                  BIGSERIAL PRIMARY KEY,
    encrypted_tx        BYTEA,
    decryption_key      BYTEA,
    slot                BIGINT,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

DO
$$
    BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'transaction_v1') THEN
            CREATE TYPE transaction_v1 AS
            (
                id                  BIGINT,
                encrypted_tx        BYTEA,
                decryption_key      BYTEA,
                slot                BIGINT,
                created_at          TIMESTAMP WITH TIME ZONE,
                updated_at          TIMESTAMP WITH TIME ZONE,
            );
        END IF;
    END
$$;


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE transaction;

DROP TYPE transaction_v1;
-- +goose StatementEnd

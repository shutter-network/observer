-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS proposer_duties
(
    id                          BIGSERIAL PRIMARY KEY,
    public_key                  TEXT NOT NULL,
    validator_index             BIGINT NOT NULL,
    slot                        BIGINT UNIQUE NOT NULL,
    created_at                  TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    updated_at                  TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd

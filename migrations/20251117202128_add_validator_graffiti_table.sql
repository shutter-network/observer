-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS validator_graffiti
(
    validator_index             BIGINT PRIMARY KEY,
    graffiti                    TEXT NOT NULL,
    block_number                BIGINT NOT NULL,
    created_at                  TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    updated_at                  TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE validator_graffiti;
-- +goose StatementEnd

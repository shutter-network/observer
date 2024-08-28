-- +goose Up
-- +goose StatementBegin
DELETE FROM validator_registration_message;
DELETE FROM validator_registry_events_synced_until;

CREATE TABLE IF NOT EXISTS validator_status
(
    id                          SERIAL PRIMARY KEY,
    validator_index             BIGINT UNIQUE,
    status                      TEXT NOT NULL,
    created_at                  TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL,
    updated_at                  TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd

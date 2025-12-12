-- +goose Up
-- +goose StatementBegin
ALTER TABLE validator_graffiti
    DROP CONSTRAINT IF EXISTS validator_graffiti_pkey,
    DROP CONSTRAINT IF EXISTS validator_graffiti_validator_index_key,
    DROP COLUMN IF EXISTS id,
    ADD CONSTRAINT validator_graffiti_pkey PRIMARY KEY (validator_index);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE validator_graffiti
    DROP CONSTRAINT IF EXISTS validator_graffiti_pkey;

ALTER TABLE validator_graffiti
    ADD COLUMN IF NOT EXISTS id BIGSERIAL;

UPDATE validator_graffiti
SET id = nextval(pg_get_serial_sequence('validator_graffiti', 'id'))
WHERE id IS NULL;

ALTER TABLE validator_graffiti
    ADD CONSTRAINT validator_graffiti_pkey PRIMARY KEY (id),
    ADD CONSTRAINT IF NOT EXISTS validator_graffiti_validator_index_key UNIQUE (validator_index);
-- +goose StatementEnd

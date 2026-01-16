-- +goose Up
-- +goose StatementBegin
CREATE TYPE slot_status_val AS ENUM 
(
    'proposed', 
    'missed'
);

CREATE TABLE IF NOT EXISTS slot_status
(
    slot                BIGINT PRIMARY KEY,
    status              slot_status_val NOT NULL,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()  NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE slot_status;
DROP TYPE slot_status_val;
-- +goose StatementEnd

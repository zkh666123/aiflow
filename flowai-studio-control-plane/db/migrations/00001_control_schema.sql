-- +goose Up
CREATE TABLE control.schema_metadata (
    key text PRIMARY KEY,
    value text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO control.schema_metadata (key, value)
VALUES ('schema_version', '1')
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value,
    updated_at = now();

-- +goose Down
DROP TABLE control.schema_metadata;

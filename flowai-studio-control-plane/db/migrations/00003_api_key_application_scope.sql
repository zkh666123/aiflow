-- +goose Up
ALTER TABLE control.api_keys
DROP CONSTRAINT api_keys_application_id_fkey;

ALTER TABLE control.api_keys
ADD CONSTRAINT api_keys_application_id_fkey
FOREIGN KEY (application_id)
REFERENCES control.applications(id)
ON DELETE CASCADE;

-- +goose Down
ALTER TABLE control.api_keys
DROP CONSTRAINT api_keys_application_id_fkey;

ALTER TABLE control.api_keys
ADD CONSTRAINT api_keys_application_id_fkey
FOREIGN KEY (application_id)
REFERENCES control.applications(id)
ON DELETE SET NULL;

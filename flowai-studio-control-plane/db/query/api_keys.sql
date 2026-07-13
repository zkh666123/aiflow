-- name: CreateAPIKey :one
INSERT INTO control.api_keys (
    name,
    key_digest,
    key_prefix,
    scopes,
    expires_at,
    user_id,
    application_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, name, key_digest, key_prefix, scopes, is_active, last_used_at,
          expires_at, user_id, application_id, created_at;

-- name: ListAPIKeys :many
SELECT id, name, key_digest, key_prefix, scopes, is_active, last_used_at,
       expires_at, user_id, application_id, created_at
FROM control.api_keys
WHERE user_id = sqlc.arg(user_id)::uuid
  AND (
      sqlc.narg(application_id)::uuid IS NULL
      OR application_id = sqlc.narg(application_id)::uuid
  )
ORDER BY created_at DESC, id;

-- name: GetAPIKeyByID :one
SELECT id, name, key_digest, key_prefix, scopes, is_active, last_used_at,
       expires_at, user_id, application_id, created_at
FROM control.api_keys
WHERE id = $1;

-- name: GetAPIKeyByDigest :one
SELECT id, name, key_digest, key_prefix, scopes, is_active, last_used_at,
       expires_at, user_id, application_id, created_at
FROM control.api_keys
WHERE key_digest = $1;

-- name: TouchAPIKey :exec
UPDATE control.api_keys
SET last_used_at = now()
WHERE id = $1;

-- name: SetAPIKeyActive :one
UPDATE control.api_keys
SET is_active = $3
WHERE id = $1 AND user_id = $2
RETURNING id, name, key_digest, key_prefix, scopes, is_active, last_used_at,
          expires_at, user_id, application_id, created_at;

-- name: DeleteAPIKey :one
DELETE FROM control.api_keys
WHERE id = $1 AND user_id = $2
RETURNING id;

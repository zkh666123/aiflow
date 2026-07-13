-- name: CreateAppShare :one
INSERT INTO control.app_shares (application_id, share_link, is_public)
VALUES ($1, $2, $3)
RETURNING id, application_id, share_link, is_public, access_count, embed_config,
          created_at, updated_at;

-- name: GetAppShareByApplicationID :one
SELECT id, application_id, share_link, is_public, access_count, embed_config,
       created_at, updated_at
FROM control.app_shares
WHERE application_id = $1;

-- name: GetPublicAppShareByLink :one
SELECT s.id,
       s.application_id,
       s.share_link,
       s.is_public,
       s.access_count,
       s.embed_config,
       s.created_at,
       s.updated_at,
       a.name,
       a.description,
       a.icon,
       a.status
FROM control.app_shares AS s
JOIN control.applications AS a ON a.id = s.application_id
WHERE s.share_link = $1 AND s.is_public = true;

-- name: IncrementAppShareAccess :exec
UPDATE control.app_shares
SET access_count = access_count + 1
WHERE id = $1;

-- name: UpdateAppShare :one
UPDATE control.app_shares
SET is_public = CASE
        WHEN sqlc.arg(set_is_public)::boolean THEN sqlc.arg(is_public)::boolean
        ELSE is_public
    END,
    embed_config = CASE
        WHEN sqlc.arg(set_embed_config)::boolean THEN sqlc.narg(embed_config)::jsonb
        ELSE embed_config
    END
WHERE application_id = sqlc.arg(application_id)::uuid
RETURNING id, application_id, share_link, is_public, access_count, embed_config,
          created_at, updated_at;

-- name: DeleteAppShare :one
DELETE FROM control.app_shares
WHERE application_id = $1
RETURNING id;

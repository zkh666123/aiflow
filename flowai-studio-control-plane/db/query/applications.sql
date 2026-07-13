-- name: CreateApplication :one
INSERT INTO control.applications (name, description, icon, status, owner_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, name, description, icon, status, share_link, owner_id, created_at, updated_at;

-- name: ListApplicationsForUser :many
WITH application_access AS (
    SELECT a.id AS application_id, 'owner'::text AS access_type, 4 AS access_rank
    FROM control.applications AS a
    WHERE a.owner_id = sqlc.arg(user_id)::uuid

    UNION ALL

    SELECT ta.application_id,
           ta.permission AS access_type,
           CASE ta.permission
               WHEN 'full_access' THEN 3
               WHEN 'can_edit' THEN 2
               ELSE 1
           END AS access_rank
    FROM control.team_applications AS ta
    JOIN control.team_members AS tm ON tm.team_id = ta.team_id
    WHERE tm.user_id = sqlc.arg(user_id)::uuid
),
best_access AS (
    SELECT application_id,
           (array_agg(access_type ORDER BY access_rank DESC))[1] AS access_type
    FROM application_access
    GROUP BY application_id
)
SELECT a.id,
       a.name,
       a.description,
       a.icon,
       a.status,
       a.share_link,
       a.owner_id,
       a.created_at,
       a.updated_at,
       best_access.access_type
FROM control.applications AS a
JOIN best_access ON best_access.application_id = a.id
ORDER BY a.updated_at DESC, a.id;

-- name: GetApplicationByID :one
SELECT id, name, description, icon, status, share_link, owner_id, created_at, updated_at
FROM control.applications
WHERE id = $1;

-- name: UpdateApplication :one
UPDATE control.applications
SET name = CASE
        WHEN sqlc.arg(set_name)::boolean THEN sqlc.arg(name)::text
        ELSE name
    END,
    description = CASE
        WHEN sqlc.arg(set_description)::boolean THEN sqlc.narg(description)::text
        ELSE description
    END,
    icon = CASE
        WHEN sqlc.arg(set_icon)::boolean THEN sqlc.narg(icon)::text
        ELSE icon
    END,
    status = CASE
        WHEN sqlc.arg(set_status)::boolean THEN sqlc.arg(status)::text
        ELSE status
    END
WHERE id = sqlc.arg(id)::uuid
RETURNING id, name, description, icon, status, share_link, owner_id, created_at, updated_at;

-- name: DeleteApplication :one
DELETE FROM control.applications
WHERE id = $1
RETURNING id;

-- name: SetApplicationStatus :one
UPDATE control.applications
SET status = $2
WHERE id = $1
RETURNING id, name, description, icon, status, share_link, owner_id, created_at, updated_at;

-- name: SetApplicationShareLink :exec
UPDATE control.applications
SET share_link = $2
WHERE id = $1;

-- name: GetApplicationOwnerID :one
SELECT owner_id
FROM control.applications
WHERE id = $1;

-- name: ListApplicationAccessForUser :many
SELECT a.owner_id,
       u.global_role,
       tm.role AS team_role,
       ta.permission AS team_application_permission
FROM control.applications AS a
CROSS JOIN control.users AS u
LEFT JOIN control.team_applications AS ta
    ON ta.application_id = a.id
LEFT JOIN control.team_members AS tm
    ON tm.team_id = ta.team_id
   AND tm.user_id = u.id
WHERE a.id = sqlc.arg(application_id)::uuid
  AND u.id = sqlc.arg(user_id)::uuid;

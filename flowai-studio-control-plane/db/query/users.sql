-- name: CreateUser :one
INSERT INTO control.users (username, password_hash)
VALUES ($1, $2)
RETURNING id, username, password_hash, avatar, global_role, created_at, updated_at;

-- name: GetUserByUsername :one
SELECT id, username, password_hash, avatar, global_role, created_at, updated_at
FROM control.users
WHERE username = $1;

-- name: GetUserByID :one
SELECT id, username, password_hash, avatar, global_role, created_at, updated_at
FROM control.users
WHERE id = $1;

-- name: UpdateUserProfile :one
UPDATE control.users
SET username = CASE
        WHEN sqlc.arg(set_username)::boolean THEN sqlc.arg(username)::text
        ELSE username
    END,
    avatar = CASE
        WHEN sqlc.arg(set_avatar)::boolean THEN sqlc.narg(avatar)::text
        ELSE avatar
    END
WHERE id = sqlc.arg(id)::uuid
RETURNING id, username, password_hash, avatar, global_role, created_at, updated_at;

-- name: GetUserGlobalRole :one
SELECT global_role
FROM control.users
WHERE id = $1;

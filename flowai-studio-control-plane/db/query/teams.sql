-- name: CreateTeam :one
INSERT INTO control.teams (name, description, avatar, owner_id)
VALUES ($1, $2, $3, $4)
RETURNING id, name, description, avatar, owner_id, created_at, updated_at;

-- name: ListTeamsForUser :many
SELECT t.id,
       t.name,
       t.description,
       t.avatar,
       t.owner_id,
       t.created_at,
       t.updated_at,
       membership.role AS my_role,
       (SELECT count(*) FROM control.team_members AS members WHERE members.team_id = t.id) AS member_count,
       (SELECT count(*) FROM control.team_applications AS apps WHERE apps.team_id = t.id) AS app_count
FROM control.teams AS t
JOIN control.team_members AS membership
  ON membership.team_id = t.id
 AND membership.user_id = $1
ORDER BY t.updated_at DESC, t.id;

-- name: GetTeamByID :one
SELECT id, name, description, avatar, owner_id, created_at, updated_at
FROM control.teams
WHERE id = $1;

-- name: UpdateTeam :one
UPDATE control.teams
SET name = CASE
        WHEN sqlc.arg(set_name)::boolean THEN sqlc.arg(name)::text
        ELSE name
    END,
    description = CASE
        WHEN sqlc.arg(set_description)::boolean THEN sqlc.narg(description)::text
        ELSE description
    END,
    avatar = CASE
        WHEN sqlc.arg(set_avatar)::boolean THEN sqlc.narg(avatar)::text
        ELSE avatar
    END
WHERE id = sqlc.arg(id)::uuid
RETURNING id, name, description, avatar, owner_id, created_at, updated_at;

-- name: DeleteTeam :one
DELETE FROM control.teams
WHERE id = $1
RETURNING id;

-- name: CreateTeamMember :one
INSERT INTO control.team_members (team_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING id, team_id, user_id, role, joined_at;

-- name: GetTeamMembership :one
SELECT id, team_id, user_id, role, joined_at
FROM control.team_members
WHERE team_id = $1 AND user_id = $2;

-- name: GetTeamMemberByID :one
SELECT id, team_id, user_id, role, joined_at
FROM control.team_members
WHERE id = $1;

-- name: ListTeamMembers :many
SELECT tm.id,
       tm.team_id,
       tm.user_id,
       tm.role,
       tm.joined_at,
       u.username,
       u.avatar,
       u.created_at AS user_created_at
FROM control.team_members AS tm
JOIN control.users AS u ON u.id = tm.user_id
WHERE tm.team_id = $1
ORDER BY tm.joined_at, tm.id;

-- name: UpdateTeamMemberRole :one
UPDATE control.team_members
SET role = $3
WHERE team_id = $1 AND id = $2
RETURNING id, team_id, user_id, role, joined_at;

-- name: DeleteTeamMember :one
DELETE FROM control.team_members
WHERE team_id = $1 AND id = $2
RETURNING id;

-- name: DeleteTeamMembershipByUser :one
DELETE FROM control.team_members
WHERE team_id = $1 AND user_id = $2
RETURNING id;

-- name: CreateTeamApplication :one
INSERT INTO control.team_applications (team_id, application_id, permission)
VALUES ($1, $2, $3)
RETURNING id, team_id, application_id, permission, added_at;

-- name: GetTeamApplicationByID :one
SELECT id, team_id, application_id, permission, added_at
FROM control.team_applications
WHERE id = $1;

-- name: ListTeamApplications :many
SELECT ta.id,
       ta.team_id,
       ta.application_id,
       ta.permission,
       ta.added_at,
       a.name,
       a.description,
       a.icon,
       a.status
FROM control.team_applications AS ta
JOIN control.applications AS a ON a.id = ta.application_id
WHERE ta.team_id = $1
ORDER BY ta.added_at, ta.id;

-- name: UpdateTeamApplicationPermission :one
UPDATE control.team_applications
SET permission = $3
WHERE team_id = $1 AND id = $2
RETURNING id, team_id, application_id, permission, added_at;

-- name: DeleteTeamApplication :one
DELETE FROM control.team_applications
WHERE team_id = $1 AND id = $2
RETURNING id;

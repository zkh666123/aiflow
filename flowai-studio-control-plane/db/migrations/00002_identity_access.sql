-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION control.set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TABLE control.users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username varchar(20) NOT NULL UNIQUE,
    password_hash text NOT NULL,
    avatar text,
    global_role text NOT NULL DEFAULT 'member'
        CHECK (global_role IN ('admin', 'member')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (username ~ '^[A-Za-z0-9_]{3,20}$')
);

CREATE TABLE control.applications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(100) NOT NULL CHECK (length(btrim(name)) > 0),
    description varchar(500),
    icon text,
    status text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'published', 'archived')),
    share_link varchar(64) UNIQUE,
    owner_id uuid NOT NULL REFERENCES control.users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE control.teams (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(50) NOT NULL CHECK (length(btrim(name)) > 0),
    description varchar(200),
    avatar text,
    owner_id uuid NOT NULL REFERENCES control.users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE control.team_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id uuid NOT NULL REFERENCES control.teams(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES control.users(id) ON DELETE CASCADE,
    role text NOT NULL DEFAULT 'viewer'
        CHECK (role IN ('owner', 'admin', 'editor', 'viewer')),
    joined_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (team_id, user_id)
);

CREATE TABLE control.team_applications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id uuid NOT NULL REFERENCES control.teams(id) ON DELETE CASCADE,
    application_id uuid NOT NULL REFERENCES control.applications(id) ON DELETE CASCADE,
    permission text NOT NULL DEFAULT 'can_view'
        CHECK (permission IN ('full_access', 'can_edit', 'can_view')),
    added_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (team_id, application_id)
);

CREATE TABLE control.api_keys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(100) NOT NULL CHECK (length(btrim(name)) > 0),
    key_digest bytea NOT NULL UNIQUE,
    key_prefix varchar(7) NOT NULL,
    scopes jsonb NOT NULL DEFAULT '["app:read", "workflow:execute"]'::jsonb
        CHECK (jsonb_typeof(scopes) = 'array'),
    is_active boolean NOT NULL DEFAULT true,
    last_used_at timestamptz,
    expires_at timestamptz,
    user_id uuid NOT NULL REFERENCES control.users(id) ON DELETE CASCADE,
    application_id uuid REFERENCES control.applications(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE control.app_shares (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id uuid NOT NULL UNIQUE REFERENCES control.applications(id) ON DELETE CASCADE,
    share_link varchar(64) NOT NULL UNIQUE,
    is_public boolean NOT NULL DEFAULT false,
    access_count integer NOT NULL DEFAULT 0 CHECK (access_count >= 0),
    embed_config jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX applications_owner_updated_idx
    ON control.applications (owner_id, updated_at DESC);
CREATE INDEX team_members_user_idx
    ON control.team_members (user_id, team_id);
CREATE INDEX team_applications_application_idx
    ON control.team_applications (application_id, team_id);
CREATE INDEX api_keys_user_created_idx
    ON control.api_keys (user_id, created_at DESC);
CREATE INDEX api_keys_prefix_idx
    ON control.api_keys (key_prefix);

CREATE TRIGGER users_set_updated_at
BEFORE UPDATE ON control.users
FOR EACH ROW EXECUTE FUNCTION control.set_updated_at();

CREATE TRIGGER applications_set_updated_at
BEFORE UPDATE ON control.applications
FOR EACH ROW EXECUTE FUNCTION control.set_updated_at();

CREATE TRIGGER teams_set_updated_at
BEFORE UPDATE ON control.teams
FOR EACH ROW EXECUTE FUNCTION control.set_updated_at();

CREATE TRIGGER app_shares_set_updated_at
BEFORE UPDATE ON control.app_shares
FOR EACH ROW EXECUTE FUNCTION control.set_updated_at();

-- +goose Down
DROP TABLE control.app_shares;
DROP TABLE control.api_keys;
DROP TABLE control.team_applications;
DROP TABLE control.team_members;
DROP TABLE control.teams;
DROP TABLE control.applications;
DROP TABLE control.users;
DROP FUNCTION control.set_updated_at();

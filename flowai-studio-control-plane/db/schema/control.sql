-- sqlc needs the externally bootstrapped schema declared before it parses migrations.
CREATE SCHEMA control;

CREATE TABLE control.schema_metadata (
    key text PRIMARY KEY,
    value text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

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
    application_id uuid REFERENCES control.applications(id) ON DELETE CASCADE,
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

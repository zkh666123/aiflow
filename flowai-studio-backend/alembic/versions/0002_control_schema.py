"""Create the control schema owned by the Python backend."""

from collections.abc import Sequence

from alembic import op

revision: str = "0002_control_schema"
down_revision: str | None = "0001_ai_schema"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.execute("CREATE SCHEMA IF NOT EXISTS control")
    op.execute(
        """
        CREATE OR REPLACE FUNCTION control.set_updated_at()
        RETURNS trigger LANGUAGE plpgsql AS $$
        BEGIN NEW.updated_at = now(); RETURN NEW; END;
        $$
        """
    )
    op.execute(
        """
        CREATE TABLE IF NOT EXISTS control.users (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
            username varchar(20) NOT NULL UNIQUE,
            password_hash text NOT NULL,
            avatar text,
            global_role text NOT NULL DEFAULT 'member' CHECK (global_role IN ('admin', 'member')),
            created_at timestamptz NOT NULL DEFAULT now(),
            updated_at timestamptz NOT NULL DEFAULT now(),
            CHECK (username ~ '^[A-Za-z0-9_]{3,20}$')
        );
        CREATE TABLE IF NOT EXISTS control.applications (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
            name varchar(100) NOT NULL CHECK (length(btrim(name)) > 0),
            description varchar(500), icon text,
            status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
            share_link varchar(64) UNIQUE,
            owner_id uuid NOT NULL REFERENCES control.users(id) ON DELETE CASCADE,
            created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
        );
        CREATE TABLE IF NOT EXISTS control.teams (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
            name varchar(50) NOT NULL CHECK (length(btrim(name)) > 0),
            description varchar(200), avatar text,
            owner_id uuid NOT NULL REFERENCES control.users(id) ON DELETE CASCADE,
            created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
        );
        CREATE TABLE IF NOT EXISTS control.team_members (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
            team_id uuid NOT NULL REFERENCES control.teams(id) ON DELETE CASCADE,
            user_id uuid NOT NULL REFERENCES control.users(id) ON DELETE CASCADE,
            role text NOT NULL DEFAULT 'viewer' CHECK (role IN ('owner', 'admin', 'editor', 'viewer')),
            joined_at timestamptz NOT NULL DEFAULT now(), UNIQUE (team_id, user_id)
        );
        CREATE TABLE IF NOT EXISTS control.team_applications (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
            team_id uuid NOT NULL REFERENCES control.teams(id) ON DELETE CASCADE,
            application_id uuid NOT NULL REFERENCES control.applications(id) ON DELETE CASCADE,
            permission text NOT NULL DEFAULT 'can_view' CHECK (permission IN ('full_access', 'can_edit', 'can_view')),
            added_at timestamptz NOT NULL DEFAULT now(), UNIQUE (team_id, application_id)
        );
        CREATE TABLE IF NOT EXISTS control.api_keys (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
            name varchar(100) NOT NULL CHECK (length(btrim(name)) > 0),
            key_digest bytea NOT NULL UNIQUE, key_prefix varchar(7) NOT NULL,
            scopes jsonb NOT NULL DEFAULT '["app:read", "workflow:execute"]'::jsonb CHECK (jsonb_typeof(scopes) = 'array'),
            is_active boolean NOT NULL DEFAULT true, last_used_at timestamptz, expires_at timestamptz,
            user_id uuid NOT NULL REFERENCES control.users(id) ON DELETE CASCADE,
            application_id uuid REFERENCES control.applications(id) ON DELETE SET NULL,
            created_at timestamptz NOT NULL DEFAULT now()
        );
        CREATE TABLE IF NOT EXISTS control.app_shares (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
            application_id uuid NOT NULL UNIQUE REFERENCES control.applications(id) ON DELETE CASCADE,
            share_link varchar(64) NOT NULL UNIQUE, is_public boolean NOT NULL DEFAULT false,
            access_count integer NOT NULL DEFAULT 0 CHECK (access_count >= 0), embed_config jsonb,
            created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
        )
        """
    )
    op.execute(
        """
        CREATE INDEX IF NOT EXISTS applications_owner_updated_idx ON control.applications (owner_id, updated_at DESC);
        CREATE INDEX IF NOT EXISTS team_members_user_idx ON control.team_members (user_id, team_id);
        CREATE INDEX IF NOT EXISTS team_applications_application_idx ON control.team_applications (application_id, team_id);
        CREATE INDEX IF NOT EXISTS api_keys_user_created_idx ON control.api_keys (user_id, created_at DESC);
        CREATE INDEX IF NOT EXISTS api_keys_prefix_idx ON control.api_keys (key_prefix)
        """
    )
    for table in ("users", "applications", "teams", "app_shares"):
        op.execute(f"DROP TRIGGER IF EXISTS {table}_set_updated_at ON control.{table}")
        op.execute(
            f"CREATE TRIGGER {table}_set_updated_at BEFORE UPDATE ON control.{table} "
            "FOR EACH ROW EXECUTE FUNCTION control.set_updated_at()"
        )


def downgrade() -> None:
    for table in (
        "app_shares", "api_keys", "team_applications", "team_members", "teams", "applications", "users"
    ):
        op.execute(f"DROP TABLE IF EXISTS control.{table} CASCADE")
    op.execute("DROP FUNCTION IF EXISTS control.set_updated_at()")

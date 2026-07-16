"""Create workflow, version, template, execution, and trace tables."""

from collections.abc import Sequence

from alembic import op

revision: str = "0003_workflow_schema"
down_revision: str | None = "0002_control_schema"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.execute(
        """
        CREATE TABLE control.workflows (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name varchar(100) NOT NULL,
            description varchar(500), application_id uuid NOT NULL REFERENCES control.applications(id) ON DELETE CASCADE,
            owner_id uuid NOT NULL REFERENCES control.users(id) ON DELETE CASCADE,
            nodes jsonb NOT NULL DEFAULT '[]'::jsonb, edges jsonb NOT NULL DEFAULT '[]'::jsonb,
            variables jsonb NOT NULL DEFAULT '{}'::jsonb, settings jsonb NOT NULL DEFAULT '{}'::jsonb,
            current_version integer NOT NULL DEFAULT 1,
            created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
        );
        CREATE TABLE control.workflow_versions (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workflow_id uuid NOT NULL REFERENCES control.workflows(id) ON DELETE CASCADE,
            version integer NOT NULL, label varchar(100), description varchar(1000), nodes jsonb NOT NULL,
            edges jsonb NOT NULL, variables jsonb NOT NULL DEFAULT '{}'::jsonb, created_by uuid REFERENCES control.users(id) ON DELETE SET NULL,
            is_published boolean NOT NULL DEFAULT false, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(workflow_id,version)
        );
        CREATE TABLE control.workflow_templates (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name varchar(100) NOT NULL, description varchar(500), icon text, screenshot text,
            category text NOT NULL, tags jsonb NOT NULL DEFAULT '[]'::jsonb, nodes jsonb NOT NULL DEFAULT '[]'::jsonb,
            edges jsonb NOT NULL DEFAULT '[]'::jsonb, variables jsonb NOT NULL DEFAULT '{}'::jsonb,
            download_count integer NOT NULL DEFAULT 0, rating double precision NOT NULL DEFAULT 0, rating_count integer NOT NULL DEFAULT 0,
            status text NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','published','archived')), is_official boolean NOT NULL DEFAULT false,
            user_id uuid REFERENCES control.users(id) ON DELETE SET NULL,
            created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
        );
        CREATE TABLE control.template_ratings (
            template_id uuid NOT NULL REFERENCES control.workflow_templates(id) ON DELETE CASCADE,
            user_id uuid NOT NULL REFERENCES control.users(id) ON DELETE CASCADE, rating smallint NOT NULL CHECK(rating BETWEEN 1 AND 5),
            created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(template_id,user_id)
        );
        CREATE TABLE control.workflow_executions (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workflow_id uuid NOT NULL REFERENCES control.workflows(id) ON DELETE CASCADE,
            user_id uuid REFERENCES control.users(id) ON DELETE SET NULL, status text NOT NULL DEFAULT 'pending', inputs jsonb,
            context jsonb, logs jsonb, error text, duration_ms integer, started_at timestamptz NOT NULL DEFAULT now(),
            completed_at timestamptz, created_at timestamptz NOT NULL DEFAULT now()
        );
        CREATE TABLE control.traces (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workflow_id uuid NOT NULL REFERENCES control.workflows(id) ON DELETE CASCADE,
            execution_id uuid REFERENCES control.workflow_executions(id) ON DELETE SET NULL, user_id uuid REFERENCES control.users(id) ON DELETE SET NULL,
            status text NOT NULL DEFAULT 'running', duration_ms integer, input jsonb, output jsonb, error text,
            started_at timestamptz NOT NULL DEFAULT now(), completed_at timestamptz, created_at timestamptz NOT NULL DEFAULT now()
        );
        CREATE TABLE control.spans (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(), trace_id uuid NOT NULL REFERENCES control.traces(id) ON DELETE CASCADE,
            parent_span_id uuid REFERENCES control.spans(id) ON DELETE SET NULL, node_id text, name text NOT NULL, kind text NOT NULL,
            status text NOT NULL DEFAULT 'running', input jsonb, output jsonb, error text, metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
            started_at timestamptz NOT NULL DEFAULT now(), completed_at timestamptz, duration_ms integer
        );
        CREATE INDEX workflows_app_updated_idx ON control.workflows(application_id,updated_at DESC);
        CREATE INDEX workflow_versions_workflow_idx ON control.workflow_versions(workflow_id,version DESC);
        CREATE INDEX workflow_templates_search_idx ON control.workflow_templates(status,category,created_at DESC);
        CREATE INDEX workflow_executions_workflow_idx ON control.workflow_executions(workflow_id,created_at DESC);
        CREATE INDEX traces_workflow_idx ON control.traces(workflow_id,started_at DESC);
        CREATE INDEX spans_trace_idx ON control.spans(trace_id,started_at)
        """
    )
    for table in ("workflows", "workflow_templates", "template_ratings"):
        op.execute(f"CREATE TRIGGER {table}_set_updated_at BEFORE UPDATE ON control.{table} FOR EACH ROW EXECUTE FUNCTION control.set_updated_at()")


def downgrade() -> None:
    for table in ("spans", "traces", "workflow_executions", "template_ratings", "workflow_templates", "workflow_versions", "workflows"):
        op.execute(f"DROP TABLE IF EXISTS control.{table} CASCADE")

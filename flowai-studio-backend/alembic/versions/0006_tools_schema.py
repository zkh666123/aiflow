"""Create MCP and custom skill persistence."""
from collections.abc import Sequence
from alembic import op
revision:str="0006_tools_schema";down_revision:str|None="0005_rag_schema";branch_labels:str|Sequence[str]|None=None;depends_on:str|Sequence[str]|None=None
def upgrade()->None:
    op.execute("""CREATE TABLE ai.mcp_servers(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),user_id uuid NOT NULL,name text NOT NULL,description text,
      transport_type text NOT NULL DEFAULT 'http',command text,args jsonb NOT NULL DEFAULT '[]'::jsonb,environment jsonb NOT NULL DEFAULT '{}'::jsonb,url text,is_active boolean NOT NULL DEFAULT true,
      state text NOT NULL DEFAULT 'disconnected',tools jsonb NOT NULL DEFAULT '[]'::jsonb,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now());
    CREATE TABLE ai.skills(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),user_id uuid NOT NULL,name text NOT NULL,description text,skill_type text NOT NULL,
      code text,configuration jsonb NOT NULL DEFAULT '{}'::jsonb,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now());
    CREATE INDEX mcp_servers_user_idx ON ai.mcp_servers(user_id,updated_at DESC);CREATE INDEX skills_user_idx ON ai.skills(user_id,updated_at DESC)""")
def downgrade()->None:
    op.execute("DROP TABLE IF EXISTS ai.skills");op.execute("DROP TABLE IF EXISTS ai.mcp_servers")

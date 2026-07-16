"""Create chat and token usage persistence."""

from collections.abc import Sequence
from alembic import op

revision: str="0004_ai_runtime_schema"; down_revision: str|None="0003_workflow_schema"
branch_labels: str|Sequence[str]|None=None; depends_on: str|Sequence[str]|None=None


def upgrade()->None:
    op.execute("""
    CREATE TABLE ai.chat_sessions(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),user_id uuid NOT NULL,model text NOT NULL,title text,
        created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now());
    CREATE TABLE ai.chat_messages(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),session_id uuid NOT NULL REFERENCES ai.chat_sessions(id) ON DELETE CASCADE,
        role text NOT NULL,content text NOT NULL,metadata jsonb NOT NULL DEFAULT '{}'::jsonb,created_at timestamptz NOT NULL DEFAULT now());
    CREATE TABLE ai.token_usage(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),user_id uuid,workflow_id uuid,execution_id uuid,node_id text,
        provider text NOT NULL,model text NOT NULL,prompt_tokens integer NOT NULL DEFAULT 0,completion_tokens integer NOT NULL DEFAULT 0,
        total_tokens integer NOT NULL DEFAULT 0,cost numeric(18,8) NOT NULL DEFAULT 0,created_at timestamptz NOT NULL DEFAULT now());
    CREATE INDEX chat_sessions_user_idx ON ai.chat_sessions(user_id,updated_at DESC);
    CREATE INDEX token_usage_user_idx ON ai.token_usage(user_id,created_at DESC);
    CREATE INDEX token_usage_model_idx ON ai.token_usage(model,created_at DESC)
    """)


def downgrade()->None:
    for table in ("token_usage","chat_messages","chat_sessions"): op.execute(f"DROP TABLE IF EXISTS ai.{table} CASCADE")

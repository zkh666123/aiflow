"""Create knowledge base, document, chunk, vector, and FTS storage."""
from collections.abc import Sequence
from alembic import op
revision:str="0005_rag_schema";down_revision:str|None="0004_ai_runtime_schema";branch_labels:str|Sequence[str]|None=None;depends_on:str|Sequence[str]|None=None
def upgrade()->None:
    op.execute("""
    CREATE TABLE ai.knowledge_bases(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),user_id uuid NOT NULL,name text NOT NULL,description text,
      embedding_provider text NOT NULL DEFAULT 'ollama',embedding_model text NOT NULL DEFAULT 'nomic-embed-text',chunk_size integer NOT NULL DEFAULT 500,
      chunk_overlap integer NOT NULL DEFAULT 50,top_k integer NOT NULL DEFAULT 5,similarity_threshold double precision NOT NULL DEFAULT .7,
      retrieval_mode text NOT NULL DEFAULT 'hybrid',vector_weight double precision NOT NULL DEFAULT .7,rrf_k integer NOT NULL DEFAULT 60,
      reranker_enabled boolean NOT NULL DEFAULT false,reranker_provider text NOT NULL DEFAULT 'none',reranker_model text NOT NULL DEFAULT '',reranker_top_n integer,
      created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),UNIQUE(user_id,name));
    CREATE TABLE ai.documents(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),knowledge_base_id uuid NOT NULL REFERENCES ai.knowledge_bases(id) ON DELETE CASCADE,
      name text NOT NULL,mime_type text NOT NULL,size integer NOT NULL,status text NOT NULL DEFAULT 'processing',error text,metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
      created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now());
    CREATE TABLE ai.document_chunks(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),document_id uuid NOT NULL REFERENCES ai.documents(id) ON DELETE CASCADE,
      knowledge_base_id uuid NOT NULL REFERENCES ai.knowledge_bases(id) ON DELETE CASCADE,chunk_index integer NOT NULL,content text NOT NULL,metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
      embedding vector,search_vector tsvector GENERATED ALWAYS AS (to_tsvector('simple',content)) STORED,created_at timestamptz NOT NULL DEFAULT now(),UNIQUE(document_id,chunk_index));
    CREATE INDEX knowledge_bases_user_idx ON ai.knowledge_bases(user_id,updated_at DESC);CREATE INDEX documents_kb_idx ON ai.documents(knowledge_base_id,created_at DESC);
    CREATE INDEX chunks_kb_idx ON ai.document_chunks(knowledge_base_id,chunk_index);CREATE INDEX chunks_fts_idx ON ai.document_chunks USING gin(search_vector)
    """)
def downgrade()->None:
    for table in ("document_chunks","documents","knowledge_bases"):op.execute(f"DROP TABLE IF EXISTS ai.{table} CASCADE")

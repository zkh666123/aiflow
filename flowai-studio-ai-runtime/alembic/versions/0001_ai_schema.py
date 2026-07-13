"""Create the AI schema metadata table."""

from typing import Sequence

from alembic import op
import sqlalchemy as sa

revision: str = "0001_ai_schema"
down_revision: str | None = None
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.create_table(
        "schema_metadata",
        sa.Column("key", sa.Text(), nullable=False),
        sa.Column("value", sa.Text(), nullable=False),
        sa.Column(
            "updated_at",
            sa.DateTime(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.PrimaryKeyConstraint("key"),
        schema="ai",
    )
    op.execute(
        "INSERT INTO ai.schema_metadata (key, value) VALUES ('schema_version', '1')"
    )


def downgrade() -> None:
    op.drop_table("schema_metadata", schema="ai")

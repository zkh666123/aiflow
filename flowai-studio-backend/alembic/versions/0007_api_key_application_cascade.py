"""Revoke application-scoped API keys when their application is deleted."""

from collections.abc import Sequence

from alembic import op

revision: str = "0007_api_key_application_cascade"
down_revision: str | None = "0006_tools_schema"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.execute(
        """
        ALTER TABLE control.api_keys
        DROP CONSTRAINT IF EXISTS api_keys_application_id_fkey;
        ALTER TABLE control.api_keys
        ADD CONSTRAINT api_keys_application_id_fkey
        FOREIGN KEY (application_id) REFERENCES control.applications(id) ON DELETE CASCADE
        """
    )


def downgrade() -> None:
    op.execute(
        """
        ALTER TABLE control.api_keys
        DROP CONSTRAINT IF EXISTS api_keys_application_id_fkey;
        ALTER TABLE control.api_keys
        ADD CONSTRAINT api_keys_application_id_fkey
        FOREIGN KEY (application_id) REFERENCES control.applications(id) ON DELETE SET NULL
        """
    )

from __future__ import annotations

from typing import Any

from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.errors import APIError
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.identity.rbac import authorize_application
from aiflow_runtime.identity.service import require_uuid


def workflow_data(row: Any) -> dict[str, Any]:
    return {
        "id": row["id"], "name": row["name"], "description": row["description"],
        "applicationId": row["application_id"], "nodes": row["nodes"], "edges": row["edges"],
        "variables": row["variables"], "currentVersion": row["current_version"],
        "createdAt": row["created_at"], "updatedAt": row["updated_at"],
    }


async def load_workflow(session: AsyncSession, principal: Principal, workflow_id: str, permission: str = "workflow:read") -> Any:
    value = require_uuid(workflow_id, "workflowId")
    result = await session.execute(
        text("""SELECT id::text,name,description,application_id::text,owner_id::text,nodes,edges,variables,current_version,created_at,updated_at
                FROM control.workflows WHERE id=CAST(:id AS uuid)"""), {"id": value}
    )
    row = result.mappings().first()
    if row is None:
        raise APIError(404, "NOT_FOUND", "Workflow not found")
    await authorize_application(session, principal, row["application_id"], permission)
    return row

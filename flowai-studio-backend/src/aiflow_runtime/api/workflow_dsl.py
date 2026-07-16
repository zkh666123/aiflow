from __future__ import annotations

import json
from typing import Annotated, Any, Literal

import yaml
from fastapi import APIRouter, Depends, Query
from fastapi.responses import Response
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.envelope import success
from aiflow_runtime.api.errors import APIError
from aiflow_runtime.identity.auth import current_principal
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.identity.rbac import authorize_application
from aiflow_runtime.identity.service import require_uuid, validate_name
from aiflow_runtime.infrastructure.database import get_session
from aiflow_runtime.workflow.dsl import export_document, parse_dsl, validate_dsl
from aiflow_runtime.workflow.schemas import ImportDSLRequest
from aiflow_runtime.workflow.service import load_workflow, workflow_data

router = APIRouter(prefix="/api/workflow-dsl", tags=["workflow-dsl"])
Session = Annotated[AsyncSession, Depends(get_session)]
CurrentUser = Annotated[Principal, Depends(current_principal)]


@router.get("/{workflow_id}/export")
async def export_dsl(workflow_id: str, principal: CurrentUser, session: Session, format_name: Literal["yaml", "json"] = Query(default="yaml", alias="format")) -> Response:
    row = await load_workflow(session, principal, workflow_id)
    document = export_document(row)
    if format_name == "json":
        content, media_type, extension = json.dumps(document, ensure_ascii=False, indent=2), "application/json", "json"
    else:
        content, media_type, extension = yaml.safe_dump(document, allow_unicode=True, sort_keys=False), "text/yaml", "yaml"
    return Response(content, media_type=media_type, headers={"Content-Disposition": f'attachment; filename="workflow-{workflow_id[:8]}.{extension}"'})


@router.post("/import", status_code=201)
async def import_dsl(body: ImportDSLRequest, principal: CurrentUser, session: Session) -> Any:
    if body.application_id is None:
        raise APIError(400, "BAD_REQUEST", "Application ID is required")
    app_id = require_uuid(body.application_id, "applicationId")
    await authorize_application(session, principal, app_id, "workflow:create")
    document = parse_dsl(body.dsl, body.format)
    snapshot = validate_dsl(document)
    metadata = document.get("metadata") or {}
    name = validate_name(body.name_override or metadata.get("name") or "Imported Workflow", 100)
    result = await session.execute(
        text("""INSERT INTO control.workflows(name,description,application_id,owner_id,nodes,edges,variables)
                VALUES(:name,:description,CAST(:app AS uuid),CAST(:owner AS uuid),CAST(:nodes AS jsonb),CAST(:edges AS jsonb),CAST(:variables AS jsonb))
                RETURNING id::text,name,description,application_id::text,nodes,edges,variables,current_version,created_at,updated_at"""),
        {"name": name, "description": metadata.get("description"), "app": app_id, "owner": principal.user_id,
         "nodes": json.dumps(snapshot["nodes"]), "edges": json.dumps(snapshot["edges"]), "variables": json.dumps(snapshot["variables"])},
    )
    await session.commit()
    return success(workflow_data(result.mappings().one()), status_code=201)


@router.post("/validate")
async def validate_document(body: ImportDSLRequest, _principal: CurrentUser) -> Any:
    snapshot = validate_dsl(parse_dsl(body.dsl, body.format))
    return success({"valid": True, "version": "1.0", "nodeCount": len(snapshot["nodes"]), "edgeCount": len(snapshot["edges"]), "errors": []})

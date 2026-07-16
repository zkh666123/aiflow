from __future__ import annotations

import json
from typing import Annotated, Any

from fastapi import APIRouter, Depends
from fastapi import Request
from fastapi.responses import StreamingResponse
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.envelope import success
from aiflow_runtime.api.errors import APIError
from aiflow_runtime.identity.auth import current_principal
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.identity.rbac import authorize_application
from aiflow_runtime.identity.service import require_uuid, validate_name
from aiflow_runtime.infrastructure.database import get_session
from aiflow_runtime.workflow.events import sse
from aiflow_runtime.workflow.executors import UnavailableNodeServices
from aiflow_runtime.workflow.runtime import WorkflowRuntime
from aiflow_runtime.workflow.schemas import CreateWorkflowRequest, RunWorkflowRequest, UpdateWorkflowRequest
from aiflow_runtime.workflow.service import load_workflow, workflow_data

router = APIRouter(prefix="/api/workflows", tags=["workflows"])
Session = Annotated[AsyncSession, Depends(get_session)]
CurrentUser = Annotated[Principal, Depends(current_principal)]


@router.post("", status_code=201)
async def create_workflow(body: CreateWorkflowRequest, principal: CurrentUser, session: Session) -> Any:
    app_id = require_uuid(body.application_id, "applicationId")
    await authorize_application(session, principal, app_id, "workflow:create")
    result = await session.execute(
        text("""
            INSERT INTO control.workflows(name,description,application_id,owner_id,nodes,edges,variables)
            VALUES(:name,:description,CAST(:app AS uuid),CAST(:owner AS uuid),CAST(:nodes AS jsonb),CAST(:edges AS jsonb),CAST(:variables AS jsonb))
            RETURNING id::text,name,description,application_id::text,nodes,edges,variables,current_version,created_at,updated_at
        """),
        {"name": validate_name(body.name, 100), "description": body.description, "app": app_id, "owner": principal.user_id,
         "nodes": json.dumps(body.nodes), "edges": json.dumps(body.edges), "variables": json.dumps(body.variables)},
    )
    await session.commit()
    return success(workflow_data(result.mappings().one()), status_code=201)


@router.get("/app/{app_id}")
async def list_by_app(app_id: str, principal: CurrentUser, session: Session) -> Any:
    value = require_uuid(app_id, "appId")
    await authorize_application(session, principal, value, "workflow:read")
    result = await session.execute(
        text("""SELECT id::text,name,description,application_id::text,nodes,edges,variables,current_version,created_at,updated_at
                FROM control.workflows WHERE application_id=CAST(:app AS uuid) ORDER BY updated_at DESC"""), {"app": value}
    )
    return success([workflow_data(row) for row in result.mappings()])


@router.get("/{workflow_id}")
async def get_workflow(workflow_id: str, principal: CurrentUser, session: Session) -> Any:
    return success(workflow_data(await load_workflow(session, principal, workflow_id)))


@router.patch("/{workflow_id}")
async def update_workflow(body: UpdateWorkflowRequest, workflow_id: str, principal: CurrentUser, session: Session) -> Any:
    current = await load_workflow(session, principal, workflow_id, "workflow:update")
    values: dict[str, Any] = {"id": current["id"]}
    assignments: list[str] = []
    if "application_id" in body.model_fields_set:
        if body.application_id is None:
            raise APIError(400, "BAD_REQUEST", "applicationId must be a UUID")
        target = require_uuid(body.application_id, "applicationId")
        await authorize_application(session, principal, target, "workflow:create")
        assignments.append("application_id=CAST(:application_id AS uuid)")
        values["application_id"] = target
    for field in ("name", "description", "nodes", "edges", "variables"):
        if field not in body.model_fields_set:
            continue
        value = getattr(body, field)
        if field in {"name", "nodes", "edges", "variables"} and value is None:
            raise APIError(400, "BAD_REQUEST", f"{field} cannot be null")
        if field == "name":
            value = validate_name(value, 100)
        if field in {"nodes", "edges", "variables"}:
            assignments.append(f"{field}=CAST(:{field} AS jsonb)")
            value = json.dumps(value)
        else:
            assignments.append(f"{field}=:{field}")
        values[field] = value
    if not assignments:
        assignments.append("updated_at=updated_at")
    result = await session.execute(
        text(f"""UPDATE control.workflows SET {','.join(assignments)} WHERE id=CAST(:id AS uuid)
                RETURNING id::text,name,description,application_id::text,nodes,edges,variables,current_version,created_at,updated_at"""), values
    )
    await session.commit()
    return success(workflow_data(result.mappings().one()))


@router.delete("/{workflow_id}")
async def delete_workflow(workflow_id: str, principal: CurrentUser, session: Session) -> Any:
    row = await load_workflow(session, principal, workflow_id, "workflow:delete")
    await session.execute(text("DELETE FROM control.workflows WHERE id=CAST(:id AS uuid)"), {"id": row["id"]})
    await session.commit()
    return success({"success": True})


@router.post("/{workflow_id}/run")
async def run_workflow(body: RunWorkflowRequest, workflow_id: str, request: Request, principal: CurrentUser, session: Session) -> Any:
    workflow=await load_workflow(session,principal,workflow_id,"workflow:execute")
    runtime=WorkflowRuntime(session,request.app.state.redis,getattr(request.app.state,"node_services",UnavailableNodeServices()))
    terminal:dict[str,Any]|None=None
    async for value in runtime.events(workflow,principal.user_id,body.inputs,body.control): terminal=value
    if terminal is None: raise APIError(500,"INTERNAL_ERROR","Workflow produced no result")
    if terminal["type"]=="error": raise APIError(500,"WORKFLOW_EXECUTION_ERROR",terminal["data"]["message"],terminal["data"])
    return success(terminal["data"])


@router.post("/{workflow_id}/run/stream")
async def stream_workflow(body: RunWorkflowRequest, workflow_id: str, request: Request, principal: CurrentUser, session: Session) -> StreamingResponse:
    workflow=await load_workflow(session,principal,workflow_id,"workflow:execute")
    runtime=WorkflowRuntime(session,request.app.state.redis,getattr(request.app.state,"node_services",UnavailableNodeServices()))
    async def stream() -> Any:
        async for value in runtime.events(workflow,principal.user_id,body.inputs,body.control): yield sse(value)
    return StreamingResponse(stream(),media_type="text/event-stream",headers={"Cache-Control":"no-cache","X-Accel-Buffering":"no"})


@router.post("/{workflow_id}/cancel/{execution_id}",status_code=201)
async def cancel_execution(workflow_id:str,execution_id:str,principal:CurrentUser,session:Session,request:Request)->Any:
    await load_workflow(session,principal,workflow_id,"workflow:execute")
    from aiflow_runtime.workflow.state import ExecutionState
    state=ExecutionState(request.app.state.redis,require_uuid(execution_id,"executionId"),principal.user_id); await state.cancel()
    return success({"success":True,"message":"Execution cancellation requested"},status_code=201)


@router.get("/{workflow_id}/running")
async def running_executions(workflow_id:str,principal:CurrentUser,session:Session,request:Request)->Any:
    workflow=await load_workflow(session,principal,workflow_id,"workflow:read")
    ids=await request.app.state.redis.smembers(f"flowai:running:{principal.user_id}"); executions=[]
    for execution_id in ids:
        values=await request.app.state.redis.hgetall(f"flowai:execution:{execution_id}")
        if values.get("workflowId")==workflow["id"]: executions.append({"executionId":execution_id,**values})
    return success({"executions":executions})

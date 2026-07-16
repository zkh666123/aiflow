from __future__ import annotations

from typing import Annotated,Any

from fastapi import APIRouter,Depends,Query
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.envelope import success
from aiflow_runtime.api.errors import APIError
from aiflow_runtime.identity.auth import current_principal
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.identity.service import require_uuid
from aiflow_runtime.infrastructure.database import get_session

router=APIRouter(prefix="/api/traces",tags=["traces"]); Session=Annotated[AsyncSession,Depends(get_session)]; CurrentUser=Annotated[Principal,Depends(current_principal)]


def trace_data(row:Any)->dict[str,Any]:
    return {"id":row["id"],"workflowId":row["workflow_id"],"executionId":row["execution_id"],"status":row["status"],"duration":row["duration_ms"],
            "input":row["input"],"output":row["output"],"error":row["error"],"startedAt":row["started_at"],"completedAt":row["completed_at"],"createdAt":row["created_at"]}


TRACE_SELECT="SELECT id::text,workflow_id::text,execution_id::text,status,duration_ms,input,output,error,started_at,completed_at,created_at FROM control.traces"


@router.get("/slow/list")
async def slow_traces(_principal:CurrentUser,session:Session,workflow_id:str|None=Query(default=None,alias="workflowId"),limit:int=10)->Any:
    workflow=require_uuid(workflow_id,"workflowId") if workflow_id else None
    result=await session.execute(text(TRACE_SELECT+" WHERE (:workflow IS NULL OR workflow_id=CAST(:workflow AS uuid)) ORDER BY duration_ms DESC NULLS LAST LIMIT :limit"),{"workflow":workflow,"limit":min(max(limit,1),100)})
    return success([trace_data(row) for row in result.mappings()])


@router.get("/stats/overview")
async def trace_stats(_principal:CurrentUser,session:Session,workflow_id:str|None=Query(default=None,alias="workflowId"))->Any:
    workflow=require_uuid(workflow_id,"workflowId") if workflow_id else None
    result=await session.execute(text("""SELECT COUNT(*)::int AS total,COUNT(*) FILTER(WHERE status='success')::int AS success,
        COUNT(*) FILTER(WHERE status='failed')::int AS failed,COALESCE(AVG(duration_ms),0)::float AS average_duration
        FROM control.traces WHERE (:workflow IS NULL OR workflow_id=CAST(:workflow AS uuid))"""),{"workflow":workflow})
    row=result.mappings().one(); return success({"total":row["total"],"success":row["success"],"failed":row["failed"],"averageDuration":row["average_duration"]})


@router.get("/workflow/{workflow_id}")
async def workflow_traces(workflow_id:str,_principal:CurrentUser,session:Session,limit:int=20)->Any:
    result=await session.execute(text(TRACE_SELECT+" WHERE workflow_id=CAST(:id AS uuid) ORDER BY started_at DESC LIMIT :limit"),{"id":require_uuid(workflow_id,"workflowId"),"limit":min(max(limit,1),100)})
    return success([trace_data(row) for row in result.mappings()])


@router.get("/{trace_id}")
async def get_trace(trace_id:str,_principal:CurrentUser,session:Session)->Any:
    result=await session.execute(text(TRACE_SELECT+" WHERE id=CAST(:id AS uuid)"),{"id":require_uuid(trace_id,"traceId")}); row=result.mappings().first()
    if row is None: raise APIError(404,"NOT_FOUND","Trace not found")
    spans=await session.execute(text("""SELECT id::text,parent_span_id::text,node_id,name,kind,status,input,output,error,metadata,started_at,completed_at,duration_ms
        FROM control.spans WHERE trace_id=CAST(:id AS uuid) ORDER BY started_at"""),{"id":trace_id})
    return success({**trace_data(row),"spans":[dict(item) for item in spans.mappings()]})

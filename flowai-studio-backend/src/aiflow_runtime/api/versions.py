from __future__ import annotations

import json
from typing import Annotated, Any

from fastapi import APIRouter, Depends, Query
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.envelope import success
from aiflow_runtime.api.errors import APIError
from aiflow_runtime.identity.auth import current_principal
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.infrastructure.database import get_session
from aiflow_runtime.workflow.schemas import CreateVersionRequest
from aiflow_runtime.workflow.service import load_workflow

router=APIRouter(prefix="/api/workflows/{workflow_id}/versions",tags=["versions"])
Session=Annotated[AsyncSession,Depends(get_session)]; CurrentUser=Annotated[Principal,Depends(current_principal)]


def version_data(row: Any) -> dict[str,Any]:
    return {"id":row["id"],"workflowId":row["workflow_id"],"version":row["version"],"label":row["label"],"description":row["description"],
            "nodes":row["nodes"],"edges":row["edges"],"variables":row["variables"],"createdBy":row["created_by"],"isPublished":row["is_published"],"createdAt":row["created_at"]}


async def get_version_row(session:AsyncSession,workflow_id:str,version:int)->Any:
    result=await session.execute(text("""SELECT id::text,workflow_id::text,version,label,description,nodes,edges,variables,created_by::text,is_published,created_at
        FROM control.workflow_versions WHERE workflow_id=CAST(:workflow AS uuid) AND version=:version"""),{"workflow":workflow_id,"version":version})
    row=result.mappings().first()
    if row is None: raise APIError(404,"NOT_FOUND","Workflow version not found")
    return row


@router.post("",status_code=201)
async def create_version(body:CreateVersionRequest,workflow_id:str,principal:CurrentUser,session:Session)->Any:
    workflow=await load_workflow(session,principal,workflow_id,"workflow:update")
    number=(await session.execute(text("SELECT COALESCE(MAX(version),0)+1 FROM control.workflow_versions WHERE workflow_id=CAST(:id AS uuid)"),{"id":workflow["id"]})).scalar_one()
    result=await session.execute(text("""INSERT INTO control.workflow_versions(workflow_id,version,label,description,nodes,edges,variables,created_by,is_published)
        VALUES(CAST(:workflow AS uuid),:version,:label,:description,CAST(:nodes AS jsonb),CAST(:edges AS jsonb),CAST(:variables AS jsonb),CAST(:user AS uuid),:published)
        RETURNING id::text,workflow_id::text,version,label,description,nodes,edges,variables,created_by::text,is_published,created_at"""),
        {"workflow":workflow["id"],"version":number,"label":body.label,"description":body.description,"nodes":json.dumps(workflow["nodes"]),"edges":json.dumps(workflow["edges"]),"variables":json.dumps(workflow["variables"]),"user":principal.user_id,"published":body.is_published})
    await session.execute(text("UPDATE control.workflows SET current_version=:version WHERE id=CAST(:id AS uuid)"),{"version":number,"id":workflow["id"]}); await session.commit()
    return success(version_data(result.mappings().one()),status_code=201)


@router.get("")
async def list_versions(workflow_id:str,principal:CurrentUser,session:Session)->Any:
    workflow=await load_workflow(session,principal,workflow_id)
    result=await session.execute(text("""SELECT id::text,workflow_id::text,version,label,description,nodes,edges,variables,created_by::text,is_published,created_at
        FROM control.workflow_versions WHERE workflow_id=CAST(:id AS uuid) ORDER BY version DESC"""),{"id":workflow["id"]})
    return success([version_data(row) for row in result.mappings()])


@router.get("/compare")
async def compare_versions(workflow_id:str,principal:CurrentUser,session:Session,from_version:int=Query(alias="from"),to_version:int=Query(alias="to"))->Any:
    workflow=await load_workflow(session,principal,workflow_id)
    async def snapshot(number:int)->dict[str,Any]:
        if number==0:return {"nodes":workflow["nodes"],"edges":workflow["edges"],"variables":workflow["variables"]}
        row=await get_version_row(session,workflow["id"],number); return {"nodes":row["nodes"],"edges":row["edges"],"variables":row["variables"]}
    left,right=await snapshot(from_version),await snapshot(to_version)
    def diff(items1:list[Any],items2:list[Any])->dict[str,Any]:
        one={item.get("id"):item for item in items1}; two={item.get("id"):item for item in items2}
        return {"added":[two[key] for key in two.keys()-one.keys()],"removed":[one[key] for key in one.keys()-two.keys()],"modified":[{"before":one[key],"after":two[key]} for key in one.keys()&two.keys() if one[key]!=two[key]]}
    nodes,edges=diff(left["nodes"],right["nodes"]),diff(left["edges"],right["edges"])
    return success({"from":from_version,"to":to_version,"nodes":nodes,"edges":edges,"variables":{"before":left["variables"],"after":right["variables"],"changed":left["variables"]!=right["variables"]}})


@router.get("/{version}")
async def get_version(workflow_id:str,version:int,principal:CurrentUser,session:Session)->Any:
    workflow=await load_workflow(session,principal,workflow_id); return success(version_data(await get_version_row(session,workflow["id"],version)))


@router.post("/{version}/rollback",status_code=201)
async def rollback(workflow_id:str,version:int,principal:CurrentUser,session:Session)->Any:
    workflow=await load_workflow(session,principal,workflow_id,"workflow:update"); target=await get_version_row(session,workflow["id"],version)
    await session.execute(text("""UPDATE control.workflows SET nodes=CAST(:nodes AS jsonb),edges=CAST(:edges AS jsonb),variables=CAST(:variables AS jsonb),current_version=:version
        WHERE id=CAST(:id AS uuid)"""),{"nodes":json.dumps(target["nodes"]),"edges":json.dumps(target["edges"]),"variables":json.dumps(target["variables"]),"version":version,"id":workflow["id"]})
    await session.commit(); return success({"success":True,"version":version},status_code=201)


@router.delete("/{version}")
async def delete_version(workflow_id:str,version:int,principal:CurrentUser,session:Session)->Any:
    workflow=await load_workflow(session,principal,workflow_id,"workflow:update")
    result=await session.execute(text("DELETE FROM control.workflow_versions WHERE workflow_id=CAST(:id AS uuid) AND version=:version"),{"id":workflow["id"],"version":version})
    if result.rowcount==0: raise APIError(404,"NOT_FOUND","Workflow version not found")
    await session.commit(); return success({"success":True})

from __future__ import annotations
import json
from typing import Annotated,Any
from fastapi import APIRouter,Depends,Request
from pydantic import Field
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession
from aiflow_runtime.api.envelope import success
from aiflow_runtime.api.errors import APIError
from aiflow_runtime.identity.auth import current_principal
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.identity.schemas import StrictModel
from aiflow_runtime.identity.service import require_uuid,validate_name
from aiflow_runtime.infrastructure.database import get_session
router=APIRouter(prefix="/api/skill",tags=["skills"]);Session=Annotated[AsyncSession,Depends(get_session)];CurrentUser=Annotated[Principal,Depends(current_principal)]
class SkillRequest(StrictModel):name:str;description:str|None=None;skill_type:str=Field(default="python",alias="type");code:str|None=None;configuration:dict[str,Any]=Field(default_factory=dict)
class ExecuteRequest(StrictModel):params:dict[str,Any]=Field(default_factory=dict)
SELECT="SELECT id::text,name,description,skill_type,code,configuration,created_at,updated_at FROM ai.skills"
async def owned(session:AsyncSession,user:str,skill_id:str)->Any:
    result=await session.execute(text(SELECT+" WHERE id=CAST(:id AS uuid) AND user_id=CAST(:user AS uuid)"),{"id":require_uuid(skill_id),"user":user});row=result.mappings().first()
    if row is None:raise APIError(404,"NOT_FOUND","Skill not found")
    return row
@router.get("/builtin/list")
async def builtin(_principal:CurrentUser)->Any:return success([{"id":"calculator","name":"Calculator","type":"builtin","description":"Safe arithmetic expressions"},{"id":"python","name":"Python","type":"builtin","description":"Restricted Python sandbox"}])
@router.post("",status_code=201)
async def create(body:SkillRequest,principal:CurrentUser,session:Session)->Any:
    result=await session.execute(text("""INSERT INTO ai.skills(user_id,name,description,skill_type,code,configuration) VALUES(CAST(:user AS uuid),:name,:description,:type,:code,CAST(:config AS jsonb))
      RETURNING id::text,name,description,skill_type,code,configuration,created_at,updated_at"""),{"user":principal.user_id,"name":validate_name(body.name,100),"description":body.description,"type":body.skill_type,"code":body.code,"config":json.dumps(body.configuration)});await session.commit();return success(dict(result.mappings().one()),status_code=201)
@router.get("")
async def listing(principal:CurrentUser,session:Session)->Any:
    result=await session.execute(text(SELECT+" WHERE user_id=CAST(:user AS uuid) ORDER BY updated_at DESC"),{"user":principal.user_id});return success([dict(row) for row in result.mappings()])
@router.get("/{skill_id}")
async def get(skill_id:str,principal:CurrentUser,session:Session)->Any:return success(dict(await owned(session,principal.user_id,skill_id)))
@router.put("/{skill_id}")
async def update(body:SkillRequest,skill_id:str,principal:CurrentUser,session:Session)->Any:
    row=await owned(session,principal.user_id,skill_id);result=await session.execute(text("""UPDATE ai.skills SET name=:name,description=:description,skill_type=:type,code=:code,configuration=CAST(:config AS jsonb) WHERE id=CAST(:id AS uuid)
      RETURNING id::text,name,description,skill_type,code,configuration,created_at,updated_at"""),{"id":row["id"],"name":validate_name(body.name,100),"description":body.description,"type":body.skill_type,"code":body.code,"config":json.dumps(body.configuration)});await session.commit();return success(dict(result.mappings().one()))
@router.delete("/{skill_id}")
async def delete(skill_id:str,principal:CurrentUser,session:Session)->Any:
    row=await owned(session,principal.user_id,skill_id);await session.execute(text("DELETE FROM ai.skills WHERE id=CAST(:id AS uuid)"),{"id":row["id"]});await session.commit();return success({"success":True})
@router.post("/{skill_id}/execute",status_code=201)
async def execute(body:ExecuteRequest,skill_id:str,request:Request,principal:CurrentUser,session:Session)->Any:
    if skill_id=="calculator":return success({"result":request.app.state.skills and __import__('aiflow_runtime.ai.skills',fromlist=['calculate']).calculate(str(body.params.get('expression','0')))},status_code=201)
    row=await owned(session,principal.user_id,skill_id);result=await request.app.state.skills.execute_python(str(row["code"] or ""));return success(result,status_code=201)

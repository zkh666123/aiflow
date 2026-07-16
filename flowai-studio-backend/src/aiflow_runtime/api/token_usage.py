from __future__ import annotations

from typing import Annotated,Any
from fastapi import APIRouter,Depends,Query
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.envelope import success
from aiflow_runtime.identity.auth import current_principal
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.infrastructure.database import get_session

router=APIRouter(prefix="/api/token-usage",tags=["token-usage"]);Session=Annotated[AsyncSession,Depends(get_session)];CurrentUser=Annotated[Principal,Depends(current_principal)]
@router.get("")
async def usage(principal:CurrentUser,session:Session,limit:int=Query(default=100,ge=1,le=1000))->Any:
    result=await session.execute(text("SELECT id::text,provider,model,prompt_tokens,completion_tokens,total_tokens,cost,created_at FROM ai.token_usage WHERE user_id=CAST(:user AS uuid) ORDER BY created_at DESC LIMIT :limit"),{"user":principal.user_id,"limit":limit});return success([dict(row) for row in result.mappings()])
@router.get("/cost-report")
async def cost_report(principal:CurrentUser,session:Session)->Any:
    result=await session.execute(text("SELECT provider,model,SUM(total_tokens)::int AS total_tokens,SUM(cost)::float AS cost FROM ai.token_usage WHERE user_id=CAST(:user AS uuid) GROUP BY provider,model ORDER BY cost DESC"),{"user":principal.user_id});return success([dict(row) for row in result.mappings()])
@router.get("/model-ranking")
async def ranking(principal:CurrentUser,session:Session)->Any:
    result=await session.execute(text("SELECT model,COUNT(*)::int AS requests,SUM(total_tokens)::int AS total_tokens,SUM(cost)::float AS cost FROM ai.token_usage WHERE user_id=CAST(:user AS uuid) GROUP BY model ORDER BY requests DESC"),{"user":principal.user_id});return success([dict(row) for row in result.mappings()])

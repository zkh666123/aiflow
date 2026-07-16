from __future__ import annotations
from typing import Annotated,Any
from fastapi import APIRouter,Depends,Request
from aiflow_runtime.api.envelope import success
from aiflow_runtime.identity.auth import current_principal
from aiflow_runtime.identity.models import Principal
CurrentUser=Annotated[Principal,Depends(current_principal)];router=APIRouter(tags=["operations"])
LIMITS={"workflow:run":{"maxRequests":30,"windowSeconds":60,"maxConcurrent":3},"ai:global":{"maxRequests":60,"windowSeconds":60,"maxConcurrent":5},"knowledge_base:retrieve":{"maxRequests":120,"windowSeconds":60}}
@router.get("/api/rate-limit/config")
async def config(_principal:CurrentUser)->Any:return success({"limits":LIMITS})
@router.get("/api/rate-limit/quota/{user_id}")
async def quota(user_id:str,request:Request,_principal:CurrentUser)->Any:
    values=[]
    for name,item in LIMITS.items():
        _,remaining=await request.app.state.limits.token(f"flowai:quota:{name}:{user_id}",item["maxRequests"],item["maxRequests"]/item["windowSeconds"]);values.append({"name":name,"remaining":remaining,"max":item["maxRequests"],"windowSeconds":item["windowSeconds"]})
    return success({"quotas":values})
@router.get("/api/rate-limit/circuit-breakers")
async def circuits(request:Request,_principal:CurrentUser)->Any:return success({"circuitBreakers":[{"name":name,**await request.app.state.limits.circuit(name)} for name in ("workflow","ai","knowledge_base")]})
@router.post("/api/rate-limit/circuit-breakers/{name}/reset",status_code=201)
async def reset(name:str,request:Request,_principal:CurrentUser)->Any:await request.app.state.limits.reset(name);return success({"success":True,"message":f"Circuit breaker [{name}] has been reset"},status_code=201)
@router.get("/api/health/cache-stats")
async def cache_stats(request:Request)->Any:return success(request.app.state.cache.stats())

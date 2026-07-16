from __future__ import annotations

from typing import Any
from fastapi import APIRouter,Request

from aiflow_runtime.ai.providers import MODELS
from aiflow_runtime.api.envelope import success
from aiflow_runtime.api.errors import APIError

router=APIRouter(prefix="/api/llm",tags=["models"])


@router.get("/models")
async def models()->Any:return success(MODELS)
@router.get("/models/list")
async def model_list()->Any:return success([{"label":item["name"],"value":item["id"],"provider":item["provider"]} for item in MODELS])
@router.get("/models/{model_id}")
async def model(model_id:str)->Any:
    item=next((value for value in MODELS if value["id"]==model_id),None)
    if item is None:raise APIError(404,"NOT_FOUND","Model not found")
    return success(item)
@router.get("/health")
async def health(request:Request)->Any:
    states={provider:bool(getattr(request.app.state.settings,field,None)) for provider,field in {"openai":"openai_api_key","anthropic":"anthropic_api_key","gemini":"gemini_api_key","qwen":"qwen_api_key"}.items()};states["ollama"]=True
    return success(states)
@router.get("/cost")
async def cost()->Any:return success([{"model":item["id"],"inputCost":item["inputCost"],"outputCost":item["outputCost"]} for item in MODELS])
@router.get("/ollama/discover")
async def discover(request:Request)->Any:return success(await request.app.state.providers.discover_ollama())

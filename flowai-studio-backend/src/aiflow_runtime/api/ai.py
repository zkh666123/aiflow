from __future__ import annotations

import json
from typing import Annotated,Any

from fastapi import APIRouter,Depends,Request
from fastapi.responses import StreamingResponse
from pydantic import Field
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.envelope import success
from aiflow_runtime.api.errors import APIError
from aiflow_runtime.identity.auth import current_principal
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.identity.schemas import StrictModel
from aiflow_runtime.infrastructure.database import get_session

router=APIRouter(prefix="/api/ai",tags=["ai"]);Session=Annotated[AsyncSession,Depends(get_session)];CurrentUser=Annotated[Principal,Depends(current_principal)]


class AIRequest(StrictModel):
    model:str="llama3.2";prompt:str|None=None;messages:list[dict[str,str]]|None=None;system_prompt:str|None=Field(default=None,alias="systemPrompt")
    temperature:float=.7;max_tokens:int=Field(default=1024,alias="maxTokens");session_id:str|None=Field(default=None,alias="sessionId")


async def complete(body:AIRequest,request:Request)->Any:
    messages=body.messages or ([{"role":"system","content":body.system_prompt}] if body.system_prompt else [])+[{"role":"user","content":body.prompt or ""}]
    return await request.app.state.providers.complete(body.model,messages,body.temperature,body.max_tokens)


@router.post("/run",status_code=201)
async def run(body:AIRequest,request:Request,_principal:CurrentUser)->Any:
    result=await complete(body,request);return success({"content":result.content,"model":result.model,"usage":{"promptTokens":result.prompt_tokens,"completionTokens":result.completion_tokens}},status_code=201)


@router.post("/stream-run",status_code=201)
async def stream_run(body:AIRequest,request:Request,_principal:CurrentUser)->StreamingResponse:
    async def stream()->Any:
        try:
            result=await complete(body,request)
            for chunk in result.content.splitlines(keepends=True) or [result.content]:yield f"data: {json.dumps({'type':'token','data':chunk},ensure_ascii=False)}\n\n"
            yield f"data: {json.dumps({'type':'done','data':{'content':result.content}},ensure_ascii=False)}\n\n"
        except Exception as exc:yield f"data: {json.dumps({'type':'error','message':str(exc)},ensure_ascii=False)}\n\n"
    return StreamingResponse(stream(),media_type="text/event-stream",headers={"Cache-Control":"no-cache","X-Accel-Buffering":"no"})


@router.post("/chat",status_code=201)
async def chat(body:AIRequest,request:Request,principal:CurrentUser,session:Session)->Any:
    result=await complete(body,request)
    if body.session_id:
        session_id=body.session_id
    else:
        created=await session.execute(text("INSERT INTO ai.chat_sessions(user_id,model,title) VALUES(CAST(:user AS uuid),:model,:title) RETURNING id::text"),{"user":principal.user_id,"model":body.model,"title":(body.prompt or "New chat")[:100]});session_id=created.scalar_one()
    await session.execute(text("INSERT INTO ai.chat_messages(session_id,role,content) VALUES(CAST(:session AS uuid),'user',:prompt),(CAST(:session AS uuid),'assistant',:answer)"),{"session":session_id,"prompt":body.prompt or "","answer":result.content});await session.commit()
    return success({"sessionId":session_id,"content":result.content,"model":result.model},status_code=201)


@router.get("/chat-histories")
async def histories(principal:CurrentUser,session:Session)->Any:
    result=await session.execute(text("SELECT id::text,model,title,created_at,updated_at FROM ai.chat_sessions WHERE user_id=CAST(:user AS uuid) ORDER BY updated_at DESC"),{"user":principal.user_id})
    return success([{"id":row["id"],"model":row["model"],"title":row["title"],"createdAt":row["created_at"],"updatedAt":row["updated_at"]} for row in result.mappings()])


@router.get("/chat-histories/{session_id}")
async def history(session_id:str,principal:CurrentUser,session:Session)->Any:
    owner=await session.execute(text("SELECT id::text,model,title,created_at,updated_at FROM ai.chat_sessions WHERE id=CAST(:id AS uuid) AND user_id=CAST(:user AS uuid)"),{"id":session_id,"user":principal.user_id});row=owner.mappings().first()
    if row is None:raise APIError(404,"NOT_FOUND","Chat history not found")
    messages=await session.execute(text("SELECT id::text,role,content,metadata,created_at FROM ai.chat_messages WHERE session_id=CAST(:id AS uuid) ORDER BY created_at"),{"id":session_id})
    return success({"id":row["id"],"model":row["model"],"title":row["title"],"messages":[dict(item) for item in messages.mappings()]})

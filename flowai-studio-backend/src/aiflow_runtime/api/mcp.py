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
router=APIRouter(prefix="/api/mcp",tags=["mcp"]);Session=Annotated[AsyncSession,Depends(get_session)];CurrentUser=Annotated[Principal,Depends(current_principal)]
class MCPRequest(StrictModel):name:str;description:str|None=None;transport_type:str=Field(default="http",alias="transportType");command:str|None=None;args:list[str]=Field(default_factory=list);env:dict[str,str]=Field(default_factory=dict);url:str|None=None;is_active:bool=True
class CallRequest(StrictModel):tool_name:str=Field(alias="toolName");args:dict[str,Any]=Field(default_factory=dict)
SELECT="SELECT id::text,name,description,transport_type,command,args,environment,url,is_active,state,tools,created_at,updated_at FROM ai.mcp_servers"
async def owned(session:AsyncSession,user:str,server_id:str)->Any:
    result=await session.execute(text(SELECT+" WHERE id=CAST(:id AS uuid) AND user_id=CAST(:user AS uuid)"),{"id":require_uuid(server_id),"user":user});row=result.mappings().first()
    if row is None:raise APIError(404,"NOT_FOUND","MCP server not found")
    return row
@router.post("/servers",status_code=201)
async def create(body:MCPRequest,principal:CurrentUser,session:Session)->Any:
    result=await session.execute(text("""INSERT INTO ai.mcp_servers(user_id,name,description,transport_type,command,args,environment,url,is_active) VALUES(CAST(:user AS uuid),:name,:description,:transport,:command,CAST(:args AS jsonb),CAST(:env AS jsonb),:url,:active)
      RETURNING id::text,name,description,transport_type,command,args,environment,url,is_active,state,tools,created_at,updated_at"""),{"user":principal.user_id,"name":validate_name(body.name,100),"description":body.description,"transport":body.transport_type,"command":body.command,"args":json.dumps(body.args),"env":json.dumps(body.env),"url":body.url,"active":body.is_active});await session.commit();return success(dict(result.mappings().one()),status_code=201)
@router.get("/servers")
async def listing(principal:CurrentUser,session:Session)->Any:
    result=await session.execute(text(SELECT+" WHERE user_id=CAST(:user AS uuid) ORDER BY updated_at DESC"),{"user":principal.user_id});return success([dict(row) for row in result.mappings()])
@router.get("/servers/{server_id}")
async def get(server_id:str,principal:CurrentUser,session:Session)->Any:return success(dict(await owned(session,principal.user_id,server_id)))
@router.put("/servers/{server_id}")
async def update(body:MCPRequest,server_id:str,principal:CurrentUser,session:Session)->Any:
    row=await owned(session,principal.user_id,server_id);await session.execute(text("""UPDATE ai.mcp_servers SET name=:name,description=:description,transport_type=:transport,command=:command,args=CAST(:args AS jsonb),environment=CAST(:env AS jsonb),url=:url,is_active=:active WHERE id=CAST(:id AS uuid)"""),{"id":row["id"],"name":body.name,"description":body.description,"transport":body.transport_type,"command":body.command,"args":json.dumps(body.args),"env":json.dumps(body.env),"url":body.url,"active":body.is_active});await session.commit();return success(dict(await owned(session,principal.user_id,server_id)))
@router.delete("/servers/{server_id}")
async def delete(server_id:str,principal:CurrentUser,session:Session)->Any:
    row=await owned(session,principal.user_id,server_id);await session.execute(text("DELETE FROM ai.mcp_servers WHERE id=CAST(:id AS uuid)"),{"id":row["id"]});await session.commit();return success({"success":True})
async def rpc(request:Request,row:Any,method:str,params:dict[str,Any])->Any:
    if not row["url"]:raise APIError(400,"BAD_REQUEST","HTTP MCP URL is required")
    response=await request.app.state.http_client.post(row["url"],json={"jsonrpc":"2.0","id":1,"method":method,"params":params});response.raise_for_status();data=response.json()
    if "error" in data:raise APIError(502,"MCP_ERROR",str(data["error"]));return data.get("result")
@router.post("/servers/{server_id}/connect",status_code=201)
async def connect(server_id:str,request:Request,principal:CurrentUser,session:Session)->Any:
    row=await owned(session,principal.user_id,server_id);tools=await rpc(request,row,"tools/list",{});await session.execute(text("UPDATE ai.mcp_servers SET state='connected',tools=CAST(:tools AS jsonb) WHERE id=CAST(:id AS uuid)"),{"id":row["id"],"tools":json.dumps((tools or {}).get("tools",[]))});await session.commit();return success({"state":"connected","tools":(tools or {}).get("tools",[])},status_code=201)
@router.post("/servers/{server_id}/disconnect",status_code=201)
async def disconnect(server_id:str,principal:CurrentUser,session:Session)->Any:
    row=await owned(session,principal.user_id,server_id);await session.execute(text("UPDATE ai.mcp_servers SET state='disconnected' WHERE id=CAST(:id AS uuid)"),{"id":row["id"]});await session.commit();return success({"state":"disconnected"},status_code=201)
@router.get("/servers/{server_id}/tools")
async def tools(server_id:str,principal:CurrentUser,session:Session)->Any:return success((await owned(session,principal.user_id,server_id))["tools"])
@router.post("/servers/{server_id}/tools/call",status_code=201)
async def call(body:CallRequest,server_id:str,request:Request,principal:CurrentUser,session:Session)->Any:return success(await rpc(request,await owned(session,principal.user_id,server_id),"tools/call",{"name":body.tool_name,"arguments":body.args}),status_code=201)
@router.get("/tools")
async def all_tools(principal:CurrentUser,session:Session)->Any:
    result=await session.execute(text("SELECT id::text,name,tools FROM ai.mcp_servers WHERE user_id=CAST(:user AS uuid) AND state='connected'"),{"user":principal.user_id});return success([{"serverId":row["id"],"serverName":row["name"],"tools":row["tools"]} for row in result.mappings()])

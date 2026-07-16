from __future__ import annotations
import json
from typing import Annotated,Any
from fastapi import APIRouter,BackgroundTasks,Depends,File,Form,Request,UploadFile
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
from aiflow_runtime.ai.documents import chunks,parse_document

router=APIRouter(prefix="/api/rag",tags=["rag"]);Session=Annotated[AsyncSession,Depends(get_session)];CurrentUser=Annotated[Principal,Depends(current_principal)]
class KBRequest(StrictModel):
    name:str;description:str|None=None;embedding_provider:str=Field(default="ollama",alias="embeddingProvider");embedding_model:str=Field(default="nomic-embed-text",alias="embeddingModel")
    chunk_size:int=Field(default=500,ge=50,le=4000,alias="chunkSize");chunk_overlap:int=Field(default=50,ge=0,le=1000,alias="chunkOverlap")
    top_k:int=Field(default=5,ge=1,le=100,alias="topK");similarity_threshold:float=Field(default=.7,ge=0,le=1,alias="similarityThreshold")
    retrieval_mode:str=Field(default="hybrid",alias="retrievalMode");vector_weight:float=Field(default=.7,ge=0,le=1,alias="vectorWeight");rrf_k:int=Field(default=60,ge=1,alias="rrfK")
class RetrieveRequest(StrictModel):
    knowledge_base_id:str=Field(alias="knowledgeBaseId");query:str;top_k:int=Field(default=5,alias="topK")

def kb_data(row:Any)->dict[str,Any]:
    return {"id":row["id"],"name":row["name"],"description":row["description"],"embeddingProvider":row["embedding_provider"],"embeddingModel":row["embedding_model"],
      "chunkSize":row["chunk_size"],"chunkOverlap":row["chunk_overlap"],"topK":row["top_k"],"similarityThreshold":row["similarity_threshold"],
      "retrievalMode":row["retrieval_mode"],"vectorWeight":row["vector_weight"],"rrfK":row["rrf_k"],"createdAt":row["created_at"],"updatedAt":row["updated_at"]}
KB_SELECT="SELECT id::text,name,description,embedding_provider,embedding_model,chunk_size,chunk_overlap,top_k,similarity_threshold,retrieval_mode,vector_weight,rrf_k,created_at,updated_at FROM ai.knowledge_bases"
async def owned_kb(session:AsyncSession,user_id:str,kb_id:str)->Any:
    result=await session.execute(text(KB_SELECT+" WHERE id=CAST(:id AS uuid) AND user_id=CAST(:user AS uuid)"),{"id":require_uuid(kb_id,"id"),"user":user_id});row=result.mappings().first()
    if row is None:raise APIError(404,"NOT_FOUND","Knowledge base not found")
    return row
@router.post("/knowledge-bases",status_code=201)
async def create_kb(body:KBRequest,principal:CurrentUser,session:Session)->Any:
    result=await session.execute(text("""INSERT INTO ai.knowledge_bases(user_id,name,description,embedding_provider,embedding_model,chunk_size,chunk_overlap,top_k,similarity_threshold,retrieval_mode,vector_weight,rrf_k)
      VALUES(CAST(:user AS uuid),:name,:description,:provider,:model,:size,:overlap,:top,:threshold,:mode,:weight,:rrf)
      RETURNING id::text,name,description,embedding_provider,embedding_model,chunk_size,chunk_overlap,top_k,similarity_threshold,retrieval_mode,vector_weight,rrf_k,created_at,updated_at"""),
      {"user":principal.user_id,"name":validate_name(body.name,100),"description":body.description,"provider":body.embedding_provider,"model":body.embedding_model,"size":body.chunk_size,"overlap":body.chunk_overlap,"top":body.top_k,"threshold":body.similarity_threshold,"mode":body.retrieval_mode,"weight":body.vector_weight,"rrf":body.rrf_k})
    await session.commit();return success(kb_data(result.mappings().one()),status_code=201)
@router.get("/knowledge-bases")
async def list_kb(principal:CurrentUser,session:Session)->Any:
    result=await session.execute(text(KB_SELECT+" WHERE user_id=CAST(:user AS uuid) ORDER BY updated_at DESC"),{"user":principal.user_id});return success([kb_data(row) for row in result.mappings()])
@router.get("/knowledge-bases/{kb_id}")
async def get_kb(kb_id:str,principal:CurrentUser,session:Session)->Any:return success(kb_data(await owned_kb(session,principal.user_id,kb_id)))
@router.patch("/knowledge-bases/{kb_id}")
async def update_kb(body:KBRequest,kb_id:str,principal:CurrentUser,session:Session)->Any:
    row=await owned_kb(session,principal.user_id,kb_id)
    result=await session.execute(text("""UPDATE ai.knowledge_bases SET name=:name,description=:description,embedding_provider=:provider,embedding_model=:model,chunk_size=:size,chunk_overlap=:overlap,top_k=:top,similarity_threshold=:threshold,retrieval_mode=:mode,vector_weight=:weight,rrf_k=:rrf
      WHERE id=CAST(:id AS uuid) RETURNING id::text,name,description,embedding_provider,embedding_model,chunk_size,chunk_overlap,top_k,similarity_threshold,retrieval_mode,vector_weight,rrf_k,created_at,updated_at"""),
      {"id":row["id"],"name":validate_name(body.name,100),"description":body.description,"provider":body.embedding_provider,"model":body.embedding_model,"size":body.chunk_size,"overlap":body.chunk_overlap,"top":body.top_k,"threshold":body.similarity_threshold,"mode":body.retrieval_mode,"weight":body.vector_weight,"rrf":body.rrf_k})
    await session.commit();return success(kb_data(result.mappings().one()))
@router.delete("/knowledge-bases/{kb_id}")
async def delete_kb(kb_id:str,principal:CurrentUser,session:Session)->Any:
    row=await owned_kb(session,principal.user_id,kb_id);await session.execute(text("DELETE FROM ai.knowledge_bases WHERE id=CAST(:id AS uuid)"),{"id":row["id"]});await session.commit();return success({"success":True})

async def ingest(app:Any,document_id:str,kb:dict[str,Any],data:bytes,filename:str,mime_type:str)->None:
    async with app.state.database.sessions() as session:
        try:
            content=parse_document(data,filename,mime_type);parts=chunks(content,kb["chunk_size"],kb["chunk_overlap"])
            if parts:
                try:
                    vectors=await app.state.embeddings.embed(parts,kb["embedding_provider"],kb["embedding_model"])
                except Exception:
                    vectors=[None] * len(parts)
            else:
                vectors=[]
            for index,(part,vector) in enumerate(zip(parts,vectors,strict=True)):
                await session.execute(text("""INSERT INTO ai.document_chunks(document_id,knowledge_base_id,chunk_index,content,embedding)
                  VALUES(CAST(:document AS uuid),CAST(:kb AS uuid),:index,:content,CAST(:embedding AS vector))"""),{"document":document_id,"kb":kb["id"],"index":index,"content":part,"embedding":None if vector is None else "["+",".join(map(str,vector))+"]"})
            await session.execute(text("UPDATE ai.documents SET status='completed' WHERE id=CAST(:id AS uuid)"),{"id":document_id});await session.commit()
        except Exception as exc:
            await session.rollback();await session.execute(text("UPDATE ai.documents SET status='failed',error=:error WHERE id=CAST(:id AS uuid)"),{"id":document_id,"error":str(exc)[:1000]});await session.commit()
@router.post("/documents/upload",status_code=201)
async def upload(background:BackgroundTasks,request:Request,principal:CurrentUser,session:Session,knowledge_base_id:str=Form(alias="knowledgeBaseId"),file:UploadFile=File(...))->Any:
    kb=dict(await owned_kb(session,principal.user_id,knowledge_base_id));data=await file.read()
    result=await session.execute(text("""INSERT INTO ai.documents(knowledge_base_id,name,mime_type,size) VALUES(CAST(:kb AS uuid),:name,:mime,:size)
      RETURNING id::text,knowledge_base_id::text,name,mime_type,size,status,error,created_at,updated_at"""),{"kb":kb["id"],"name":file.filename or "document","mime":file.content_type or "application/octet-stream","size":len(data)})
    await session.commit();row=result.mappings().one();background.add_task(ingest,request.app,row["id"],kb,data,row["name"],row["mime_type"]);return success(dict(row),status_code=201)
@router.get("/documents/{document_id}/chunks")
async def document_chunks(document_id:str,principal:CurrentUser,session:Session)->Any:
    result=await session.execute(text("""SELECT c.id::text,c.chunk_index,c.content,c.metadata,c.created_at FROM ai.document_chunks c JOIN ai.documents d ON d.id=c.document_id JOIN ai.knowledge_bases k ON k.id=d.knowledge_base_id
      WHERE d.id=CAST(:id AS uuid) AND k.user_id=CAST(:user AS uuid) ORDER BY c.chunk_index"""),{"id":require_uuid(document_id,"documentId"),"user":principal.user_id});return success([dict(row) for row in result.mappings()])
@router.delete("/documents/{document_id}")
async def delete_document(document_id:str,principal:CurrentUser,session:Session)->Any:
    result=await session.execute(text("""DELETE FROM ai.documents d USING ai.knowledge_bases k WHERE d.knowledge_base_id=k.id AND d.id=CAST(:id AS uuid) AND k.user_id=CAST(:user AS uuid)"""),{"id":require_uuid(document_id,"documentId"),"user":principal.user_id})
    if result.rowcount==0:raise APIError(404,"NOT_FOUND","Document not found")
    await session.commit();return success({"success":True})
@router.post("/retrieve",status_code=201)
async def retrieve(body:RetrieveRequest,principal:CurrentUser,session:Session,request:Request)->Any:
    kb=await owned_kb(session,principal.user_id,body.knowledge_base_id);results=await request.app.state.retrieval.retrieve(session,kb["id"],body.query,body.top_k);return success({"query":body.query,"results":results},status_code=201)

from __future__ import annotations
from typing import Any
from rank_bm25 import BM25Okapi
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession
from aiflow_runtime.workflow.executors import NodeResult

class RetrievalService:
    def __init__(self,sessions:Any)->None:self.sessions=sessions
    async def retrieve(self,session:AsyncSession,kb_id:str,query:str,top_k:int=5)->list[dict[str,Any]]:
        result=await session.execute(text("""SELECT id::text,document_id::text,chunk_index,content,metadata,
            ts_rank_cd(search_vector,plainto_tsquery('simple',:query)) AS fts_score FROM ai.document_chunks
            WHERE knowledge_base_id=CAST(:kb AS uuid) ORDER BY fts_score DESC LIMIT 500"""),{"kb":kb_id,"query":query})
        rows=[dict(row) for row in result.mappings()]
        if not rows:return []
        bm25=BM25Okapi([row["content"].lower().split() for row in rows]);scores=bm25.get_scores(query.lower().split())
        fts_rank=sorted(range(len(rows)),key=lambda i:rows[i]["fts_score"],reverse=True);bm_rank=sorted(range(len(rows)),key=lambda i:scores[i],reverse=True)
        fused:dict[int,float]={}
        for rank,index in enumerate(fts_rank):fused[index]=fused.get(index,0)+.5/(60+rank+1)
        for rank,index in enumerate(bm_rank):fused[index]=fused.get(index,0)+.5/(60+rank+1)
        output=[]
        for index,score in sorted(fused.items(),key=lambda item:item[1],reverse=True)[:top_k]:output.append({**rows[index],"score":score,"bm25Score":float(scores[index])})
        return output
    async def execute_node(self,node:dict[str,Any],context:dict[str,Any],inputs:dict[str,Any])->NodeResult:
        data=node.get("data") or {};query=str(data.get("query") or inputs.get("query") or inputs.get("input") or "")
        async with self.sessions() as session:results=await self.retrieve(session,str(data["knowledgeBaseId"]),query,int(data.get("topK",5)))
        return NodeResult({"query":query,"results":results,"content":"\n\n".join(item["content"] for item in results)})

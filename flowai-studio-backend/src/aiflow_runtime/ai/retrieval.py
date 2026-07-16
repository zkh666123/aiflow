from __future__ import annotations
import math
from collections import Counter
from typing import Any
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
        documents=[row["content"].lower().split() for row in rows]
        terms=query.lower().split();average_length=sum(map(len,documents))/len(documents);frequencies=[Counter(document) for document in documents]
        document_frequency={term:sum(1 for frequency in frequencies if term in frequency) for term in set(terms)}
        scores=[]
        for document,frequency in zip(documents,frequencies,strict=True):
            score=0.0
            for term in terms:
                count=frequency.get(term,0);idf=math.log(1+(len(documents)-document_frequency.get(term,0)+.5)/(document_frequency.get(term,0)+.5))
                score+=idf*(count*2.5)/(count+1.5*(1-.75+.75*len(document)/max(average_length,1))) if count else 0
            scores.append(score)
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

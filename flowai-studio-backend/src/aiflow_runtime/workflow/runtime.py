from __future__ import annotations

import asyncio
import json
import time
from collections import deque
from collections.abc import AsyncIterator
from typing import Any

from redis.asyncio import Redis
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.workflow.events import event, now_ms, progress
from aiflow_runtime.workflow.executors import ExternalNodeServices, execute_node
from aiflow_runtime.workflow.graph import PreparedGraph, prepare_graph
from aiflow_runtime.workflow.schemas import ExecutionControl
from aiflow_runtime.workflow.state import ExecutionState


class WorkflowRuntime:
    def __init__(self, session: AsyncSession, redis: Redis, services: ExternalNodeServices) -> None:
        self.session=session; self.redis=redis; self.services=services

    async def events(
        self, workflow: Any, user_id: str, inputs: dict[str,Any], control: ExecutionControl,
    ) -> AsyncIterator[dict[str,Any]]:
        graph=prepare_graph(workflow["nodes"],workflow["edges"])
        execution_id,trace_id=await self._create_records(workflow["id"],user_id,inputs)
        state=ExecutionState(self.redis,execution_id,user_id); await state.start(workflow["id"])
        started=time.monotonic(); total=len(graph.nodes); executed=skipped=failed=beat=0; current_node: str|None=None
        context: dict[str,Any]={}
        control_data=control.model_dump(by_alias=True)
        yield event("workflow_start",{"executionId":execution_id,"totalNodes":total,"control":control_data})
        remaining={node_id:{edge["source"] for edge in graph.incoming.get(node_id,[])} for node_id in graph.nodes}
        ready=deque(node_id for node_id,parents in remaining.items() if not parents); finished:set[str]=set()

        def stats()->dict[str,Any]:
            return {"executed":executed,"skipped":skipped,"failed":failed,"total":total,"durationMs":int((time.monotonic()-started)*1000)}

        def advance(node_id:str)->None:
            for edge in graph.outgoing.get(node_id,[]):
                target=edge["target"]; remaining[target].discard(node_id)
                if not remaining[target] and target not in finished and target not in ready: ready.append(target)

        try:
            while ready:
                if control.workflow_timeout_ms and (time.monotonic()-started)*1000 >= control.workflow_timeout_ms:
                    raise TimeoutError("Workflow execution timed out")
                if await state.cancelled(): raise asyncio.CancelledError("Execution cancellation requested")
                node_id=ready.popleft()
                if node_id in finished: continue
                current_node=node_id; node=graph.nodes[node_id]
                await state.update("running",node_id,context)
                yield event("node_status",{"nodeId":node_id,"status":"running","progress":progress(executed,total,node_id)})
                node_started=time.monotonic(); result=None; last_error:Exception|None=None
                for attempt in range(control.max_retries+1):
                    if attempt:
                        delay_ms=1000*(2**(attempt-1))
                        yield event("node_status",{"nodeId":node_id,"status":"retrying","attempt":attempt,"delayMs":delay_ms,"error":str(last_error)})
                        await asyncio.sleep(delay_ms/1000)
                    timeout_limits = [value for value in (
                        control.node_timeout_ms,
                        control.workflow_timeout_ms-int((time.monotonic()-started)*1000) if control.workflow_timeout_ms else 0,
                    ) if value > 0]
                    timeout_seconds=min(timeout_limits)/1000 if timeout_limits else None
                    task=asyncio.create_task(asyncio.wait_for(execute_node(node,context,inputs,self.services),timeout_seconds))
                    try:
                        heartbeat_seconds=control.heartbeat_interval_ms/1000 if control.heartbeat_interval_ms else None
                        while heartbeat_seconds:
                            done,_=await asyncio.wait({task},timeout=heartbeat_seconds)
                            if done: break
                            beat+=1
                            yield event("heartbeat",{"beat":beat,"elapsedMs":int((time.monotonic()-started)*1000),"timestamp":now_ms(),"progress":progress(executed,total,node_id)})
                            if await state.cancelled(): task.cancel(); raise asyncio.CancelledError("Execution cancellation requested")
                        result=await task; last_error=None; break
                    except Exception as exc:
                        last_error=exc
                        if attempt>=control.max_retries: break
                if last_error is not None:
                    failed+=1; finished.add(node_id)
                    timeout=isinstance(last_error,(TimeoutError,asyncio.TimeoutError))
                    yield event("node_status",{"nodeId":node_id,"status":"timeout" if timeout else "failed","error":str(last_error) or "Node execution failed"})
                    await self._span(trace_id,node_id,node["type"],"failed",None,str(last_error),node_started)
                    if not control.continue_on_error: raise RuntimeError(f"Error executing node {node_id}: {last_error}")
                    advance(node_id); continue
                assert result is not None
                for trace in result.traces:
                    yield event("agent_trace",{"nodeId":node_id,"trace":trace})
                context[node_id]=result.output; executed+=1; finished.add(node_id)
                duration=int((time.monotonic()-node_started)*1000)
                await self._span(trace_id,node_id,node["type"],"success",result.output,None,node_started)
                yield event("node_status",{"nodeId":node_id,"status":"success","output":result.output,"durationMs":duration,"progress":progress(executed,total,node_id)})
                if node["type"]=="condition" and result.branch is not None:
                    for pruned_id in graph.branch_prune(node_id,result.branch):
                        if pruned_id in finished: continue
                        finished.add(pruned_id); skipped+=1
                        yield event("node_status",{"nodeId":pruned_id,"status":"skipped","progress":progress(executed,total,pruned_id)})
                        advance(pruned_id)
                advance(node_id)
            final_stats=stats(); await state.finish("success")
            await self._finish_records(execution_id,trace_id,"success",context,None,final_stats["durationMs"])
            yield event("done",{"finalContext":context,"stats":final_stats})
        except asyncio.CancelledError as exc:
            final_stats=stats(); await state.finish("cancelled")
            await self._finish_records(execution_id,trace_id,"cancelled",context,str(exc),final_stats["durationMs"])
            yield event("error",{"message":str(exc),"nodeId":current_node,"isTimeout":False,"isCancelled":True,"stats":final_stats})
        except Exception as exc:
            final_stats=stats(); await state.finish("failed")
            await self._finish_records(execution_id,trace_id,"failed",context,str(exc),final_stats["durationMs"])
            yield event("error",{"message":str(exc),"nodeId":current_node,"isTimeout":isinstance(exc,(TimeoutError,asyncio.TimeoutError)),"isCancelled":False,"stats":final_stats})

    async def _create_records(self,workflow_id:str,user_id:str,inputs:dict[str,Any])->tuple[str,str]:
        execution=await self.session.execute(text("""INSERT INTO control.workflow_executions(workflow_id,user_id,status,inputs)
            VALUES(CAST(:workflow AS uuid),CAST(:user AS uuid),'running',CAST(:inputs AS jsonb)) RETURNING id::text"""),
            {"workflow":workflow_id,"user":user_id,"inputs":json.dumps(inputs)})
        execution_id=execution.scalar_one()
        trace=await self.session.execute(text("""INSERT INTO control.traces(workflow_id,execution_id,user_id,status,input)
            VALUES(CAST(:workflow AS uuid),CAST(:execution AS uuid),CAST(:user AS uuid),'running',CAST(:inputs AS jsonb)) RETURNING id::text"""),
            {"workflow":workflow_id,"execution":execution_id,"user":user_id,"inputs":json.dumps(inputs)})
        await self.session.commit(); return execution_id,trace.scalar_one()

    async def _span(self,trace_id:str,node_id:str,kind:str,status:str,output:Any,error:str|None,started:float)->None:
        await self.session.execute(text("""INSERT INTO control.spans(trace_id,node_id,name,kind,status,output,error,started_at,completed_at,duration_ms)
            VALUES(CAST(:trace AS uuid),:node,:name,:kind,:status,CAST(:output AS jsonb),:error,now()-(CAST(:duration AS text)||' milliseconds')::interval,now(),:duration)"""),
            {"trace":trace_id,"node":node_id,"name":node_id,"kind":kind,"status":status,"output":json.dumps(output,default=str) if output is not None else None,
             "error":error,"duration":int((time.monotonic()-started)*1000)})
        await self.session.commit()

    async def _finish_records(self,execution_id:str,trace_id:str,status:str,context:dict[str,Any],error:str|None,duration:int)->None:
        values={"execution":execution_id,"trace":trace_id,"status":status,"context":json.dumps(context,default=str),"error":error,"duration":duration}
        await self.session.execute(text("""UPDATE control.workflow_executions SET status=:status,context=CAST(:context AS jsonb),error=:error,duration_ms=:duration,completed_at=now()
            WHERE id=CAST(:execution AS uuid)"""),values)
        await self.session.execute(text("""UPDATE control.traces SET status=:status,output=CAST(:context AS jsonb),error=:error,duration_ms=:duration,completed_at=now()
            WHERE id=CAST(:trace AS uuid)"""),values); await self.session.commit()

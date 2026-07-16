from __future__ import annotations

from typing import Any,TypedDict

from langgraph.graph import END,START,StateGraph

from aiflow_runtime.ai.providers import ProviderRouter


class AgentState(TypedDict,total=False):
    messages:list[dict[str,str]];result:str


class AgentService:
    def __init__(self,providers:ProviderRouter)->None:self.providers=providers
    async def run(self,data:dict[str,Any],prompt:str)->tuple[str,list[dict[str,Any]]]:
        model=str(data.get("model") or "llama3.2")
        async def respond(state:AgentState)->AgentState:
            result=await self.providers.complete(model,state["messages"],float(data.get("temperature",.7)),int(data.get("maxTokens",1024)))
            return {"result":result.content}
        graph=StateGraph(AgentState);graph.add_node("respond",respond);graph.add_edge(START,"respond");graph.add_edge("respond",END)
        messages=[{"role":"system","content":str(data.get("systemPrompt") or "You are a helpful agent.")},{"role":"user","content":prompt}]
        output=await graph.compile().ainvoke({"messages":messages})
        timestamp=__import__("time").time_ns()//1_000_000
        agent_id="supervisor" if data.get("agentMode")=="supervisor" else "single_agent"
        traces=[{"type":"thinking","content":"Selecting the next action","agentId":agent_id,"timestamp":timestamp}]
        if agent_id=="supervisor":
            worker=(data.get("workers") or [{"id":"default","name":"Default worker"}])[0]
            traces.extend([{"type":"worker_delegate","content":f"Delegating to {worker.get('name','worker')}","agentId":agent_id,"timestamp":timestamp},
                           {"type":"worker_result","content":output["result"],"agentId":str(worker.get("id","default")),"timestamp":timestamp}])
        traces.append({"type":"final_answer","content":output["result"],"agentId":agent_id,"timestamp":timestamp})
        return output["result"],traces

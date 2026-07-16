from __future__ import annotations

from typing import Any

from aiflow_runtime.ai.providers import ProviderRouter


class AgentService:
    def __init__(self,providers:ProviderRouter)->None:self.providers=providers
    async def run(self,data:dict[str,Any],prompt:str)->tuple[str,list[dict[str,Any]]]:
        model=str(data.get("model") or "llama3.2")
        messages=[{"role":"system","content":str(data.get("systemPrompt") or "You are a helpful agent.")},{"role":"user","content":prompt}]
        response=await self.providers.complete(model,messages,float(data.get("temperature",.7)),int(data.get("maxTokens",1024)))
        output={"result":response.content}
        timestamp=__import__("time").time_ns()//1_000_000
        agent_id="supervisor" if data.get("agentMode")=="supervisor" else "single_agent"
        traces=[{"type":"thinking","content":"Selecting the next action","agentId":agent_id,"timestamp":timestamp}]
        if agent_id=="supervisor":
            worker=(data.get("workers") or [{"id":"default","name":"Default worker"}])[0]
            traces.extend([{"type":"worker_delegate","content":f"Delegating to {worker.get('name','worker')}","agentId":agent_id,"timestamp":timestamp},
                           {"type":"worker_result","content":output["result"],"agentId":str(worker.get("id","default")),"timestamp":timestamp}])
        traces.append({"type":"final_answer","content":output["result"],"agentId":agent_id,"timestamp":timestamp})
        return output["result"],traces

from __future__ import annotations

from typing import Any

from aiflow_runtime.ai.agent import AgentService
from aiflow_runtime.ai.providers import ProviderRouter
from aiflow_runtime.ai.token_usage import TokenUsageBuffer,UsageEvent
from aiflow_runtime.workflow.executors import NodeResult


class AIExecutionServices:
    def __init__(self,providers:ProviderRouter,usage:TokenUsageBuffer)->None:self.providers=providers;self.usage=usage;self.agent=AgentService(providers);self.rag:Any=None;self.skills:Any=None
    async def execute(self,node_type:str,node:dict[str,Any],context:dict[str,Any],inputs:dict[str,Any])->NodeResult:
        data=node.get("data") or {}
        prompt=str(data.get("userPrompt") or data.get("query") or inputs.get("input") or "")
        if node_type=="llm":
            result=await self.providers.complete(str(data.get("model") or "llama3.2"),[{"role":"system","content":str(data.get("systemPrompt") or "")},{"role":"user","content":prompt}],float(data.get("temperature",.7)),int(data.get("maxTokens",1024)))
            await self.usage.add(UsageEvent(None,result.provider,result.model,result.prompt_tokens,result.completion_tokens,node_id=node["id"]))
            return NodeResult({"text":result.content,"model":result.model})
        if node_type=="agent":
            content,traces=await self.agent.run(data,prompt);return NodeResult({"text":content},traces=traces)
        if node_type=="rag" and self.rag:return await self.rag.execute_node(node,context,inputs)
        if node_type=="skill" and self.skills:return await self.skills.execute_node(node,context,inputs)
        raise RuntimeError(f"{node_type} service is not configured")

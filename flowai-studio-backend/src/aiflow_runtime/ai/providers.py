from __future__ import annotations

from dataclasses import dataclass
from typing import Any

import httpx

from aiflow_runtime.config import Settings


@dataclass(slots=True)
class ModelResult:
    content:str; provider:str; model:str; prompt_tokens:int=0; completion_tokens:int=0


MODELS=[
    {"id":"gpt-4o-mini","name":"GPT-4o Mini","provider":"openai","capabilities":["chat","tools"],"inputCost":0.15,"outputCost":0.6},
    {"id":"claude-3-5-sonnet-latest","name":"Claude 3.5 Sonnet","provider":"anthropic","capabilities":["chat","tools"],"inputCost":3.0,"outputCost":15.0},
    {"id":"gemini-2.0-flash","name":"Gemini 2.0 Flash","provider":"gemini","capabilities":["chat"],"inputCost":0.1,"outputCost":0.4},
    {"id":"qwen-plus","name":"Qwen Plus","provider":"qwen","capabilities":["chat","tools"],"inputCost":0.4,"outputCost":1.2},
    {"id":"llama3.2","name":"Llama 3.2","provider":"ollama","capabilities":["chat"],"inputCost":0.0,"outputCost":0.0},
]


class ProviderRouter:
    def __init__(self,settings:Settings,client:httpx.AsyncClient)->None:self.settings=settings;self.client=client

    def model(self,model_id:str)->dict[str,Any]:
        return next((item for item in MODELS if item["id"]==model_id),{"id":model_id,"name":model_id,"provider":"ollama","capabilities":["chat"],"inputCost":0.0,"outputCost":0.0})

    async def complete(self,model_id:str,messages:list[dict[str,str]],temperature:float=.7,max_tokens:int=1024)->ModelResult:
        info=self.model(model_id); provider=info["provider"]
        if provider in {"openai","qwen"}:
            key=self.settings.openai_api_key if provider=="openai" else self.settings.qwen_api_key
            if key is None: raise RuntimeError(f"{provider} API key is not configured")
            base="https://api.openai.com/v1" if provider=="openai" else "https://dashscope.aliyuncs.com/compatible-mode/v1"
            response=await self.client.post(f"{base}/chat/completions",headers={"Authorization":f"Bearer {key.get_secret_value()}"},json={"model":model_id,"messages":messages,"temperature":temperature,"max_tokens":max_tokens})
            response.raise_for_status(); data=response.json(); usage=data.get("usage") or {}
            return ModelResult(data["choices"][0]["message"]["content"],provider,model_id,usage.get("prompt_tokens",0),usage.get("completion_tokens",0))
        if provider=="anthropic":
            key=self.settings.anthropic_api_key
            if key is None: raise RuntimeError("anthropic API key is not configured")
            system="\n".join(message["content"] for message in messages if message["role"]=="system")
            body=[message for message in messages if message["role"]!="system"]
            response=await self.client.post("https://api.anthropic.com/v1/messages",headers={"x-api-key":key.get_secret_value(),"anthropic-version":"2023-06-01"},json={"model":model_id,"system":system,"messages":body,"temperature":temperature,"max_tokens":max_tokens})
            response.raise_for_status(); data=response.json(); usage=data.get("usage") or {}
            return ModelResult("".join(item.get("text","") for item in data.get("content",[])),provider,model_id,usage.get("input_tokens",0),usage.get("output_tokens",0))
        if provider=="gemini":
            key=self.settings.gemini_api_key
            if key is None: raise RuntimeError("Gemini API key is not configured")
            contents=[{"role":"model" if m["role"]=="assistant" else "user","parts":[{"text":m["content"]}]} for m in messages]
            response=await self.client.post(f"https://generativelanguage.googleapis.com/v1beta/models/{model_id}:generateContent",params={"key":key.get_secret_value()},json={"contents":contents,"generationConfig":{"temperature":temperature,"maxOutputTokens":max_tokens}})
            response.raise_for_status(); data=response.json(); usage=data.get("usageMetadata") or {}
            content=data["candidates"][0]["content"]["parts"][0]["text"]
            return ModelResult(content,provider,model_id,usage.get("promptTokenCount",0),usage.get("candidatesTokenCount",0))
        response=await self.client.post(f"{self.settings.ollama_base_url}/api/chat",json={"model":model_id,"messages":messages,"stream":False,"options":{"temperature":temperature,"num_predict":max_tokens}})
        response.raise_for_status(); data=response.json()
        return ModelResult(data["message"]["content"],"ollama",model_id,data.get("prompt_eval_count",0),data.get("eval_count",0))

    async def discover_ollama(self)->list[dict[str,Any]]:
        response=await self.client.get(f"{self.settings.ollama_base_url}/api/tags"); response.raise_for_status()
        return response.json().get("models",[])

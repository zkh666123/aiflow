from __future__ import annotations
from typing import Any
from aiflow_runtime.ai.providers import ProviderRouter

class Embeddings:
    def __init__(self,providers:ProviderRouter)->None:self.providers=providers
    async def embed(self,texts:list[str],provider:str,model:str)->list[list[float]]:
        client=self.providers.client;settings=self.providers.settings
        if provider in {"openai","qwen"}:
            key=settings.openai_api_key if provider=="openai" else settings.qwen_api_key
            if key is None:raise RuntimeError(f"{provider} API key is not configured")
            base="https://api.openai.com/v1" if provider=="openai" else "https://dashscope.aliyuncs.com/compatible-mode/v1"
            response=await client.post(f"{base}/embeddings",headers={"Authorization":f"Bearer {key.get_secret_value()}"},json={"model":model,"input":texts});response.raise_for_status()
            return [item["embedding"] for item in response.json()["data"]]
        values=[]
        for text in texts:
            response=await client.post(f"{settings.ollama_base_url}/api/embeddings",json={"model":model,"prompt":text});response.raise_for_status();values.append(response.json()["embedding"])
        return values

from __future__ import annotations

import json
from typing import Any

from redis.asyncio import Redis


class ExecutionState:
    def __init__(self, redis: Redis, execution_id: str, user_id: str) -> None:
        self.redis=redis; self.execution_id=execution_id; self.user_id=user_id
        self.key=f"flowai:execution:{execution_id}"; self.cancel_key=f"{self.key}:cancel"; self.running_key=f"flowai:running:{user_id}"

    async def start(self, workflow_id: str) -> None:
        await self.redis.hset(self.key,mapping={"workflowId":workflow_id,"status":"running"})
        await self.redis.expire(self.key,86400); await self.redis.sadd(self.running_key,self.execution_id); await self.redis.expire(self.running_key,86400)

    async def update(self, status: str, node_id: str | None = None, context: dict[str,Any] | None = None) -> None:
        values={"status":status}
        if node_id is not None: values["nodeId"]=node_id
        if context is not None: values["context"]=json.dumps(context,default=str)
        await self.redis.hset(self.key,mapping=values); await self.redis.expire(self.key,86400)

    async def cancel(self) -> None:
        await self.redis.set(self.cancel_key,"1",ex=86400)

    async def cancelled(self) -> bool:
        return bool(await self.redis.exists(self.cancel_key))

    async def finish(self, status: str) -> None:
        await self.update(status); await self.redis.srem(self.running_key,self.execution_id)

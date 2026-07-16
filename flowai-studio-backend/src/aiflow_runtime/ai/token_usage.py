from __future__ import annotations

import asyncio
from dataclasses import asdict,dataclass
from decimal import Decimal

from sqlalchemy import text
from sqlalchemy.ext.asyncio import async_sessionmaker,AsyncSession


@dataclass(slots=True)
class UsageEvent:
    user_id:str|None;provider:str;model:str;prompt_tokens:int;completion_tokens:int;cost:Decimal=Decimal("0");workflow_id:str|None=None;execution_id:str|None=None;node_id:str|None=None


class TokenUsageBuffer:
    def __init__(self,sessions:async_sessionmaker[AsyncSession])->None:self.sessions=sessions;self.items:list[UsageEvent]=[];self.lock=asyncio.Lock();self.task:asyncio.Task[None]|None=None
    def start(self)->None:self.task=asyncio.create_task(self._loop())
    async def add(self,item:UsageEvent)->None:
        async with self.lock:
            self.items.append(item); flush=len(self.items)>=100
        if flush: await self.flush()
    async def flush(self)->None:
        async with self.lock: items,self.items=self.items,[]
        if not items:return
        async with self.sessions() as session:
            for item in items:
                await session.execute(text("""INSERT INTO ai.token_usage(user_id,workflow_id,execution_id,node_id,provider,model,prompt_tokens,completion_tokens,total_tokens,cost)
                    VALUES(CAST(:user_id AS uuid),CAST(:workflow_id AS uuid),CAST(:execution_id AS uuid),:node_id,:provider,:model,:prompt_tokens,:completion_tokens,:total_tokens,:cost)"""),
                    {**asdict(item),"total_tokens":item.prompt_tokens+item.completion_tokens})
            await session.commit()
    async def _loop(self)->None:
        try:
            while True: await asyncio.sleep(10);await self.flush()
        except asyncio.CancelledError:return
    async def close(self)->None:
        if self.task:self.task.cancel()
        await self.flush()

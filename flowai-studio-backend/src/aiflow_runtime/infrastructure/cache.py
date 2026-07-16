from __future__ import annotations
import json,random,time
from collections import OrderedDict
from typing import Any
from redis.asyncio import Redis
class LayeredCache:
    def __init__(self,redis:Redis,capacity:int=256)->None:self.redis=redis;self.capacity=capacity;self.local:OrderedDict[str,tuple[float,Any]]=OrderedDict();self.hits=0;self.misses=0
    async def get(self,key:str)->Any:
        item=self.local.get(key)
        if item and item[0]>time.time():self.hits+=1;self.local.move_to_end(key);return item[1]
        raw=await self.redis.get(f"flowai:cache:{key}")
        if raw is None:self.misses+=1;return None
        self.hits+=1;value=json.loads(raw);self.local[key]=(time.time()+30,value);return value
    async def set(self,key:str,value:Any,ttl:int=300)->None:
        effective=max(1,int(ttl*(.9+random.random()*.2)));self.local[key]=(time.time()+min(30,effective),value);self.local.move_to_end(key)
        while len(self.local)>self.capacity:self.local.popitem(last=False)
        await self.redis.set(f"flowai:cache:{key}",json.dumps(value,default=str),ex=effective)
    def stats(self)->dict[str,int]:return {"l1Size":len(self.local),"l1Capacity":self.capacity,"hits":self.hits,"misses":self.misses}

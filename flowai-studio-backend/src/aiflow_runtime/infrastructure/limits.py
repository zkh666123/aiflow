from __future__ import annotations
import time
from redis.asyncio import Redis
TOKEN_BUCKET="""local tokens=tonumber(redis.call('HGET',KEYS[1],'tokens') or ARGV[1]);local last=tonumber(redis.call('HGET',KEYS[1],'last') or ARGV[3]);local now=tonumber(ARGV[3]);tokens=math.min(tonumber(ARGV[1]),tokens+(now-last)*tonumber(ARGV[2]));local allowed=0;if tokens>=1 then tokens=tokens-1;allowed=1 end;redis.call('HSET',KEYS[1],'tokens',tokens,'last',now);redis.call('EXPIRE',KEYS[1],tonumber(ARGV[4]));return {allowed,math.floor(tokens)}"""
class RuntimeLimits:
    def __init__(self,redis:Redis)->None:self.redis=redis
    async def token(self,key:str,capacity:int,rate:float,ttl:int=3600)->tuple[bool,int]:
        allowed,remaining=await self.redis.eval(TOKEN_BUCKET,1,key,capacity,rate,time.time(),ttl);return bool(allowed),int(remaining)
    async def circuit(self,name:str)->dict[str,object]:
        values=await self.redis.hgetall(f"flowai:circuit:{name}");return {"state":values.get("state","closed"),"failures":int(values.get("failures",0)),"openedAt":values.get("openedAt")}
    async def reset(self,name:str)->None:await self.redis.delete(f"flowai:circuit:{name}")

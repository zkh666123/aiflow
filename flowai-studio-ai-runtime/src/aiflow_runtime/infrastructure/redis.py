from __future__ import annotations

from fastapi import Request
from redis.asyncio import Redis


def create_redis(url: str) -> Redis:
    return Redis.from_url(url, decode_responses=True)


def get_redis(request: Request) -> Redis:
    return request.app.state.redis

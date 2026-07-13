from __future__ import annotations

import asyncio

import grpc
import redis.asyncio as redis
from google.protobuf import struct_pb2
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncEngine, create_async_engine

from aiflow.v1 import (
    common_pb2,
    documents_pb2_grpc,
    execution_pb2_grpc,
    mcp_pb2_grpc,
    models_pb2_grpc,
    retrieval_pb2_grpc,
)
from aiflow_runtime.config import Settings
from aiflow_runtime.grpc.auth import ServiceTokenInterceptor
from aiflow_runtime.grpc.model_service import HealthCheck, ModelService
from aiflow_runtime.grpc.unimplemented import (
    DocumentService,
    ExecutionService,
    McpService,
    RetrievalService,
)


def _component(
    name: str,
    version: str = "",
    details: dict[str, str] | None = None,
) -> common_pb2.HealthComponent:
    detail_message = struct_pb2.Struct()
    if details:
        detail_message.update(details)
    return common_pb2.HealthComponent(
        name=name,
        state=common_pb2.HEALTH_STATE_HEALTHY,
        version=version,
        details=detail_message,
    )


def database_check(engine: AsyncEngine) -> HealthCheck:
    async def check() -> common_pb2.HealthComponent:
        async with engine.connect() as connection:
            await connection.execute(text("SELECT 1"))
        return _component("database")

    return check


def pgvector_check(engine: AsyncEngine) -> HealthCheck:
    async def check() -> common_pb2.HealthComponent:
        async with engine.connect() as connection:
            result = await connection.execute(
                text("SELECT extversion::text FROM pg_extension WHERE extname = 'vector'")
            )
            version = result.scalar_one()
        return _component("pgvector", version=version)

    return check


def redis_check(client: redis.Redis) -> HealthCheck:
    async def check() -> common_pb2.HealthComponent:
        if not await client.ping():
            raise RuntimeError("Redis ping failed")
        return _component("redis")

    return check


def create_server(
    settings: Settings,
    checks: list[HealthCheck],
) -> grpc.aio.Server:
    server = grpc.aio.server(
        interceptors=[
            ServiceTokenInterceptor(settings.grpc_token.get_secret_value()),
        ]
    )
    execution_pb2_grpc.add_ExecutionServiceServicer_to_server(
        ExecutionService(), server
    )
    documents_pb2_grpc.add_DocumentServiceServicer_to_server(DocumentService(), server)
    retrieval_pb2_grpc.add_RetrievalServiceServicer_to_server(
        RetrievalService(), server
    )
    mcp_pb2_grpc.add_McpServiceServicer_to_server(McpService(), server)
    models_pb2_grpc.add_ModelServiceServicer_to_server(
        ModelService(checks, settings.health_timeout_seconds), server
    )
    if server.add_insecure_port(settings.grpc_address) == 0:
        raise RuntimeError(f"unable to bind AI runtime to {settings.grpc_address}")
    return server


async def serve() -> None:
    settings = Settings()
    engine = create_async_engine(settings.database_url, pool_pre_ping=True)
    redis_client = redis.from_url(settings.redis_url, decode_responses=True)
    checks = [
        database_check(engine),
        pgvector_check(engine),
        redis_check(redis_client),
    ]
    server = create_server(settings, checks)

    try:
        await server.start()
        await server.wait_for_termination()
    finally:
        await server.stop(grace=5)
        await redis_client.aclose()
        await engine.dispose()


if __name__ == "__main__":
    asyncio.run(serve())

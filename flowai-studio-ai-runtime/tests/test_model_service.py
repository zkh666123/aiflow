import asyncio

import pytest

from aiflow.v1 import common_pb2, models_pb2
from aiflow_runtime.grpc.model_service import ModelService


def component(name: str, state: int) -> common_pb2.HealthComponent:
    return common_pb2.HealthComponent(name=name, state=state)


@pytest.mark.asyncio
async def test_health_check_aggregates_dependency_state() -> None:
    async def database() -> common_pb2.HealthComponent:
        return component("database", common_pb2.HEALTH_STATE_HEALTHY)

    async def redis() -> common_pb2.HealthComponent:
        return component("redis", common_pb2.HEALTH_STATE_UNHEALTHY)

    service = ModelService(checks=[database, redis], timeout_seconds=0.1)
    response = await service.HealthCheck(models_pb2.ModelServiceHealthCheckRequest(), None)

    assert response.report.state == common_pb2.HEALTH_STATE_DEGRADED
    assert [item.name for item in response.report.components] == ["database", "redis"]


@pytest.mark.asyncio
async def test_health_check_redacts_dependency_exceptions_and_timeouts() -> None:
    async def leaks_secret() -> common_pb2.HealthComponent:
        raise RuntimeError("postgresql://user:secret@127.0.0.1/db")

    async def hangs() -> common_pb2.HealthComponent:
        await asyncio.sleep(1)
        return component("late", common_pb2.HEALTH_STATE_HEALTHY)

    service = ModelService(checks=[leaks_secret, hangs], timeout_seconds=0.01)
    response = await service.HealthCheck(models_pb2.ModelServiceHealthCheckRequest(), None)
    serialized = response.SerializeToString()

    assert response.report.state == common_pb2.HEALTH_STATE_DEGRADED
    assert all(item.message == "unavailable" for item in response.report.components)
    assert b"secret" not in serialized

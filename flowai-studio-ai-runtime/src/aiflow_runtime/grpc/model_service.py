from __future__ import annotations

import asyncio
from collections.abc import Awaitable, Callable, Sequence

from google.protobuf import timestamp_pb2

from aiflow.v1 import common_pb2, models_pb2, models_pb2_grpc

HealthCheck = Callable[[], Awaitable[common_pb2.HealthComponent]]


class ModelService(models_pb2_grpc.ModelServiceServicer):
    def __init__(
        self,
        checks: Sequence[HealthCheck],
        timeout_seconds: float = 2.0,
    ) -> None:
        self._checks = tuple(checks)
        self._timeout_seconds = timeout_seconds

    async def ListModels(
        self,
        request: models_pb2.ListModelsRequest,
        context: object,
    ) -> models_pb2.ListModelsResponse:
        del request, context
        return models_pb2.ListModelsResponse()

    async def HealthCheck(
        self,
        request: models_pb2.ModelServiceHealthCheckRequest,
        context: object,
    ) -> models_pb2.ModelServiceHealthCheckResponse:
        del request, context
        components = []
        for index, check in enumerate(self._checks, start=1):
            try:
                component = await asyncio.wait_for(check(), self._timeout_seconds)
            except Exception:
                component = common_pb2.HealthComponent(
                    name=f"dependency_{index}",
                    state=common_pb2.HEALTH_STATE_UNHEALTHY,
                    message="unavailable",
                )
            components.append(component)

        healthy = all(
            item.state == common_pb2.HEALTH_STATE_HEALTHY for item in components
        )
        state = (
            common_pb2.HEALTH_STATE_HEALTHY
            if healthy
            else common_pb2.HEALTH_STATE_DEGRADED
        )
        checked_at = timestamp_pb2.Timestamp()
        checked_at.GetCurrentTime()
        report = common_pb2.HealthReport(
            state=state,
            components=components,
            checked_at=checked_at,
        )
        return models_pb2.ModelServiceHealthCheckResponse(report=report)

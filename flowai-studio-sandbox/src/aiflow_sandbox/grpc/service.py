from __future__ import annotations

import grpc
from google.protobuf import timestamp_pb2

from aiflow.v1 import common_pb2, sandbox_pb2, sandbox_pb2_grpc
from aiflow_sandbox.wasi.artifact import WasiArtifact


class SandboxService(sandbox_pb2_grpc.SandboxServiceServicer):
    def __init__(self, artifact: WasiArtifact) -> None:
        self._artifact = artifact

    async def HealthCheck(
        self,
        request: sandbox_pb2.SandboxServiceHealthCheckRequest,
        context: grpc.aio.ServicerContext,
    ) -> sandbox_pb2.SandboxServiceHealthCheckResponse:
        del request, context
        artifact = self._artifact.inspect()
        state = (
            common_pb2.HEALTH_STATE_HEALTHY
            if artifact.ready
            else common_pb2.HEALTH_STATE_NOT_READY
        )
        message = "" if artifact.ready else "runtime unavailable"
        checked_at = timestamp_pb2.Timestamp()
        checked_at.GetCurrentTime()
        report = common_pb2.HealthReport(
            state=state,
            components=[
                common_pb2.HealthComponent(
                    name="wasi_runtime",
                    state=state,
                    message=message,
                )
            ],
            checked_at=checked_at,
        )
        return sandbox_pb2.SandboxServiceHealthCheckResponse(report=report)

    async def ExecutePython(
        self,
        request: sandbox_pb2.ExecutePythonRequest,
        context: grpc.aio.ServicerContext,
    ) -> sandbox_pb2.ExecutePythonResponse:
        del request
        artifact = self._artifact.inspect()
        if not artifact.ready:
            await context.abort(
                grpc.StatusCode.FAILED_PRECONDITION,
                "sandbox runtime is not ready",
            )
        await context.abort(
            grpc.StatusCode.UNIMPLEMENTED,
            "sandbox execution is not implemented",
        )

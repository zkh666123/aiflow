from __future__ import annotations

import grpc
from google.protobuf import timestamp_pb2

from aiflow.v1 import common_pb2, sandbox_pb2, sandbox_pb2_grpc
from aiflow_sandbox.native.runner import NativePythonRunner


class SandboxService(sandbox_pb2_grpc.SandboxServiceServicer):
    def __init__(self, runner: NativePythonRunner) -> None:
        self._runner = runner

    async def HealthCheck(
        self,
        request: sandbox_pb2.SandboxServiceHealthCheckRequest,
        context: grpc.aio.ServicerContext,
    ) -> sandbox_pb2.SandboxServiceHealthCheckResponse:
        del request, context
        state = common_pb2.HEALTH_STATE_HEALTHY if self._runner.ready else common_pb2.HEALTH_STATE_NOT_READY
        message = "" if self._runner.ready else "runtime unavailable"
        checked_at = timestamp_pb2.Timestamp()
        checked_at.GetCurrentTime()
        report = common_pb2.HealthReport(
            state=state,
            components=[
                common_pb2.HealthComponent(
                    name="native_python",
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
        if not self._runner.ready:
            await context.abort(
                grpc.StatusCode.FAILED_PRECONDITION,
                "sandbox runtime is not ready",
            )
        result = await self._runner.execute(request.code, request.limits)
        return sandbox_pb2.ExecutePythonResponse(
            status=result.status,
            failure_code=result.failure_code,
            exit_code=result.exit_code,
            stdout=result.stdout,
            stderr=result.stderr,
            duration_millis=result.duration_millis,
        )

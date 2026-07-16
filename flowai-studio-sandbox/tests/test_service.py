import sys
from pathlib import Path

import grpc
import pytest

from aiflow.v1 import common_pb2, sandbox_pb2, sandbox_pb2_grpc
from aiflow_sandbox.grpc.auth import SERVICE_TOKEN_METADATA_KEY, ServiceTokenInterceptor
from aiflow_sandbox.grpc.service import SandboxService
from aiflow_sandbox.native.runner import NativePythonRunner


@pytest.mark.asyncio
async def test_sandbox_is_authenticated_healthy_and_executes_python() -> None:
    token = "s" * 43
    server = grpc.aio.server(interceptors=[ServiceTokenInterceptor(token)])
    sandbox_pb2_grpc.add_SandboxServiceServicer_to_server(
        SandboxService(NativePythonRunner(Path(sys.executable))), server
    )
    port = server.add_insecure_port("127.0.0.1:0")
    await server.start()

    channel = grpc.aio.insecure_channel(f"127.0.0.1:{port}")
    stub = sandbox_pb2_grpc.SandboxServiceStub(channel)
    metadata = ((SERVICE_TOKEN_METADATA_KEY, token),)

    try:
        with pytest.raises(grpc.aio.AioRpcError) as unauthenticated:
            await stub.HealthCheck(sandbox_pb2.SandboxServiceHealthCheckRequest())
        assert unauthenticated.value.code() == grpc.StatusCode.UNAUTHENTICATED

        health = await stub.HealthCheck(
            sandbox_pb2.SandboxServiceHealthCheckRequest(), metadata=metadata
        )
        assert health.report.state == common_pb2.HEALTH_STATE_HEALTHY
        assert health.report.components[0].name == "native_python"

        response = await stub.ExecutePython(
            sandbox_pb2.ExecutePythonRequest(
                code="print(sum(range(5)))",
                limits=sandbox_pb2.SandboxLimits(timeout_millis=1_000, output_bytes=4_096),
            ),
            metadata=metadata,
        )
        assert response.status == sandbox_pb2.SANDBOX_EXECUTION_STATUS_SUCCEEDED
        assert response.stdout.splitlines() == ["10"]
        assert response.stderr == ""
    finally:
        await channel.close()
        await server.stop(grace=None)

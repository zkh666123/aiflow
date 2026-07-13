import grpc
import pytest

from aiflow.v1 import common_pb2, sandbox_pb2, sandbox_pb2_grpc
from aiflow_sandbox.grpc.auth import SERVICE_TOKEN_METADATA_KEY, ServiceTokenInterceptor
from aiflow_sandbox.grpc.service import SandboxService
from aiflow_sandbox.wasi.artifact import WasiArtifact


@pytest.mark.asyncio
async def test_sandbox_is_authenticated_and_fails_closed_without_wasi(tmp_path) -> None:
    token = "s" * 43
    artifact = WasiArtifact(tmp_path / "python.wasm", "a" * 64)
    server = grpc.aio.server(interceptors=[ServiceTokenInterceptor(token)])
    sandbox_pb2_grpc.add_SandboxServiceServicer_to_server(
        SandboxService(artifact), server
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
        assert health.report.state == common_pb2.HEALTH_STATE_NOT_READY
        assert health.report.components[0].name == "wasi_runtime"
        assert "python.wasm" not in health.report.components[0].message

        with pytest.raises(grpc.aio.AioRpcError) as unavailable:
            await stub.ExecutePython(
                sandbox_pb2.ExecutePythonRequest(code="print('unsafe')"),
                metadata=metadata,
            )
        assert unavailable.value.code() == grpc.StatusCode.FAILED_PRECONDITION
    finally:
        await channel.close()
        await server.stop(grace=None)

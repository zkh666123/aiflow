from __future__ import annotations

import asyncio

import grpc

from aiflow.v1 import sandbox_pb2_grpc
from aiflow_sandbox.config import Settings
from aiflow_sandbox.grpc.auth import ServiceTokenInterceptor
from aiflow_sandbox.grpc.service import SandboxService
from aiflow_sandbox.wasi.artifact import WasiArtifact


def create_server(settings: Settings) -> grpc.aio.Server:
    artifact = WasiArtifact(
        settings.wasi_python_path,
        settings.wasi_python_sha256,
    )
    server = grpc.aio.server(
        interceptors=[
            ServiceTokenInterceptor(settings.grpc_token.get_secret_value()),
        ]
    )
    sandbox_pb2_grpc.add_SandboxServiceServicer_to_server(
        SandboxService(artifact), server
    )
    if server.add_insecure_port(settings.grpc_address) == 0:
        raise RuntimeError(f"unable to bind sandbox to {settings.grpc_address}")
    return server


async def serve() -> None:
    server = create_server(Settings())
    try:
        await server.start()
        await server.wait_for_termination()
    finally:
        await server.stop(grace=5)


if __name__ == "__main__":
    asyncio.run(serve())

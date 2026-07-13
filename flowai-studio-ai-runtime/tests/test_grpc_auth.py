import grpc
import pytest

from aiflow.v1 import models_pb2, models_pb2_grpc
from aiflow_runtime.grpc.auth import SERVICE_TOKEN_METADATA_KEY, ServiceTokenInterceptor
from aiflow_runtime.grpc.model_service import ModelService


@pytest.mark.asyncio
async def test_service_token_authentication_on_a_real_grpc_server() -> None:
    token = "s" * 43
    server = grpc.aio.server(interceptors=[ServiceTokenInterceptor(token)])
    models_pb2_grpc.add_ModelServiceServicer_to_server(ModelService(checks=[]), server)
    port = server.add_insecure_port("127.0.0.1:0")
    await server.start()

    channel = grpc.aio.insecure_channel(f"127.0.0.1:{port}")
    stub = models_pb2_grpc.ModelServiceStub(channel)
    request = models_pb2.ListModelsRequest()

    try:
        with pytest.raises(grpc.aio.AioRpcError) as missing:
            await stub.ListModels(request)
        assert missing.value.code() == grpc.StatusCode.UNAUTHENTICATED

        with pytest.raises(grpc.aio.AioRpcError) as wrong:
            await stub.ListModels(
                request,
                metadata=((SERVICE_TOKEN_METADATA_KEY, "wrong-token"),),
            )
        assert wrong.value.code() == grpc.StatusCode.UNAUTHENTICATED

        response = await stub.ListModels(
            request,
            metadata=((SERVICE_TOKEN_METADATA_KEY, token),),
        )
        assert list(response.models) == []
    finally:
        await channel.close()
        await server.stop(grace=None)

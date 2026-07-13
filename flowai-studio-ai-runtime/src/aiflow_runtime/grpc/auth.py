from __future__ import annotations

import hmac
from collections.abc import AsyncIterator, Callable
from typing import Any

import grpc

SERVICE_TOKEN_METADATA_KEY = "x-flowai-service-token"


class ServiceTokenInterceptor(grpc.aio.ServerInterceptor):
    """Reject unauthenticated calls while preserving each RPC cardinality."""

    def __init__(self, expected_token: str) -> None:
        if len(expected_token) < 32 or not expected_token.strip():
            raise ValueError("expected service token is invalid")
        self._expected_token = expected_token

    async def intercept_service(
        self,
        continuation: Callable[[grpc.HandlerCallDetails], Any],
        handler_call_details: grpc.HandlerCallDetails,
    ) -> grpc.RpcMethodHandler | None:
        handler = await continuation(handler_call_details)
        if handler is None:
            return None

        provided = next(
            (
                item.value
                for item in handler_call_details.invocation_metadata or ()
                if item.key.lower() == SERVICE_TOKEN_METADATA_KEY
            ),
            "",
        )
        if provided and hmac.compare_digest(provided, self._expected_token):
            return handler

        return _unauthenticated_handler(handler)


async def _abort(context: grpc.aio.ServicerContext) -> None:
    await context.abort(grpc.StatusCode.UNAUTHENTICATED, "invalid service token")


def _unauthenticated_handler(handler: grpc.RpcMethodHandler) -> grpc.RpcMethodHandler:
    if handler.request_streaming and handler.response_streaming:

        async def deny_stream_stream(
            request_iterator: AsyncIterator[Any],
            context: grpc.aio.ServicerContext,
        ) -> AsyncIterator[Any]:
            del request_iterator
            await _abort(context)
            if False:
                yield None

        return grpc.stream_stream_rpc_method_handler(
            deny_stream_stream,
            request_deserializer=handler.request_deserializer,
            response_serializer=handler.response_serializer,
        )

    if handler.request_streaming:

        async def deny_stream_unary(
            request_iterator: AsyncIterator[Any],
            context: grpc.aio.ServicerContext,
        ) -> Any:
            del request_iterator
            await _abort(context)

        return grpc.stream_unary_rpc_method_handler(
            deny_stream_unary,
            request_deserializer=handler.request_deserializer,
            response_serializer=handler.response_serializer,
        )

    if handler.response_streaming:

        async def deny_unary_stream(
            request: Any,
            context: grpc.aio.ServicerContext,
        ) -> AsyncIterator[Any]:
            del request
            await _abort(context)
            if False:
                yield None

        return grpc.unary_stream_rpc_method_handler(
            deny_unary_stream,
            request_deserializer=handler.request_deserializer,
            response_serializer=handler.response_serializer,
        )

    async def deny_unary_unary(
        request: Any,
        context: grpc.aio.ServicerContext,
    ) -> Any:
        del request
        await _abort(context)

    return grpc.unary_unary_rpc_method_handler(
        deny_unary_unary,
        request_deserializer=handler.request_deserializer,
        response_serializer=handler.response_serializer,
    )

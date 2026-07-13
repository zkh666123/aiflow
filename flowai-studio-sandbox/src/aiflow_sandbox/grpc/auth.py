from __future__ import annotations

import hmac
from collections.abc import Callable
from typing import Any

import grpc

SERVICE_TOKEN_METADATA_KEY = "x-flowai-service-token"


class ServiceTokenInterceptor(grpc.aio.ServerInterceptor):
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

        async def deny(request: Any, context: grpc.aio.ServicerContext) -> Any:
            del request
            await context.abort(
                grpc.StatusCode.UNAUTHENTICATED,
                "invalid service token",
            )

        return grpc.unary_unary_rpc_method_handler(
            deny,
            request_deserializer=handler.request_deserializer,
            response_serializer=handler.response_serializer,
        )

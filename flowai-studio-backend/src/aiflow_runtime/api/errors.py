from __future__ import annotations

from typing import Any

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from starlette.exceptions import HTTPException as StarletteHTTPException
from starlette.responses import Response

from aiflow_runtime.api.envelope import failure


class APIError(Exception):
    def __init__(self, status_code: int, code: str, message: str, data: Any = None) -> None:
        super().__init__(message)
        self.status_code = status_code
        self.code = code
        self.message = message
        self.data = data


def install_error_handlers(app: FastAPI) -> None:
    @app.exception_handler(APIError)
    async def api_error_handler(_request: Request, exc: APIError) -> Response:
        return failure(exc.code, exc.message, exc.status_code, exc.data)

    @app.exception_handler(RequestValidationError)
    async def validation_error_handler(_request: Request, exc: RequestValidationError) -> Response:
        details = [
            {"field": ".".join(str(part) for part in error["loc"]), "message": error["msg"]}
            for error in exc.errors()
        ]
        return failure("BAD_REQUEST", "request validation failed", 400, details)

    @app.exception_handler(StarletteHTTPException)
    async def http_error_handler(_request: Request, exc: StarletteHTTPException) -> Response:
        code = "NOT_FOUND" if exc.status_code == 404 else "HTTP_ERROR"
        return failure(code, str(exc.detail), exc.status_code)

    @app.exception_handler(Exception)
    async def unhandled_error_handler(_request: Request, _exc: Exception) -> Response:
        return failure("INTERNAL_ERROR", "internal server error", 500)

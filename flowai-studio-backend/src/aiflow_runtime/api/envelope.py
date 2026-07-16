from __future__ import annotations

from datetime import UTC, datetime
from typing import Any

from fastapi.encoders import jsonable_encoder
from fastapi.responses import ORJSONResponse


def timestamp() -> str:
    return datetime.now(UTC).isoformat(timespec="milliseconds").replace("+00:00", "Z")


def payload(
    *,
    success: bool,
    code: str,
    message: str,
    data: Any = None,
) -> dict[str, Any]:
    return {
        "success": success,
        "code": code,
        "message": message,
        "data": jsonable_encoder(data),
        "timestamp": timestamp(),
    }


def success(data: Any = None, message: str = "success", status_code: int = 200) -> ORJSONResponse:
    return ORJSONResponse(
        status_code=status_code,
        content=payload(success=True, code="SUCCESS", message=message, data=data),
    )


def failure(code: str, message: str, status_code: int, data: Any = None) -> ORJSONResponse:
    return ORJSONResponse(
        status_code=status_code,
        content=payload(success=False, code=code, message=message, data=data),
    )

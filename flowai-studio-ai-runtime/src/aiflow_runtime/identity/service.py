from __future__ import annotations

import re
from typing import Any
from uuid import UUID

from sqlalchemy import Result, text
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.errors import APIError

USERNAME_PATTERN = re.compile(r"^[A-Za-z0-9_]{3,20}$")


def require_uuid(value: str, field: str = "id") -> str:
    try:
        return str(UUID(value))
    except ValueError as exc:
        raise APIError(400, "BAD_REQUEST", f"{field} must be a UUID") from exc


def validate_name(value: str, maximum: int, field: str = "Name") -> str:
    result = value.strip()
    if not result or len(result) > maximum:
        raise APIError(400, "BAD_REQUEST", f"{field} must contain 1-{maximum} characters")
    return result


def result_mapping(result: Result[Any], not_found: str) -> dict[str, Any]:
    row = result.mappings().first()
    if row is None:
        raise APIError(404, "NOT_FOUND", not_found)
    return dict(row)


async def global_role(session: AsyncSession, user_id: str) -> str:
    result = await session.execute(
        text("SELECT global_role FROM control.users WHERE id = CAST(:user_id AS uuid)"),
        {"user_id": user_id},
    )
    role = result.scalar_one_or_none()
    if role is None:
        raise APIError(401, "UNAUTHORIZED", "User does not exist")
    return str(role)

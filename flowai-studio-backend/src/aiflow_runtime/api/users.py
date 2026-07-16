from __future__ import annotations

from typing import Annotated, Any

from fastapi import APIRouter, Depends, Request
from sqlalchemy import text
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.envelope import success
from aiflow_runtime.api.errors import APIError
from aiflow_runtime.identity.auth import (
    check_login,
    current_principal,
    dummy_hash,
    hash_password,
    issue_token,
    record_login_failure,
    reset_login,
    validate_username,
    verify_password,
)
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.identity.schemas import LoginRequest, RegisterRequest, UpdateProfileRequest
from aiflow_runtime.infrastructure.database import get_session

router = APIRouter(prefix="/api/users", tags=["users"])
Session = Annotated[AsyncSession, Depends(get_session)]
CurrentUser = Annotated[Principal, Depends(current_principal)]


@router.post("/register", status_code=201)
async def register(body: RegisterRequest, session: Session) -> Any:
    username = validate_username(body.username)
    encoded = hash_password(body.password)
    try:
        result = await session.execute(
            text("""
                INSERT INTO control.users (username,password_hash) VALUES (:username,:password_hash)
                RETURNING id::text,username,created_at
            """),
            {"username": username, "password_hash": encoded},
        )
        await session.commit()
    except IntegrityError as exc:
        await session.rollback()
        raise APIError(409, "CONFLICT", "Username already exists") from exc
    row = result.mappings().one()
    return success({"id": row["id"], "username": row["username"], "createdAt": row["created_at"]}, status_code=201)


@router.post("/login", status_code=201)
async def login(body: LoginRequest, request: Request, session: Session) -> Any:
    if not body.username.strip() or not body.password:
        raise APIError(400, "BAD_REQUEST", "Username and password are required")
    redis = request.app.state.redis
    await check_login(redis, body.username)
    result = await session.execute(
        text("SELECT id::text,username,password_hash,global_role FROM control.users WHERE username=:username"),
        {"username": body.username},
    )
    row = result.mappings().first()
    encoded = str(row["password_hash"]) if row else dummy_hash()
    if not verify_password(body.password, encoded) or row is None:
        await record_login_failure(redis, body.username)
        raise APIError(401, "UNAUTHORIZED", "Invalid username or password")
    await reset_login(redis, body.username)
    principal = Principal(str(row["id"]), str(row["username"]), str(row["global_role"]))
    token = issue_token(request.app.state.settings, principal)
    return success({"user": {"id": principal.user_id, "username": principal.username}, "token": token}, status_code=201)


@router.get("/profile")
async def profile(principal: CurrentUser, session: Session) -> Any:
    result = await session.execute(
        text("SELECT id::text,username,avatar,created_at FROM control.users WHERE id=CAST(:id AS uuid)"),
        {"id": principal.user_id},
    )
    row = result.mappings().first()
    if row is None:
        raise APIError(401, "UNAUTHORIZED", "User does not exist")
    return success({"id": row["id"], "username": row["username"], "avatar": row["avatar"], "createdAt": row["created_at"]})


@router.patch("/profile")
async def update_profile(body: UpdateProfileRequest, principal: CurrentUser, session: Session) -> Any:
    values: dict[str, Any] = {"id": principal.user_id}
    assignments: list[str] = []
    if "username" in body.model_fields_set:
        if body.username is None:
            raise APIError(400, "BAD_REQUEST", "Username must be a string")
        assignments.append("username=:username")
        values["username"] = validate_username(body.username)
    if "avatar" in body.model_fields_set:
        if body.avatar is not None and len(body.avatar) > 2048:
            raise APIError(400, "BAD_REQUEST", "Avatar URL must not exceed 2048 characters")
        assignments.append("avatar=:avatar")
        values["avatar"] = body.avatar
    if not assignments:
        assignments.append("updated_at=updated_at")
    try:
        result = await session.execute(
            text(f"UPDATE control.users SET {','.join(assignments)} WHERE id=CAST(:id AS uuid) RETURNING id::text,username,avatar"),
            values,
        )
        await session.commit()
    except IntegrityError as exc:
        await session.rollback()
        raise APIError(409, "CONFLICT", "Username already exists") from exc
    row = result.mappings().first()
    if row is None:
        raise APIError(401, "UNAUTHORIZED", "User does not exist")
    return success(dict(row))

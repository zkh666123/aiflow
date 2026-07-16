from __future__ import annotations

import hashlib
import math
import re
from datetime import UTC, datetime, timedelta
from typing import Annotated, Any

import jwt
from fastapi import Depends, Request
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from pwdlib import PasswordHash
from redis.asyncio import Redis
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.errors import APIError
from aiflow_runtime.config import Settings
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.infrastructure.database import get_session

_bearer = HTTPBearer(auto_error=False)
_passwords = PasswordHash.recommended()
_dummy_hash: str | None = None
_username_pattern = re.compile(r"^[A-Za-z0-9_]{3,20}$")

LOGIN_CHECK = """
local locked = tonumber(redis.call('HGET', KEYS[1], 'locked') or '0')
if locked == 1 then local ttl=redis.call('PTTL',KEYS[1]); if ttl>0 then return {1,ttl} end; redis.call('DEL',KEYS[1]) end
return {0,0}
"""
LOGIN_FAILURE = """
local attempts=redis.call('HINCRBY',KEYS[1],'attempts',1)
if attempts>=5 then redis.call('HSET',KEYS[1],'locked',1); redis.call('PEXPIRE',KEYS[1],900000); return {1,900000} end
redis.call('PEXPIRE',KEYS[1],3600000); return {0,0}
"""


def validate_username(username: str) -> str:
    if not _username_pattern.fullmatch(username):
        raise APIError(400, "BAD_REQUEST", "Username must be 3-20 letters, numbers, or underscores")
    return username


def validate_password(password: str) -> str:
    if len(password) < 6 or len(password.encode("utf-8")) > 72:
        raise APIError(400, "BAD_REQUEST", "Password must contain 6-72 bytes")
    return password


def hash_password(password: str) -> str:
    return _passwords.hash(validate_password(password))


def verify_password(password: str, encoded: str) -> bool:
    try:
        return _passwords.verify(password, encoded)
    except Exception:
        return False


def expiration_delta(value: str) -> timedelta:
    match = re.fullmatch(r"(\d+)([smhd])", value.strip())
    if not match:
        return timedelta(days=7)
    amount = int(match.group(1))
    return {
        "s": timedelta(seconds=amount),
        "m": timedelta(minutes=amount),
        "h": timedelta(hours=amount),
        "d": timedelta(days=amount),
    }[match.group(2)]


def issue_token(settings: Settings, principal: Principal) -> str:
    now = datetime.now(UTC)
    return jwt.encode(
        {"userId": principal.user_id, "username": principal.username, "iat": now, "exp": now + expiration_delta(settings.jwt_expiration)},
        settings.jwt_secret.get_secret_value(),
        algorithm="HS256",
    )


async def current_principal(
    request: Request,
    credentials: Annotated[HTTPAuthorizationCredentials | None, Depends(_bearer)],
    session: Annotated[AsyncSession, Depends(get_session)],
) -> Principal:
    if credentials is None or credentials.scheme != "Bearer":
        raise APIError(401, "UNAUTHORIZED", "Access token is missing")
    settings: Settings = request.app.state.settings
    try:
        claims = jwt.decode(credentials.credentials, settings.jwt_secret.get_secret_value(), algorithms=["HS256"], options={"require": ["exp", "iat"]})
        user_id = str(claims["userId"])
        username = str(claims["username"])
    except Exception as exc:
        raise APIError(401, "UNAUTHORIZED", "Invalid or expired token") from exc
    result = await session.execute(
        text("SELECT global_role FROM control.users WHERE id=CAST(:id AS uuid) AND username=:username"),
        {"id": user_id, "username": username},
    )
    role = result.scalar_one_or_none()
    if role is None:
        raise APIError(401, "UNAUTHORIZED", "Invalid or expired token")
    return Principal(user_id=user_id, username=username, global_role=str(role))


def login_key(username: str) -> str:
    return "flowai:auth:login:" + hashlib.sha256(username.encode()).hexdigest()


async def check_login(redis: Redis, username: str) -> None:
    locked, retry_ms = await redis.eval(LOGIN_CHECK, 1, login_key(username))
    if int(locked) == 1:
        minutes = max(1, math.ceil(int(retry_ms) / 60000))
        raise APIError(401, "UNAUTHORIZED", f"账户已被锁定，请 {minutes} 分钟后再试")


async def record_login_failure(redis: Redis, username: str) -> None:
    locked, retry_ms = await redis.eval(LOGIN_FAILURE, 1, login_key(username))
    if int(locked) == 1:
        minutes = max(1, math.ceil(int(retry_ms) / 60000))
        raise APIError(401, "UNAUTHORIZED", f"账户已被锁定，请 {minutes} 分钟后再试")


async def reset_login(redis: Redis, username: str) -> None:
    await redis.delete(login_key(username))


def dummy_hash() -> str:
    global _dummy_hash
    if _dummy_hash is None:
        _dummy_hash = _passwords.hash("flowai-invalid-user-password")
    return _dummy_hash

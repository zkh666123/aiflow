from __future__ import annotations

import hashlib
import hmac
import secrets
from datetime import UTC, datetime
from typing import Annotated, Any

from fastapi import APIRouter, Depends, Query, Request
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.envelope import success
from aiflow_runtime.api.errors import APIError
from aiflow_runtime.identity.auth import current_principal
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.identity.schemas import CreateAPIKeyRequest, ToggleAPIKeyRequest
from aiflow_runtime.identity.service import require_uuid, validate_name
from aiflow_runtime.infrastructure.database import get_session

router = APIRouter(prefix="/api/api-keys", tags=["api-keys"])
Session = Annotated[AsyncSession, Depends(get_session)]
CurrentUser = Annotated[Principal, Depends(current_principal)]
DEFAULT_SCOPES = ["app:read", "workflow:execute"]


def api_key_data(row: Any) -> dict[str, Any]:
    data = {
        "id": row["id"], "name": row["name"], "keyPrefix": row["key_prefix"], "scopes": row["scopes"],
        "isActive": row["is_active"], "createdAt": row["created_at"],
    }
    if row.get("last_used_at") is not None:
        data["lastUsedAt"] = row["last_used_at"]
    if row.get("expires_at") is not None:
        data["expiresAt"] = row["expires_at"]
    if row.get("application_id") is not None:
        data["applicationId"] = row["application_id"]
    return data


@router.post("", status_code=201)
async def create_api_key(body: CreateAPIKeyRequest, request: Request, principal: CurrentUser, session: Session) -> Any:
    scopes = body.scopes if body.scopes is not None else DEFAULT_SCOPES
    if len(scopes) != len(set(scopes)):
        raise APIError(400, "BAD_REQUEST", "API key scopes must be unique")
    if body.expires_at is not None and body.expires_at <= datetime.now(UTC):
        raise APIError(400, "BAD_REQUEST", "Expiration time must be in the future")
    application_id = None
    if body.application_id is not None:
        application_id = require_uuid(body.application_id, "applicationId")
        owner = await session.execute(text("SELECT owner_id::text FROM control.applications WHERE id=CAST(:id AS uuid)"), {"id": application_id})
        owner_id = owner.scalar_one_or_none()
        if owner_id is None:
            raise APIError(404, "NOT_FOUND", "Application not found")
        if owner_id != principal.user_id:
            raise APIError(403, "FORBIDDEN", "Only the application owner can create an API key")
    raw_key = "sk-" + secrets.token_hex(32)
    secret = request.app.state.settings.api_key_hmac_secret.get_secret_value().encode()
    digest = hmac.new(secret, raw_key.encode(), hashlib.sha256).digest()
    result = await session.execute(
        text("""
            INSERT INTO control.api_keys(name,key_digest,key_prefix,scopes,is_active,expires_at,user_id,application_id)
            VALUES(:name,:digest,:prefix,CAST(:scopes AS jsonb),true,:expires,CAST(:user AS uuid),CAST(:app AS uuid))
            RETURNING id::text,name,key_prefix,scopes,is_active,last_used_at,expires_at,application_id::text,created_at
        """),
        {"name": validate_name(body.name, 100), "digest": digest, "prefix": raw_key[:7], "scopes": __import__("json").dumps(scopes), "expires": body.expires_at, "user": principal.user_id, "app": application_id},
    )
    await session.commit()
    return success({**api_key_data(result.mappings().one()), "key": raw_key}, status_code=201)


@router.get("")
async def list_api_keys(
    principal: CurrentUser,
    session: Session,
    application_id: str | None = Query(default=None, alias="applicationId"),
) -> Any:
    app = require_uuid(application_id, "applicationId") if application_id else None
    result = await session.execute(
        text("""
            SELECT id::text,name,key_prefix,scopes,is_active,last_used_at,expires_at,application_id::text,created_at
            FROM control.api_keys WHERE user_id=CAST(:user AS uuid)
              AND (CAST(:app AS uuid) IS NULL OR application_id=CAST(:app AS uuid)) ORDER BY created_at DESC
        """), {"user": principal.user_id, "app": app}
    )
    return success([api_key_data(row) for row in result.mappings()])


@router.delete("/{key_id}")
async def delete_api_key(key_id: str, principal: CurrentUser, session: Session) -> Any:
    result = await session.execute(
        text("DELETE FROM control.api_keys WHERE id=CAST(:id AS uuid) AND user_id=CAST(:user AS uuid)"),
        {"id": require_uuid(key_id, "keyId"), "user": principal.user_id},
    )
    if result.rowcount == 0:
        raise APIError(404, "NOT_FOUND", "API key not found")
    await session.commit()
    return success({"success": True})


@router.patch("/{key_id}/toggle")
async def toggle_api_key(body: ToggleAPIKeyRequest, key_id: str, principal: CurrentUser, session: Session) -> Any:
    result = await session.execute(
        text("""UPDATE control.api_keys SET is_active=:active WHERE id=CAST(:id AS uuid) AND user_id=CAST(:user AS uuid)
                RETURNING id::text,name,is_active"""),
        {"active": body.is_active, "id": require_uuid(key_id, "keyId"), "user": principal.user_id},
    )
    row = result.mappings().first()
    if row is None:
        raise APIError(404, "NOT_FOUND", "API key not found")
    await session.commit()
    return success({"id": row["id"], "name": row["name"], "isActive": row["is_active"]})

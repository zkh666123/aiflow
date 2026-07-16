from __future__ import annotations

import json
import secrets
from typing import Annotated, Any
from urllib.parse import urlparse

from fastapi import APIRouter, Depends, Request
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.envelope import success
from aiflow_runtime.api.errors import APIError
from aiflow_runtime.identity.auth import current_principal
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.identity.schemas import UpdateShareRequest
from aiflow_runtime.identity.service import require_uuid
from aiflow_runtime.infrastructure.database import get_session

router = APIRouter(tags=["sharing"])
Session = Annotated[AsyncSession, Depends(get_session)]
CurrentUser = Annotated[Principal, Depends(current_principal)]


def share_data(row: Any) -> dict[str, Any]:
    return {
        "id": row["id"], "applicationId": row["application_id"], "shareLink": row["share_link"],
        "isPublic": row["is_public"], "accessCount": row["access_count"], "embedConfig": row["embed_config"],
        "createdAt": row["created_at"], "updatedAt": row["updated_at"],
    }


async def require_owner(session: AsyncSession, principal: Principal, app_id: str) -> str:
    value = require_uuid(app_id, "appId")
    result = await session.execute(text("SELECT owner_id::text FROM control.applications WHERE id=CAST(:id AS uuid)"), {"id": value})
    owner = result.scalar_one_or_none()
    if owner is None:
        raise APIError(404, "NOT_FOUND", "Application not found")
    if owner != principal.user_id and principal.global_role != "admin":
        raise APIError(403, "FORBIDDEN", "Only the application owner can manage sharing")
    return value


async def fetch_share(session: AsyncSession, app_id: str) -> Any:
    result = await session.execute(
        text("""SELECT id::text,application_id::text,share_link,is_public,access_count,embed_config,created_at,updated_at
                FROM control.app_shares WHERE application_id=CAST(:id AS uuid)"""), {"id": app_id}
    )
    row = result.mappings().first()
    if row is None:
        raise APIError(404, "NOT_FOUND", "Application share not found")
    return row


@router.post("/api/apps/{app_id}/share", status_code=201)
async def generate_share(app_id: str, principal: CurrentUser, session: Session) -> Any:
    value = await require_owner(session, principal, app_id)
    existing = await session.execute(
        text("""SELECT id::text,application_id::text,share_link,is_public,access_count,embed_config,created_at,updated_at
                FROM control.app_shares WHERE application_id=CAST(:id AS uuid)"""), {"id": value}
    )
    row = existing.mappings().first()
    if row is None:
        link = "share-" + secrets.token_hex(16)
        result = await session.execute(
            text("""INSERT INTO control.app_shares(application_id,share_link) VALUES(CAST(:id AS uuid),:link)
                    RETURNING id::text,application_id::text,share_link,is_public,access_count,embed_config,created_at,updated_at"""),
            {"id": value, "link": link},
        )
        await session.execute(text("UPDATE control.applications SET share_link=:link WHERE id=CAST(:id AS uuid)"), {"id": value, "link": link})
        await session.commit()
        row = result.mappings().one()
    return success(share_data(row), status_code=201)


@router.get("/api/apps/{app_id}/share")
async def get_share(app_id: str, principal: CurrentUser, session: Session) -> Any:
    value = await require_owner(session, principal, app_id)
    return success(share_data(await fetch_share(session, value)))


@router.patch("/api/apps/{app_id}/share")
async def update_share(body: UpdateShareRequest, app_id: str, principal: CurrentUser, session: Session) -> Any:
    value = await require_owner(session, principal, app_id)
    assignments: list[str] = []
    params: dict[str, Any] = {"id": value}
    if "is_public" in body.model_fields_set:
        assignments.append("is_public=:public")
        params["public"] = body.is_public
    if "embed_config" in body.model_fields_set:
        if body.embed_config is None:
            raise APIError(400, "BAD_REQUEST", "embedConfig must be an object")
        for origin in body.embed_config.allowed_origins:
            parsed = urlparse(origin)
            if parsed.scheme not in {"http", "https"} or not parsed.netloc or parsed.username or parsed.query or parsed.fragment or parsed.path not in {"", "/"}:
                raise APIError(400, "BAD_REQUEST", "Allowed origins must be absolute HTTP(S) origins")
        if len(body.embed_config.width) > 32 or len(body.embed_config.height) > 32:
            raise APIError(400, "BAD_REQUEST", "Embed dimensions are too long")
        assignments.append("embed_config=CAST(:embed AS jsonb)")
        params["embed"] = json.dumps(body.embed_config.model_dump(by_alias=True, exclude_none=True))
    if not assignments:
        raise APIError(400, "BAD_REQUEST", "At least one share setting is required")
    result = await session.execute(
        text(f"""UPDATE control.app_shares SET {','.join(assignments)} WHERE application_id=CAST(:id AS uuid)
                RETURNING id::text,application_id::text,share_link,is_public,access_count,embed_config,created_at,updated_at"""), params
    )
    row = result.mappings().first()
    if row is None:
        raise APIError(404, "NOT_FOUND", "Application share not found")
    await session.commit()
    return success(share_data(row))


@router.delete("/api/apps/{app_id}/share")
async def revoke_share(app_id: str, principal: CurrentUser, session: Session) -> Any:
    value = await require_owner(session, principal, app_id)
    result = await session.execute(text("DELETE FROM control.app_shares WHERE application_id=CAST(:id AS uuid)"), {"id": value})
    if result.rowcount == 0:
        raise APIError(404, "NOT_FOUND", "Application share not found")
    await session.execute(text("UPDATE control.applications SET share_link=NULL WHERE id=CAST(:id AS uuid)"), {"id": value})
    await session.commit()
    return success({"success": True})


@router.get("/api/apps/{app_id}/embed")
async def embed_code(app_id: str, request: Request, principal: CurrentUser, session: Session) -> Any:
    value = await require_owner(session, principal, app_id)
    row = await fetch_share(session, value)
    frontend = request.app.state.settings.frontend_url
    share_url = f"{frontend}/share/{row['share_link']}"
    config = row["embed_config"] or {}
    theme = config.get("theme", "light")
    script = f'<script src="{frontend}/embed.js" data-app="{row["share_link"]}" data-theme="{theme}"></script>'
    iframe = f'<iframe src="{share_url}" width="100%" height="600" frameborder="0" style="border-radius: 8px;"></iframe>'
    return success({"shareUrl": share_url, "iframeCode": iframe, "scriptTag": script, "scriptCode": script, "embedConfig": row["embed_config"]})


@router.get("/api/share/{share_link}")
async def public_share(share_link: str, session: Session) -> Any:
    if len(share_link) != 38 or not share_link.startswith("share-"):
        raise APIError(404, "NOT_FOUND", "Shared application not found")
    result = await session.execute(
        text("""
            SELECT s.id::text,s.application_id::text,s.share_link,s.is_public,s.embed_config,
                   a.name,a.description,a.icon,a.status
            FROM control.app_shares s JOIN control.applications a ON a.id=s.application_id
            WHERE s.share_link=:link AND s.is_public=true
        """), {"link": share_link}
    )
    row = result.mappings().first()
    if row is None:
        raise APIError(404, "NOT_FOUND", "Shared application not found")
    await session.execute(text("UPDATE control.app_shares SET access_count=access_count+1 WHERE id=CAST(:id AS uuid)"), {"id": row["id"]})
    await session.commit()
    return success({"id": row["application_id"], "shareLink": row["share_link"], "isPublic": row["is_public"], "name": row["name"], "description": row["description"], "icon": row["icon"], "status": row["status"], "embedConfig": row["embed_config"]})

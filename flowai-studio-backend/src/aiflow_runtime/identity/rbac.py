from __future__ import annotations

from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.errors import APIError
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.identity.service import require_uuid

EDITOR = {"app:read", "app:update", "workflow:create", "workflow:read", "workflow:update", "workflow:delete", "workflow:execute", "team:read"}
VIEWER = {"app:read", "workflow:read", "team:read"}
FULL_ACCESS = {"app:read", "app:update", "app:delete", "app:publish", "app:share", "workflow:create", "workflow:read", "workflow:update", "workflow:delete", "workflow:execute"}
CAN_EDIT = {"app:read", "app:update", "workflow:create", "workflow:read", "workflow:update", "workflow:execute"}
CAN_VIEW = {"app:read", "workflow:read"}


def role_allows(role: str, permission: str) -> bool:
    if role in {"owner", "admin"}:
        return True
    return permission in (EDITOR if role == "editor" else VIEWER if role == "viewer" else set())


def grant_allows(grant: str, permission: str) -> bool:
    return permission in {"full_access": FULL_ACCESS, "can_edit": CAN_EDIT, "can_view": CAN_VIEW}.get(grant, set())


async def authorize_application(session: AsyncSession, principal: Principal, application_id: str, permission: str) -> None:
    app_id = require_uuid(application_id, "id")
    result = await session.execute(
        text("""
            SELECT a.owner_id::text, tm.role, ta.permission
            FROM control.applications a
            LEFT JOIN control.team_applications ta ON ta.application_id=a.id
            LEFT JOIN control.team_members tm ON tm.team_id=ta.team_id AND tm.user_id=CAST(:user_id AS uuid)
            WHERE a.id=CAST(:app_id AS uuid)
        """),
        {"user_id": principal.user_id, "app_id": app_id},
    )
    rows = result.mappings().all()
    if not rows:
        raise APIError(404, "NOT_FOUND", "Application not found")
    if principal.global_role == "admin" or rows[0]["owner_id"] == principal.user_id:
        return
    if any(row["role"] and (role_allows(row["role"], permission) or grant_allows(row["permission"], permission)) for row in rows):
        return
    raise APIError(403, "FORBIDDEN", "Insufficient application permission")


async def authorize_team(session: AsyncSession, principal: Principal, team_id: str, permission: str) -> None:
    value = require_uuid(team_id, "teamId")
    result = await session.execute(
        text("""
            SELECT t.owner_id::text, tm.role FROM control.teams t
            LEFT JOIN control.team_members tm ON tm.team_id=t.id AND tm.user_id=CAST(:user_id AS uuid)
            WHERE t.id=CAST(:team_id AS uuid)
        """),
        {"user_id": principal.user_id, "team_id": value},
    )
    row = result.mappings().first()
    if row is None:
        raise APIError(404, "NOT_FOUND", "Team not found")
    if principal.global_role == "admin" or row["owner_id"] == principal.user_id or role_allows(str(row["role"] or ""), permission):
        return
    raise APIError(403, "FORBIDDEN", "Insufficient team permission")

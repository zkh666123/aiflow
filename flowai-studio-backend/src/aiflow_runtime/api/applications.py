from __future__ import annotations

from typing import Annotated, Any

from fastapi import APIRouter, Depends
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.envelope import success
from aiflow_runtime.api.errors import APIError
from aiflow_runtime.identity.auth import current_principal
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.identity.rbac import authorize_application
from aiflow_runtime.identity.schemas import CreateApplicationRequest, UpdateApplicationRequest
from aiflow_runtime.identity.service import require_uuid, validate_name
from aiflow_runtime.infrastructure.database import get_session

router = APIRouter(prefix="/api/apps", tags=["applications"])
Session = Annotated[AsyncSession, Depends(get_session)]
CurrentUser = Annotated[Principal, Depends(current_principal)]


def public_application(row: Any) -> dict[str, Any]:
    return {
        "id": row["id"], "name": row["name"], "description": row["description"], "icon": row["icon"],
        "status": row["status"], "createdAt": row["created_at"], "updatedAt": row["updated_at"],
    }


@router.post("", status_code=201)
async def create_application(body: CreateApplicationRequest, principal: CurrentUser, session: Session) -> Any:
    result = await session.execute(
        text("""
            INSERT INTO control.applications (name,description,icon,status,owner_id)
            VALUES (:name,:description,:icon,:status,CAST(:owner_id AS uuid))
            RETURNING id::text,name,description,icon,status,created_at,updated_at
        """),
        {"name": validate_name(body.name, 100), "description": body.description, "icon": body.icon, "status": body.status, "owner_id": principal.user_id},
    )
    await session.commit()
    return success(public_application(result.mappings().one()), status_code=201)


@router.get("")
async def list_applications(principal: CurrentUser, session: Session) -> Any:
    result = await session.execute(
        text("""
            SELECT DISTINCT a.id::text,a.name,a.description,a.icon,a.status,a.created_at,a.updated_at,
                CASE WHEN a.owner_id=CAST(:user_id AS uuid) THEN 'owner' ELSE ta.permission END AS access_type
            FROM control.applications a
            LEFT JOIN control.team_applications ta ON ta.application_id=a.id
            LEFT JOIN control.team_members tm ON tm.team_id=ta.team_id
            WHERE a.owner_id=CAST(:user_id AS uuid) OR tm.user_id=CAST(:user_id AS uuid) OR :is_admin
            ORDER BY a.updated_at DESC
        """),
        {"user_id": principal.user_id, "is_admin": principal.global_role == "admin"},
    )
    return success([{**public_application(row), "accessType": row["access_type"]} for row in result.mappings()])


@router.get("/{app_id}")
async def get_application(app_id: str, principal: CurrentUser, session: Session) -> Any:
    await authorize_application(session, principal, app_id, "app:read")
    result = await session.execute(
        text("""SELECT id::text,name,description,icon,status,share_link,owner_id::text,created_at,updated_at
                FROM control.applications WHERE id=CAST(:id AS uuid)"""), {"id": require_uuid(app_id)}
    )
    row = result.mappings().one()
    return success({**public_application(row), "shareLink": row["share_link"], "userId": row["owner_id"], "workflows": []})


@router.patch("/{app_id}")
async def update_application(body: UpdateApplicationRequest, app_id: str, principal: CurrentUser, session: Session) -> Any:
    await authorize_application(session, principal, app_id, "app:update")
    assignments: list[str] = []
    values: dict[str, Any] = {"id": require_uuid(app_id)}
    for field, column in (("name", "name"), ("description", "description"), ("icon", "icon"), ("status", "status")):
        if field in body.model_fields_set:
            value = getattr(body, field)
            if field in {"name", "status"} and value is None:
                raise APIError(400, "BAD_REQUEST", f"{field.capitalize()} must be a string")
            values[field] = validate_name(value, 100) if field == "name" else value
            assignments.append(f"{column}=:{field}")
    if not assignments:
        assignments.append("updated_at=updated_at")
    result = await session.execute(
        text(f"UPDATE control.applications SET {','.join(assignments)} WHERE id=CAST(:id AS uuid) RETURNING id::text,name,description,icon,status,created_at,updated_at"), values
    )
    await session.commit()
    return success(public_application(result.mappings().one()))


@router.delete("/{app_id}")
async def delete_application(app_id: str, principal: CurrentUser, session: Session) -> Any:
    await authorize_application(session, principal, app_id, "app:delete")
    await session.execute(text("DELETE FROM control.applications WHERE id=CAST(:id AS uuid)"), {"id": require_uuid(app_id)})
    await session.commit()
    return success({"success": True})


async def set_status(app_id: str, status: str, permission: str, principal: Principal, session: AsyncSession) -> Any:
    await authorize_application(session, principal, app_id, permission)
    result = await session.execute(
        text("UPDATE control.applications SET status=:status WHERE id=CAST(:id AS uuid) RETURNING id::text,name,status"),
        {"id": require_uuid(app_id), "status": status},
    )
    await session.commit()
    return success(dict(result.mappings().one()))


@router.patch("/{app_id}/publish")
async def publish(app_id: str, principal: CurrentUser, session: Session) -> Any:
    return await set_status(app_id, "published", "app:publish", principal, session)


@router.patch("/{app_id}/unpublish")
async def unpublish(app_id: str, principal: CurrentUser, session: Session) -> Any:
    return await set_status(app_id, "draft", "app:publish", principal, session)


@router.patch("/{app_id}/archive")
async def archive(app_id: str, principal: CurrentUser, session: Session) -> Any:
    return await set_status(app_id, "archived", "app:update", principal, session)


@router.patch("/{app_id}/unarchive")
async def unarchive(app_id: str, principal: CurrentUser, session: Session) -> Any:
    return await set_status(app_id, "draft", "app:update", principal, session)

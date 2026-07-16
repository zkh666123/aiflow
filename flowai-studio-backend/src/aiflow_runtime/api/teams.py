from __future__ import annotations

from typing import Annotated, Any

from fastapi import APIRouter, Depends
from sqlalchemy import text
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.envelope import success
from aiflow_runtime.api.errors import APIError
from aiflow_runtime.identity.auth import current_principal
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.identity.rbac import authorize_team
from aiflow_runtime.identity.schemas import (
    AddMemberRequest,
    AddTeamApplicationRequest,
    CreateTeamRequest,
    UpdateMemberRoleRequest,
    UpdateTeamApplicationRequest,
    UpdateTeamRequest,
)
from aiflow_runtime.identity.service import require_uuid, validate_name
from aiflow_runtime.infrastructure.database import get_session

router = APIRouter(prefix="/api/teams", tags=["teams"])
Session = Annotated[AsyncSession, Depends(get_session)]
CurrentUser = Annotated[Principal, Depends(current_principal)]


def team_data(row: Any) -> dict[str, Any]:
    return {
        "id": row["id"], "name": row["name"], "description": row["description"], "avatar": row["avatar"],
        "ownerId": row["owner_id"], "myRole": row.get("my_role"), "memberCount": int(row.get("member_count", 0)),
        "appCount": int(row.get("app_count", 0)), "createdAt": row["created_at"], "updatedAt": row["updated_at"],
    }


def member_data(row: Any) -> dict[str, Any]:
    data = {"id": row["id"], "teamId": row["team_id"], "userId": row["user_id"], "role": row["role"], "joinedAt": row["joined_at"]}
    if "username" in row:
        data["user"] = {"id": row["user_id"], "username": row["username"], "avatar": row["avatar"], "createdAt": row["user_created_at"]}
    return data


def team_application_data(row: Any) -> dict[str, Any]:
    data = {"id": row["id"], "teamId": row["team_id"], "applicationId": row["application_id"], "permission": row["permission"], "addedAt": row["added_at"]}
    if "name" in row:
        data["application"] = {"id": row["application_id"], "name": row["name"], "description": row["description"], "icon": row["icon"], "status": row["status"]}
    return data


@router.post("", status_code=201)
async def create_team(body: CreateTeamRequest, principal: CurrentUser, session: Session) -> Any:
    result = await session.execute(
        text("""INSERT INTO control.teams(name,description,avatar,owner_id)
                VALUES(:name,:description,:avatar,CAST(:owner_id AS uuid))
                RETURNING id::text,name,description,avatar,owner_id::text,created_at,updated_at"""),
        {"name": validate_name(body.name, 50), "description": body.description, "avatar": body.avatar, "owner_id": principal.user_id},
    )
    row = result.mappings().one()
    await session.execute(
        text("INSERT INTO control.team_members(team_id,user_id,role) VALUES(CAST(:team_id AS uuid),CAST(:user_id AS uuid),'owner')"),
        {"team_id": row["id"], "user_id": principal.user_id},
    )
    await session.commit()
    return success({**team_data(row), "myRole": "owner", "memberCount": 1, "appCount": 0}, status_code=201)


@router.get("")
async def list_teams(principal: CurrentUser, session: Session) -> Any:
    result = await session.execute(
        text("""
            SELECT t.id::text,t.name,t.description,t.avatar,t.owner_id::text,t.created_at,t.updated_at,
                   tm.role AS my_role,COUNT(DISTINCT members.id) AS member_count,COUNT(DISTINCT ta.id) AS app_count
            FROM control.teams t JOIN control.team_members tm ON tm.team_id=t.id AND tm.user_id=CAST(:user_id AS uuid)
            LEFT JOIN control.team_members members ON members.team_id=t.id
            LEFT JOIN control.team_applications ta ON ta.team_id=t.id
            GROUP BY t.id,tm.role ORDER BY t.updated_at DESC
        """), {"user_id": principal.user_id}
    )
    return success([team_data(row) for row in result.mappings()])


@router.get("/{team_id}")
async def get_team(team_id: str, principal: CurrentUser, session: Session) -> Any:
    await authorize_team(session, principal, team_id, "team:read")
    value = require_uuid(team_id, "teamId")
    result = await session.execute(
        text("""
            SELECT t.id::text,t.name,t.description,t.avatar,t.owner_id::text,t.created_at,t.updated_at,
                   tm.role AS my_role,
                   (SELECT COUNT(*) FROM control.team_members m WHERE m.team_id=t.id) AS member_count,
                   (SELECT COUNT(*) FROM control.team_applications x WHERE x.team_id=t.id) AS app_count
            FROM control.teams t LEFT JOIN control.team_members tm ON tm.team_id=t.id AND tm.user_id=CAST(:user_id AS uuid)
            WHERE t.id=CAST(:team_id AS uuid)
        """), {"team_id": value, "user_id": principal.user_id}
    )
    row = result.mappings().one()
    members = await session.execute(
        text("""SELECT m.id::text,m.team_id::text,m.user_id::text,m.role,m.joined_at,u.username,u.avatar,u.created_at AS user_created_at
                FROM control.team_members m JOIN control.users u ON u.id=m.user_id WHERE m.team_id=CAST(:team_id AS uuid) ORDER BY m.joined_at"""), {"team_id": value}
    )
    apps = await session.execute(
        text("""SELECT ta.id::text,ta.team_id::text,ta.application_id::text,ta.permission,ta.added_at,a.name,a.description,a.icon,a.status
                FROM control.team_applications ta JOIN control.applications a ON a.id=ta.application_id
                WHERE ta.team_id=CAST(:team_id AS uuid) ORDER BY ta.added_at"""), {"team_id": value}
    )
    return success({**team_data(row), "members": [member_data(item) for item in members.mappings()], "applications": [team_application_data(item) for item in apps.mappings()]})


@router.patch("/{team_id}")
async def update_team(body: UpdateTeamRequest, team_id: str, principal: CurrentUser, session: Session) -> Any:
    await authorize_team(session, principal, team_id, "team:update")
    assignments: list[str] = []
    values: dict[str, Any] = {"id": require_uuid(team_id, "teamId")}
    for field in ("name", "description", "avatar"):
        if field in body.model_fields_set:
            value = getattr(body, field)
            if field == "name" and value is None:
                raise APIError(400, "BAD_REQUEST", "Name must be a string")
            values[field] = validate_name(value, 50) if field == "name" else value
            assignments.append(f"{field}=:{field}")
    if not assignments:
        assignments.append("updated_at=updated_at")
    result = await session.execute(
        text(f"UPDATE control.teams SET {','.join(assignments)} WHERE id=CAST(:id AS uuid) RETURNING id::text,name,description,avatar,owner_id::text,created_at,updated_at"), values
    )
    await session.commit()
    return success(team_data(result.mappings().one()))


@router.delete("/{team_id}")
async def delete_team(team_id: str, principal: CurrentUser, session: Session) -> Any:
    await authorize_team(session, principal, team_id, "team:delete")
    result = await session.execute(text("DELETE FROM control.teams WHERE id=CAST(:id AS uuid) AND owner_id=CAST(:owner AS uuid)"), {"id": require_uuid(team_id, "teamId"), "owner": principal.user_id})
    if result.rowcount == 0:
        raise APIError(403, "FORBIDDEN", "Only the team owner can delete the team")
    await session.commit()
    return success({"success": True})


@router.post("/{team_id}/members", status_code=201)
async def add_member(body: AddMemberRequest, team_id: str, principal: CurrentUser, session: Session) -> Any:
    await authorize_team(session, principal, team_id, "team:manage-members")
    if body.user_id == principal.user_id:
        raise APIError(400, "BAD_REQUEST", "You cannot add yourself")
    try:
        result = await session.execute(
            text("""INSERT INTO control.team_members(team_id,user_id,role) VALUES(CAST(:team AS uuid),CAST(:user AS uuid),:role)
                    RETURNING id::text,team_id::text,user_id::text,role,joined_at"""),
            {"team": require_uuid(team_id, "teamId"), "user": require_uuid(body.user_id, "userId"), "role": body.role},
        )
        await session.commit()
    except IntegrityError as exc:
        await session.rollback()
        raise APIError(409, "CONFLICT", "User is already a team member or does not exist") from exc
    return success(member_data(result.mappings().one()), status_code=201)


@router.patch("/{team_id}/members/{member_id}")
async def update_member(body: UpdateMemberRoleRequest, team_id: str, member_id: str, principal: CurrentUser, session: Session) -> Any:
    await authorize_team(session, principal, team_id, "team:manage-members")
    result = await session.execute(
        text("""UPDATE control.team_members SET role=:role WHERE id=CAST(:id AS uuid) AND team_id=CAST(:team AS uuid) AND role<>'owner'
                RETURNING id::text,team_id::text,user_id::text,role,joined_at"""),
        {"role": body.role, "id": require_uuid(member_id, "memberId"), "team": require_uuid(team_id, "teamId")},
    )
    row = result.mappings().first()
    if row is None:
        raise APIError(404, "NOT_FOUND", "Team member not found")
    await session.commit()
    return success(member_data(row))


@router.delete("/{team_id}/members/{member_id}")
async def remove_member(team_id: str, member_id: str, principal: CurrentUser, session: Session) -> Any:
    await authorize_team(session, principal, team_id, "team:manage-members")
    result = await session.execute(
        text("DELETE FROM control.team_members WHERE id=CAST(:id AS uuid) AND team_id=CAST(:team AS uuid) AND role<>'owner'"),
        {"id": require_uuid(member_id, "memberId"), "team": require_uuid(team_id, "teamId")},
    )
    if result.rowcount == 0:
        raise APIError(404, "NOT_FOUND", "Team member not found")
    await session.commit()
    return success({"success": True})


@router.post("/{team_id}/leave", status_code=201)
async def leave_team(team_id: str, principal: CurrentUser, session: Session) -> Any:
    result = await session.execute(
        text("DELETE FROM control.team_members WHERE team_id=CAST(:team AS uuid) AND user_id=CAST(:user AS uuid) AND role<>'owner'"),
        {"team": require_uuid(team_id, "teamId"), "user": principal.user_id},
    )
    if result.rowcount == 0:
        raise APIError(403, "FORBIDDEN", "Team owner cannot leave the team")
    await session.commit()
    return success({"success": True}, status_code=201)


@router.post("/{team_id}/apps", status_code=201)
async def add_application(body: AddTeamApplicationRequest, team_id: str, principal: CurrentUser, session: Session) -> Any:
    await authorize_team(session, principal, team_id, "team:update")
    app_id = require_uuid(body.application_id, "applicationId")
    owner = await session.execute(text("SELECT owner_id::text FROM control.applications WHERE id=CAST(:id AS uuid)"), {"id": app_id})
    if owner.scalar_one_or_none() != principal.user_id and principal.global_role != "admin":
        raise APIError(403, "FORBIDDEN", "Only the application owner can add it to a team")
    try:
        result = await session.execute(
            text("""INSERT INTO control.team_applications(team_id,application_id,permission)
                    VALUES(CAST(:team AS uuid),CAST(:app AS uuid),:permission)
                    RETURNING id::text,team_id::text,application_id::text,permission,added_at"""),
            {"team": require_uuid(team_id, "teamId"), "app": app_id, "permission": body.permission},
        )
        await session.commit()
    except IntegrityError as exc:
        await session.rollback()
        raise APIError(409, "CONFLICT", "Application already belongs to the team") from exc
    return success(team_application_data(result.mappings().one()), status_code=201)


@router.patch("/{team_id}/apps/{team_app_id}")
async def update_application_permission(body: UpdateTeamApplicationRequest, team_id: str, team_app_id: str, principal: CurrentUser, session: Session) -> Any:
    await authorize_team(session, principal, team_id, "team:update")
    result = await session.execute(
        text("""UPDATE control.team_applications SET permission=:permission
                WHERE id=CAST(:id AS uuid) AND team_id=CAST(:team AS uuid)
                RETURNING id::text,team_id::text,application_id::text,permission,added_at"""),
        {"permission": body.permission, "id": require_uuid(team_app_id, "teamAppId"), "team": require_uuid(team_id, "teamId")},
    )
    row = result.mappings().first()
    if row is None:
        raise APIError(404, "NOT_FOUND", "Team application not found")
    await session.commit()
    return success(team_application_data(row))


@router.delete("/{team_id}/apps/{team_app_id}")
async def remove_application(team_id: str, team_app_id: str, principal: CurrentUser, session: Session) -> Any:
    await authorize_team(session, principal, team_id, "team:update")
    result = await session.execute(
        text("DELETE FROM control.team_applications WHERE id=CAST(:id AS uuid) AND team_id=CAST(:team AS uuid)"),
        {"id": require_uuid(team_app_id, "teamAppId"), "team": require_uuid(team_id, "teamId")},
    )
    if result.rowcount == 0:
        raise APIError(404, "NOT_FOUND", "Team application not found")
    await session.commit()
    return success({"success": True})

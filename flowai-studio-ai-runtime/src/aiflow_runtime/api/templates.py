from __future__ import annotations

import json
import math
from typing import Annotated, Any

from fastapi import APIRouter, Depends, Query
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from aiflow_runtime.api.envelope import success
from aiflow_runtime.api.errors import APIError
from aiflow_runtime.identity.auth import current_principal
from aiflow_runtime.identity.models import Principal
from aiflow_runtime.identity.rbac import authorize_application
from aiflow_runtime.identity.service import require_uuid, validate_name
from aiflow_runtime.infrastructure.database import get_session
from aiflow_runtime.workflow.schemas import CreateTemplateRequest, ImportTemplateRequest, RateTemplateRequest, UpdateTemplateRequest
from aiflow_runtime.workflow.service import load_workflow, workflow_data

router = APIRouter(prefix="/templates", tags=["templates"])
Session = Annotated[AsyncSession, Depends(get_session)]
CurrentUser = Annotated[Principal, Depends(current_principal)]


def template_data(row: Any) -> dict[str, Any]:
    return {"id": row["id"], "name": row["name"], "description": row["description"], "icon": row["icon"],
            "screenshot": row["screenshot"], "category": row["category"], "tags": row["tags"], "nodes": row["nodes"],
            "edges": row["edges"], "variables": row["variables"], "downloadCount": row["download_count"],
            "rating": row["rating"], "ratingCount": row["rating_count"], "status": row["status"],
            "isOfficial": row["is_official"], "userId": row["user_id"], "createdAt": row["created_at"], "updatedAt": row["updated_at"]}


async def get_template_row(session: AsyncSession, template_id: str) -> Any:
    result = await session.execute(text("""SELECT id::text,name,description,icon,screenshot,category,tags,nodes,edges,variables,download_count,rating,rating_count,status,is_official,user_id::text,created_at,updated_at
        FROM control.workflow_templates WHERE id=CAST(:id AS uuid)"""), {"id": require_uuid(template_id, "id")})
    row = result.mappings().first()
    if row is None: raise APIError(404, "NOT_FOUND", "Template not found")
    return row


def require_template_owner(row: Any, principal: Principal) -> None:
    if row["user_id"] != principal.user_id and principal.global_role != "admin":
        raise APIError(403, "FORBIDDEN", "Only the template owner can modify it")


@router.post("", status_code=201)
async def create_template(body: CreateTemplateRequest, principal: CurrentUser, session: Session) -> Any:
    nodes: list[Any] = []; edges: list[Any] = []; variables: dict[str, Any] = {}
    if body.source_workflow_id:
        workflow = await load_workflow(session, principal, body.source_workflow_id)
        nodes, edges, variables = workflow["nodes"], workflow["edges"], workflow["variables"]
    result = await session.execute(text("""INSERT INTO control.workflow_templates(name,description,icon,screenshot,category,tags,nodes,edges,variables,is_official,user_id)
        VALUES(:name,:description,:icon,:screenshot,:category,CAST(:tags AS jsonb),CAST(:nodes AS jsonb),CAST(:edges AS jsonb),CAST(:variables AS jsonb),:official,CAST(:user AS uuid))
        RETURNING id::text,name,description,icon,screenshot,category,tags,nodes,edges,variables,download_count,rating,rating_count,status,is_official,user_id::text,created_at,updated_at"""),
        {"name": validate_name(body.name,100), "description": body.description, "icon": body.icon, "screenshot": body.screenshot,
         "category": body.category, "tags": json.dumps(body.tags), "nodes": json.dumps(nodes), "edges": json.dumps(edges),
         "variables": json.dumps(variables), "official": body.is_official and principal.global_role == "admin", "user": principal.user_id})
    await session.commit()
    return success(template_data(result.mappings().one()), status_code=201)


@router.get("")
async def list_templates(
    _principal: CurrentUser, session: Session, keyword: str | None = None, category: str | None = None,
    tag: str | None = None, is_official: bool | None = Query(default=None, alias="isOfficial"), sort: str = "newest",
    page: int = Query(default=1, ge=1), page_size: int = Query(default=20, ge=1, le=100, alias="pageSize"),
) -> Any:
    conditions = ["status='published'"]; params: dict[str, Any] = {"limit": page_size, "offset": (page-1)*page_size}
    if keyword: conditions.append("(name ILIKE :keyword OR description ILIKE :keyword)"); params["keyword"] = f"%{keyword}%"
    if category: conditions.append("category=:category"); params["category"] = category
    if tag: conditions.append("tags ? :tag"); params["tag"] = tag
    if is_official is not None: conditions.append("is_official=:official"); params["official"] = is_official
    order = {"popular": "download_count DESC", "rating": "rating DESC", "newest": "created_at DESC"}.get(sort, "created_at DESC")
    where = " AND ".join(conditions)
    total = (await session.execute(text(f"SELECT COUNT(*) FROM control.workflow_templates WHERE {where}"), params)).scalar_one()
    result = await session.execute(text(f"""SELECT id::text,name,description,icon,screenshot,category,tags,nodes,edges,variables,download_count,rating,rating_count,status,is_official,user_id::text,created_at,updated_at
        FROM control.workflow_templates WHERE {where} ORDER BY {order} LIMIT :limit OFFSET :offset"""), params)
    return success({"items": [template_data(row) for row in result.mappings()], "total": total, "page": page, "pageSize": page_size, "totalPages": math.ceil(total/page_size)})


@router.get("/categories")
async def categories(_principal: CurrentUser, session: Session) -> Any:
    result = await session.execute(text("SELECT category,COUNT(*) AS count FROM control.workflow_templates WHERE status='published' GROUP BY category ORDER BY category"))
    return success([{"category": row["category"], "count": row["count"]} for row in result.mappings()])


@router.get("/{template_id}")
async def get_template(template_id: str, _principal: CurrentUser, session: Session) -> Any:
    return success(template_data(await get_template_row(session, template_id)))


@router.patch("/{template_id}")
async def update_template(body: UpdateTemplateRequest, template_id: str, principal: CurrentUser, session: Session) -> Any:
    current = await get_template_row(session, template_id); require_template_owner(current, principal)
    assignments: list[str] = []; values: dict[str, Any] = {"id": current["id"]}
    for field in ("name","description","icon","screenshot","category","tags"):
        if field not in body.model_fields_set: continue
        value = getattr(body,field)
        if field == "name":
            if value is None: raise APIError(400,"BAD_REQUEST","Name must be a string")
            value = validate_name(value,100)
        if field == "tags": assignments.append("tags=CAST(:tags AS jsonb)"); value=json.dumps(value or [])
        else: assignments.append(f"{field}=:{field}")
        values[field]=value
    if not assignments: assignments.append("updated_at=updated_at")
    result=await session.execute(text(f"""UPDATE control.workflow_templates SET {','.join(assignments)} WHERE id=CAST(:id AS uuid)
        RETURNING id::text,name,description,icon,screenshot,category,tags,nodes,edges,variables,download_count,rating,rating_count,status,is_official,user_id::text,created_at,updated_at"""),values)
    await session.commit(); return success(template_data(result.mappings().one()))


async def set_template_status(template_id: str, status: str, principal: Principal, session: AsyncSession) -> Any:
    current=await get_template_row(session,template_id); require_template_owner(current,principal)
    result=await session.execute(text("UPDATE control.workflow_templates SET status=:status WHERE id=CAST(:id AS uuid) RETURNING id::text,name,status"),{"status":status,"id":current["id"]})
    await session.commit(); return success(dict(result.mappings().one()))


@router.post("/{template_id}/publish", status_code=201)
async def publish_template(template_id: str, principal: CurrentUser, session: Session) -> Any:
    response=await set_template_status(template_id,"published",principal,session); response.status_code=201; return response


@router.post("/{template_id}/archive", status_code=201)
async def archive_template(template_id: str, principal: CurrentUser, session: Session) -> Any:
    response=await set_template_status(template_id,"archived",principal,session); response.status_code=201; return response


@router.post("/{template_id}/import", status_code=201)
async def import_template(body: ImportTemplateRequest, template_id: str, principal: CurrentUser, session: Session) -> Any:
    template=await get_template_row(session,template_id); app_id=require_uuid(body.application_id,"applicationId")
    await authorize_application(session,principal,app_id,"workflow:create")
    result=await session.execute(text("""INSERT INTO control.workflows(name,description,application_id,owner_id,nodes,edges,variables)
        VALUES(:name,:description,CAST(:app AS uuid),CAST(:owner AS uuid),CAST(:nodes AS jsonb),CAST(:edges AS jsonb),CAST(:variables AS jsonb))
        RETURNING id::text,name,description,application_id::text,nodes,edges,variables,current_version,created_at,updated_at"""),
        {"name":validate_name(body.name or template["name"],100),"description":template["description"],"app":app_id,"owner":principal.user_id,
         "nodes":json.dumps(template["nodes"]),"edges":json.dumps(template["edges"]),"variables":json.dumps(template["variables"])})
    await session.execute(text("UPDATE control.workflow_templates SET download_count=download_count+1 WHERE id=CAST(:id AS uuid)"),{"id":template["id"]})
    await session.commit(); return success(workflow_data(result.mappings().one()),status_code=201)


@router.post("/{template_id}/rate", status_code=201)
async def rate_template(body: RateTemplateRequest, template_id: str, principal: CurrentUser, session: Session) -> Any:
    template=await get_template_row(session,template_id)
    await session.execute(text("""INSERT INTO control.template_ratings(template_id,user_id,rating) VALUES(CAST(:template AS uuid),CAST(:user AS uuid),:rating)
        ON CONFLICT(template_id,user_id) DO UPDATE SET rating=EXCLUDED.rating,updated_at=now()"""),{"template":template["id"],"user":principal.user_id,"rating":body.rating})
    await session.execute(text("""UPDATE control.workflow_templates t SET rating=x.average,rating_count=x.count
        FROM (SELECT AVG(rating)::float AS average,COUNT(*)::int AS count FROM control.template_ratings WHERE template_id=CAST(:id AS uuid)) x WHERE t.id=CAST(:id AS uuid)"""),{"id":template["id"]})
    await session.commit(); updated=await get_template_row(session,template_id)
    return success({"rating":updated["rating"],"ratingCount":updated["rating_count"]},status_code=201)


@router.delete("/{template_id}")
async def delete_template(template_id: str, principal: CurrentUser, session: Session) -> Any:
    template=await get_template_row(session,template_id); require_template_owner(template,principal)
    await session.execute(text("DELETE FROM control.workflow_templates WHERE id=CAST(:id AS uuid)"),{"id":template["id"]}); await session.commit()
    return success({"success":True})

from __future__ import annotations

from datetime import datetime
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)


class RegisterRequest(StrictModel):
    username: str
    password: str


class LoginRequest(RegisterRequest):
    pass


class UpdateProfileRequest(StrictModel):
    username: str | None = None
    avatar: str | None = None


class CreateApplicationRequest(StrictModel):
    name: str
    description: str | None = None
    icon: str | None = None
    status: Literal["draft", "published", "archived"] = "draft"


class UpdateApplicationRequest(StrictModel):
    name: str | None = None
    description: str | None = None
    icon: str | None = None
    status: Literal["draft", "published", "archived"] | None = None


class CreateTeamRequest(StrictModel):
    name: str
    description: str | None = None
    avatar: str | None = None


class UpdateTeamRequest(StrictModel):
    name: str | None = None
    description: str | None = None
    avatar: str | None = None


class AddMemberRequest(StrictModel):
    user_id: str = Field(alias="userId")
    role: Literal["admin", "editor", "viewer"] = "viewer"


class UpdateMemberRoleRequest(StrictModel):
    role: Literal["admin", "editor", "viewer"]


class AddTeamApplicationRequest(StrictModel):
    application_id: str = Field(alias="applicationId")
    permission: Literal["full_access", "can_edit", "can_view"] = "can_view"


class UpdateTeamApplicationRequest(StrictModel):
    permission: Literal["full_access", "can_edit", "can_view"]


APIKeyScope = Literal[
    "app:read",
    "app:write",
    "app:execute",
    "workflow:read",
    "workflow:write",
    "workflow:execute",
    "knowledge:read",
    "knowledge:write",
]


class CreateAPIKeyRequest(StrictModel):
    name: str
    application_id: str | None = Field(default=None, alias="applicationId")
    scopes: list[APIKeyScope] | None = None
    expires_at: datetime | None = Field(default=None, alias="expiresAt")


class ToggleAPIKeyRequest(StrictModel):
    is_active: bool = Field(alias="isActive")


class EmbedConfig(StrictModel):
    allowed_origins: list[str] = Field(default_factory=list, alias="allowedOrigins")
    theme: Literal["light", "dark", "auto"] = "light"
    enabled: bool | None = None
    width: str = ""
    height: str = ""
    show_header: bool | None = Field(default=None, alias="showHeader")


class UpdateShareRequest(StrictModel):
    is_public: bool | None = Field(default=None, alias="isPublic")
    embed_config: EmbedConfig | None = Field(default=None, alias="embedConfig")

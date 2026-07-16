from __future__ import annotations

from typing import Any, Literal

from pydantic import Field

from aiflow_runtime.identity.schemas import StrictModel


class CreateWorkflowRequest(StrictModel):
    name: str
    description: str | None = None
    application_id: str = Field(alias="applicationId")
    nodes: list[dict[str, Any]] = Field(default_factory=list)
    edges: list[dict[str, Any]] = Field(default_factory=list)
    variables: dict[str, Any] = Field(default_factory=dict)


class UpdateWorkflowRequest(StrictModel):
    name: str | None = None
    description: str | None = None
    application_id: str | None = Field(default=None, alias="applicationId")
    nodes: list[dict[str, Any]] | None = None
    edges: list[dict[str, Any]] | None = None
    variables: dict[str, Any] | None = None


class ExecutionControl(StrictModel):
    workflow_timeout_ms: int = Field(default=300000, ge=0, le=3600000, alias="workflowTimeoutMs")
    node_timeout_ms: int = Field(default=60000, ge=0, le=600000, alias="nodeTimeoutMs")
    heartbeat_interval_ms: int = Field(default=15000, ge=0, le=60000, alias="heartbeatIntervalMs")
    max_retries: int = Field(default=0, ge=0, le=5, alias="maxRetries")
    continue_on_error: bool = Field(default=False, alias="continueOnError")


class RunWorkflowRequest(StrictModel):
    inputs: dict[str, Any]
    session_id: str | None = Field(default=None, alias="sessionId")
    control: ExecutionControl = Field(default_factory=ExecutionControl)


class CreateVersionRequest(StrictModel):
    label: str | None = None
    description: str | None = None
    is_published: bool = Field(default=False, alias="isPublished")


TemplateCategory = Literal["productivity", "customer-service", "content-creation", "data-analysis", "education", "development", "other"]


class CreateTemplateRequest(StrictModel):
    name: str
    description: str | None = None
    icon: str | None = None
    screenshot: str | None = None
    category: TemplateCategory
    tags: list[str] = Field(default_factory=list)
    is_official: bool = Field(default=False, alias="isOfficial")
    source_workflow_id: str | None = Field(default=None, alias="sourceWorkflowId")


class UpdateTemplateRequest(StrictModel):
    name: str | None = None
    description: str | None = None
    icon: str | None = None
    screenshot: str | None = None
    category: TemplateCategory | None = None
    tags: list[str] | None = None


class RateTemplateRequest(StrictModel):
    rating: int = Field(ge=1, le=5)


class ImportTemplateRequest(StrictModel):
    application_id: str = Field(alias="applicationId")
    name: str | None = None


class ImportDSLRequest(StrictModel):
    dsl: str
    format: Literal["yaml", "json"]
    application_id: str | None = Field(default=None, alias="applicationId")
    name_override: str | None = Field(default=None, alias="nameOverride")

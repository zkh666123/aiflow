from __future__ import annotations

from typing import Any, TypedDict


class WorkflowSnapshot(TypedDict):
    nodes: list[dict[str, Any]]
    edges: list[dict[str, Any]]
    variables: dict[str, Any]

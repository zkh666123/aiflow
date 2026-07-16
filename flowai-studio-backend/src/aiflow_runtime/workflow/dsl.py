from __future__ import annotations

import json
from datetime import UTC, datetime
from typing import Any

import yaml

from aiflow_runtime.api.errors import APIError

NODE_TYPES = {"start", "userInput", "llm", "rag", "skill", "condition", "output", "agent"}


def parse_dsl(content: str, format_name: str) -> dict[str, Any]:
    try:
        value = json.loads(content) if format_name == "json" else yaml.safe_load(content)
    except Exception as exc:
        raise APIError(400, "BAD_REQUEST", "Invalid workflow DSL") from exc
    if not isinstance(value, dict):
        raise APIError(400, "BAD_REQUEST", "Workflow DSL must be an object")
    return value


def validate_dsl(value: dict[str, Any]) -> dict[str, Any]:
    errors: list[str] = []
    if value.get("version") != "1.0": errors.append("version must be 1.0")
    if value.get("kind") != "Workflow": errors.append("kind must be Workflow")
    spec = value.get("spec")
    if not isinstance(spec, dict): errors.append("spec must be an object"); spec = {}
    nodes = spec.get("nodes", [])
    edges = spec.get("edges", [])
    if not isinstance(nodes, list): errors.append("spec.nodes must be an array"); nodes = []
    if not isinstance(edges, list): errors.append("spec.edges must be an array"); edges = []
    ids: set[str] = set()
    for node in nodes:
        if not isinstance(node, dict) or not node.get("id"): errors.append("every node requires id"); continue
        if node["id"] in ids: errors.append(f"duplicate node id: {node['id']}")
        ids.add(node["id"])
        node_type = "userInput" if node.get("type") == "user-input" else node.get("type")
        if node_type not in NODE_TYPES: errors.append(f"unsupported node type: {node_type}")
        node["type"] = node_type
    for edge in edges:
        if not isinstance(edge, dict) or edge.get("source") not in ids or edge.get("target") not in ids:
            errors.append("edge source and target must reference nodes")
    if errors:
        raise APIError(400, "BAD_REQUEST", "Workflow DSL validation failed", errors)
    return {"nodes": nodes, "edges": edges, "variables": spec.get("variables") or {}}


def export_document(row: Any) -> dict[str, Any]:
    return {
        "version": "1.0", "kind": "Workflow",
        "metadata": {"name": row["name"], "description": row["description"] or "", "exportedAt": datetime.now(UTC).isoformat(), "engine": "FlowAI Studio Python"},
        "spec": {"nodes": row["nodes"], "edges": row["edges"], "variables": row["variables"]},
    }

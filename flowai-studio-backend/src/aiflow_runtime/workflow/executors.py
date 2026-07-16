from __future__ import annotations

import operator
import re
from dataclasses import dataclass, field
from typing import Any, Protocol


@dataclass(slots=True)
class NodeResult:
    output: Any
    branch: bool | None = None
    traces: list[dict[str, Any]] = field(default_factory=list)


class ExternalNodeServices(Protocol):
    async def execute(self, node_type: str, node: dict[str, Any], context: dict[str, Any], inputs: dict[str, Any]) -> NodeResult: ...


class UnavailableNodeServices:
    async def execute(self, node_type: str, node: dict[str, Any], context: dict[str, Any], inputs: dict[str, Any]) -> NodeResult:
        raise RuntimeError(f"{node_type} service is not configured")


def resolve(value: Any, context: dict[str, Any], inputs: dict[str, Any]) -> Any:
    if not isinstance(value, str): return value
    values = {**inputs, **context}
    exact = re.fullmatch(r"\{\{\s*([^}]+)\s*\}\}", value)
    if exact: return nested(values, exact.group(1).strip())
    return re.sub(r"\{\{\s*([^}]+)\s*\}\}", lambda match: str(nested(values, match.group(1).strip()) or ""), value)


def nested(values: dict[str, Any], path: str) -> Any:
    current: Any = values
    for part in path.split("."):
        if not isinstance(current, dict): return None
        current = current.get(part)
    return current


def evaluate_condition(data: dict[str, Any], context: dict[str, Any], inputs: dict[str, Any]) -> bool:
    conditions = data.get("conditions") or []
    operations = {"eq": operator.eq, "==": operator.eq, "neq": operator.ne, "!=": operator.ne, "gt": operator.gt,
                  ">": operator.gt, "gte": operator.ge, ">=": operator.ge, "lt": operator.lt, "<": operator.lt,
                  "lte": operator.le, "<=": operator.le, "contains": lambda left, right: right in left}
    for condition in conditions:
        left = nested({**inputs, **context}, str(condition.get("variable", "")))
        right = resolve(condition.get("value"), context, inputs)
        operation = operations.get(str(condition.get("operator", "eq")))
        if operation is None or not operation(left, right): return False
    return True


async def execute_node(node: dict[str, Any], context: dict[str, Any], inputs: dict[str, Any], services: ExternalNodeServices) -> NodeResult:
    node_type = node["type"]; data = node.get("data") or {}
    if node_type == "start":
        values = dict(inputs)
        for variable in data.get("variables") or []: values[str(variable.get("key"))] = resolve(variable.get("value"), context, inputs)
        return NodeResult(values)
    if node_type == "userInput":
        field_name = str(data.get("inputField") or "input")
        return NodeResult(inputs.get(field_name))
    if node_type == "condition":
        selected = evaluate_condition(data, context, inputs)
        return NodeResult({"result": selected}, branch=selected)
    if node_type == "output":
        return NodeResult(resolve(data.get("outputValue"), context, inputs))
    return await services.execute(node_type, node, context, inputs)

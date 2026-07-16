from __future__ import annotations

from collections import defaultdict, deque
from dataclasses import dataclass
from typing import Any

from aiflow_runtime.api.errors import APIError
from aiflow_runtime.workflow.dsl import NODE_TYPES


@dataclass(slots=True)
class PreparedGraph:
    nodes: dict[str, dict[str, Any]]
    outgoing: dict[str, list[dict[str, Any]]]
    incoming: dict[str, list[dict[str, Any]]]
    indegree: dict[str, int]
    start_id: str

    def branch_prune(self, node_id: str, selected: bool) -> set[str]:
        edges = self.outgoing.get(node_id, [])
        selected_edges = [edge for edge in edges if branch_value(edge) is selected]
        other_edges = [edge for edge in edges if edge not in selected_edges]
        selected_reachable = self.reachable({edge["target"] for edge in selected_edges})
        other_reachable = self.reachable({edge["target"] for edge in other_edges})
        return other_reachable - selected_reachable

    def reachable(self, roots: set[str]) -> set[str]:
        seen: set[str] = set()
        queue = deque(roots)
        while queue:
            node_id = queue.popleft()
            if node_id in seen:
                continue
            seen.add(node_id)
            queue.extend(edge["target"] for edge in self.outgoing.get(node_id, []))
        return seen


def branch_value(edge: dict[str, Any]) -> bool:
    value = str(edge.get("sourceHandle") or edge.get("label") or "true").lower()
    return value not in {"false", "0", "no", "else"}


def prepare_graph(nodes: list[dict[str, Any]], edges: list[dict[str, Any]]) -> PreparedGraph:
    node_map: dict[str, dict[str, Any]] = {}
    for node in nodes:
        node_id = str(node.get("id") or "")
        node_type = node.get("type")
        if not node_id or node_id in node_map:
            raise APIError(400, "BAD_REQUEST", "Workflow node IDs must be unique and non-empty")
        if node_type not in NODE_TYPES:
            raise APIError(400, "BAD_REQUEST", f"Unsupported node type: {node_type}")
        node_map[node_id] = node
    starts = [node_id for node_id, node in node_map.items() if node.get("type") == "start"]
    if len(starts) != 1:
        raise APIError(400, "BAD_REQUEST", "Workflow must contain exactly one Start node")
    outgoing: dict[str, list[dict[str, Any]]] = defaultdict(list)
    incoming: dict[str, list[dict[str, Any]]] = defaultdict(list)
    indegree = {node_id: 0 for node_id in node_map}
    edge_ids: set[str] = set()
    for edge in edges:
        edge_id = str(edge.get("id") or "")
        source, target = str(edge.get("source") or ""), str(edge.get("target") or "")
        if not edge_id or edge_id in edge_ids or source not in node_map or target not in node_map:
            raise APIError(400, "BAD_REQUEST", "Workflow edges must be unique and reference valid nodes")
        edge_ids.add(edge_id); outgoing[source].append(edge); incoming[target].append(edge); indegree[target] += 1
    runtime_indegree = dict(indegree)
    queue = deque(node_id for node_id, degree in runtime_indegree.items() if degree == 0)
    visited: list[str] = []
    while queue:
        node_id = queue.popleft(); visited.append(node_id)
        for edge in outgoing[node_id]:
            runtime_indegree[edge["target"]] -= 1
            if runtime_indegree[edge["target"]] == 0: queue.append(edge["target"])
    if len(visited) != len(node_map):
        raise APIError(400, "BAD_REQUEST", "Workflow graph contains a cycle")
    graph = PreparedGraph(node_map, dict(outgoing), dict(incoming), indegree, starts[0])
    reachable = graph.reachable({starts[0]})
    if any(node_id not in reachable for node_id in node_map):
        raise APIError(400, "BAD_REQUEST", "Workflow contains unreachable nodes")
    if not any(node_map[node_id].get("type") == "output" for node_id in reachable):
        raise APIError(400, "BAD_REQUEST", "Workflow must contain a reachable Output node")
    return graph

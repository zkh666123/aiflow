import pytest

from aiflow_runtime.api.errors import APIError
from aiflow_runtime.workflow.dsl import validate_dsl
from aiflow_runtime.workflow.graph import prepare_graph


def node(node_id, kind):
    return {"id": node_id, "type": kind, "position": {"x": 0, "y": 0}, "data": {}}


def edge(edge_id, source, target, handle=None):
    value = {"id": edge_id, "source": source, "target": target}
    if handle is not None:
        value["sourceHandle"] = handle
    return value


def test_graph_builds_join_indegree():
    graph = prepare_graph([node("s", "start"), node("a", "llm"), node("b", "rag"), node("o", "output")], [edge("1", "s", "a"), edge("2", "s", "b"), edge("3", "a", "o"), edge("4", "b", "o")])
    assert graph.indegree["o"] == 2


def test_graph_rejects_cycle():
    with pytest.raises(APIError, match="cycle"):
        prepare_graph([node("s", "start"), node("o", "output")], [edge("1", "s", "o"), edge("2", "o", "s")])


def test_condition_prunes_only_exclusive_branch():
    graph = prepare_graph([node("s", "start"), node("c", "condition"), node("t", "llm"), node("f", "rag"), node("o", "output")], [edge("1", "s", "c"), edge("2", "c", "t", "true"), edge("3", "c", "f", "false"), edge("4", "t", "o"), edge("5", "f", "o")])
    assert graph.branch_prune("c", True) == {"f"}


def test_dsl_normalizes_legacy_user_input():
    document = {"version": "1.0", "kind": "Workflow", "metadata": {}, "spec": {"nodes": [node("s", "start"), node("i", "user-input"), node("o", "output")], "edges": [edge("1", "s", "i"), edge("2", "i", "o")]}}
    snapshot = validate_dsl(document)
    assert snapshot["nodes"][1]["type"] == "userInput"

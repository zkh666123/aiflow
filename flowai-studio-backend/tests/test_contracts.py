import json
import re
from pathlib import Path

from aiflow_runtime.app import app


def routes(items, prefix=""):
    for route in items:
        if hasattr(route, "methods") and hasattr(route, "path"):
            for method in route.methods or []:
                if method not in {"HEAD", "OPTIONS"}:
                    yield method, prefix + route.path
        elif hasattr(route, "original_router"):
            yield from routes(route.original_router.routes, prefix + route.include_context.prefix)


def normalized(item):
    return item[0], re.sub(r":[^/]+|\{[^}]+\}", ":param", item[1])


def test_all_frozen_routes_are_registered():
    contract = json.loads((Path(__file__).parents[2] / "contracts/http/routes.json").read_text(encoding="utf-8"))
    expected = {normalized((item["method"], item["path"])) for item in contract["routes"]}
    actual = {normalized(item) for item in routes(app.routes)}
    assert expected <= actual


def test_response_envelope_shape():
    from aiflow_runtime.api.envelope import payload
    assert set(payload(success=True, code="SUCCESS", message="ok", data={})) == {"success", "code", "message", "data", "timestamp"}

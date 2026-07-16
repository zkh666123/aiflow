from __future__ import annotations

import json
import time
from typing import Any


def event(event_type: str, data: dict[str, Any]) -> dict[str, Any]:
    return {"type": event_type, "data": data}


def sse(value: dict[str, Any]) -> str:
    return f"data: {json.dumps(value, ensure_ascii=False, separators=(',', ':'), default=str)}\n\n"


def progress(executed: int, total: int, current: str | None = None) -> dict[str, Any]:
    data: dict[str, Any] = {"executed": executed, "total": total, "percentage": int(executed * 100 / total) if total else 100}
    if current is not None: data["currentNode"] = current
    return data


def now_ms() -> int:
    return int(time.time() * 1000)

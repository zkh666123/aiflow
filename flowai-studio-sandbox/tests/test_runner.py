import sys
from pathlib import Path

import pytest

from aiflow.v1 import sandbox_pb2
from aiflow_sandbox.native.runner import NativePythonRunner


@pytest.mark.asyncio
async def test_runner_rejects_imports() -> None:
    result = await NativePythonRunner(Path(sys.executable)).execute(
        "import socket\nprint(socket.gethostname())",
        sandbox_pb2.SandboxLimits(timeout_millis=1_000, output_bytes=4_096),
    )
    assert result.status == sandbox_pb2.SANDBOX_EXECUTION_STATUS_FAILED
    assert "Import is not available" in result.stderr


@pytest.mark.asyncio
async def test_runner_enforces_timeout() -> None:
    result = await NativePythonRunner(Path(sys.executable)).execute(
        "while True:\n    pass",
        sandbox_pb2.SandboxLimits(timeout_millis=50, output_bytes=4_096),
    )
    assert result.status == sandbox_pb2.SANDBOX_EXECUTION_STATUS_TIMED_OUT
    assert result.failure_code == sandbox_pb2.SANDBOX_FAILURE_CODE_TIMEOUT


@pytest.mark.asyncio
async def test_runner_enforces_output_limit() -> None:
    result = await NativePythonRunner(Path(sys.executable)).execute(
        "print('x' * 2048)",
        sandbox_pb2.SandboxLimits(timeout_millis=1_000, output_bytes=1_024),
    )
    assert result.status == sandbox_pb2.SANDBOX_EXECUTION_STATUS_RESOURCE_EXHAUSTED
    assert result.failure_code == sandbox_pb2.SANDBOX_FAILURE_CODE_OUTPUT_LIMIT

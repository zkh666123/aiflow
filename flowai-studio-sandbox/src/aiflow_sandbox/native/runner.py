from __future__ import annotations

import asyncio
import os
import subprocess
import sys
import tempfile
import time
from dataclasses import dataclass
from pathlib import Path

from aiflow.v1 import sandbox_pb2


@dataclass(frozen=True, slots=True)
class ExecutionResult:
    status: int
    failure_code: int
    exit_code: int
    stdout: str
    stderr: str
    duration_millis: int


class NativePythonRunner:
    def __init__(self, executable: Path) -> None:
        self.executable = executable.resolve()
        self.child = Path(__file__).with_name("child.py").resolve()

    @property
    def ready(self) -> bool:
        return self.executable.is_file() and self.child.is_file()

    async def execute(self, code: str, limits: sandbox_pb2.SandboxLimits) -> ExecutionResult:
        started = time.perf_counter()
        if not self.ready:
            return self._result(
                sandbox_pb2.SANDBOX_EXECUTION_STATUS_NOT_READY,
                sandbox_pb2.SANDBOX_FAILURE_CODE_RUNTIME_MISSING,
                -1,
                "",
                "sandbox runtime is not ready",
                started,
            )
        if len(code.encode("utf-8")) > 64 * 1024:
            return self._result(
                sandbox_pb2.SANDBOX_EXECUTION_STATUS_RESOURCE_EXHAUSTED,
                sandbox_pb2.SANDBOX_FAILURE_CODE_OUTPUT_LIMIT,
                -1,
                "",
                "source code exceeds 64 KiB",
                started,
            )

        timeout = min(max((limits.timeout_millis or 10_000) / 1000, 0.05), 10.0)
        output_limit = min(max(limits.output_bytes or 65_536, 1_024), 1_048_576)
        creationflags = subprocess.CREATE_NO_WINDOW if sys.platform == "win32" else 0
        environment = {
            "PYTHONHASHSEED": "0",
            "PYTHONIOENCODING": "utf-8",
            "PYTHONUTF8": "1",
        }
        for name in ("SYSTEMROOT", "WINDIR", "TEMP", "TMP"):
            if value := os.environ.get(name):
                environment[name] = value

        with tempfile.TemporaryDirectory(prefix="flowai-sandbox-") as working_directory:
            process = await asyncio.create_subprocess_exec(
                str(self.executable),
                "-I",
                "-S",
                str(self.child),
                cwd=working_directory,
                env=environment,
                stdin=asyncio.subprocess.PIPE,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
                creationflags=creationflags,
            )
            try:
                stdout, stderr, overflowed = await asyncio.wait_for(
                    self._communicate_limited(process, code.encode("utf-8"), output_limit),
                    timeout=timeout,
                )
            except TimeoutError:
                process.kill()
                await process.wait()
                return self._result(
                    sandbox_pb2.SANDBOX_EXECUTION_STATUS_TIMED_OUT,
                    sandbox_pb2.SANDBOX_FAILURE_CODE_TIMEOUT,
                    -1,
                    "",
                    "execution timed out",
                    started,
                )
            except asyncio.CancelledError:
                process.kill()
                await process.wait()
                raise

        if overflowed:
            return self._result(
                sandbox_pb2.SANDBOX_EXECUTION_STATUS_RESOURCE_EXHAUSTED,
                sandbox_pb2.SANDBOX_FAILURE_CODE_OUTPUT_LIMIT,
                process.returncode or -1,
                stdout.decode("utf-8", errors="replace"),
                "output limit exceeded",
                started,
            )
        status = (
            sandbox_pb2.SANDBOX_EXECUTION_STATUS_SUCCEEDED
            if process.returncode == 0
            else sandbox_pb2.SANDBOX_EXECUTION_STATUS_FAILED
        )
        failure = (
            sandbox_pb2.SANDBOX_FAILURE_CODE_UNSPECIFIED
            if process.returncode == 0
            else sandbox_pb2.SANDBOX_FAILURE_CODE_GUEST_ERROR
        )
        return self._result(
            status,
            failure,
            process.returncode or 0,
            stdout.decode("utf-8", errors="replace"),
            stderr.decode("utf-8", errors="replace"),
            started,
        )

    @staticmethod
    async def _communicate_limited(
        process: asyncio.subprocess.Process,
        payload: bytes,
        limit: int,
    ) -> tuple[bytes, bytes, bool]:
        assert process.stdin is not None
        assert process.stdout is not None
        assert process.stderr is not None
        remaining = limit
        lock = asyncio.Lock()
        overflow = asyncio.Event()

        async def read_stream(stream: asyncio.StreamReader) -> bytes:
            nonlocal remaining
            chunks: list[bytes] = []
            while chunk := await stream.read(4096):
                async with lock:
                    allowed = min(len(chunk), remaining)
                    if allowed:
                        chunks.append(chunk[:allowed])
                        remaining -= allowed
                    if allowed < len(chunk):
                        overflow.set()
                if overflow.is_set():
                    break
            return b"".join(chunks)

        stdout_task = asyncio.create_task(read_stream(process.stdout))
        stderr_task = asyncio.create_task(read_stream(process.stderr))
        wait_task = asyncio.create_task(process.wait())
        overflow_task = asyncio.create_task(overflow.wait())
        process.stdin.write(payload)
        await process.stdin.drain()
        process.stdin.close()

        try:
            await asyncio.wait(
                {wait_task, overflow_task},
                return_when=asyncio.FIRST_COMPLETED,
            )
            if overflow.is_set() and process.returncode is None:
                process.kill()
            await wait_task
            stdout, stderr = await asyncio.gather(stdout_task, stderr_task)
            return stdout, stderr, overflow.is_set()
        finally:
            for task in (stdout_task, stderr_task, wait_task, overflow_task):
                if not task.done():
                    task.cancel()
            await asyncio.gather(
                stdout_task,
                stderr_task,
                wait_task,
                overflow_task,
                return_exceptions=True,
            )

    @staticmethod
    def _result(
        status: int,
        failure_code: int,
        exit_code: int,
        stdout: str,
        stderr: str,
        started: float,
    ) -> ExecutionResult:
        return ExecutionResult(
            status=status,
            failure_code=failure_code,
            exit_code=exit_code,
            stdout=stdout,
            stderr=stderr,
            duration_millis=max(0, round((time.perf_counter() - started) * 1000)),
        )

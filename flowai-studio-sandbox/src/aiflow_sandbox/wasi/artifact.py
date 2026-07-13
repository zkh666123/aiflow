from __future__ import annotations

from dataclasses import dataclass
from hashlib import sha256
from pathlib import Path

from aiflow.v1 import sandbox_pb2


@dataclass(frozen=True, slots=True)
class ArtifactState:
    ready: bool
    failure_code: int


@dataclass(frozen=True, slots=True)
class WasiArtifact:
    path: Path
    expected_sha256: str

    def inspect(self) -> ArtifactState:
        if not self.path.is_file():
            return ArtifactState(
                ready=False,
                failure_code=sandbox_pb2.SANDBOX_FAILURE_CODE_RUNTIME_MISSING,
            )
        if len(self.expected_sha256) != 64:
            return ArtifactState(
                ready=False,
                failure_code=sandbox_pb2.SANDBOX_FAILURE_CODE_RUNTIME_INVALID,
            )

        digest = sha256()
        with self.path.open("rb") as handle:
            for chunk in iter(lambda: handle.read(1024 * 1024), b""):
                digest.update(chunk)

        if digest.hexdigest() != self.expected_sha256.lower():
            return ArtifactState(
                ready=False,
                failure_code=sandbox_pb2.SANDBOX_FAILURE_CODE_RUNTIME_INVALID,
            )
        return ArtifactState(
            ready=True,
            failure_code=sandbox_pb2.SANDBOX_FAILURE_CODE_UNSPECIFIED,
        )

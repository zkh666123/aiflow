from hashlib import sha256

from aiflow.v1 import sandbox_pb2
from aiflow_sandbox.wasi.artifact import WasiArtifact


def test_missing_artifact_is_not_ready(tmp_path) -> None:
    artifact = WasiArtifact(tmp_path / "python.wasm", "a" * 64)

    state = artifact.inspect()

    assert not state.ready
    assert state.failure_code == sandbox_pb2.SANDBOX_FAILURE_CODE_RUNTIME_MISSING


def test_artifact_requires_the_expected_sha256(tmp_path) -> None:
    path = tmp_path / "python.wasm"
    path.write_bytes(b"verified-wasi-runtime")
    expected = sha256(path.read_bytes()).hexdigest()

    assert WasiArtifact(path, expected).inspect().ready
    invalid = WasiArtifact(path, "0" * 64).inspect()
    assert not invalid.ready
    assert invalid.failure_code == sandbox_pb2.SANDBOX_FAILURE_CODE_RUNTIME_INVALID

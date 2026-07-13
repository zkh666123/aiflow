import pytest
from pydantic import ValidationError

from aiflow_runtime.config import Settings


def make_settings(**overrides: str) -> Settings:
    values = {
        "grpc_address": "127.0.0.1:50051",
        "grpc_token": "t" * 43,
        "database_url": "postgresql+psycopg://user:pass@127.0.0.1/db",
        "redis_url": "redis://127.0.0.1:6379/0",
    }
    values.update(overrides)
    return Settings(**values)


def test_accepts_loopback_runtime_configuration() -> None:
    settings = make_settings()

    assert settings.grpc_address == "127.0.0.1:50051"
    assert settings.health_timeout_seconds == 2.0


@pytest.mark.parametrize("address", ["0.0.0.0:50051", "192.168.1.10:50051", "localhost"])
def test_rejects_non_loopback_or_incomplete_grpc_addresses(address: str) -> None:
    with pytest.raises(ValidationError):
        make_settings(grpc_address=address)


@pytest.mark.parametrize("token", ["", "short", " " * 32])
def test_rejects_missing_or_weak_service_tokens(token: str) -> None:
    with pytest.raises(ValidationError):
        make_settings(grpc_token=token)

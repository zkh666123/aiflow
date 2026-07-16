from __future__ import annotations

import ipaddress
import sys
from pathlib import Path

from pydantic import Field, SecretStr, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=None,
        extra="ignore",
        populate_by_name=True,
    )

    grpc_address: str = Field(
        default="127.0.0.1:50052",
        validation_alias="FLOWAI_SANDBOX_GRPC_ADDR",
    )
    grpc_token: SecretStr = Field(validation_alias="FLOWAI_GRPC_TOKEN")
    python_executable: Path = Field(
        default=Path(sys.executable),
        validation_alias="FLOWAI_SANDBOX_PYTHON",
    )

    @field_validator("grpc_address")
    @classmethod
    def validate_loopback_address(cls, value: str) -> str:
        try:
            host, port_text = value.rsplit(":", 1)
            port = int(port_text)
        except (ValueError, TypeError) as exc:
            raise ValueError("gRPC address must use host:port") from exc
        if not 1 <= port <= 65535:
            raise ValueError("gRPC port is outside the valid range")
        if host != "localhost":
            try:
                if not ipaddress.ip_address(host).is_loopback:
                    raise ValueError("gRPC address must be loopback-only")
            except ValueError as exc:
                raise ValueError("gRPC address must be loopback-only") from exc
        return value

    @field_validator("grpc_token")
    @classmethod
    def validate_service_token(cls, value: SecretStr) -> SecretStr:
        secret = value.get_secret_value()
        if len(secret) < 32 or not secret.strip():
            raise ValueError("service token must contain at least 32 non-blank characters")
        return value

    @field_validator("python_executable")
    @classmethod
    def validate_python_executable(cls, value: Path) -> Path:
        resolved = value.expanduser().resolve()
        if not resolved.is_file():
            raise ValueError("sandbox Python executable does not exist")
        return resolved

from __future__ import annotations

import ipaddress
import re
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
    wasi_python_path: Path = Field(
        default=Path("flowai-studio-sandbox/runtime/python.wasm"),
        validation_alias="FLOWAI_WASI_PYTHON_PATH",
    )
    wasi_python_sha256: str = Field(
        default="",
        validation_alias="FLOWAI_WASI_PYTHON_SHA256",
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

    @field_validator("wasi_python_sha256")
    @classmethod
    def validate_sha256(cls, value: str) -> str:
        normalized = value.strip().lower()
        if normalized and not re.fullmatch(r"[0-9a-f]{64}", normalized):
            raise ValueError("WASI SHA-256 must be empty or 64 hexadecimal characters")
        return normalized

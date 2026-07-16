from __future__ import annotations

import ipaddress
from urllib.parse import urlparse

from pydantic import Field, SecretStr, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=None,
        extra="ignore",
        populate_by_name=True,
    )

    http_address: str = Field(default="127.0.0.1:3001", validation_alias="FLOWAI_HTTP_ADDR")
    sandbox_address: str = Field(default="127.0.0.1:50052", validation_alias="FLOWAI_SANDBOX_GRPC_ADDR")
    grpc_token: SecretStr | None = Field(default=None, validation_alias="FLOWAI_GRPC_TOKEN")
    jwt_secret: SecretStr = Field(validation_alias="FLOWAI_JWT_SECRET")
    api_key_hmac_secret: SecretStr = Field(validation_alias="FLOWAI_API_KEY_HMAC_SECRET")
    api_key_hmac_previous_secret: SecretStr | None = Field(
        default=None,
        validation_alias="FLOWAI_API_KEY_HMAC_PREVIOUS_SECRET",
    )
    database_url: str = Field(validation_alias="FLOWAI_DATABASE_URL")
    redis_url: str = Field(validation_alias="FLOWAI_REDIS_URL")
    frontend_url: str = Field(default="http://127.0.0.1:5173", validation_alias="FLOWAI_FRONTEND_URL")
    jwt_expiration: str = Field(default="168h", validation_alias="FLOWAI_JWT_EXPIRATION")
    openai_api_key: SecretStr | None = Field(default=None, validation_alias="OPENAI_API_KEY")
    anthropic_api_key: SecretStr | None = Field(default=None, validation_alias="ANTHROPIC_API_KEY")
    gemini_api_key: SecretStr | None = Field(default=None, validation_alias="GEMINI_API_KEY")
    qwen_api_key: SecretStr | None = Field(default=None, validation_alias="DASHSCOPE_API_KEY")
    ollama_base_url: str = Field(default="http://127.0.0.1:11434", validation_alias="OLLAMA_BASE_URL")
    health_timeout_seconds: float = Field(default=2.0, gt=0, le=30)

    @field_validator("http_address", "sandbox_address")
    @classmethod
    def validate_loopback_address(cls, value: str) -> str:
        try:
            host, port_text = value.rsplit(":", 1)
            port = int(port_text)
        except (ValueError, TypeError) as exc:
            raise ValueError("service address must use host:port") from exc

        if not 1 <= port <= 65535:
            raise ValueError("gRPC port is outside the valid range")
        if host != "localhost":
            try:
                if not ipaddress.ip_address(host).is_loopback:
                    raise ValueError("service address must be loopback-only")
            except ValueError as exc:
                raise ValueError("service address must be loopback-only") from exc
        return value

    @field_validator("grpc_token", "jwt_secret", "api_key_hmac_secret", "api_key_hmac_previous_secret")
    @classmethod
    def validate_service_token(cls, value: SecretStr | None) -> SecretStr | None:
        if value is None or value.get_secret_value() == "":
            return None
        secret = value.get_secret_value()
        if len(secret) < 32 or not secret.strip():
            raise ValueError("secrets must contain at least 32 non-blank characters")
        return value

    @field_validator("frontend_url", "ollama_base_url")
    @classmethod
    def validate_http_url(cls, value: str) -> str:
        parsed = urlparse(value)
        if parsed.scheme not in {"http", "https"} or not parsed.netloc or parsed.username:
            raise ValueError("URL must be absolute HTTP(S) without credentials")
        return value.rstrip("/")

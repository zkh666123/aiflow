from __future__ import annotations

import secrets
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from typing import Any

import uvicorn
from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from redis.asyncio import Redis
from sqlalchemy import text

from aiflow_runtime.api.envelope import success
from aiflow_runtime.api.errors import install_error_handlers
from aiflow_runtime.api import api_keys, applications, shares, teams, users
from aiflow_runtime.config import Settings
from aiflow_runtime.infrastructure.database import Database
from aiflow_runtime.infrastructure.redis import create_redis


def create_app(settings: Settings | None = None) -> FastAPI:
    resolved_settings = settings or Settings()

    @asynccontextmanager
    async def lifespan(app: FastAPI) -> AsyncIterator[None]:
        app.state.settings = resolved_settings
        app.state.database = Database(resolved_settings.database_url)
        app.state.redis = create_redis(resolved_settings.redis_url)
        try:
            yield
        finally:
            redis_client: Redis = app.state.redis
            await redis_client.aclose()
            await app.state.database.dispose()

    app = FastAPI(
        title="FlowAI Studio API",
        version="1.0.0",
        docs_url="/api/docs",
        openapi_url="/api/openapi.json",
        lifespan=lifespan,
    )
    app.add_middleware(
        CORSMiddleware,
        allow_origins=[resolved_settings.frontend_url],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    @app.middleware("http")
    async def request_id(request: Request, call_next: Any) -> Any:
        value = request.headers.get("X-Request-ID") or secrets.token_hex(16)
        response = await call_next(request)
        response.headers["X-Request-ID"] = value
        return response

    install_error_handlers(app)
    app.include_router(users.router)
    app.include_router(applications.router)
    app.include_router(teams.router)
    app.include_router(api_keys.router)
    app.include_router(shares.router)

    @app.get("/api/health")
    async def health(request: Request) -> Any:
        components: dict[str, str] = {}
        try:
            async with request.app.state.database.engine.connect() as connection:
                await connection.execute(text("SELECT 1"))
                result = await connection.execute(
                    text("SELECT extversion::text FROM pg_extension WHERE extname = 'vector'")
                )
                components["postgresql"] = "healthy"
                components["pgvector"] = result.scalar_one_or_none() or "missing"
        except Exception:
            components["postgresql"] = "unhealthy"
            components["pgvector"] = "unknown"
        try:
            components["redis"] = "healthy" if await request.app.state.redis.ping() else "unhealthy"
        except Exception:
            components["redis"] = "unhealthy"
        state = "healthy" if all(value not in {"unhealthy", "missing", "unknown"} for value in components.values()) else "degraded"
        return success({"status": state, "service": "python-backend", "components": components})

    return app


app = create_app()


def main() -> None:
    settings = Settings()
    host, port_text = settings.http_address.rsplit(":", 1)
    uvicorn.run("aiflow_runtime.app:app", host=host, port=int(port_text), factory=False)


if __name__ == "__main__":
    main()

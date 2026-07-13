from __future__ import annotations

from collections.abc import AsyncIterator

import grpc

from aiflow.v1 import (
    documents_pb2,
    documents_pb2_grpc,
    execution_pb2,
    execution_pb2_grpc,
    mcp_pb2,
    mcp_pb2_grpc,
    retrieval_pb2,
    retrieval_pb2_grpc,
)


async def _abort(context: grpc.aio.ServicerContext) -> None:
    await context.abort(grpc.StatusCode.UNIMPLEMENTED, "service method not implemented")


class ExecutionService(execution_pb2_grpc.ExecutionServiceServicer):
    async def ExecuteNode(
        self,
        request: execution_pb2.ExecuteNodeRequest,
        context: grpc.aio.ServicerContext,
    ) -> AsyncIterator[execution_pb2.ExecuteNodeResponse]:
        del request
        await _abort(context)
        if False:
            yield execution_pb2.ExecuteNodeResponse()


class DocumentService(documents_pb2_grpc.DocumentServiceServicer):
    async def IngestDocument(
        self,
        request_iterator: AsyncIterator[documents_pb2.IngestDocumentRequest],
        context: grpc.aio.ServicerContext,
    ) -> documents_pb2.IngestDocumentResponse:
        del request_iterator
        await _abort(context)


class RetrievalService(retrieval_pb2_grpc.RetrievalServiceServicer):
    async def Retrieve(
        self,
        request: retrieval_pb2.RetrieveRequest,
        context: grpc.aio.ServicerContext,
    ) -> retrieval_pb2.RetrieveResponse:
        del request
        await _abort(context)


class McpService(mcp_pb2_grpc.McpServiceServicer):
    async def ManageMcp(
        self,
        request: mcp_pb2.ManageMcpRequest,
        context: grpc.aio.ServicerContext,
    ) -> mcp_pb2.ManageMcpResponse:
        del request
        await _abort(context)

from aiflow.v1 import common_pb2 as _common_pb2
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class IngestionState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    INGESTION_STATE_UNSPECIFIED: _ClassVar[IngestionState]
    INGESTION_STATE_QUEUED: _ClassVar[IngestionState]
    INGESTION_STATE_PROCESSING: _ClassVar[IngestionState]
    INGESTION_STATE_COMPLETED: _ClassVar[IngestionState]
    INGESTION_STATE_FAILED: _ClassVar[IngestionState]
INGESTION_STATE_UNSPECIFIED: IngestionState
INGESTION_STATE_QUEUED: IngestionState
INGESTION_STATE_PROCESSING: IngestionState
INGESTION_STATE_COMPLETED: IngestionState
INGESTION_STATE_FAILED: IngestionState

class ChunkingOptions(_message.Message):
    __slots__ = ("chunk_size", "chunk_overlap")
    CHUNK_SIZE_FIELD_NUMBER: _ClassVar[int]
    CHUNK_OVERLAP_FIELD_NUMBER: _ClassVar[int]
    chunk_size: int
    chunk_overlap: int
    def __init__(self, chunk_size: _Optional[int] = ..., chunk_overlap: _Optional[int] = ...) -> None: ...

class IngestDocumentMetadata(_message.Message):
    __slots__ = ("context", "upload_id", "knowledge_base_id", "filename", "content_type", "size_bytes", "sha256", "chunking")
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    UPLOAD_ID_FIELD_NUMBER: _ClassVar[int]
    KNOWLEDGE_BASE_ID_FIELD_NUMBER: _ClassVar[int]
    FILENAME_FIELD_NUMBER: _ClassVar[int]
    CONTENT_TYPE_FIELD_NUMBER: _ClassVar[int]
    SIZE_BYTES_FIELD_NUMBER: _ClassVar[int]
    SHA256_FIELD_NUMBER: _ClassVar[int]
    CHUNKING_FIELD_NUMBER: _ClassVar[int]
    context: _common_pb2.RequestContext
    upload_id: str
    knowledge_base_id: str
    filename: str
    content_type: str
    size_bytes: int
    sha256: str
    chunking: ChunkingOptions
    def __init__(self, context: _Optional[_Union[_common_pb2.RequestContext, _Mapping]] = ..., upload_id: _Optional[str] = ..., knowledge_base_id: _Optional[str] = ..., filename: _Optional[str] = ..., content_type: _Optional[str] = ..., size_bytes: _Optional[int] = ..., sha256: _Optional[str] = ..., chunking: _Optional[_Union[ChunkingOptions, _Mapping]] = ...) -> None: ...

class DocumentChunk(_message.Message):
    __slots__ = ("sequence", "data")
    SEQUENCE_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    sequence: int
    data: bytes
    def __init__(self, sequence: _Optional[int] = ..., data: _Optional[bytes] = ...) -> None: ...

class IngestDocumentRequest(_message.Message):
    __slots__ = ("metadata", "chunk")
    METADATA_FIELD_NUMBER: _ClassVar[int]
    CHUNK_FIELD_NUMBER: _ClassVar[int]
    metadata: IngestDocumentMetadata
    chunk: DocumentChunk
    def __init__(self, metadata: _Optional[_Union[IngestDocumentMetadata, _Mapping]] = ..., chunk: _Optional[_Union[DocumentChunk, _Mapping]] = ...) -> None: ...

class IngestDocumentResponse(_message.Message):
    __slots__ = ("upload_id", "document_id", "ingestion_job_id", "state", "error")
    UPLOAD_ID_FIELD_NUMBER: _ClassVar[int]
    DOCUMENT_ID_FIELD_NUMBER: _ClassVar[int]
    INGESTION_JOB_ID_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    upload_id: str
    document_id: str
    ingestion_job_id: str
    state: IngestionState
    error: _common_pb2.ServiceError
    def __init__(self, upload_id: _Optional[str] = ..., document_id: _Optional[str] = ..., ingestion_job_id: _Optional[str] = ..., state: _Optional[_Union[IngestionState, str]] = ..., error: _Optional[_Union[_common_pb2.ServiceError, _Mapping]] = ...) -> None: ...

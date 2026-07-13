from aiflow.v1 import common_pb2 as _common_pb2
from google.protobuf import struct_pb2 as _struct_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RetrievalMode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    RETRIEVAL_MODE_UNSPECIFIED: _ClassVar[RetrievalMode]
    RETRIEVAL_MODE_VECTOR: _ClassVar[RetrievalMode]
    RETRIEVAL_MODE_KEYWORD: _ClassVar[RetrievalMode]
    RETRIEVAL_MODE_HYBRID: _ClassVar[RetrievalMode]

class RerankerProvider(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    RERANKER_PROVIDER_UNSPECIFIED: _ClassVar[RerankerProvider]
    RERANKER_PROVIDER_NONE: _ClassVar[RerankerProvider]
    RERANKER_PROVIDER_COHERE: _ClassVar[RerankerProvider]
    RERANKER_PROVIDER_OLLAMA: _ClassVar[RerankerProvider]
RETRIEVAL_MODE_UNSPECIFIED: RetrievalMode
RETRIEVAL_MODE_VECTOR: RetrievalMode
RETRIEVAL_MODE_KEYWORD: RetrievalMode
RETRIEVAL_MODE_HYBRID: RetrievalMode
RERANKER_PROVIDER_UNSPECIFIED: RerankerProvider
RERANKER_PROVIDER_NONE: RerankerProvider
RERANKER_PROVIDER_COHERE: RerankerProvider
RERANKER_PROVIDER_OLLAMA: RerankerProvider

class RerankerOptions(_message.Message):
    __slots__ = ("provider", "model", "top_n", "allow_fallback")
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    TOP_N_FIELD_NUMBER: _ClassVar[int]
    ALLOW_FALLBACK_FIELD_NUMBER: _ClassVar[int]
    provider: RerankerProvider
    model: str
    top_n: int
    allow_fallback: bool
    def __init__(self, provider: _Optional[_Union[RerankerProvider, str]] = ..., model: _Optional[str] = ..., top_n: _Optional[int] = ..., allow_fallback: bool = ...) -> None: ...

class RetrieveRequest(_message.Message):
    __slots__ = ("context", "knowledge_base_id", "query", "mode", "top_k", "vector_weight", "keyword_weight", "reranker", "filters")
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    KNOWLEDGE_BASE_ID_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    MODE_FIELD_NUMBER: _ClassVar[int]
    TOP_K_FIELD_NUMBER: _ClassVar[int]
    VECTOR_WEIGHT_FIELD_NUMBER: _ClassVar[int]
    KEYWORD_WEIGHT_FIELD_NUMBER: _ClassVar[int]
    RERANKER_FIELD_NUMBER: _ClassVar[int]
    FILTERS_FIELD_NUMBER: _ClassVar[int]
    context: _common_pb2.RequestContext
    knowledge_base_id: str
    query: str
    mode: RetrievalMode
    top_k: int
    vector_weight: float
    keyword_weight: float
    reranker: RerankerOptions
    filters: _struct_pb2.Struct
    def __init__(self, context: _Optional[_Union[_common_pb2.RequestContext, _Mapping]] = ..., knowledge_base_id: _Optional[str] = ..., query: _Optional[str] = ..., mode: _Optional[_Union[RetrievalMode, str]] = ..., top_k: _Optional[int] = ..., vector_weight: _Optional[float] = ..., keyword_weight: _Optional[float] = ..., reranker: _Optional[_Union[RerankerOptions, _Mapping]] = ..., filters: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class RetrievalHit(_message.Message):
    __slots__ = ("chunk_id", "document_id", "content", "metadata", "vector_score", "keyword_score", "fused_score", "reranker_score")
    CHUNK_ID_FIELD_NUMBER: _ClassVar[int]
    DOCUMENT_ID_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    VECTOR_SCORE_FIELD_NUMBER: _ClassVar[int]
    KEYWORD_SCORE_FIELD_NUMBER: _ClassVar[int]
    FUSED_SCORE_FIELD_NUMBER: _ClassVar[int]
    RERANKER_SCORE_FIELD_NUMBER: _ClassVar[int]
    chunk_id: str
    document_id: str
    content: str
    metadata: _struct_pb2.Struct
    vector_score: float
    keyword_score: float
    fused_score: float
    reranker_score: float
    def __init__(self, chunk_id: _Optional[str] = ..., document_id: _Optional[str] = ..., content: _Optional[str] = ..., metadata: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., vector_score: _Optional[float] = ..., keyword_score: _Optional[float] = ..., fused_score: _Optional[float] = ..., reranker_score: _Optional[float] = ...) -> None: ...

class RetrieveResponse(_message.Message):
    __slots__ = ("hits", "reranker_applied", "reranker_fallback", "reranker_error")
    HITS_FIELD_NUMBER: _ClassVar[int]
    RERANKER_APPLIED_FIELD_NUMBER: _ClassVar[int]
    RERANKER_FALLBACK_FIELD_NUMBER: _ClassVar[int]
    RERANKER_ERROR_FIELD_NUMBER: _ClassVar[int]
    hits: _containers.RepeatedCompositeFieldContainer[RetrievalHit]
    reranker_applied: bool
    reranker_fallback: bool
    reranker_error: _common_pb2.ServiceError
    def __init__(self, hits: _Optional[_Iterable[_Union[RetrievalHit, _Mapping]]] = ..., reranker_applied: bool = ..., reranker_fallback: bool = ..., reranker_error: _Optional[_Union[_common_pb2.ServiceError, _Mapping]] = ...) -> None: ...

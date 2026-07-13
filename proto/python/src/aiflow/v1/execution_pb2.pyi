import datetime

from aiflow.v1 import common_pb2 as _common_pb2
from google.protobuf import struct_pb2 as _struct_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class TraceType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    TRACE_TYPE_UNSPECIFIED: _ClassVar[TraceType]
    TRACE_TYPE_THINKING: _ClassVar[TraceType]
    TRACE_TYPE_TOOL_CALL: _ClassVar[TraceType]
    TRACE_TYPE_TOOL_RESULT: _ClassVar[TraceType]
    TRACE_TYPE_RETRIEVAL: _ClassVar[TraceType]
    TRACE_TYPE_DELEGATION: _ClassVar[TraceType]
    TRACE_TYPE_WORKER_RESULT: _ClassVar[TraceType]
    TRACE_TYPE_FINAL_ANSWER: _ClassVar[TraceType]
    TRACE_TYPE_ERROR: _ClassVar[TraceType]
TRACE_TYPE_UNSPECIFIED: TraceType
TRACE_TYPE_THINKING: TraceType
TRACE_TYPE_TOOL_CALL: TraceType
TRACE_TYPE_TOOL_RESULT: TraceType
TRACE_TYPE_RETRIEVAL: TraceType
TRACE_TYPE_DELEGATION: TraceType
TRACE_TYPE_WORKER_RESULT: TraceType
TRACE_TYPE_FINAL_ANSWER: TraceType
TRACE_TYPE_ERROR: TraceType

class LlmNodeSpec(_message.Message):
    __slots__ = ("provider", "model", "system_prompt", "prompt", "temperature", "max_tokens", "stop_sequences")
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    SYSTEM_PROMPT_FIELD_NUMBER: _ClassVar[int]
    PROMPT_FIELD_NUMBER: _ClassVar[int]
    TEMPERATURE_FIELD_NUMBER: _ClassVar[int]
    MAX_TOKENS_FIELD_NUMBER: _ClassVar[int]
    STOP_SEQUENCES_FIELD_NUMBER: _ClassVar[int]
    provider: str
    model: str
    system_prompt: str
    prompt: str
    temperature: float
    max_tokens: int
    stop_sequences: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, provider: _Optional[str] = ..., model: _Optional[str] = ..., system_prompt: _Optional[str] = ..., prompt: _Optional[str] = ..., temperature: _Optional[float] = ..., max_tokens: _Optional[int] = ..., stop_sequences: _Optional[_Iterable[str]] = ...) -> None: ...

class RagNodeSpec(_message.Message):
    __slots__ = ("knowledge_base_id", "query", "top_k", "enable_reranker")
    KNOWLEDGE_BASE_ID_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    TOP_K_FIELD_NUMBER: _ClassVar[int]
    ENABLE_RERANKER_FIELD_NUMBER: _ClassVar[int]
    knowledge_base_id: str
    query: str
    top_k: int
    enable_reranker: bool
    def __init__(self, knowledge_base_id: _Optional[str] = ..., query: _Optional[str] = ..., top_k: _Optional[int] = ..., enable_reranker: bool = ...) -> None: ...

class AgentNodeSpec(_message.Message):
    __slots__ = ("mode", "provider", "model", "instructions", "tool_ids", "max_iterations", "configuration")
    MODE_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    INSTRUCTIONS_FIELD_NUMBER: _ClassVar[int]
    TOOL_IDS_FIELD_NUMBER: _ClassVar[int]
    MAX_ITERATIONS_FIELD_NUMBER: _ClassVar[int]
    CONFIGURATION_FIELD_NUMBER: _ClassVar[int]
    mode: str
    provider: str
    model: str
    instructions: str
    tool_ids: _containers.RepeatedScalarFieldContainer[str]
    max_iterations: int
    configuration: _struct_pb2.Struct
    def __init__(self, mode: _Optional[str] = ..., provider: _Optional[str] = ..., model: _Optional[str] = ..., instructions: _Optional[str] = ..., tool_ids: _Optional[_Iterable[str]] = ..., max_iterations: _Optional[int] = ..., configuration: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class SkillNodeSpec(_message.Message):
    __slots__ = ("skill_id", "arguments")
    SKILL_ID_FIELD_NUMBER: _ClassVar[int]
    ARGUMENTS_FIELD_NUMBER: _ClassVar[int]
    skill_id: str
    arguments: _struct_pb2.Struct
    def __init__(self, skill_id: _Optional[str] = ..., arguments: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class ExecuteNodeRequest(_message.Message):
    __slots__ = ("context", "execution_id", "node_id", "inputs", "llm", "rag", "agent", "skill")
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    EXECUTION_ID_FIELD_NUMBER: _ClassVar[int]
    NODE_ID_FIELD_NUMBER: _ClassVar[int]
    INPUTS_FIELD_NUMBER: _ClassVar[int]
    LLM_FIELD_NUMBER: _ClassVar[int]
    RAG_FIELD_NUMBER: _ClassVar[int]
    AGENT_FIELD_NUMBER: _ClassVar[int]
    SKILL_FIELD_NUMBER: _ClassVar[int]
    context: _common_pb2.RequestContext
    execution_id: str
    node_id: str
    inputs: _struct_pb2.Struct
    llm: LlmNodeSpec
    rag: RagNodeSpec
    agent: AgentNodeSpec
    skill: SkillNodeSpec
    def __init__(self, context: _Optional[_Union[_common_pb2.RequestContext, _Mapping]] = ..., execution_id: _Optional[str] = ..., node_id: _Optional[str] = ..., inputs: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., llm: _Optional[_Union[LlmNodeSpec, _Mapping]] = ..., rag: _Optional[_Union[RagNodeSpec, _Mapping]] = ..., agent: _Optional[_Union[AgentNodeSpec, _Mapping]] = ..., skill: _Optional[_Union[SkillNodeSpec, _Mapping]] = ...) -> None: ...

class NodeStarted(_message.Message):
    __slots__ = ("provider", "model")
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    provider: str
    model: str
    def __init__(self, provider: _Optional[str] = ..., model: _Optional[str] = ...) -> None: ...

class TokenDelta(_message.Message):
    __slots__ = ("text",)
    TEXT_FIELD_NUMBER: _ClassVar[int]
    text: str
    def __init__(self, text: _Optional[str] = ...) -> None: ...

class AgentTrace(_message.Message):
    __slots__ = ("type", "message", "data")
    TYPE_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    type: TraceType
    message: str
    data: _struct_pb2.Struct
    def __init__(self, type: _Optional[_Union[TraceType, str]] = ..., message: _Optional[str] = ..., data: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class NodeOutput(_message.Message):
    __slots__ = ("value",)
    VALUE_FIELD_NUMBER: _ClassVar[int]
    value: _struct_pb2.Struct
    def __init__(self, value: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class UsageReported(_message.Message):
    __slots__ = ("usage",)
    USAGE_FIELD_NUMBER: _ClassVar[int]
    usage: _common_pb2.TokenUsage
    def __init__(self, usage: _Optional[_Union[_common_pb2.TokenUsage, _Mapping]] = ...) -> None: ...

class NodeFailed(_message.Message):
    __slots__ = ("error",)
    ERROR_FIELD_NUMBER: _ClassVar[int]
    error: _common_pb2.ServiceError
    def __init__(self, error: _Optional[_Union[_common_pb2.ServiceError, _Mapping]] = ...) -> None: ...

class NodeCompleted(_message.Message):
    __slots__ = ("success",)
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    success: bool
    def __init__(self, success: bool = ...) -> None: ...

class ExecuteNodeResponse(_message.Message):
    __slots__ = ("execution_id", "node_id", "sequence", "emitted_at", "started", "token", "trace", "output", "usage", "error", "done")
    EXECUTION_ID_FIELD_NUMBER: _ClassVar[int]
    NODE_ID_FIELD_NUMBER: _ClassVar[int]
    SEQUENCE_FIELD_NUMBER: _ClassVar[int]
    EMITTED_AT_FIELD_NUMBER: _ClassVar[int]
    STARTED_FIELD_NUMBER: _ClassVar[int]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    TRACE_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    DONE_FIELD_NUMBER: _ClassVar[int]
    execution_id: str
    node_id: str
    sequence: int
    emitted_at: _timestamp_pb2.Timestamp
    started: NodeStarted
    token: TokenDelta
    trace: AgentTrace
    output: NodeOutput
    usage: UsageReported
    error: NodeFailed
    done: NodeCompleted
    def __init__(self, execution_id: _Optional[str] = ..., node_id: _Optional[str] = ..., sequence: _Optional[int] = ..., emitted_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., started: _Optional[_Union[NodeStarted, _Mapping]] = ..., token: _Optional[_Union[TokenDelta, _Mapping]] = ..., trace: _Optional[_Union[AgentTrace, _Mapping]] = ..., output: _Optional[_Union[NodeOutput, _Mapping]] = ..., usage: _Optional[_Union[UsageReported, _Mapping]] = ..., error: _Optional[_Union[NodeFailed, _Mapping]] = ..., done: _Optional[_Union[NodeCompleted, _Mapping]] = ...) -> None: ...

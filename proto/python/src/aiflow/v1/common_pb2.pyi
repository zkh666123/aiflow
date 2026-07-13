import datetime

from google.protobuf import struct_pb2 as _struct_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ErrorCode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ERROR_CODE_UNSPECIFIED: _ClassVar[ErrorCode]
    ERROR_CODE_INVALID_ARGUMENT: _ClassVar[ErrorCode]
    ERROR_CODE_UNAUTHENTICATED: _ClassVar[ErrorCode]
    ERROR_CODE_FORBIDDEN: _ClassVar[ErrorCode]
    ERROR_CODE_NOT_FOUND: _ClassVar[ErrorCode]
    ERROR_CODE_CONFLICT: _ClassVar[ErrorCode]
    ERROR_CODE_RATE_LIMITED: _ClassVar[ErrorCode]
    ERROR_CODE_TIMEOUT: _ClassVar[ErrorCode]
    ERROR_CODE_CANCELLED: _ClassVar[ErrorCode]
    ERROR_CODE_PROVIDER_UNAVAILABLE: _ClassVar[ErrorCode]
    ERROR_CODE_RESOURCE_EXHAUSTED: _ClassVar[ErrorCode]
    ERROR_CODE_INTERNAL: _ClassVar[ErrorCode]

class HealthState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    HEALTH_STATE_UNSPECIFIED: _ClassVar[HealthState]
    HEALTH_STATE_HEALTHY: _ClassVar[HealthState]
    HEALTH_STATE_DEGRADED: _ClassVar[HealthState]
    HEALTH_STATE_UNHEALTHY: _ClassVar[HealthState]
    HEALTH_STATE_NOT_READY: _ClassVar[HealthState]
ERROR_CODE_UNSPECIFIED: ErrorCode
ERROR_CODE_INVALID_ARGUMENT: ErrorCode
ERROR_CODE_UNAUTHENTICATED: ErrorCode
ERROR_CODE_FORBIDDEN: ErrorCode
ERROR_CODE_NOT_FOUND: ErrorCode
ERROR_CODE_CONFLICT: ErrorCode
ERROR_CODE_RATE_LIMITED: ErrorCode
ERROR_CODE_TIMEOUT: ErrorCode
ERROR_CODE_CANCELLED: ErrorCode
ERROR_CODE_PROVIDER_UNAVAILABLE: ErrorCode
ERROR_CODE_RESOURCE_EXHAUSTED: ErrorCode
ERROR_CODE_INTERNAL: ErrorCode
HEALTH_STATE_UNSPECIFIED: HealthState
HEALTH_STATE_HEALTHY: HealthState
HEALTH_STATE_DEGRADED: HealthState
HEALTH_STATE_UNHEALTHY: HealthState
HEALTH_STATE_NOT_READY: HealthState

class RequestContext(_message.Message):
    __slots__ = ("request_id", "trace_id", "caller", "idempotency_key", "deadline")
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    TRACE_ID_FIELD_NUMBER: _ClassVar[int]
    CALLER_FIELD_NUMBER: _ClassVar[int]
    IDEMPOTENCY_KEY_FIELD_NUMBER: _ClassVar[int]
    DEADLINE_FIELD_NUMBER: _ClassVar[int]
    request_id: str
    trace_id: str
    caller: str
    idempotency_key: str
    deadline: _timestamp_pb2.Timestamp
    def __init__(self, request_id: _Optional[str] = ..., trace_id: _Optional[str] = ..., caller: _Optional[str] = ..., idempotency_key: _Optional[str] = ..., deadline: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ServiceError(_message.Message):
    __slots__ = ("code", "message", "retryable", "metadata")
    class MetadataEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    CODE_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    RETRYABLE_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    code: ErrorCode
    message: str
    retryable: bool
    metadata: _containers.ScalarMap[str, str]
    def __init__(self, code: _Optional[_Union[ErrorCode, str]] = ..., message: _Optional[str] = ..., retryable: bool = ..., metadata: _Optional[_Mapping[str, str]] = ...) -> None: ...

class TokenUsage(_message.Message):
    __slots__ = ("prompt_tokens", "completion_tokens", "total_tokens", "provider", "model", "estimated_cost")
    PROMPT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    COMPLETION_TOKENS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_TOKENS_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    ESTIMATED_COST_FIELD_NUMBER: _ClassVar[int]
    prompt_tokens: int
    completion_tokens: int
    total_tokens: int
    provider: str
    model: str
    estimated_cost: str
    def __init__(self, prompt_tokens: _Optional[int] = ..., completion_tokens: _Optional[int] = ..., total_tokens: _Optional[int] = ..., provider: _Optional[str] = ..., model: _Optional[str] = ..., estimated_cost: _Optional[str] = ...) -> None: ...

class HealthComponent(_message.Message):
    __slots__ = ("name", "state", "version", "message", "details")
    NAME_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    DETAILS_FIELD_NUMBER: _ClassVar[int]
    name: str
    state: HealthState
    version: str
    message: str
    details: _struct_pb2.Struct
    def __init__(self, name: _Optional[str] = ..., state: _Optional[_Union[HealthState, str]] = ..., version: _Optional[str] = ..., message: _Optional[str] = ..., details: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class HealthReport(_message.Message):
    __slots__ = ("state", "components", "checked_at")
    STATE_FIELD_NUMBER: _ClassVar[int]
    COMPONENTS_FIELD_NUMBER: _ClassVar[int]
    CHECKED_AT_FIELD_NUMBER: _ClassVar[int]
    state: HealthState
    components: _containers.RepeatedCompositeFieldContainer[HealthComponent]
    checked_at: _timestamp_pb2.Timestamp
    def __init__(self, state: _Optional[_Union[HealthState, str]] = ..., components: _Optional[_Iterable[_Union[HealthComponent, _Mapping]]] = ..., checked_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

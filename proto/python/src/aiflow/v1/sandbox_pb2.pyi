from aiflow.v1 import common_pb2 as _common_pb2
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class SandboxExecutionStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SANDBOX_EXECUTION_STATUS_UNSPECIFIED: _ClassVar[SandboxExecutionStatus]
    SANDBOX_EXECUTION_STATUS_SUCCEEDED: _ClassVar[SandboxExecutionStatus]
    SANDBOX_EXECUTION_STATUS_FAILED: _ClassVar[SandboxExecutionStatus]
    SANDBOX_EXECUTION_STATUS_TIMED_OUT: _ClassVar[SandboxExecutionStatus]
    SANDBOX_EXECUTION_STATUS_CANCELLED: _ClassVar[SandboxExecutionStatus]
    SANDBOX_EXECUTION_STATUS_RESOURCE_EXHAUSTED: _ClassVar[SandboxExecutionStatus]
    SANDBOX_EXECUTION_STATUS_NOT_READY: _ClassVar[SandboxExecutionStatus]

class SandboxFailureCode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SANDBOX_FAILURE_CODE_UNSPECIFIED: _ClassVar[SandboxFailureCode]
    SANDBOX_FAILURE_CODE_RUNTIME_MISSING: _ClassVar[SandboxFailureCode]
    SANDBOX_FAILURE_CODE_RUNTIME_INVALID: _ClassVar[SandboxFailureCode]
    SANDBOX_FAILURE_CODE_GUEST_ERROR: _ClassVar[SandboxFailureCode]
    SANDBOX_FAILURE_CODE_TIMEOUT: _ClassVar[SandboxFailureCode]
    SANDBOX_FAILURE_CODE_MEMORY_LIMIT: _ClassVar[SandboxFailureCode]
    SANDBOX_FAILURE_CODE_OUTPUT_LIMIT: _ClassVar[SandboxFailureCode]
    SANDBOX_FAILURE_CODE_CANCELLED: _ClassVar[SandboxFailureCode]
    SANDBOX_FAILURE_CODE_INTERNAL: _ClassVar[SandboxFailureCode]
SANDBOX_EXECUTION_STATUS_UNSPECIFIED: SandboxExecutionStatus
SANDBOX_EXECUTION_STATUS_SUCCEEDED: SandboxExecutionStatus
SANDBOX_EXECUTION_STATUS_FAILED: SandboxExecutionStatus
SANDBOX_EXECUTION_STATUS_TIMED_OUT: SandboxExecutionStatus
SANDBOX_EXECUTION_STATUS_CANCELLED: SandboxExecutionStatus
SANDBOX_EXECUTION_STATUS_RESOURCE_EXHAUSTED: SandboxExecutionStatus
SANDBOX_EXECUTION_STATUS_NOT_READY: SandboxExecutionStatus
SANDBOX_FAILURE_CODE_UNSPECIFIED: SandboxFailureCode
SANDBOX_FAILURE_CODE_RUNTIME_MISSING: SandboxFailureCode
SANDBOX_FAILURE_CODE_RUNTIME_INVALID: SandboxFailureCode
SANDBOX_FAILURE_CODE_GUEST_ERROR: SandboxFailureCode
SANDBOX_FAILURE_CODE_TIMEOUT: SandboxFailureCode
SANDBOX_FAILURE_CODE_MEMORY_LIMIT: SandboxFailureCode
SANDBOX_FAILURE_CODE_OUTPUT_LIMIT: SandboxFailureCode
SANDBOX_FAILURE_CODE_CANCELLED: SandboxFailureCode
SANDBOX_FAILURE_CODE_INTERNAL: SandboxFailureCode

class SandboxLimits(_message.Message):
    __slots__ = ("fuel", "timeout_millis", "memory_bytes", "output_bytes")
    FUEL_FIELD_NUMBER: _ClassVar[int]
    TIMEOUT_MILLIS_FIELD_NUMBER: _ClassVar[int]
    MEMORY_BYTES_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_BYTES_FIELD_NUMBER: _ClassVar[int]
    fuel: int
    timeout_millis: int
    memory_bytes: int
    output_bytes: int
    def __init__(self, fuel: _Optional[int] = ..., timeout_millis: _Optional[int] = ..., memory_bytes: _Optional[int] = ..., output_bytes: _Optional[int] = ...) -> None: ...

class ExecutePythonRequest(_message.Message):
    __slots__ = ("context", "code", "limits")
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    CODE_FIELD_NUMBER: _ClassVar[int]
    LIMITS_FIELD_NUMBER: _ClassVar[int]
    context: _common_pb2.RequestContext
    code: str
    limits: SandboxLimits
    def __init__(self, context: _Optional[_Union[_common_pb2.RequestContext, _Mapping]] = ..., code: _Optional[str] = ..., limits: _Optional[_Union[SandboxLimits, _Mapping]] = ...) -> None: ...

class ExecutePythonResponse(_message.Message):
    __slots__ = ("status", "failure_code", "exit_code", "stdout", "stderr", "duration_millis")
    STATUS_FIELD_NUMBER: _ClassVar[int]
    FAILURE_CODE_FIELD_NUMBER: _ClassVar[int]
    EXIT_CODE_FIELD_NUMBER: _ClassVar[int]
    STDOUT_FIELD_NUMBER: _ClassVar[int]
    STDERR_FIELD_NUMBER: _ClassVar[int]
    DURATION_MILLIS_FIELD_NUMBER: _ClassVar[int]
    status: SandboxExecutionStatus
    failure_code: SandboxFailureCode
    exit_code: int
    stdout: str
    stderr: str
    duration_millis: int
    def __init__(self, status: _Optional[_Union[SandboxExecutionStatus, str]] = ..., failure_code: _Optional[_Union[SandboxFailureCode, str]] = ..., exit_code: _Optional[int] = ..., stdout: _Optional[str] = ..., stderr: _Optional[str] = ..., duration_millis: _Optional[int] = ...) -> None: ...

class SandboxServiceHealthCheckRequest(_message.Message):
    __slots__ = ("context", "include_details")
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_DETAILS_FIELD_NUMBER: _ClassVar[int]
    context: _common_pb2.RequestContext
    include_details: bool
    def __init__(self, context: _Optional[_Union[_common_pb2.RequestContext, _Mapping]] = ..., include_details: bool = ...) -> None: ...

class SandboxServiceHealthCheckResponse(_message.Message):
    __slots__ = ("report",)
    REPORT_FIELD_NUMBER: _ClassVar[int]
    report: _common_pb2.HealthReport
    def __init__(self, report: _Optional[_Union[_common_pb2.HealthReport, _Mapping]] = ...) -> None: ...

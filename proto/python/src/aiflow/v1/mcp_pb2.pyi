from aiflow.v1 import common_pb2 as _common_pb2
from google.protobuf import struct_pb2 as _struct_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class McpTransport(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MCP_TRANSPORT_UNSPECIFIED: _ClassVar[McpTransport]
    MCP_TRANSPORT_STDIO: _ClassVar[McpTransport]
    MCP_TRANSPORT_HTTP: _ClassVar[McpTransport]

class McpConnectionState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MCP_CONNECTION_STATE_UNSPECIFIED: _ClassVar[McpConnectionState]
    MCP_CONNECTION_STATE_DISCONNECTED: _ClassVar[McpConnectionState]
    MCP_CONNECTION_STATE_CONNECTING: _ClassVar[McpConnectionState]
    MCP_CONNECTION_STATE_CONNECTED: _ClassVar[McpConnectionState]
    MCP_CONNECTION_STATE_ERROR: _ClassVar[McpConnectionState]
MCP_TRANSPORT_UNSPECIFIED: McpTransport
MCP_TRANSPORT_STDIO: McpTransport
MCP_TRANSPORT_HTTP: McpTransport
MCP_CONNECTION_STATE_UNSPECIFIED: McpConnectionState
MCP_CONNECTION_STATE_DISCONNECTED: McpConnectionState
MCP_CONNECTION_STATE_CONNECTING: McpConnectionState
MCP_CONNECTION_STATE_CONNECTED: McpConnectionState
MCP_CONNECTION_STATE_ERROR: McpConnectionState

class ConfigureMcp(_message.Message):
    __slots__ = ("server_id", "name", "transport", "endpoint", "arguments", "environment", "headers")
    class EnvironmentEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    class HeadersEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    TRANSPORT_FIELD_NUMBER: _ClassVar[int]
    ENDPOINT_FIELD_NUMBER: _ClassVar[int]
    ARGUMENTS_FIELD_NUMBER: _ClassVar[int]
    ENVIRONMENT_FIELD_NUMBER: _ClassVar[int]
    HEADERS_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    name: str
    transport: McpTransport
    endpoint: str
    arguments: _containers.RepeatedScalarFieldContainer[str]
    environment: _containers.ScalarMap[str, str]
    headers: _containers.ScalarMap[str, str]
    def __init__(self, server_id: _Optional[str] = ..., name: _Optional[str] = ..., transport: _Optional[_Union[McpTransport, str]] = ..., endpoint: _Optional[str] = ..., arguments: _Optional[_Iterable[str]] = ..., environment: _Optional[_Mapping[str, str]] = ..., headers: _Optional[_Mapping[str, str]] = ...) -> None: ...

class ConnectMcp(_message.Message):
    __slots__ = ("server_id",)
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    def __init__(self, server_id: _Optional[str] = ...) -> None: ...

class DisconnectMcp(_message.Message):
    __slots__ = ("server_id",)
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    def __init__(self, server_id: _Optional[str] = ...) -> None: ...

class DiscoverMcpTools(_message.Message):
    __slots__ = ("server_id", "refresh")
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    REFRESH_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    refresh: bool
    def __init__(self, server_id: _Optional[str] = ..., refresh: bool = ...) -> None: ...

class CallMcpTool(_message.Message):
    __slots__ = ("server_id", "tool_name", "arguments")
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    TOOL_NAME_FIELD_NUMBER: _ClassVar[int]
    ARGUMENTS_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    tool_name: str
    arguments: _struct_pb2.Struct
    def __init__(self, server_id: _Optional[str] = ..., tool_name: _Optional[str] = ..., arguments: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class ManageMcpRequest(_message.Message):
    __slots__ = ("context", "configure", "connect", "disconnect", "discover", "call")
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    CONFIGURE_FIELD_NUMBER: _ClassVar[int]
    CONNECT_FIELD_NUMBER: _ClassVar[int]
    DISCONNECT_FIELD_NUMBER: _ClassVar[int]
    DISCOVER_FIELD_NUMBER: _ClassVar[int]
    CALL_FIELD_NUMBER: _ClassVar[int]
    context: _common_pb2.RequestContext
    configure: ConfigureMcp
    connect: ConnectMcp
    disconnect: DisconnectMcp
    discover: DiscoverMcpTools
    call: CallMcpTool
    def __init__(self, context: _Optional[_Union[_common_pb2.RequestContext, _Mapping]] = ..., configure: _Optional[_Union[ConfigureMcp, _Mapping]] = ..., connect: _Optional[_Union[ConnectMcp, _Mapping]] = ..., disconnect: _Optional[_Union[DisconnectMcp, _Mapping]] = ..., discover: _Optional[_Union[DiscoverMcpTools, _Mapping]] = ..., call: _Optional[_Union[CallMcpTool, _Mapping]] = ...) -> None: ...

class McpTool(_message.Message):
    __slots__ = ("name", "description", "input_schema")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    INPUT_SCHEMA_FIELD_NUMBER: _ClassVar[int]
    name: str
    description: str
    input_schema: _struct_pb2.Struct
    def __init__(self, name: _Optional[str] = ..., description: _Optional[str] = ..., input_schema: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class McpServerResult(_message.Message):
    __slots__ = ("server_id", "state", "tools")
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    TOOLS_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    state: McpConnectionState
    tools: _containers.RepeatedCompositeFieldContainer[McpTool]
    def __init__(self, server_id: _Optional[str] = ..., state: _Optional[_Union[McpConnectionState, str]] = ..., tools: _Optional[_Iterable[_Union[McpTool, _Mapping]]] = ...) -> None: ...

class McpToolResult(_message.Message):
    __slots__ = ("value", "is_error")
    VALUE_FIELD_NUMBER: _ClassVar[int]
    IS_ERROR_FIELD_NUMBER: _ClassVar[int]
    value: _struct_pb2.Struct
    is_error: bool
    def __init__(self, value: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., is_error: bool = ...) -> None: ...

class ManageMcpResponse(_message.Message):
    __slots__ = ("error", "server", "tool")
    ERROR_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    TOOL_FIELD_NUMBER: _ClassVar[int]
    error: _common_pb2.ServiceError
    server: McpServerResult
    tool: McpToolResult
    def __init__(self, error: _Optional[_Union[_common_pb2.ServiceError, _Mapping]] = ..., server: _Optional[_Union[McpServerResult, _Mapping]] = ..., tool: _Optional[_Union[McpToolResult, _Mapping]] = ...) -> None: ...

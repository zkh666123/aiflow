from aiflow.v1 import common_pb2 as _common_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ModelCapability(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MODEL_CAPABILITY_UNSPECIFIED: _ClassVar[ModelCapability]
    MODEL_CAPABILITY_TEXT: _ClassVar[ModelCapability]
    MODEL_CAPABILITY_VISION: _ClassVar[ModelCapability]
    MODEL_CAPABILITY_TOOLS: _ClassVar[ModelCapability]
    MODEL_CAPABILITY_JSON: _ClassVar[ModelCapability]
    MODEL_CAPABILITY_EMBEDDING: _ClassVar[ModelCapability]
    MODEL_CAPABILITY_RERANKING: _ClassVar[ModelCapability]
MODEL_CAPABILITY_UNSPECIFIED: ModelCapability
MODEL_CAPABILITY_TEXT: ModelCapability
MODEL_CAPABILITY_VISION: ModelCapability
MODEL_CAPABILITY_TOOLS: ModelCapability
MODEL_CAPABILITY_JSON: ModelCapability
MODEL_CAPABILITY_EMBEDDING: ModelCapability
MODEL_CAPABILITY_RERANKING: ModelCapability

class ListModelsRequest(_message.Message):
    __slots__ = ("context", "provider", "required_capabilities", "configured_only", "healthy_only")
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    REQUIRED_CAPABILITIES_FIELD_NUMBER: _ClassVar[int]
    CONFIGURED_ONLY_FIELD_NUMBER: _ClassVar[int]
    HEALTHY_ONLY_FIELD_NUMBER: _ClassVar[int]
    context: _common_pb2.RequestContext
    provider: str
    required_capabilities: _containers.RepeatedScalarFieldContainer[ModelCapability]
    configured_only: bool
    healthy_only: bool
    def __init__(self, context: _Optional[_Union[_common_pb2.RequestContext, _Mapping]] = ..., provider: _Optional[str] = ..., required_capabilities: _Optional[_Iterable[_Union[ModelCapability, str]]] = ..., configured_only: bool = ..., healthy_only: bool = ...) -> None: ...

class Model(_message.Message):
    __slots__ = ("provider", "model_id", "display_name", "capabilities", "context_window", "health", "input_cost_per_million", "output_cost_per_million", "configured")
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    MODEL_ID_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    CAPABILITIES_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_WINDOW_FIELD_NUMBER: _ClassVar[int]
    HEALTH_FIELD_NUMBER: _ClassVar[int]
    INPUT_COST_PER_MILLION_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_COST_PER_MILLION_FIELD_NUMBER: _ClassVar[int]
    CONFIGURED_FIELD_NUMBER: _ClassVar[int]
    provider: str
    model_id: str
    display_name: str
    capabilities: _containers.RepeatedScalarFieldContainer[ModelCapability]
    context_window: int
    health: _common_pb2.HealthState
    input_cost_per_million: str
    output_cost_per_million: str
    configured: bool
    def __init__(self, provider: _Optional[str] = ..., model_id: _Optional[str] = ..., display_name: _Optional[str] = ..., capabilities: _Optional[_Iterable[_Union[ModelCapability, str]]] = ..., context_window: _Optional[int] = ..., health: _Optional[_Union[_common_pb2.HealthState, str]] = ..., input_cost_per_million: _Optional[str] = ..., output_cost_per_million: _Optional[str] = ..., configured: bool = ...) -> None: ...

class ListModelsResponse(_message.Message):
    __slots__ = ("models",)
    MODELS_FIELD_NUMBER: _ClassVar[int]
    models: _containers.RepeatedCompositeFieldContainer[Model]
    def __init__(self, models: _Optional[_Iterable[_Union[Model, _Mapping]]] = ...) -> None: ...

class ModelServiceHealthCheckRequest(_message.Message):
    __slots__ = ("context", "include_details")
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_DETAILS_FIELD_NUMBER: _ClassVar[int]
    context: _common_pb2.RequestContext
    include_details: bool
    def __init__(self, context: _Optional[_Union[_common_pb2.RequestContext, _Mapping]] = ..., include_details: bool = ...) -> None: ...

class ModelServiceHealthCheckResponse(_message.Message):
    __slots__ = ("report",)
    REPORT_FIELD_NUMBER: _ClassVar[int]
    report: _common_pb2.HealthReport
    def __init__(self, report: _Optional[_Union[_common_pb2.HealthReport, _Mapping]] = ...) -> None: ...

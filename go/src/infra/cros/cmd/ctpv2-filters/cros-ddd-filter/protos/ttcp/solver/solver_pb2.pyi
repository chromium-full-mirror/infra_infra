from protos.protodocs import protodocs_pb2 as _protodocs_pb2
from protos.ttcp.syntax import syntax_pb2 as _syntax_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class EqcCategory(_message.Message):
    __slots__ = ["name", "value"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    name: str
    value: str
    def __init__(self, name: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...

class LegacyTarget(_message.Message):
    __slots__ = ["board", "models"]
    BOARD_FIELD_NUMBER: _ClassVar[int]
    MODELS_FIELD_NUMBER: _ClassVar[int]
    board: str
    models: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, board: _Optional[str] = ..., models: _Optional[_Iterable[str]] = ...) -> None: ...

class SolvedCategory(_message.Message):
    __slots__ = ["classes", "expression", "request_id"]
    CLASSES_FIELD_NUMBER: _ClassVar[int]
    EXPRESSION_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    classes: _containers.RepeatedCompositeFieldContainer[SolvedClass]
    expression: _syntax_pb2.CategoryExpression
    request_id: str
    def __init__(self, expression: _Optional[_Union[_syntax_pb2.CategoryExpression, _Mapping]] = ..., classes: _Optional[_Iterable[_Union[SolvedClass, _Mapping]]] = ..., request_id: _Optional[str] = ...) -> None: ...

class SolvedClass(_message.Message):
    __slots__ = ["dimensions", "expression", "legacy_solutions", "name", "targets"]
    class TargetsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: SolvedTarget
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[SolvedTarget, _Mapping]] = ...) -> None: ...
    DIMENSIONS_FIELD_NUMBER: _ClassVar[int]
    EXPRESSION_FIELD_NUMBER: _ClassVar[int]
    LEGACY_SOLUTIONS_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    TARGETS_FIELD_NUMBER: _ClassVar[int]
    dimensions: _containers.RepeatedCompositeFieldContainer[EqcCategory]
    expression: _syntax_pb2.Class
    legacy_solutions: _containers.RepeatedCompositeFieldContainer[LegacyTarget]
    name: str
    targets: _containers.MessageMap[str, SolvedTarget]
    def __init__(self, expression: _Optional[_Union[_syntax_pb2.Class, _Mapping]] = ..., targets: _Optional[_Mapping[str, SolvedTarget]] = ..., legacy_solutions: _Optional[_Iterable[_Union[LegacyTarget, _Mapping]]] = ..., name: _Optional[str] = ..., dimensions: _Optional[_Iterable[_Union[EqcCategory, _Mapping]]] = ...) -> None: ...

class SolvedTarget(_message.Message):
    __slots__ = ["device_id", "image_id", "info"]
    DEVICE_ID_FIELD_NUMBER: _ClassVar[int]
    IMAGE_ID_FIELD_NUMBER: _ClassVar[int]
    INFO_FIELD_NUMBER: _ClassVar[int]
    device_id: str
    image_id: str
    info: SolvedTargetInfo
    def __init__(self, device_id: _Optional[str] = ..., image_id: _Optional[str] = ..., info: _Optional[_Union[SolvedTargetInfo, _Mapping]] = ...) -> None: ...

class SolvedTargetInfo(_message.Message):
    __slots__ = ["board", "image_variant", "model", "swarming_labels"]
    BOARD_FIELD_NUMBER: _ClassVar[int]
    IMAGE_VARIANT_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    SWARMING_LABELS_FIELD_NUMBER: _ClassVar[int]
    board: str
    image_variant: str
    model: str
    swarming_labels: _containers.RepeatedCompositeFieldContainer[SwarmingLabel]
    def __init__(self, board: _Optional[str] = ..., model: _Optional[str] = ..., image_variant: _Optional[str] = ..., swarming_labels: _Optional[_Iterable[_Union[SwarmingLabel, _Mapping]]] = ...) -> None: ...

class SwarmingLabel(_message.Message):
    __slots__ = ["label", "value"]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    label: str
    value: str
    def __init__(self, label: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...

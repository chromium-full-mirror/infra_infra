from protos.protodocs import protodocs_pb2 as _protodocs_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class And(_message.Message):
    __slots__ = ["sub_expressions"]
    SUB_EXPRESSIONS_FIELD_NUMBER: _ClassVar[int]
    sub_expressions: _containers.RepeatedCompositeFieldContainer[Expression]
    def __init__(self, sub_expressions: _Optional[_Iterable[_Union[Expression, _Mapping]]] = ...) -> None: ...

class Category(_message.Message):
    __slots__ = ["combinatorial", "description", "enumerated", "name", "report_category", "union"]
    COMBINATORIAL_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    ENUMERATED_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    REPORT_CATEGORY_FIELD_NUMBER: _ClassVar[int]
    UNION_FIELD_NUMBER: _ClassVar[int]
    combinatorial: CombinatorialCategory
    description: str
    enumerated: EnumeratedCategory
    name: str
    report_category: str
    union: UnionCategory
    def __init__(self, name: _Optional[str] = ..., description: _Optional[str] = ..., combinatorial: _Optional[_Union[CombinatorialCategory, _Mapping]] = ..., enumerated: _Optional[_Union[EnumeratedCategory, _Mapping]] = ..., union: _Optional[_Union[UnionCategory, _Mapping]] = ..., report_category: _Optional[str] = ...) -> None: ...

class CategoryExpression(_message.Message):
    __slots__ = ["name", "value"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    name: str
    value: Category
    def __init__(self, name: _Optional[str] = ..., value: _Optional[_Union[Category, _Mapping]] = ...) -> None: ...

class Class(_message.Message):
    __slots__ = ["description", "expression", "name"]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    EXPRESSION_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    description: str
    expression: Expression
    name: str
    def __init__(self, name: _Optional[str] = ..., description: _Optional[str] = ..., expression: _Optional[_Union[Expression, _Mapping]] = ...) -> None: ...

class ClassExpression(_message.Message):
    __slots__ = ["name", "value"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    name: str
    value: Class
    def __init__(self, name: _Optional[str] = ..., value: _Optional[_Union[Class, _Mapping]] = ...) -> None: ...

class Collection(_message.Message):
    __slots__ = ["categories", "classes", "description", "name", "owners", "report_categories"]
    class CategoriesEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: Category
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[Category, _Mapping]] = ...) -> None: ...
    class ClassesEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: Class
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[Class, _Mapping]] = ...) -> None: ...
    class ReportCategoriesEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: ReportCategoryList
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[ReportCategoryList, _Mapping]] = ...) -> None: ...
    CATEGORIES_FIELD_NUMBER: _ClassVar[int]
    CLASSES_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    OWNERS_FIELD_NUMBER: _ClassVar[int]
    REPORT_CATEGORIES_FIELD_NUMBER: _ClassVar[int]
    categories: _containers.MessageMap[str, Category]
    classes: _containers.MessageMap[str, Class]
    description: str
    name: str
    owners: _containers.RepeatedScalarFieldContainer[str]
    report_categories: _containers.MessageMap[str, ReportCategoryList]
    def __init__(self, name: _Optional[str] = ..., owners: _Optional[_Iterable[str]] = ..., description: _Optional[str] = ..., categories: _Optional[_Mapping[str, Category]] = ..., classes: _Optional[_Mapping[str, Class]] = ..., report_categories: _Optional[_Mapping[str, ReportCategoryList]] = ...) -> None: ...

class CombinatorialCategory(_message.Message):
    __slots__ = ["subcategories"]
    SUBCATEGORIES_FIELD_NUMBER: _ClassVar[int]
    subcategories: _containers.RepeatedCompositeFieldContainer[CategoryExpression]
    def __init__(self, subcategories: _Optional[_Iterable[_Union[CategoryExpression, _Mapping]]] = ...) -> None: ...

class Condition(_message.Message):
    __slots__ = ["int_equal", "int_greater", "int_greater_or_equal", "int_in_set", "int_less", "int_less_or_equal", "present", "property_path", "str_equal", "str_in_set", "str_regex_match"]
    INT_EQUAL_FIELD_NUMBER: _ClassVar[int]
    INT_GREATER_FIELD_NUMBER: _ClassVar[int]
    INT_GREATER_OR_EQUAL_FIELD_NUMBER: _ClassVar[int]
    INT_IN_SET_FIELD_NUMBER: _ClassVar[int]
    INT_LESS_FIELD_NUMBER: _ClassVar[int]
    INT_LESS_OR_EQUAL_FIELD_NUMBER: _ClassVar[int]
    PRESENT_FIELD_NUMBER: _ClassVar[int]
    PROPERTY_PATH_FIELD_NUMBER: _ClassVar[int]
    STR_EQUAL_FIELD_NUMBER: _ClassVar[int]
    STR_IN_SET_FIELD_NUMBER: _ClassVar[int]
    STR_REGEX_MATCH_FIELD_NUMBER: _ClassVar[int]
    int_equal: int
    int_greater: int
    int_greater_or_equal: int
    int_in_set: IntSet
    int_less: int
    int_less_or_equal: int
    present: bool
    property_path: str
    str_equal: str
    str_in_set: StringSet
    str_regex_match: str
    def __init__(self, property_path: _Optional[str] = ..., present: bool = ..., str_equal: _Optional[str] = ..., str_in_set: _Optional[_Union[StringSet, _Mapping]] = ..., int_equal: _Optional[int] = ..., int_in_set: _Optional[_Union[IntSet, _Mapping]] = ..., int_less: _Optional[int] = ..., int_less_or_equal: _Optional[int] = ..., int_greater: _Optional[int] = ..., int_greater_or_equal: _Optional[int] = ..., str_regex_match: _Optional[str] = ...) -> None: ...

class EnumeratedCategory(_message.Message):
    __slots__ = ["classes", "name"]
    CLASSES_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    classes: _containers.RepeatedCompositeFieldContainer[ClassExpression]
    name: str
    def __init__(self, name: _Optional[str] = ..., classes: _Optional[_Iterable[_Union[ClassExpression, _Mapping]]] = ...) -> None: ...

class Expression(_message.Message):
    __slots__ = ["property", "true"]
    AND_FIELD_NUMBER: _ClassVar[int]
    NOT_FIELD_NUMBER: _ClassVar[int]
    OR_FIELD_NUMBER: _ClassVar[int]
    PROPERTY_FIELD_NUMBER: _ClassVar[int]
    TRUE_FIELD_NUMBER: _ClassVar[int]
    property: Condition
    true: globals()['True']
    def __init__(self, property: _Optional[_Union[Condition, _Mapping]] = ..., true: _Optional[_Union[globals()['True'], _Mapping]] = ..., **kwargs) -> None: ...

class IntSet(_message.Message):
    __slots__ = ["values"]
    VALUES_FIELD_NUMBER: _ClassVar[int]
    values: _containers.RepeatedScalarFieldContainer[int]
    def __init__(self, values: _Optional[_Iterable[int]] = ...) -> None: ...

class MatchReplace(_message.Message):
    __slots__ = ["match", "property", "replace"]
    MATCH_FIELD_NUMBER: _ClassVar[int]
    PROPERTY_FIELD_NUMBER: _ClassVar[int]
    REPLACE_FIELD_NUMBER: _ClassVar[int]
    match: str
    property: str
    replace: str
    def __init__(self, match: _Optional[str] = ..., replace: _Optional[str] = ..., property: _Optional[str] = ...) -> None: ...

class Not(_message.Message):
    __slots__ = ["sub_expression"]
    SUB_EXPRESSION_FIELD_NUMBER: _ClassVar[int]
    sub_expression: Expression
    def __init__(self, sub_expression: _Optional[_Union[Expression, _Mapping]] = ...) -> None: ...

class Or(_message.Message):
    __slots__ = ["sub_expressions"]
    SUB_EXPRESSIONS_FIELD_NUMBER: _ClassVar[int]
    sub_expressions: _containers.RepeatedCompositeFieldContainer[Expression]
    def __init__(self, sub_expressions: _Optional[_Iterable[_Union[Expression, _Mapping]]] = ...) -> None: ...

class ReportCategory(_message.Message):
    __slots__ = ["name", "value"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    name: str
    value: ReportCategoryValue
    def __init__(self, name: _Optional[str] = ..., value: _Optional[_Union[ReportCategoryValue, _Mapping]] = ...) -> None: ...

class ReportCategoryList(_message.Message):
    __slots__ = ["categories"]
    CATEGORIES_FIELD_NUMBER: _ClassVar[int]
    categories: _containers.RepeatedCompositeFieldContainer[ReportCategory]
    def __init__(self, categories: _Optional[_Iterable[_Union[ReportCategory, _Mapping]]] = ...) -> None: ...

class ReportCategoryValue(_message.Message):
    __slots__ = ["overrides", "property"]
    OVERRIDES_FIELD_NUMBER: _ClassVar[int]
    PROPERTY_FIELD_NUMBER: _ClassVar[int]
    overrides: _containers.RepeatedCompositeFieldContainer[MatchReplace]
    property: str
    def __init__(self, property: _Optional[str] = ..., overrides: _Optional[_Iterable[_Union[MatchReplace, _Mapping]]] = ...) -> None: ...

class StringSet(_message.Message):
    __slots__ = ["values"]
    VALUES_FIELD_NUMBER: _ClassVar[int]
    values: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, values: _Optional[_Iterable[str]] = ...) -> None: ...

class True(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class UnionCategory(_message.Message):
    __slots__ = ["subcategories"]
    SUBCATEGORIES_FIELD_NUMBER: _ClassVar[int]
    subcategories: _containers.RepeatedCompositeFieldContainer[CategoryExpression]
    def __init__(self, subcategories: _Optional[_Iterable[_Union[CategoryExpression, _Mapping]]] = ...) -> None: ...

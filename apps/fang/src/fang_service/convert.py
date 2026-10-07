"""Conversion between plain dicts and protobuf Struct values."""

from __future__ import annotations

import math
from typing import Any

from google.protobuf.json_format import MessageToDict, ParseDict
from google.protobuf.struct_pb2 import Struct


def _json_safe(value: Any) -> Any:
    """Make a value representable in a protobuf Struct.

    A Struct number cannot carry NaN or infinity: ``ParseDict`` accepts them but
    ``MessageToDict`` raises on the way back out ("Fail to serialize NaN for
    Value.number_value"), and a real feature row routinely holds NaN, for
    instance a missing stellar radius. Missing values therefore travel as null.
    numpy values are unwrapped first because ``ParseDict`` rejects them.
    """
    if type(value).__module__ == "numpy":
        value = value.tolist()
    if isinstance(value, float) and not math.isfinite(value):
        return None
    if isinstance(value, dict):
        return {key: _json_safe(item) for key, item in value.items()}
    if isinstance(value, (list, tuple)):
        return [_json_safe(item) for item in value]
    return value


def to_struct(mapping: dict[str, Any]) -> Struct:
    struct = Struct()
    ParseDict(_json_safe(mapping), struct)
    return struct


def from_struct(struct: Struct) -> dict[str, Any]:
    return MessageToDict(struct)

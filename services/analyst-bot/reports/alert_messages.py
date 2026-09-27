"""Alert labels and message templates, from shared/content/alert_messages.json.

The rendered message is posted to Discord and stored in fired_alerts.message,
and momentum-api serves the labels to Alarm History, so both read one source.
No fallback: a missing file, or a type without a label and message, fails at
import (see shared_content).
"""
from __future__ import annotations

from typing import Mapping

from reports import shared_content

AlertMessagesError = shared_content.SharedContentError


def _load() -> Mapping[str, Mapping[str, str]]:
    data = shared_content.load("ALERT_MESSAGES_PATH", "alert_messages.json")
    types = data.get("alert_types") if isinstance(data, dict) else None
    if not isinstance(types, dict) or not types:
        raise AlertMessagesError("alert_messages.json lacks a non-empty 'alert_types'")
    for kind, entry in types.items():
        if not isinstance(entry, dict) or not entry.get("label") or not entry.get("message"):
            raise AlertMessagesError(f"alert_messages.json: {kind!r} lacks a label or message")
    return types


_TYPES = _load()

KINDS = frozenset(_TYPES)


def label(kind: str) -> str:
    return _TYPES[kind]["label"]


def render(kind: str, **fields: object) -> str:
    """The alert's message. Raises KeyError for a kind the file does not describe."""
    return _TYPES[kind]["message"].format(**fields)

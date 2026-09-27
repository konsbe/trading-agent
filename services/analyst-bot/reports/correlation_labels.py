"""Correlation card labels, from shared/content.

shared/content/correlation_labels.json holds the correlation card's labels by
stored code — cluster names, cluster tiers, the not-evaluated wording, pattern
names and the net-count wording. momentum-api serves the same labels to the web
app, so the Discord embed and the Stock Detail card label a reading the same
way. An unknown code is shown as stored.
"""
from __future__ import annotations

from typing import Mapping, Optional

from reports import shared_content

CorrelationLabelsError = shared_content.SharedContentError

# Set by the builder, never stored: the cluster ran no comparison (checks_run 0),
# so its stored 0 / "mixed_positive" is not a reading.
NOT_EVALUATED = "not_evaluated"


def _load() -> Mapping:
    data = shared_content.load("CORRELATION_LABELS_PATH", "correlation_labels.json")
    for key in ("clusters", "cluster_tiers", "patterns", "net_signal", "text"):
        if not isinstance(data, dict) or not isinstance(data.get(key), dict) or not data[key]:
            raise CorrelationLabelsError(f"correlation_labels.json lacks a non-empty {key!r}")
    if not data.get("not_evaluated"):
        raise CorrelationLabelsError("correlation_labels.json lacks 'not_evaluated'")
    return data


_L = _load()


def cluster_name(key: str) -> str:
    return _L["clusters"].get(key, key)


def tier(code: Optional[str]) -> Optional[str]:
    """Display label for a cluster / composite tier code, or the not-evaluated wording."""
    if code == NOT_EVALUATED:
        return _L["not_evaluated"]
    if code is None:
        return None
    return _L["cluster_tiers"].get(code, code)


def pattern(key: str) -> str:
    return _L["patterns"].get(key, key)


def net(code: Optional[str]) -> Optional[str]:
    if code is None:
        return None
    return _L["net_signal"].get(code, code)


def text(key: str) -> str:
    """Headings: patterns_heading, net_count, met."""
    return _L["text"][key]
